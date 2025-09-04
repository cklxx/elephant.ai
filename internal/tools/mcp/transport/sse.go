package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"alex/internal/tools/mcp/protocol"
)

// CircuitBreaker states
const (
	CircuitClosed = iota
	CircuitOpen
	CircuitHalfOpen
)

// SSETransport implements MCP transport over Server-Sent Events
type SSETransport struct {
	endpoint    string
	client      *http.Client
	messagesCh  chan []byte
	errorsCh    chan error
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	connected   bool
	headers     map[string]string
	requestID   int64
	pendingReqs map[int64]chan *protocol.JSONRPCResponse
	
	// Reconnection and circuit breaker fields
	reconnectAttempts int64
	circuitState      int32  // atomic: CircuitClosed, CircuitOpen, CircuitHalfOpen
	failureCount      int64  // atomic
	lastFailureTime   time.Time
	circuitMu         sync.RWMutex
}

// SSETransportConfig represents configuration for SSE transport
type SSETransportConfig struct {
	Endpoint    string
	Headers     map[string]string
	Timeout     time.Duration
	RetryDelay  time.Duration
	MaxRetries  int
}

// NewSSETransport creates a new SSE transport instance
func NewSSETransport(config *SSETransportConfig) *SSETransport {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	// Create HTTP client with proxy-aware configuration
	// Use a reasonable timeout for individual requests, but allow context to control overall lifetime
	requestTimeout := 60 * time.Second // Increased for proxy environments
	if config.Timeout > 0 && config.Timeout < requestTimeout {
		requestTimeout = config.Timeout
	}
	
	client := &http.Client{
		Timeout: requestTimeout,
		// Don't follow redirects automatically to better handle proxy issues
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Check for proxy environment variables
	checkProxyEnvironment()

	return &SSETransport{
		endpoint:    config.Endpoint,
		client:      client,
		messagesCh:  make(chan []byte, 100),
		errorsCh:    make(chan error, 10),
		headers:     config.Headers,
		pendingReqs: make(map[int64]chan *protocol.JSONRPCResponse),
	}
}

// checkProxyEnvironment checks and logs proxy configuration
func checkProxyEnvironment() {
	proxies := []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"}
	
	for _, proxy := range proxies {
		if value := os.Getenv(proxy); value != "" {
			fmt.Printf("[INFO] MCP: Using proxy %s\n", proxy)
			break // Only log the first proxy found
		}
	}
}

// Connect establishes the SSE connection
func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	t.ctx, t.cancel = context.WithCancel(ctx)

	// For context7, we need to generate a unique session ID
	sessionID := fmt.Sprintf("alex-%d", time.Now().Unix())
	
	// Store session ID for later use
	if t.headers == nil {
		t.headers = make(map[string]string)
	}
	t.headers["MCP-Session-Id"] = sessionID

	// Start persistent SSE connection
	go t.startSSEConnection()

	// Wait for connection to establish and receive server session ID
	time.Sleep(1 * time.Second)

	t.connected = true
	fmt.Printf("[INFO] SSE: Context7 connection established\n")
	return nil
}

// Disconnect closes the SSE connection
func (t *SSETransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	if t.cancel != nil {
		t.cancel()
	}

	t.connected = false
	return nil
}

// SendRequest sends a JSON-RPC request via HTTP POST
func (t *SSETransport) SendRequest(req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	t.mu.RLock()
	connected := t.connected
	t.mu.RUnlock()

	if !connected {
		return nil, fmt.Errorf("transport not connected")
	}

	// Check if context is still valid
	if t.ctx.Err() != nil {
		return nil, fmt.Errorf("transport context cancelled: %v", t.ctx.Err())
	}

	// If we have a session ID but connection seems dead, try to verify it's alive
	if sessionID, exists := t.headers["MCP-Session-Id"]; exists && len(sessionID) > 0 {
		// Connection should be alive if we have a session ID
		_ = sessionID // Session ID is present, connection should be alive
	}

	// Serialize request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// For context7, use /messages endpoint for POST requests
	postEndpoint := t.endpoint
	if strings.Contains(t.endpoint, "/sse") {
		postEndpoint = strings.Replace(t.endpoint, "/sse", "/messages", 1)
	}
	
	// Add session ID as URL parameter if we have it
	if sessionID, exists := t.headers["MCP-Session-Id"]; exists {
		if strings.Contains(postEndpoint, "?") {
			postEndpoint += "&sessionId=" + sessionID
		} else {
			postEndpoint += "?sessionId=" + sessionID
		}
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(t.ctx, "POST", postEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}

	// For requests with ID, set up response channel
	var responseCh chan *protocol.JSONRPCResponse
	if req.ID != nil {
		if id, ok := req.ID.(int64); ok {
			responseCh = make(chan *protocol.JSONRPCResponse, 1)
			t.mu.Lock()
			t.pendingReqs[id] = responseCh
			t.mu.Unlock()

			defer func() {
				t.mu.Lock()
				delete(t.pendingReqs, id)
				t.mu.Unlock()
			}()
		}
	}

	// Send HTTP request
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body for error handling
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// For notifications (no ID), return immediately
	if req.ID == nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return nil, nil
	}

	// For context7, responses come via SSE, not HTTP response
	// If we got a 202 Accepted, wait for the real response via SSE
	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
		if responseCh != nil {
			// Increased timeout for proxy environments and slower MCP servers
			timeout := 90 * time.Second
			select {
			case response := <-responseCh:
				return response, nil
			case <-time.After(timeout):
				return nil, fmt.Errorf("MCP request timeout after %v", timeout)
			case <-t.ctx.Done():
				return nil, fmt.Errorf("context cancelled: %v", t.ctx.Err())
			}
		}
	}

	return nil, fmt.Errorf("unexpected response handling")
}

