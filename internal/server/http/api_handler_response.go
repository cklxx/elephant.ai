package http

import (
	"encoding/json"
	"net/http"
)

type apiErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
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
