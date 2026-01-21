package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"alex/internal/logging"
	jsonrpc "alex/internal/mcp"
)

type acpHTTPServer struct {
	server *acpServer
	logger logging.Logger
}

func newACPHTTPServer(server *acpServer) *acpHTTPServer {
	if server == nil {
		return nil
	}
	return &acpHTTPServer{
		server: server,
		logger: logging.NewComponentLogger("ACPHTTPServer"),
	}
}

func (h *acpHTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/acp/sse", h.handleSSE)
	mux.HandleFunc("/acp/rpc", h.handleRPC)
	return mux
}

func (h *acpHTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	clientID, err := clientIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	transport := h.ensureTransport(clientID)
	if transport == nil {
		http.Error(w, "transport unavailable", http.StatusInternalServerError)
		return
	}
	if err := transport.Stream(r.Context(), w); err != nil && !errors.Is(err, context.Canceled) {
		h.logger.Warn("ACP SSE stream failed: %v", err)
	}
}

func (h *acpHTTPServer) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	clientID, err := clientIDFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	transport := h.ensureTransport(clientID)
	if transport == nil {
		http.Error(w, "transport unavailable", http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	req, resp, err := parseRPCPayload(body)
	if err != nil {
		http.Error(w, "invalid json-rpc payload", http.StatusBadRequest)
		return
	}

	if resp != nil {
		transport.DeliverResponse(resp)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req == nil {
		http.Error(w, "invalid json-rpc payload", http.StatusBadRequest)
		return
	}

	if req.IsNotification() {
		go h.server.handleNotification(context.Background(), req, clientID)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	go h.server.handleRequest(context.Background(), req, clientID)
	w.WriteHeader(http.StatusAccepted)
}

func (h *acpHTTPServer) ensureTransport(clientID string) *sseTransport {
	if clientID == "" || h == nil || h.server == nil {
		return nil
	}
	if existing := h.server.getTransport(clientID); existing != nil {
		if transport, ok := existing.(*sseTransport); ok {
			return transport
		}
		return nil
	}
	transport := newSSETransport(clientID, h.logger)
	h.server.registerTransport(clientID, transport)
	return transport
}

func clientIDFromRequest(r *http.Request) (string, error) {
	if r == nil || r.URL == nil {
		return "", fmt.Errorf("client_id is required")
	}
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	if clientID == "" {
		return "", fmt.Errorf("client_id is required")
	}
	return clientID, nil
}

type sseTransport struct {
	clientID string
	logger   logging.Logger

	sendCh chan []byte

	pendingMu sync.Mutex
	pending   map[string]chan *jsonrpc.Response
	idGen     atomic.Int64

	streamMu     sync.Mutex
	streamCancel context.CancelFunc
	streamID     int64
}

func newSSETransport(clientID string, logger logging.Logger) *sseTransport {
	if logger == nil {
		logger = logging.NewComponentLogger("ACPSSETransport")
	}
	return &sseTransport{
		clientID: clientID,
		logger:   logger,
		sendCh:   make(chan []byte, 128),
		pending:  make(map[string]chan *jsonrpc.Response),
	}
}

func (t *sseTransport) Call(ctx context.Context, method string, params map[string]any) (*jsonrpc.Response, error) {
	if t == nil {
		return nil, fmt.Errorf("acp transport not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	id := t.nextID()
	key := strconv.FormatInt(id, 10)
	respCh := make(chan *jsonrpc.Response, 1)

	t.pendingMu.Lock()
	t.pending[key] = respCh
	t.pendingMu.Unlock()

	req := jsonrpc.NewRequest(id, method, params)
	payload, err := json.Marshal(req)
	if err != nil {
		t.pendingMu.Lock()
		delete(t.pending, key)
		t.pendingMu.Unlock()
		return nil, err
	}

	if err := t.send(ctx, payload); err != nil {
		t.pendingMu.Lock()
		delete(t.pending, key)
		t.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		t.pendingMu.Lock()
		delete(t.pending, key)
		t.pendingMu.Unlock()
		return nil, ctx.Err()
	}
}

func (t *sseTransport) Notify(method string, params map[string]any) error {
	if t == nil {
		return fmt.Errorf("acp transport not initialized")
	}
	payload, err := json.Marshal(jsonrpc.NewNotification(method, params))
	if err != nil {
		return err
	}
	return t.send(context.Background(), payload)
}

func (t *sseTransport) SendResponse(resp *jsonrpc.Response) error {
	if t == nil {
		return fmt.Errorf("acp transport not initialized")
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return t.send(context.Background(), payload)
}

func (t *sseTransport) DeliverResponse(resp *jsonrpc.Response) bool {
	if t == nil || resp == nil {
		return false
	}
	key := fmt.Sprintf("%v", resp.ID)
	t.pendingMu.Lock()
	ch, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.pendingMu.Unlock()
	if !ok {
		return false
	}
	ch <- resp
	return true
}

func (t *sseTransport) Stream(ctx context.Context, w http.ResponseWriter) error {
	if t == nil {
		return fmt.Errorf("acp transport not initialized")
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	streamCtx, cancel := context.WithCancel(ctx)
	t.streamMu.Lock()
	t.streamID++
	streamID := t.streamID
	if t.streamCancel != nil {
		t.streamCancel()
	}
	t.streamCancel = cancel
	t.streamMu.Unlock()

	defer func() {
		t.streamMu.Lock()
		if t.streamID == streamID {
			t.streamCancel = nil
		}
		t.streamMu.Unlock()
	}()

	for {
		select {
		case <-streamCtx.Done():
			return streamCtx.Err()
		case payload := <-t.sendCh:
			if err := writeSSEPayload(w, payload); err != nil {
				return err
			}
			flusher.Flush()
		}
	}
}

func (t *sseTransport) send(ctx context.Context, payload []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case t.sendCh <- payload:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *sseTransport) nextID() int64 {
	return t.idGen.Add(1)
}

func writeSSEPayload(w io.Writer, payload []byte) error {
	if _, err := w.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		return err
	}
	return nil
}
