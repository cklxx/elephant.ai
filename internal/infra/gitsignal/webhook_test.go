package gitsignal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/domain/signal"
)

func makeHMACSignature(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler_Ping(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("secret", sink, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader("{}"))
	body := "{}"
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", makeHMACSignature("secret", body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(sink) != 0 {
		t.Fatal("ping should not produce events")
	}
}

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("secret", sink, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader("{}"))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestWebhookHandler_NoSecretSkipsVerification(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("", sink, nil)

	body := `{"ref":"refs/heads/main","commits":[{"sha":"abc","commit":{"message":"fix","author":{"name":"alice","date":"2026-03-20T00:00:00Z"}}}],"repository":{"full_name":"org/repo"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	// No signature header.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(sink) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink))
	}
	evt := <-sink
	if evt.Kind != signal.SignalCommitPushed {
		t.Errorf("expected commit.pushed, got %s", evt.Kind)
	}
}

func TestWebhookHandler_PullRequestOpened(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("", sink, nil)

	body := `{"action":"opened","number":42,"pull_request":{"number":42,"title":"feat: x","state":"open","user":{"login":"alice"},"head":{"ref":"feat/PROJ-1"},"base":{"ref":"main"},"html_url":"https://github.com/org/repo/pull/42"},"repository":{"full_name":"org/repo"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(sink) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink))
	}
	evt := <-sink
	if evt.Kind != signal.SignalPROpened {
		t.Errorf("expected pr.opened, got %s", evt.Kind)
	}
	if evt.LinkedTicketID != "PROJ-1" {
		t.Errorf("expected ticket PROJ-1, got %q", evt.LinkedTicketID)
	}
}

func TestWebhookHandler_ReviewApproved(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("", sink, nil)

	body := `{"action":"submitted","review":{"state":"approved"},"pull_request":{"number":10,"state":"open","user":{"login":"bob"},"head":{"ref":"fix/x"},"base":{"ref":"main"}},"repository":{"full_name":"org/repo"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request_review")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	evt := <-sink
	if evt.Kind != signal.SignalPRApproved {
		t.Errorf("expected pr.approved, got %s", evt.Kind)
	}
	if evt.PR.ReviewState != signal.ReviewApproved {
		t.Errorf("expected ReviewState=approved, got %s", evt.PR.ReviewState)
	}
}

func TestWebhookHandler_BranchCreated(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("", sink, nil)

	body := `{"ref":"feat/PROJ-99","ref_type":"branch","repository":{"full_name":"org/repo"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "create")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	evt := <-sink
	if evt.Kind != signal.SignalBranchCreated {
		t.Errorf("expected branch.created, got %s", evt.Kind)
	}
	if evt.LinkedTicketID != "PROJ-99" {
		t.Errorf("expected PROJ-99, got %q", evt.LinkedTicketID)
	}
}

func TestWebhookHandler_UnknownEventType(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("", sink, nil)

	body := `{"repository":{"full_name":"org/repo"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "fork")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(sink) != 0 {
		t.Fatal("unknown event should not produce events")
	}
}

func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	h := NewWebhookHandler("", sink, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/webhooks/github", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestVerifySignature_ValidSignature(t *testing.T) {
	h := NewWebhookHandler("mysecret", nil, nil)
	body := []byte(`{"test":true}`)
	sig := makeHMACSignature("mysecret", string(body))
	if !h.verifySignature(sig, body) {
		t.Fatal("expected valid signature")
	}
}

func TestVerifySignature_MissingHeader(t *testing.T) {
	h := NewWebhookHandler("mysecret", nil, nil)
	if h.verifySignature("", []byte("{}")) {
		t.Fatal("expected invalid for missing header")
	}
}

func TestVerifySignature_WrongPrefix(t *testing.T) {
	h := NewWebhookHandler("mysecret", nil, nil)
	if h.verifySignature("sha1=abc", []byte("{}")) {
		t.Fatal("expected invalid for wrong prefix")
	}
}

// Verify the exported handler reads the body correctly via httptest.
func TestWebhookHandler_Integration(t *testing.T) {
	sink := make(chan signal.SignalEvent, 10)
	secret := "integration-secret"
	h := NewWebhookHandler(secret, sink, nil)

	body := `{"action":"closed","number":7,"pull_request":{"number":7,"merged":true,"state":"closed","user":{"login":"alice"},"head":{"ref":"feat/ABC-42"},"base":{"ref":"main"},"html_url":"https://github.com/org/repo/pull/7"},"repository":{"full_name":"org/repo"}}`
	sig := makeHMACSignature(secret, body)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Delivery", "delivery-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		respBody, _ := io.ReadAll(w.Body)
		t.Fatalf("expected 200, got %d: %s", w.Code, string(respBody))
	}
	if len(sink) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink))
	}
	evt := <-sink
	if evt.Kind != signal.SignalPRMerged {
		t.Errorf("expected pr.merged, got %s", evt.Kind)
	}
	if evt.ID != "delivery-123" {
		t.Errorf("expected delivery ID, got %q", evt.ID)
	}
	if evt.LinkedTicketID != "ABC-42" {
		t.Errorf("expected ABC-42, got %q", evt.LinkedTicketID)
	}
}
