package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type apiErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// requireMethod validates that the request uses the expected HTTP method.
// Returns true if the method matches; writes a 405 error and returns false otherwise.
func (h *APIHandler) requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
	return false
}

// decodeJSONBody reads an HTTP request body (with an optional size limit), decodes
// it into v using json.Decoder with DisallowUnknownFields, and validates that the
// body contains exactly one JSON object. If maxSize > 0, the body is wrapped with
// http.MaxBytesReader to enforce the limit. Returns true on success; writes an
// appropriate error response and returns false on any failure.
func (h *APIHandler) decodeJSONBody(w http.ResponseWriter, r *http.Request, v any, maxSize int64) bool {
	var body io.ReadCloser = r.Body
	if maxSize > 0 {
		body = http.MaxBytesReader(w, r.Body, maxSize)
		defer func() { _ = body.Close() }()
	}

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			h.writeJSONError(w, http.StatusBadRequest, "Request body is empty", err)
		case errors.As(err, &syntaxErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON at position %d", syntaxErr.Offset), err)
		case errors.As(err, &typeErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid value for field '%s'", typeErr.Field), err)
		case errors.As(err, &maxBytesErr):
			h.writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large", err)
		default:
			h.writeJSONError(w, http.StatusBadRequest, "Invalid request body", err)
		}
		return false
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return false
	}

	return true
}

func (h *APIHandler) writeJSONError(w http.ResponseWriter, status int, message string, err error) {
	if err != nil {
		h.logger.Error("HTTP %d - %s: %v", status, message, err)
	} else {
		h.logger.Warn("HTTP %d - %s", status, message)
	}

	resp := apiErrorResponse{Error: message}
	if err != nil {
		resp.Details = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
		h.logger.Error("Failed to encode error response: %v", encodeErr)
	}
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Error("Failed to encode JSON response: %v", err)
	}
}
