package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestErrorKind_String(t *testing.T) {
	tests := []struct {
		kind ErrorKind
		want string
	}{
		{Unknown, "Unknown"},
		{InvalidInput, "InvalidInput"},
		{Config, "Config"},
		{Provider, "Provider"},
		{Tool, "Tool"},
		{Temporary, "Temporary"},
		{NotFound, "NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorKind_String_OutOfRange(t *testing.T) {
	kind := ErrorKind(999)
	if got := kind.String(); got != "Unknown" {
		t.Errorf("out-of-range kind: got %q, want %q", got, "Unknown")
	}
}

func TestClassifiedError_Error_WithMessage(t *testing.T) {
	ce := &ClassifiedError{
		Kind:    Provider,
		Err:     fmt.Errorf("upstream failure"),
		Message: "LLM call failed",
	}
	got := ce.Error()
	if !strings.Contains(got, "[Provider]") {
		t.Errorf("missing kind: %q", got)
	}
	if !strings.Contains(got, "LLM call failed") {
		t.Errorf("missing message: %q", got)
	}
	if !strings.Contains(got, "upstream failure") {
		t.Errorf("missing wrapped error: %q", got)
	}
}

func TestClassifiedError_Error_WithoutMessage(t *testing.T) {
	ce := &ClassifiedError{
		Kind: Temporary,
		Err:  fmt.Errorf("connection reset"),
	}
	got := ce.Error()
	want := "[Temporary] connection reset"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClassifiedError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("root cause")
	ce := &ClassifiedError{
		Kind: Unknown,
		Err:  inner,
	}
	if ce.Unwrap() != inner {
		t.Error("Unwrap did not return the wrapped error")
	}
}

func TestClassify_MatchingClassifier(t *testing.T) {
	err := fmt.Errorf("status 404: not found")
	ce := Classify(err, HTTPClassifier())

	if ce == nil {
		t.Fatal("Classify returned nil")
	}
	if ce.Kind != NotFound {
		t.Errorf("Kind: got %v, want NotFound", ce.Kind)
	}
	if ce.Err != err {
		t.Error("Err does not wrap original error")
	}
}

func TestClassify_NoMatch(t *testing.T) {
	err := fmt.Errorf("some random error")
	ce := Classify(err, HTTPClassifier(), NetworkClassifier())

	if ce == nil {
		t.Fatal("Classify returned nil")
	}
	if ce.Kind != Unknown {
		t.Errorf("Kind: got %v, want Unknown", ce.Kind)
	}
}

func TestClassify_NilError(t *testing.T) {
	ce := Classify(nil, HTTPClassifier())
	if ce != nil {
		t.Errorf("Classify(nil): got %v, want nil", ce)
	}
}

func TestClassify_FirstMatchWins(t *testing.T) {
	// An error that matches both HTTPClassifier (429 -> Temporary) and
	// NetworkClassifier (timeout -> Temporary). Verify first classifier wins.
	err := fmt.Errorf("status 500 internal")
	ce := Classify(err, HTTPClassifier(), NetworkClassifier())
	if ce.Kind != Provider {
		t.Errorf("Kind: got %v, want Provider (from HTTPClassifier)", ce.Kind)
	}
}

func TestHTTPClassifier(t *testing.T) {
	tests := []struct {
		msg  string
		want ErrorKind
	}{
		{"status 400 bad request", InvalidInput},
		{"error 422 unprocessable", InvalidInput},
		{"status 401 unauthorized", Config},
		{"status 403 forbidden", Config},
		{"status 404 not found", NotFound},
		{"status 429 rate limited", Temporary},
		{"status 502 bad gateway", Temporary},
		{"status 503 service unavailable", Temporary},
		{"status 500 internal error", Provider},
	}

	classifier := HTTPClassifier()
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			kind, ok := classifier(fmt.Errorf("%s", tt.msg))
			if !ok {
				t.Fatal("expected match")
			}
			if kind != tt.want {
				t.Errorf("got %v, want %v", kind, tt.want)
			}
		})
	}

	t.Run("no match", func(t *testing.T) {
		_, ok := classifier(fmt.Errorf("some other error"))
		if ok {
			t.Error("expected no match")
		}
	})
}

func TestNetworkClassifier(t *testing.T) {
	tests := []string{
		"connection refused",
		"connection reset by peer",
		"timeout exceeded",
		"request timed out",
		"no such host example.com",
		"dns resolution failed",
		"unexpected eof",
		"broken pipe",
		"network unreachable",
		"i/o timeout",
	}

	classifier := NetworkClassifier()
	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			kind, ok := classifier(fmt.Errorf("%s", msg))
			if !ok {
				t.Fatal("expected match")
			}
			if kind != Temporary {
				t.Errorf("got %v, want Temporary", kind)
			}
		})
	}

	t.Run("no match", func(t *testing.T) {
		_, ok := classifier(fmt.Errorf("invalid argument"))
		if ok {
			t.Error("expected no match")
		}
	})
}

func TestToolClassifier(t *testing.T) {
	tests := []string{
		"tool execution failed",
		"function call error",
		"plugin not found",
		"extension crashed",
	}

	classifier := ToolClassifier()
	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			kind, ok := classifier(fmt.Errorf("%s", msg))
			if !ok {
				t.Fatal("expected match")
			}
			if kind != Tool {
				t.Errorf("got %v, want Tool", kind)
			}
		})
	}

	t.Run("no match", func(t *testing.T) {
		_, ok := classifier(fmt.Errorf("database error"))
		if ok {
			t.Error("expected no match")
		}
	})
}
