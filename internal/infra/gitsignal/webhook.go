// Package gitsignal — webhook.go provides an HTTP handler that receives
// GitHub webhook events, verifies their HMAC-SHA256 signature, normalizes
// them into domain SignalEvent, and pushes to a sink channel.
package gitsignal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alex/internal/domain/signal"
	"alex/internal/shared/logging"
)

const maxWebhookBody = 1 << 20 // 1 MiB

// WebhookHandler receives GitHub webhook events at POST /api/webhooks/github.
type WebhookHandler struct {
	secret []byte
	sink   chan<- signal.SignalEvent
	logger logging.Logger
}

// NewWebhookHandler creates a handler that pushes verified events to sink.
// If secret is empty, signature verification is skipped (development mode).
func NewWebhookHandler(secret string, sink chan<- signal.SignalEvent, logger logging.Logger) *WebhookHandler {
	var sec []byte
	if secret != "" {
		sec = []byte(secret)
	}
	return &WebhookHandler{
		secret: sec,
		sink:   sink,
		logger: logging.OrNop(logger),
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	if !h.verifySignature(r.Header.Get("X-Hub-Signature-256"), body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	if eventType == "ping" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
		return
	}

	events := h.normalizeWebhook(eventType, deliveryID, body)
	for _, evt := range events {
		select {
		case h.sink <- evt:
		default:
			h.logger.Warn("webhook: sink full, dropping event %s %s", evt.Kind, evt.ID)
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"ok":true,"events":%d}`, len(events))
}

// verifySignature validates the HMAC-SHA256 signature from GitHub.
func (h *WebhookHandler) verifySignature(header string, body []byte) bool {
	if len(h.secret) == 0 {
		return true // no secret configured, skip verification
	}
	if header == "" {
		return false
	}
	sig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	decoded, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, h.secret)
	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(decoded, expected)
}

// normalizeWebhook converts a GitHub webhook payload into domain events.
func (h *WebhookHandler) normalizeWebhook(eventType, deliveryID string, body []byte) []signal.SignalEvent {
	// Map webhook event type to the internal ghEvent.Type used by normalizeGHEvent.
	ghType := webhookTypeMap[eventType]
	if ghType == "" {
		return nil
	}

	// Extract repo from common webhook payload shape.
	var envelope struct {
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Repository.FullName == "" {
		h.logger.Warn("webhook: cannot extract repo from %s payload", eventType)
		return nil
	}

	// Re-wrap as ghEvent for normalizeGHEvent reuse.
	// GitHub webhook payloads are structurally compatible with event API
	// payloads — the top-level object IS the payload (no wrapping).
	e := ghEvent{
		ID:        deliveryID,
		Type:      ghType,
		Payload:   json.RawMessage(body),
		CreatedAt: time.Now(),
	}

	if evt, ok := normalizeGHEvent(e, envelope.Repository.FullName); ok {
		return []signal.SignalEvent{evt}
	}
	return nil
}

// webhookTypeMap maps GitHub webhook X-GitHub-Event values to the
// internal ghEvent.Type identifiers used by normalizeGHEvent.
var webhookTypeMap = map[string]string{
	"pull_request":        "PullRequestEvent",
	"pull_request_review": "PullRequestReviewEvent",
	"push":                "PushEvent",
	"create":              "CreateEvent",
	"delete":              "DeleteEvent",
}
