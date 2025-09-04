package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"alex/internal/tools/mcp/protocol"
)

// StdioTransport implements MCP transport over standard input/output
type StdioTransport struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	stderr      io.ReadCloser
	messagesCh  chan []byte
	errorsCh    chan error
	ctx         context.Context
	cancel      context.CancelFunc
	
	// Fine-grained locking for performance optimization
	connMu      sync.RWMutex  // Protects connection state (connected, stdin/stdout/stderr)
	reqMu       sync.RWMutex  // Protects pendingReqs map
	writeMu     sync.Mutex    // Protects stdin writes
	
	connected   int32         // atomic: 1 if connected, 0 if not
	requestID   int64         // atomic
	pendingReqs map[int64]chan *protocol.JSONRPCResponse
	requestTimestamps map[int64]time.Time  // Track request timestamps for cleanup
	config      *StdioTransportConfig
}

// StdioTransportConfig represents configuration for stdio transport
type StdioTransportConfig struct {
	Command string
	Args    []string
	Env     []string
	WorkDir string
}

// NewStdioTransport creates a new stdio transport instance
func NewStdioTransport(config *StdioTransportConfig) *StdioTransport {
	return &StdioTransport{
		messagesCh:        make(chan []byte, 100),
		errorsCh:          make(chan error, 10),
		pendingReqs:       make(map[int64]chan *protocol.JSONRPCResponse),
		requestTimestamps: make(map[int64]time.Time),
		config:            config,
	}
}

// Connect establishes the stdio connection by starting the MCP server process
func (t *StdioTransport) Connect(ctx context.Context) error {
	return t.ConnectWithConfig(ctx, t.config)
}

// ConnectWithConfig establishes the stdio connection with configuration
func (t *StdioTransport) ConnectWithConfig(ctx context.Context, config *StdioTransportConfig) error {
	t.connMu.Lock()
	defer t.connMu.Unlock()

	if atomic.LoadInt32(&t.connected) == 1 {
		return nil
	}

	if config == nil {
		config = t.config
	}
	if config == nil {
		return fmt.Errorf("no configuration provided")
	}

	t.ctx, t.cancel = context.WithCancel(ctx)

	// Create command
	t.cmd = exec.CommandContext(t.ctx, config.Command, config.Args...)
	t.cmd.Env = config.Env
	if config.WorkDir != "" {
		t.cmd.Dir = config.WorkDir
	}

	// Set up pipes
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	t.stdout = stdout

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	t.stderr = stderr

	// Start the process
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Start reading from stdout and stderr
	go t.readStdout()
	go t.readStderr()

	// Monitor process
	go t.monitorProcess()
	
	// Start cleanup routine for orphaned requests
	go t.cleanupOrphanedRequests()

	atomic.StoreInt32(&t.connected, 1)
	return nil
}

// Disconnect closes the stdio connection
func (t *StdioTransport) Disconnect() error {
	t.connMu.Lock()
	defer t.connMu.Unlock()

	if atomic.LoadInt32(&t.connected) == 0 {
		return nil
	}

	if t.cancel != nil {
		t.cancel()
	}

	if t.stdin != nil {
		_ = t.stdin.Close()
	}

	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
	}

	atomic.StoreInt32(&t.connected, 0)
	return nil
}

// SendRequest sends a JSON-RPC request via stdin
func (t *StdioTransport) SendRequest(req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error) {
	// Check connection state atomically
	if atomic.LoadInt32(&t.connected) == 0 {
		return nil, fmt.Errorf("transport not connected")
	}
	
	// Get stdin with minimal locking
	t.connMu.RLock()
	stdin := t.stdin
	t.connMu.RUnlock()
	
	if stdin == nil {
		return nil, fmt.Errorf("stdin not available")
	}

	// Serialize request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// For requests with ID, set up response channel
	var responseCh chan *protocol.JSONRPCResponse
	if req.ID != nil {
		if id, ok := req.ID.(int64); ok {
			// Check if we have too many pending requests to prevent memory exhaustion
			if t.getBoundedRequestCount() >= 1000 { // Max 1000 concurrent requests
				return nil, fmt.Errorf("too many pending requests (%d), request rejected to prevent memory exhaustion", t.getBoundedRequestCount())
			}
			
			responseCh = make(chan *protocol.JSONRPCResponse, 1)
			t.reqMu.Lock()
			t.pendingReqs[id] = responseCh
			t.requestTimestamps[id] = time.Now()
			t.reqMu.Unlock()

			defer func() {
				t.reqMu.Lock()
				delete(t.pendingReqs, id)
				delete(t.requestTimestamps, id)
				t.reqMu.Unlock()
			}()
		}
	}

	// Send request with write lock to prevent concurrent writes
	t.writeMu.Lock()
	_, err = stdin.Write(append(data, '\n'))
	t.writeMu.Unlock()
	
	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// For notifications (no ID), return immediately
	if req.ID == nil {
		return nil, nil
	}

	// For requests with ID, wait for response
	if responseCh != nil {
		select {
		case response := <-responseCh:
			return response, nil
		case <-t.ctx.Done():
			return nil, fmt.Errorf("context cancelled")
		}
	}

	return nil, fmt.Errorf("no response channel for request")
}

