package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	jsonrpc "alex/internal/infra/mcp"
)

type rpcConn struct {
	r          *bufio.Reader
	w          *bufio.Writer
	mu         sync.Mutex
	useHeaders atomic.Bool

	pendingMu sync.Mutex
	pending   map[string]chan *jsonrpc.Response
	idGen     atomic.Int64
}

type rpcTransport interface {
	Call(ctx context.Context, method string, params map[string]any) (*jsonrpc.Response, error)
	Notify(method string, params map[string]any) error
	SendResponse(resp *jsonrpc.Response) error
	DeliverResponse(resp *jsonrpc.Response) bool
}

func newRPCConn(in io.Reader, out io.Writer) *rpcConn {
	return &rpcConn{
		r:       bufio.NewReader(in),
		w:       bufio.NewWriter(out),
		pending: make(map[string]chan *jsonrpc.Response),
	}
}

func (c *rpcConn) nextID() int64 {
	return c.idGen.Add(1)
}

func (c *rpcConn) Call(ctx context.Context, method string, params map[string]any) (*jsonrpc.Response, error) {
	if ctx == nil {
		ctx = cliBaseContext()
	}
	id := c.nextID()
	key := strconv.FormatInt(id, 10)
	respCh := make(chan *jsonrpc.Response, 1)

	c.pendingMu.Lock()
	c.pending[key] = respCh
	c.pendingMu.Unlock()

	req := jsonrpc.NewRequest(id, method, params)
	if err := c.send(req); err != nil {
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

func (c *rpcConn) Notify(method string, params map[string]any) error {
	return c.send(jsonrpc.NewNotification(method, params))
}

func (c *rpcConn) DeliverResponse(resp *jsonrpc.Response) bool {
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

func (c *rpcConn) SendResponse(resp *jsonrpc.Response) error {
	return c.send(resp)
}

func (c *rpcConn) send(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.useHeaders.Load() {
		if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
			return err
		}
		if _, err := c.w.Write(data); err != nil {
			return err
		}
		return c.w.Flush()
	}

	if _, err := c.w.Write(append(data, '\n')); err != nil {
		return err
	}
	return c.w.Flush()
}

func (c *rpcConn) readMessage() ([]byte, error) {
	payload, usedHeaders, err := readRPCMessage(c.r)
	if err != nil {
		return nil, err
	}
	if usedHeaders {
		c.useHeaders.Store(true)
	}
	return payload, nil
}

func readRPCMessage(r *bufio.Reader) ([]byte, bool, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					return nil, false, io.EOF
				}
				return []byte(trimmed), false, nil
			}
			return nil, false, err
		}

		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}

		if length, ok := parseContentLength(line); ok {
			for {
				header, err := r.ReadString('\n')
				if err != nil {
					return nil, true, err
				}
				header = strings.TrimRight(header, "\r\n")
				if strings.TrimSpace(header) == "" {
					break
				}
			}

			payload := make([]byte, length)
			if _, err := io.ReadFull(r, payload); err != nil {
				return nil, true, err
			}
			return payload, true, nil
		}

		return []byte(line), false, nil
	}
}

func parseContentLength(line string) (int, bool) {
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "content-length:") {
		return 0, false
	}
	value := strings.TrimSpace(line[len("content-length:"):])
	if value == "" {
		return 0, false
	}
	length, err := strconv.Atoi(value)
	if err != nil || length < 0 {
		return 0, false
	}
	return length, true
}

func parseRPCPayload(payload []byte) (*jsonrpc.Request, *jsonrpc.Response, error) {
	var probe map[string]any
	if err := json.Unmarshal(payload, &probe); err != nil {
		return nil, nil, err
	}
	if _, ok := probe["method"]; ok {
		req, err := jsonrpc.UnmarshalRequest(payload)
		if err != nil {
			return nil, nil, err
		}
		return req, nil, nil
	}
	resp, err := jsonrpc.UnmarshalResponse(payload)
	if err != nil {
		return nil, nil, err
	}
	return nil, resp, nil
}

func bytesTrimSpace(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}