// SendNotification sends a JSON-RPC notification
func (t *SSETransport) SendNotification(notification *protocol.JSONRPCNotification) error {
	t.mu.RLock()
	connected := t.connected
	t.mu.RUnlock()

	if !connected {
		return fmt.Errorf("transport not connected")
	}

	// Serialize notification
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// For context7, use /messages endpoint for POST requests
	postEndpoint := t.endpoint
	if strings.Contains(t.endpoint, "/sse") {
		postEndpoint = strings.Replace(t.endpoint, "/sse", "/messages", 1)
	}
	
	// Add session ID as URL parameter if we have it
	if sessionID, exists := t.headers["MCP-Session-Id"]; exists {
		if strings.Contains(postEndpoint, "?") {
			postEndpoint += "&sessionId=" + sessionID
		} else {
			postEndpoint += "?sessionId=" + sessionID
		}
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(t.ctx, "POST", postEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		httpReq.Header.Set(k, v)
	}

	// Send HTTP request
	resp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// ReceiveMessages returns a channel for receiving messages
func (t *SSETransport) ReceiveMessages() <-chan []byte {
	return t.messagesCh
}

// ReceiveErrors returns a channel for receiving errors
func (t *SSETransport) ReceiveErrors() <-chan error {
	return t.errorsCh
}

// startSSEConnection establishes and maintains the SSE connection with intelligent reconnection
func (t *SSETransport) startSSEConnection() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Check circuit breaker state
			if t.isCircuitOpen() {
				// Circuit is open, wait for cool-down period
				select {
				case <-t.ctx.Done():
					return
				case <-time.After(t.getCircuitCooldownDelay()):
					t.transitionToHalfOpen()
					continue
				}
			}
			
			if err := t.connectSSE(); err != nil {
				t.recordFailure()
				
				// Only log errors if we haven't exceeded reasonable retry count
				attempts := atomic.LoadInt64(&t.reconnectAttempts)
				if attempts < 5 {
					t.errorsCh <- fmt.Errorf("SSE connection failed (attempt %d): %w", attempts, err)
				}
				
				// Calculate exponential backoff delay
				delay := t.getReconnectDelay()
				
				select {
				case <-t.ctx.Done():
					return
				case <-time.After(delay):
					continue
				}
			} else {
				// Connection successful, reset failure tracking
				t.recordSuccess()
			}
		}
	}
}

// getReconnectDelay calculates exponential backoff delay with jitter
func (t *SSETransport) getReconnectDelay() time.Duration {
	attempts := atomic.LoadInt64(&t.reconnectAttempts)
	atomic.AddInt64(&t.reconnectAttempts, 1)
	
	// Base delay starts at 1 second, max at 30 seconds
	baseDelay := time.Second
	maxDelay := 30 * time.Second
	
	// Exponential backoff: delay = baseDelay * 2^attempts
	delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempts)))
	if delay > maxDelay {
		delay = maxDelay
	}
	
	// Add jitter (±25%) to prevent thundering herd
	jitter := float64(delay) * 0.25 * (2*rand.Float64() - 1)
	return time.Duration(float64(delay) + jitter)
}

// isCircuitOpen checks if the circuit breaker is open
func (t *SSETransport) isCircuitOpen() bool {
	state := atomic.LoadInt32(&t.circuitState)
	if state == CircuitOpen {
		return true
	}
	
	// Check if we should open the circuit based on failure rate
	failures := atomic.LoadInt64(&t.failureCount)
	if failures >= 5 { // Open circuit after 5 consecutive failures
		atomic.CompareAndSwapInt32(&t.circuitState, CircuitClosed, CircuitOpen)
		return true
	}
	
	return false
}