// SendNotification sends a JSON-RPC notification via stdin
func (t *StdioTransport) SendNotification(notification *protocol.JSONRPCNotification) error {
	// Check connection state atomically
	if atomic.LoadInt32(&t.connected) == 0 {
		return fmt.Errorf("transport not connected")
	}
	
	// Get stdin with minimal locking
	t.connMu.RLock()
	stdin := t.stdin
	t.connMu.RUnlock()
	
	if stdin == nil {
		return fmt.Errorf("stdin not available")
	}

	// Serialize notification
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Send notification with write lock to prevent concurrent writes
	t.writeMu.Lock()
	_, err = stdin.Write(append(data, '\n'))
	t.writeMu.Unlock()
	
	if err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// ReceiveMessages returns a channel for receiving messages
func (t *StdioTransport) ReceiveMessages() <-chan []byte {
	return t.messagesCh
}

// ReceiveErrors returns a channel for receiving errors
func (t *StdioTransport) ReceiveErrors() <-chan error {
	return t.errorsCh
}

// readStdout reads messages from stdout
func (t *StdioTransport) readStdout() {
	defer func() {
		t.connMu.Lock()
		if t.stdout != nil {
			_ = t.stdout.Close()
		}
		t.connMu.Unlock()
	}()

	scanner := bufio.NewScanner(t.stdout)
	for scanner.Scan() {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Copy the line to avoid scanner buffer reuse issues
		message := make([]byte, len(line))
		copy(message, line)

		t.handleMessage(message)
	}

	if err := scanner.Err(); err != nil {
		select {
		case t.errorsCh <- fmt.Errorf("stdout read error: %w", err):
		case <-t.ctx.Done():
		}
	}
}

// readStderr reads error messages from stderr
func (t *StdioTransport) readStderr() {
	defer func() {
		t.connMu.Lock()
		if t.stderr != nil {
			_ = t.stderr.Close()
		}
		t.connMu.Unlock()
	}()

	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line != "" {
			select {
			case t.errorsCh <- fmt.Errorf("stderr: %s", line):
			case <-t.ctx.Done():
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case t.errorsCh <- fmt.Errorf("stderr read error: %w", err):
		case <-t.ctx.Done():
		}
	}
}

// monitorProcess monitors the MCP server process
func (t *StdioTransport) monitorProcess() {
	if t.cmd == nil {
		return
	}

	err := t.cmd.Wait()
	if err != nil {
		select {
		case t.errorsCh <- fmt.Errorf("MCP server process exited: %w", err):
		case <-t.ctx.Done():
		}
	}

	atomic.StoreInt32(&t.connected, 0)
}

// handleMessage processes incoming messages
func (t *StdioTransport) handleMessage(data []byte) {
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
func (t *StdioTransport) handleResponse(response *protocol.JSONRPCResponse) {
	if response.ID == nil {
		return
	}

	id, ok := response.ID.(float64)
	if !ok {
		return
	}

	t.reqMu.RLock()
	responseCh, exists := t.pendingReqs[int64(id)]
	t.reqMu.RUnlock()

	if exists {
		select {
		case responseCh <- response:
		case <-t.ctx.Done():
			return
		}
	}
}

// IsConnected returns the connection status
func (t *StdioTransport) IsConnected() bool {
	return atomic.LoadInt32(&t.connected) == 1
}

// NextRequestID generates a new request ID using atomic operations
func (t *StdioTransport) NextRequestID() int64 {
	return atomic.AddInt64(&t.requestID, 1)
}

// cleanupOrphanedRequests periodically cleans up requests that have been waiting too long
func (t *StdioTransport) cleanupOrphanedRequests() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()
	
	const requestTimeout = 2 * time.Minute // Timeout requests after 2 minutes
	
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			var expiredIDs []int64
			
			t.reqMu.RLock()
			for id, timestamp := range t.requestTimestamps {
				if now.Sub(timestamp) > requestTimeout {
					expiredIDs = append(expiredIDs, id)
				}
			}
			t.reqMu.RUnlock()
			
			// Clean up expired requests
			if len(expiredIDs) > 0 {
				t.reqMu.Lock()
				for _, id := range expiredIDs {
					if responseCh, exists := t.pendingReqs[id]; exists {
						// Send timeout error to the waiting goroutine
						select {
						case responseCh <- &protocol.JSONRPCResponse{
							ID: id,
							Error: &protocol.JSONRPCError{
								Code:    -32603, // Internal error
								Message: "Request timeout - cleaned up orphaned request",
							},
						}:
						default:
							// Channel might be full or already closed
						}
						delete(t.pendingReqs, id)
						delete(t.requestTimestamps, id)
					}
				}
				t.reqMu.Unlock()
				
				// Log cleanup activity (only if we cleaned up requests)
				select {
				case t.errorsCh <- fmt.Errorf("cleaned up %d orphaned requests", len(expiredIDs)):
				case <-t.ctx.Done():
					return
				}
			}
		}
	}
}

// getBoundedRequestCount returns the current number of pending requests
// This helps with monitoring and preventing unbounded growth
func (t *StdioTransport) getBoundedRequestCount() int {
	t.reqMu.RLock()
	defer t.reqMu.RUnlock()
	return len(t.pendingReqs)
}