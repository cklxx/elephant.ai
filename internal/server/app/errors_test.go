package app

import (
	"errors"
	"testing"
)

func TestNotFoundErrorWrapsErrNotFound(t *testing.T) {
	err := NotFoundError("session xyz")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected errors.Is(err, ErrNotFound), got false")
	}
	if err.Error() != "session xyz: not found" {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}

func TestValidationErrorWrapsErrValidation(t *testing.T) {
	err := ValidationError("session id required")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected errors.Is(err, ErrValidation), got false")
	}
	if err.Error() != "session id required: validation error" {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}

func TestUnavailableErrorWrapsErrUnavailable(t *testing.T) {
	err := UnavailableError("state store not configured")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected errors.Is(err, ErrUnavailable), got false")
	}
}

func TestConflictErrorWrapsErrConflict(t *testing.T) {
	err := ConflictError("cannot cancel completed task")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected errors.Is(err, ErrConflict), got false")
	}
}

func TestDomainErrorsAreDistinct(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"NotFound is not Validation", NotFoundError("x"), ErrValidation},
		{"Validation is not NotFound", ValidationError("x"), ErrNotFound},
		{"Unavailable is not Conflict", UnavailableError("x"), ErrConflict},
		{"Conflict is not Unavailable", ConflictError("x"), ErrUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if errors.Is(tc.err, tc.want) {
				t.Fatalf("errors.Is(%v, %v) should be false", tc.err, tc.want)
			}
		})
	}
}
