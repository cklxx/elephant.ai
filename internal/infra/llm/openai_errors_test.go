package llm

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	alexerrors "alex/internal/shared/errors"
)

// --- wrapRequestError ---

func TestWrapRequestError_ContextCanceled(t *testing.T) {
	err := wrapRequestError(context.Canceled)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled passthrough, got %v", err)
	}
}

func TestWrapRequestError_DeadlineExceeded(t *testing.T) {
	err := wrapRequestError(context.DeadlineExceeded)
	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected TransientError, got %T", err)
	}
}

func TestWrapRequestError_NetTimeout(t *testing.T) {
	err := wrapRequestError(&net.DNSError{IsTimeout: true})
	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected TransientError for net timeout, got %T", err)
	}
}

func TestWrapRequestError_GenericError(t *testing.T) {
	err := wrapRequestError(net.ErrClosed)
	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected TransientError for generic error, got %T", err)
	}
}

// --- mapHTTPError ---

func TestMapHTTPError_Unauthorized(t *testing.T) {
	err := mapHTTPError(401, []byte("unauthorized"), nil)
	var perr *alexerrors.PermanentError
	if !errors.As(err, &perr) {
		t.Fatalf("expected PermanentError, got %T", err)
	}
	if perr.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d", perr.StatusCode)
	}
}

func TestMapHTTPError_Forbidden(t *testing.T) {
	err := mapHTTPError(403, []byte("forbidden"), nil)
	var perr *alexerrors.PermanentError
	if !errors.As(err, &perr) {
		t.Fatalf("expected PermanentError for 403, got %T", err)
	}
}

func TestMapHTTPError_RateLimit(t *testing.T) {
	headers := http.Header{}
	headers.Set("Retry-After", "30")
	err := mapHTTPError(429, []byte("rate limited"), headers)
	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected TransientError for 429, got %T", err)
	}
	if terr.StatusCode != 429 {
		t.Fatalf("expected status 429, got %d", terr.StatusCode)
	}
	if terr.RetryAfter != 30 {
		t.Fatalf("expected RetryAfter 30, got %d", terr.RetryAfter)
	}
}

func TestMapHTTPError_Timeout(t *testing.T) {
	for _, status := range []int{408, 504} {
		err := mapHTTPError(status, nil, nil)
		var terr *alexerrors.TransientError
		if !errors.As(err, &terr) {
			t.Fatalf("expected TransientError for %d, got %T", status, err)
		}
	}
}

func TestMapHTTPError_ServerError(t *testing.T) {
	err := mapHTTPError(500, []byte("internal server error"), nil)
	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected TransientError for 500, got %T", err)
	}
}

func TestMapHTTPError_ClientError(t *testing.T) {
	err := mapHTTPError(400, []byte("bad request"), nil)
	var perr *alexerrors.PermanentError
	if !errors.As(err, &perr) {
		t.Fatalf("expected PermanentError for 400, got %T", err)
	}
}

func TestMapHTTPError_EmptyBody(t *testing.T) {
	err := mapHTTPError(500, nil, nil)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	// The TransientError wraps a baseErr that uses http.StatusText for empty body
	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected TransientError, got %T", err)
	}
	// The underlying error should contain the status text
	if terr.Err == nil {
		t.Fatal("expected wrapped error")
	}
	if !strings.Contains(terr.Err.Error(), "Internal Server Error") {
		t.Fatalf("expected status text in wrapped error, got %q", terr.Err.Error())
	}
}

func TestMapHTTPError_DefaultCase(t *testing.T) {
	// Status < 400 and not in specific cases: should be transient
	err := mapHTTPError(301, []byte("moved"), nil)
	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected TransientError for default case, got %T", err)
	}
}

// --- parseRetryAfter ---

func TestParseRetryAfter_IntegerSeconds(t *testing.T) {
	if got := parseRetryAfter("60"); got != 60 {
		t.Fatalf("expected 60, got %d", got)
	}
}

