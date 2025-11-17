package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	agentports "alex/internal/agent/ports"
	"alex/internal/utils"
)

type httpAttachmentExporter struct {
	endpoint    string
	client      *http.Client
	logger      *utils.Logger
	maxAttempts int
	backoff     time.Duration
	signingKey  []byte
}

// HTTPAttachmentExporterConfig customizes webhook exports.
type HTTPAttachmentExporterConfig struct {
	Endpoint    string
	Secret      string
	MaxAttempts int
	Backoff     time.Duration
	HTTPClient  *http.Client
}

const attachmentSignatureHeader = "X-Attachment-Signature"

// NewHTTPAttachmentExporter posts attachment metadata to an HTTP endpoint so an
// external CDN worker can persist the binaries permanently.
func NewHTTPAttachmentExporter(endpoint string) AttachmentExporter {
	return NewHTTPAttachmentExporterWithConfig(HTTPAttachmentExporterConfig{Endpoint: endpoint})
}

// NewHTTPAttachmentExporterWithConfig allows tests and operators to tweak the exporter behavior.
func NewHTTPAttachmentExporterWithConfig(cfg HTTPAttachmentExporterConfig) AttachmentExporter {
	trimmed := strings.TrimSpace(cfg.Endpoint)
	if trimmed == "" {
		return nil
	}
	attempts := cfg.MaxAttempts
	if attempts <= 0 {
		attempts = 3
	}
	backoff := cfg.Backoff
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &httpAttachmentExporter{
		endpoint:    trimmed,
		client:      client,
		logger:      utils.NewComponentLogger("AttachmentExporter"),
		maxAttempts: attempts,
		backoff:     backoff,
		signingKey:  []byte(strings.TrimSpace(cfg.Secret)),
	}
}

func (e *httpAttachmentExporter) ExportSession(ctx context.Context, sessionID string, attachments map[string]agentports.Attachment) AttachmentExportResult {
	result := AttachmentExportResult{
		AttachmentCount: len(attachments),
		ExporterKind:    "http_webhook",
		Endpoint:        e.endpoint,
	}
	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	if len(attachments) == 0 || sessionID == "" {
		result.Skipped = true
		return result
	}

	payload := struct {
		SessionID   string                           `json:"session_id"`
		Attachments map[string]agentports.Attachment `json:"attachments"`
		ExportedAt  time.Time                        `json:"exported_at"`
	}{
		SessionID:   sessionID,
		Attachments: attachments,
		ExportedAt:  time.Now().UTC(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		result.Error = fmt.Errorf("marshal attachment export payload: %w", err)
		return result
	}

	signature := e.signatureFor(body)
	for attempt := 1; attempt <= e.maxAttempts; attempt++ {
		result.Attempts = attempt
		if ctx.Err() != nil {
			result.Error = ctx.Err()
			e.logger.Warn("Attachment export aborted for session %s: %v", sessionID, result.Error)
			return result
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
		if err != nil {
			result.Error = fmt.Errorf("create attachment export request: %w", err)
			e.logger.Warn("Failed to create attachment export request: %v", err)
			return result
		}
		req.Header.Set("Content-Type", "application/json")
		if signature != "" {
			req.Header.Set(attachmentSignatureHeader, signature)
		}
		resp, err := e.client.Do(req)
		if err != nil {
			e.logger.Warn("Attachment export request failed (attempt %d/%d): %v", attempt, e.maxAttempts, err)
			result.Error = err
		} else {
			func() {
				defer resp.Body.Close()
				payload, readErr := io.ReadAll(resp.Body)
				if resp.StatusCode < 300 {
					result.Error = nil
					result.Exported = true
					e.logger.Info("Attachment export completed for session %s (%d assets) after %d attempt(s)", sessionID, len(attachments), attempt)
					if len(payload) > 0 {
						if updates := parseAttachmentExportResponse(payload); len(updates) > 0 {
							result.AttachmentUpdates = updates
						}
					}
					return
				}
				if readErr != nil {
					result.Error = fmt.Errorf("exporter returned status %d", resp.StatusCode)
					e.logger.Warn("Attachment export failed (attempt %d/%d): status=%d", attempt, e.maxAttempts, resp.StatusCode)
					return
				}
				result.Error = fmt.Errorf("exporter returned status %d", resp.StatusCode)
				e.logger.Warn("Attachment export failed (attempt %d/%d): status=%d", attempt, e.maxAttempts, resp.StatusCode)
			}()
			if result.Exported {
				return result
			}
		}
		if attempt == e.maxAttempts {
			if result.Error == nil {
				result.Error = errors.New("attachment export exhausted retries")
			}
			e.logger.Warn("Attachment export exhausted retries for session %s", sessionID)
			return result
		}
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			e.logger.Warn("Attachment export aborted for session %s: %v", sessionID, result.Error)
			return result
		case <-time.After(e.backoff * time.Duration(attempt)):
		}
	}

	return result
}

func (e *httpAttachmentExporter) signatureFor(body []byte) string {
	if e == nil || len(e.signingKey) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, e.signingKey)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func parseAttachmentExportResponse(body []byte) map[string]agentports.Attachment {
	if len(body) == 0 {
		return nil
	}
	var payload struct {
		Attachments map[string]agentports.Attachment `json:"attachments"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	if len(payload.Attachments) == 0 {
		return nil
	}
	updates := make(map[string]agentports.Attachment, len(payload.Attachments))
	for key, att := range payload.Attachments {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		updates[trimmed] = att
	}
	if len(updates) == 0 {
		return nil
	}
	return updates
}
