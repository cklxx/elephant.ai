package http

import (
	"errors"
	"net/http"

	"alex/internal/delivery/server/app"
	storage "alex/internal/domain/agent/ports/storage"
)

// mapDomainError translates a domain/service error into an HTTP status code
// and a user-facing message. It checks domain sentinel errors first, then
// falls back to known infrastructure errors (storage.ErrSessionNotFound).
//
// Returns (0, "") if the error is not a recognized domain error, letting
// the caller decide on a default (typically 500).
func mapDomainError(err error) (status int, message string) {
	if err == nil {
		return 0, ""
	}

	switch {
	case errors.Is(err, app.ErrValidation):
		return http.StatusBadRequest, err.Error()

	case errors.Is(err, app.ErrNotFound):
		return http.StatusNotFound, err.Error()

	case errors.Is(err, storage.ErrSessionNotFound):
		return http.StatusNotFound, "Session not found"

	case errors.Is(err, app.ErrShareTokenInvalid):
		return http.StatusForbidden, "Invalid share token"

	case errors.Is(err, app.ErrConflict):
		return http.StatusConflict, err.Error()

	case errors.Is(err, app.ErrUnavailable):
		return http.StatusServiceUnavailable, err.Error()

	default:
		return 0, ""
	}
}

// writeMappedError writes an error response using domain error mapping.
// If the error maps to a known domain error, the mapped status/message is used.
// Otherwise, falls back to the provided defaultStatus and defaultMsg.
func (h *APIHandler) writeMappedError(w http.ResponseWriter, err error, defaultStatus int, defaultMsg string) {
	if status, msg := mapDomainError(err); status != 0 {
		h.writeJSONError(w, status, msg, err)
		return
	}
	h.writeJSONError(w, defaultStatus, defaultMsg, err)
}
