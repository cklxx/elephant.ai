package acp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"alex/internal/logging"
	jsonrpc "alex/internal/mcp"
)

// NotificationHandler handles ACP notifications and requests.
type NotificationHandler interface {
	OnNotification(ctx context.Context, req *jsonrpc.Request)
	OnRequest(ctx context.Context, req *jsonrpc.Request) (*jsonrpc.Response, error)
}

// Client implements a minimal ACP JSON-RPC client over TCP.
type Client struct {
	addr   string
	conn   net.Conn
	rpc    *RPCConn
	logger logging.Logger

	mu       sync.Mutex
	running  bool
	readDone chan struct{}
}

// Dial connects to the ACP server and returns a client instance.
func Dial(ctx context.Context, addr string, timeout time.Duration, logger logging.Logger) (*Client, error) {
	if logger == nil {
		logger = logging.NewComponentLogger("ACPClient")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Client{
		addr:     addr,
		conn:     conn,
		rpc:      NewRPCConn(conn, conn),
		logger:   logger,
		readDone: make(chan struct{}),
	}, nil
}

// Close shuts down the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
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
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			payload, err := c.rpc.ReadMessage()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
					return
				}
				c.logger.Warn("ACP read failed: %v", err)
				return
			}
			payload = TrimPayload(payload)
			if len(payload) == 0 {
				continue
			}
			req, resp, err := ParseRPCPayload(payload)
			if err != nil {
				c.logger.Warn("ACP parse failed: %v", err)
				continue
			}
			if resp != nil {
				c.rpc.DeliverResponse(resp)
				continue
			}
			if req == nil {
				continue
			}
			if req.IsNotification() {
				if handler != nil {
					handler.OnNotification(ctx, req)
				}
				continue
			}
			var reply *jsonrpc.Response
			if handler != nil {
				resp, err := handler.OnRequest(ctx, req)
				if err != nil {
					reply = jsonrpc.NewErrorResponse(req.ID, jsonrpc.InternalError, err.Error(), nil)
				} else {
					reply = resp
				}
			}
			if reply == nil {
				reply = jsonrpc.NewResponse(req.ID, map[string]any{})
			}
			if err := c.rpc.SendResponse(reply); err != nil {
				c.logger.Warn("ACP send response failed: %v", err)
				return
			}
		}
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
	if c == nil || c.rpc == nil {
		return nil, fmt.Errorf("acp client not initialized")
	}
	return c.rpc.Call(ctx, method, params)
}

// Notify sends a JSON-RPC notification.
func (c *Client) Notify(method string, params map[string]any) error {
	if c == nil || c.rpc == nil {
		return fmt.Errorf("acp client not initialized")
	}
	return c.rpc.Notify(method, params)
}