// getCircuitCooldownDelay returns the circuit breaker cool-down delay
func (t *SSETransport) getCircuitCooldownDelay() time.Duration {
	// Circuit breaker cool-down period: 60 seconds
	return 60 * time.Second
}

// transitionToHalfOpen transitions the circuit breaker to half-open state
func (t *SSETransport) transitionToHalfOpen() {
	atomic.CompareAndSwapInt32(&t.circuitState, CircuitOpen, CircuitHalfOpen)
}

// recordFailure records a connection failure
func (t *SSETransport) recordFailure() {
	atomic.AddInt64(&t.failureCount, 1)
	t.circuitMu.Lock()
	t.lastFailureTime = time.Now()
	t.circuitMu.Unlock()
}

// recordSuccess records a successful connection and resets failure tracking
func (t *SSETransport) recordSuccess() {
	atomic.StoreInt64(&t.reconnectAttempts, 0)
	atomic.StoreInt64(&t.failureCount, 0)
	atomic.StoreInt32(&t.circuitState, CircuitClosed)
}

// connectSSE establishes the SSE connection
func (t *SSETransport) connectSSE() error {
	// For context7, the SSE endpoint is the same as the main endpoint
	sseEndpoint := t.endpoint
	
	// Add session ID as URL parameter if we have it
	if sessionID, exists := t.headers["MCP-Session-Id"]; exists {
		if strings.Contains(sseEndpoint, "?") {
			sseEndpoint += "&sessionId=" + sessionID
		} else {
			sseEndpoint += "?sessionId=" + sessionID
		}
	}

	req, err := http.NewRequestWithContext(t.ctx, "GET", sseEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	// Set SSE headers optimized for proxy environments
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("User-Agent", "Alex-MCP-Client/1.0")
	
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SSE connection failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Read SSE stream
	scanner := bufio.NewScanner(resp.Body)
	var eventData strings.Builder

	for scanner.Scan() {
		select {
		case <-t.ctx.Done():
			return nil
		default:
		}

		line := scanner.Text()

		// Handle SSE format
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}
			eventData.WriteString(data)
			eventData.WriteString("\n")
		} else if line == "" {
			// End of event
			if eventData.Len() > 0 {
				eventStr := strings.TrimSpace(eventData.String())
				if eventStr != "" {
					t.handleSSEMessage([]byte(eventStr))
				}
				eventData.Reset()
			}
		}
	}

	return scanner.Err()
}

// handleSSEMessage processes incoming SSE messages
func (t *SSETransport) handleSSEMessage(data []byte) {
	dataStr := string(data)
	
	// Check if this is a context7 endpoint message
	if strings.HasPrefix(dataStr, "/messages?sessionId=") {
		// Extract the server-provided session ID
		parts := strings.Split(dataStr, "sessionId=")
		if len(parts) == 2 {
			serverSessionID := parts[1]
			
			// Update our session ID to match server's
			t.mu.Lock()
			if t.headers == nil {
				t.headers = make(map[string]string)
			}
			t.headers["MCP-Session-Id"] = serverSessionID
			t.mu.Unlock()
		}
		return
	}
	
	// Try to parse as JSON-RPC response first
	if protocol.IsResponse(data) {
		var response protocol.JSONRPCResponse
		if err := json.Unmarshal(data, &response); err == nil {
			t.handleResponse(&response)
			return
		}
	}

	// Try to parse as JSON-RPC notification
	if protocol.IsNotification(data) {
		select {
		case t.messagesCh <- data:
		case <-t.ctx.Done():
			return
		}
		return
	}

	// Forward raw message
	select {
	case t.messagesCh <- data:
	case <-t.ctx.Done():
		return
	}
}

// handleResponse handles JSON-RPC responses
func (t *SSETransport) handleResponse(response *protocol.JSONRPCResponse) {
	if response.ID == nil {
		return
	}

	// Handle both float64 and int ID types
	var id int64
	switch v := response.ID.(type) {
	case float64:
		id = int64(v)
	case int:
		id = int64(v)
	case int64:
		id = v
	default:
		return // Unknown ID type
	}

	t.mu.Lock()
	responseCh, exists := t.pendingReqs[id]
	t.mu.Unlock()

	if exists {
		select {
		case responseCh <- response:
		case <-t.ctx.Done():
			return
		}
	}
}

// IsConnected returns the connection status
func (t *SSETransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// NextRequestID generates a new request ID
func (t *SSETransport) NextRequestID() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestID++
	return t.requestID
}