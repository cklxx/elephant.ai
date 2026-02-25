package jsonx

import (
	"errors"
	"strings"
	"testing"
)

func withMarshalImpls(
	t *testing.T,
	primary func(any) ([]byte, error),
	fallback func(any) ([]byte, error),
	primaryIndent func(any, string, string) ([]byte, error),
	fallbackIndent func(any, string, string) ([]byte, error),
) {
	t.Helper()
	prevPrimary := marshalImpl
	prevFallback := fallbackMarshalImpl
	prevPrimaryIndent := marshalIndentImpl
	prevFallbackIndent := fallbackMarshalIndentImpl

	marshalImpl = primary
	fallbackMarshalImpl = fallback
	marshalIndentImpl = primaryIndent
	fallbackMarshalIndentImpl = fallbackIndent

	t.Cleanup(func() {
		marshalImpl = prevPrimary
		fallbackMarshalImpl = prevFallback
		marshalIndentImpl = prevPrimaryIndent
		fallbackMarshalIndentImpl = prevFallbackIndent
	})
}

func TestMarshalUsesPrimaryWhenHealthy(t *testing.T) {
	withMarshalImpls(
		t,
		func(any) ([]byte, error) { return []byte(`{"source":"primary"}`), nil },
		func(any) ([]byte, error) { return []byte(`{"source":"fallback"}`), nil },
		func(any, string, string) ([]byte, error) { return []byte(`{"source":"primary-indent"}`), nil },
		func(any, string, string) ([]byte, error) { return []byte(`{"source":"fallback-indent"}`), nil },
	)

	got, err := Marshal(map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	if string(got) != `{"source":"primary"}` {
		t.Fatalf("expected primary bytes, got %s", string(got))
	}
}

func TestMarshalFallsBackWhenPrimaryPanics(t *testing.T) {
	withMarshalImpls(
		t,
		func(any) ([]byte, error) { panic("primary panic") },
		func(any) ([]byte, error) { return []byte(`{"source":"fallback"}`), nil },
		func(any, string, string) ([]byte, error) { panic("primary indent panic") },
		func(any, string, string) ([]byte, error) { return []byte(`{"source":"fallback-indent"}`), nil },
	)

	got, err := Marshal(map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("Marshal() returned error: %v", err)
	}
	if string(got) != `{"source":"fallback"}` {
		t.Fatalf("expected fallback bytes, got %s", string(got))
	}
}

func TestMarshalIndentFallsBackWhenPrimaryPanics(t *testing.T) {
	withMarshalImpls(
		t,
		func(any) ([]byte, error) { return []byte(`{"source":"primary"}`), nil },
		func(any) ([]byte, error) { return []byte(`{"source":"fallback"}`), nil },
		func(any, string, string) ([]byte, error) { panic("primary indent panic") },
		func(any, string, string) ([]byte, error) { return []byte(`{"source":"fallback-indent"}`), nil },
	)

	got, err := MarshalIndent(map[string]string{"k": "v"}, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() returned error: %v", err)
	}
	if string(got) != `{"source":"fallback-indent"}` {
		t.Fatalf("expected fallback indent bytes, got %s", string(got))
	}
}

func TestMarshalReturnsErrorWhenPrimaryAndFallbackPanic(t *testing.T) {
	withMarshalImpls(
		t,
		func(any) ([]byte, error) { panic("primary panic") },
		func(any) ([]byte, error) { panic("fallback panic") },
		func(any, string, string) ([]byte, error) { panic("primary indent panic") },
		func(any, string, string) ([]byte, error) { panic("fallback indent panic") },
	)

	_, err := Marshal(map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected error when both marshal paths panic")
	}
	if !strings.Contains(err.Error(), "primary=") || !strings.Contains(err.Error(), "fallback=") {
		t.Fatalf("expected panic context in error, got %v", err)
	}
}

func TestMarshalReturnsFallbackError(t *testing.T) {
	withMarshalImpls(
		t,
		func(any) ([]byte, error) { panic("primary panic") },
		func(any) ([]byte, error) { return nil, errors.New("fallback failure") },
		func(any, string, string) ([]byte, error) { panic("primary indent panic") },
		func(any, string, string) ([]byte, error) { return nil, errors.New("fallback indent failure") },
	)

	_, err := Marshal(map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected fallback error")
	}
	if !strings.Contains(err.Error(), "fallback failure") {
		t.Fatalf("expected fallback error to propagate, got %v", err)
	}
}
