package chromebridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const protocolVersion = 1

var errNotConnected = errors.New("chrome extension bridge is not connected")

type Config struct {
	ListenAddr string
	Token      string
	Timeout    time.Duration
}

func (c Config) withDefaults() Config {
	out := c
	out.ListenAddr = strings.TrimSpace(out.ListenAddr)
	out.Token = strings.TrimSpace(out.Token)
	if out.ListenAddr == "" {
		out.ListenAddr = "127.0.0.1:17333"
	}
	if out.Timeout <= 0 {
		out.Timeout = 15 * time.Second
	}
	return out
}

// Bridge hosts a local WebSocket endpoint that a Chrome extension connects to.
// The bridge then issues JSON-RPC requests to the extension to access Chrome APIs
// (cookies/tabs/storage/debugger).
type Bridge struct {
	cfg Config

	mu        sync.RWMutex
	ln        net.Listener
	httpSrv   *http.Server
	addr      string
	conn      *websocket.Conn
	lastHello helloMessage

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan callResult
	nextID    atomic.Uint64
}

func New(cfg Config) *Bridge {
	cfg = cfg.withDefaults()
	return &Bridge{
		cfg:     cfg,
		pending: make(map[string]chan callResult),
	}
}

func (b *Bridge) Addr() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.addr
}

func (b *Bridge) Connected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.conn != nil
}

func (b *Bridge) LastHello() (client string, version int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return strings.TrimSpace(b.lastHello.Client), b.lastHello.Version
}

func (b *Bridge) Start() error {
	if b == nil {
		return errors.New("bridge is nil")
	}
	b.mu.Lock()
	if b.ln != nil {
		b.mu.Unlock()
		return nil
	}
	cfg := b.cfg
	b.mu.Unlock()

	host, _, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid bridge_listen_addr %q: %w", cfg.ListenAddr, err)
	}
	if host == "" || host == "0.0.0.0" {
		return fmt.Errorf("bridge_listen_addr must bind to loopback, got %q", cfg.ListenAddr)
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("bridge_listen_addr must bind to loopback, got %q", cfg.ListenAddr)
		}
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %q: %w", cfg.ListenAddr, err)
	}
	addr := ln.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", b.handleWS)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	b.mu.Lock()
	b.ln = ln
	b.httpSrv = httpSrv
	b.addr = addr
	b.mu.Unlock()

	go func() {
		_ = httpSrv.Serve(ln)
	}()

	return nil
}

func (b *Bridge) Close(ctx context.Context) error {
	if b == nil {
		return nil
	}

	b.mu.Lock()
	srv := b.httpSrv
	conn := b.conn
	b.httpSrv = nil
	b.conn = nil
	b.ln = nil
	b.addr = ""
	b.lastHello = helloMessage{}
	b.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	if srv == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.Shutdown(ctx)
}

func (b *Bridge) WaitForConnected(ctx context.Context, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = b.cfg.Timeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if b.Connected() {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func (b *Bridge) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if b == nil {
		return nil, errors.New("bridge is nil")
	}
	method = strings.TrimSpace(method)
	if method == "" {
		return nil, errors.New("method is required")
	}

	b.mu.RLock()
	conn := b.conn
	timeout := b.cfg.Timeout
	b.mu.RUnlock()
	if conn == nil {
		return nil, errNotConnected
	}

	id := fmt.Sprintf("%d", b.nextID.Add(1))
	ch := make(chan callResult, 1)

	b.pendingMu.Lock()
	b.pending[id] = ch
	b.pendingMu.Unlock()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := b.writeJSON(conn, req); err != nil {
		b.pendingMu.Lock()
		delete(b.pending, id)
		b.pendingMu.Unlock()
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-callCtx.Done():
		b.pendingMu.Lock()
		delete(b.pending, id)
		b.pendingMu.Unlock()
		return nil, callCtx.Err()
	case res := <-ch:
		return res.Result, res.Err
	}
}

func (b *Bridge) handleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(*http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if err := b.accept(conn); err != nil {
		_ = conn.Close()
		return
	}
}

type helloMessage struct {
	Type    string `json:"type"`
	Token   string `json:"token,omitempty"`
	Client  string `json:"client,omitempty"`
	Version int    `json:"version,omitempty"`
}

type welcomeMessage struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
}

func (b *Bridge) accept(conn *websocket.Conn) error {
	if conn == nil {
		return errors.New("conn is nil")
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	var hello helloMessage
	if err := json.Unmarshal(data, &hello); err != nil {
		return fmt.Errorf("parse hello: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(hello.Type)) != "hello" {
		return fmt.Errorf("expected hello, got %q", hello.Type)
	}

	cfg := b.cfg.withDefaults()
	if cfg.Token != "" && hello.Token != cfg.Token {
		return errors.New("unauthorized")
	}

	_ = conn.SetReadDeadline(time.Time{})
	if err := b.writeJSON(conn, welcomeMessage{Type: "welcome", Version: protocolVersion}); err != nil {
		return err
	}

	b.mu.Lock()
	if b.conn != nil {
		_ = b.conn.Close()
		b.failAllPendingLocked(errNotConnected)
	}
	b.conn = conn
	b.lastHello = hello
	b.mu.Unlock()

	go b.readLoop(conn)
	return nil
}

func (b *Bridge) readLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		b.handleMessage(data)
	}

	b.mu.Lock()
	if b.conn == conn {
		b.conn = nil
		b.lastHello = helloMessage{}
		b.failAllPendingLocked(errNotConnected)
	}
	b.mu.Unlock()
	_ = conn.Close()
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type callResult struct {
	Result json.RawMessage
	Err    error
}

func (b *Bridge) handleMessage(data []byte) {
	var resp rpcResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}
	if strings.TrimSpace(resp.JSONRPC) != "2.0" {
		return
	}
	if resp.Method != "" {
		// Extension-to-server requests/notifications are not used by the MVP bridge.
		return
	}

	id := rpcIDToString(resp.ID)
	if id == "" {
		return
	}

	var out callResult
	out.Result = resp.Result
	if resp.Error != nil {
		out.Err = fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	b.pendingMu.Lock()
	ch := b.pending[id]
	delete(b.pending, id)
	b.pendingMu.Unlock()
	if ch == nil {
		return
	}
	ch <- out
}

func (b *Bridge) failAllPendingLocked(err error) {
	b.pendingMu.Lock()
	defer b.pendingMu.Unlock()
	for id, ch := range b.pending {
		delete(b.pending, id)
		if ch == nil {
			continue
		}
		ch <- callResult{Err: err}
	}
}

func (b *Bridge) writeJSON(conn *websocket.Conn, v any) error {
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	if conn == nil {
		return errNotConnected
	}
	return conn.WriteJSON(v)
}

func rpcIDToString(id any) string {
	switch v := id.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}