func TestParseRetryAfter_Empty(t *testing.T) {
	if got := parseRetryAfter(""); got != 0 {
		t.Fatalf("expected 0 for empty, got %d", got)
	}
}

func TestParseRetryAfter_NegativeSeconds(t *testing.T) {
	if got := parseRetryAfter("-5"); got != 0 {
		t.Fatalf("expected 0 for negative, got %d", got)
	}
}

func TestParseRetryAfter_InvalidString(t *testing.T) {
	if got := parseRetryAfter("not-a-number-or-date"); got != 0 {
		t.Fatalf("expected 0 for invalid, got %d", got)
	}
}

func TestParseRetryAfter_Zero(t *testing.T) {
	if got := parseRetryAfter("0"); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

// --- redactDataURIs ---

func TestRedactDataURIs_WithDataURI(t *testing.T) {
	input := []byte(`{"image":"data:image/png;base64,iVBORw0KGgo="}`)
	got := redactDataURIs(input)
	if strings.Contains(string(got), "iVBORw0KGgo=") {
		t.Fatal("expected base64 data redacted")
	}
	if !strings.Contains(string(got), "<redacted>") {
		t.Fatal("expected <redacted> placeholder")
	}
	if !strings.Contains(string(got), "data:image/png;base64,") {
		t.Fatal("expected media type preserved")
	}
}

func TestRedactDataURIs_NoDataURI(t *testing.T) {
	input := []byte(`{"text":"hello world"}`)
	got := redactDataURIs(input)
	if string(got) != string(input) {
		t.Fatalf("expected unchanged, got %s", got)
	}
}

func TestRedactDataURIs_Empty(t *testing.T) {
	got := redactDataURIs(nil)
	if got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
	got = redactDataURIs([]byte{})
	if len(got) != 0 {
		t.Fatalf("expected empty for empty input, got %v", got)
	}
}

func TestRedactDataURIs_MultipleURIs(t *testing.T) {
	input := []byte(`data:image/png;base64,abc123 and data:audio/mp3;base64,xyz789`)
	got := redactDataURIs(input)
	if strings.Contains(string(got), "abc123") || strings.Contains(string(got), "xyz789") {
		t.Fatal("expected all data URIs redacted")
	}
	if count := strings.Count(string(got), "<redacted>"); count != 2 {
		t.Fatalf("expected 2 redactions, got %d", count)
	}
}

// --- shouldEmbedAttachmentsInContent ---

func TestShouldEmbedAttachmentsInContent_UserWithAttachments(t *testing.T) {
	msg := ports.Message{
		Role:        "user",
		Attachments: map[string]ports.Attachment{"file.png": {}},
	}
	if !shouldEmbedAttachmentsInContent(msg) {
		t.Fatal("expected true for user message with attachments")
	}
}

func TestShouldEmbedAttachmentsInContent_NoAttachments(t *testing.T) {
	msg := ports.Message{Role: "user"}
	if shouldEmbedAttachmentsInContent(msg) {
		t.Fatal("expected false for no attachments")
	}
}

func TestShouldEmbedAttachmentsInContent_NonUserRole(t *testing.T) {
	msg := ports.Message{
		Role:        "assistant",
		Attachments: map[string]ports.Attachment{"file.png": {}},
	}
	if shouldEmbedAttachmentsInContent(msg) {
		t.Fatal("expected false for assistant role")
	}
}

func TestShouldEmbedAttachmentsInContent_ToolResult(t *testing.T) {
	msg := ports.Message{
		Role:        "user",
		Source:      ports.MessageSourceToolResult,
		Attachments: map[string]ports.Attachment{"file.png": {}},
	}
	if shouldEmbedAttachmentsInContent(msg) {
		t.Fatal("expected false for tool result source")
	}
}

func TestShouldEmbedAttachmentsInContent_CaseInsensitive(t *testing.T) {
	msg := ports.Message{
		Role:        "  USER  ",
		Attachments: map[string]ports.Attachment{"file.png": {}},
	}
	if !shouldEmbedAttachmentsInContent(msg) {
		t.Fatal("expected case-insensitive match")
	}
}
