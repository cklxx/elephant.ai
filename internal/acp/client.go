package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"alex/internal/logging"
	jsonrpc "alex/internal/mcp"
	"alex/internal/utils/id"
)

// NotificationHandler handles ACP notifications and requests.
type NotificationHandler interface {
	OnNotification(ctx context.Context, req *jsonrpc.Request)
	OnRequest(ctx context.Context, req *jsonrpc.Request) (*jsonrpc.Response, error)
}

// Client implements a minimal ACP JSON-RPC client over HTTP + SSE.
type Client struct {
	baseURL        string
	clientID       string
	httpClient     *http.Client
	requestTimeout time.Duration
	logger         logging.Logger

	mu       sync.Mutex
	running  bool
	readDone chan struct{}

	pendingMu sync.Mutex
	pending   map[string]chan *jsonrpc.Response
	idGen     atomic.Int64
}

// Dial connects to the ACP server and returns a client instance.
func Dial(ctx context.Context, addr string, timeout time.Duration, logger logging.Logger) (*Client, error) {
	if logger == nil {
		logger = logging.NewComponentLogger("ACPClient")
	}
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("acp addr is required")
	}
	baseURL, err := normalizeACPAddr(addr)
	if err != nil {
		return nil, err
	}
	_ = ctx
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		baseURL:        baseURL,
		clientID:       id.NewKSUID(),
		httpClient:     &http.Client{},
		requestTimeout: timeout,
		logger:         logger,
		readDone:       make(chan struct{}),
		pending:        make(map[string]chan *jsonrpc.Response),
	}, nil
}

// Close shuts down the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	c.running = false
	return nil
}

// Start begins reading notifications/responses until the connection closes.
func (c *Client) Start(ctx context.Context, handler NotificationHandler) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	go func() {
		defer close(c.readDone)
		c.readLoop(ctx, handler)
	}()
}

// Wait blocks until the read loop exits.
func (c *Client) Wait() {
	if c == nil || c.readDone == nil {
		return
	}
	<-c.readDone
}

// Call issues a JSON-RPC request.
func (c *Client) Call(ctx context.Context, method string, params map[string]any) (*jsonrpc.Response, error) {
	if c == nil {
		return nil, fmt.Errorf("acp client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	id := c.nextID()
	key := strconv.FormatInt(id, 10)
	respCh := make(chan *jsonrpc.Response, 1)

	c.pendingMu.Lock()
	c.pending[key] = respCh
	c.pendingMu.Unlock()

	req := jsonrpc.NewRequest(id, method, params)
	payload, err := json.Marshal(req)
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, key)
		c.pendingMu.Unlock()
		return nil, err
	}

	if err := c.post(ctx, payload); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, key)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, key)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

// Notify sends a JSON-RPC notification.
func (c *Client) Notify(method string, params map[string]any) error {
	if c == nil {
		return fmt.Errorf("acp client not initialized")
	}
	payload, err := json.Marshal(jsonrpc.NewNotification(method, params))
	if err != nil {
		return err
	}
	return c.post(context.Background(), payload)
}

func (c *Client) readLoop(ctx context.Context, handler NotificationHandler) {
	backoff := 200 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.consumeSSE(ctx, handler)
		if err != nil && ctx.Err() == nil {
			c.logger.Warn("ACP SSE read failed: %v", err)
		}
		if ctx.Err() != nil {
			return
		}
		time.Sleep(backoff)
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) consumeSSE(ctx context.Context, handler NotificationHandler) error {
	endpoint := fmt.Sprintf("%s/acp/sse?client_id=%s", c.baseURL, url.QueryEscape(c.clientID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("acp sse status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	reader := bufio.NewReader(resp.Body)
	var dataLines []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return err
			}
			return err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) == 0 {
				continue
			}
			payload := strings.Join(dataLines, "\n")
			dataLines = nil
			if strings.TrimSpace(payload) == "" {
				continue
			}
			c.handlePayload(ctx, handler, []byte(payload))
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(line[len("data:"):]))
		}
	}
}

func (c *Client) handlePayload(ctx context.Context, handler NotificationHandler, payload []byte) {
	payload = TrimPayload(payload)
	if len(payload) == 0 {
		return
	}
	req, resp, err := ParseRPCPayload(payload)
	if err != nil {
		c.logger.Warn("ACP parse failed: %v", err)
		return
	}
	if resp != nil {
		c.deliverResponse(resp)
		return
	}
	if req == nil {
		return
	}
	if req.IsNotification() {
		if handler != nil {
			handler.OnNotification(ctx, req)
		}
		return
	}

	var reply *jsonrpc.Response
	if handler != nil {
		response, err := handler.OnRequest(ctx, req)
		if err != nil {
			reply = jsonrpc.NewErrorResponse(req.ID, jsonrpc.InternalError, err.Error(), nil)
		} else {
			reply = response
		}
	}
	if reply == nil {
		reply = jsonrpc.NewResponse(req.ID, map[string]any{})
	}
	encoded, err := json.Marshal(reply)
	if err != nil {
		c.logger.Warn("ACP marshal response failed: %v", err)
		return
	}
	if err := c.post(ctx, encoded); err != nil {
		c.logger.Warn("ACP send response failed: %v", err)
	}
}

func (c *Client) nextID() int64 {
	return c.idGen.Add(1)
}

func (c *Client) deliverResponse(resp *jsonrpc.Response) bool {
	if resp == nil {
		return false
	}
	key := fmt.Sprintf("%v", resp.ID)
	c.pendingMu.Lock()
	ch, ok := c.pending[key]
	if ok {
		delete(c.pending, key)
	}
	c.pendingMu.Unlock()
	if !ok {
		return false
	}
	ch <- resp
	return true
}

func (c *Client) post(ctx context.Context, payload []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok && c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	endpoint := fmt.Sprintf("%s/acp/rpc?client_id=%s", c.baseURL, url.QueryEscape(c.clientID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &RPCStatusError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	return nil
}

func normalizeACPAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", fmt.Errorf("acp addr is required")
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/"), nil
	}
	if strings.Contains(addr, "://") {
		return "", fmt.Errorf("unsupported acp addr scheme: %s", addr)
	}
	return "http://" + strings.TrimRight(addr, "/"), nil
}
