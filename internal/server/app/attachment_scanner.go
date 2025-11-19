package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/utils"
)

type AttachmentScanner interface {
	Scan(ctx context.Context, req AttachmentScanRequest) (AttachmentScanResult, error)
}

type AttachmentScanRequest struct {
	SessionID   string
	Placeholder string
	Attachment  ports.Attachment
	MediaType   string
	Payload     []byte
}

type AttachmentScanVerdict string

const (
	AttachmentScanVerdictUnknown  AttachmentScanVerdict = "unknown"
	AttachmentScanVerdictClean    AttachmentScanVerdict = "clean"
	AttachmentScanVerdictInfected AttachmentScanVerdict = "infected"
)

type AttachmentScanResult struct {
	Verdict AttachmentScanVerdict
	Details string
}

// HTTPAttachmentScannerConfig customizes webhook-based scanners.
type HTTPAttachmentScannerConfig struct {
	Endpoint   string
	Secret     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// NewHTTPAttachmentScanner returns a scanner that POSTs payloads to an HTTP service.
func NewHTTPAttachmentScanner(cfg HTTPAttachmentScannerConfig) AttachmentScanner {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &httpAttachmentScanner{
		endpoint:   endpoint,
		client:     client,
		timeout:    timeout,
		logger:     utils.NewComponentLogger("AttachmentScanner"),
		signingKey: []byte(strings.TrimSpace(cfg.Secret)),
	}
}

type httpAttachmentScanner struct {
	endpoint   string
	client     *http.Client
	timeout    time.Duration
	logger     *utils.Logger
	signingKey []byte
}

func (s *httpAttachmentScanner) Scan(ctx context.Context, req AttachmentScanRequest) (AttachmentScanResult, error) {
	result := AttachmentScanResult{Verdict: AttachmentScanVerdictUnknown}
	payload := strings.TrimSpace(base64.StdEncoding.EncodeToString(req.Payload))
	if payload == "" {
		return result, fmt.Errorf("attachment payload missing")
	}
	body := struct {
		SessionID   string           `json:"session_id"`
		Placeholder string           `json:"placeholder"`
		MediaType   string           `json:"media_type"`
		Payload     string           `json:"payload"`
		Attachment  ports.Attachment `json:"attachment"`
		ScannedAt   time.Time        `json:"scanned_at"`
	}{
		SessionID:   req.SessionID,
		Placeholder: req.Placeholder,
		MediaType:   req.MediaType,
		Payload:     payload,
		Attachment:  req.Attachment,
		ScannedAt:   time.Now().UTC(),
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return result, fmt.Errorf("marshal attachment scan payload: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, s.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return result, fmt.Errorf("build attachment scan request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if sig := s.signatureFor(bodyBytes); sig != "" {
		httpReq.Header.Set(attachmentSignatureHeader, sig)
	}
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return result, fmt.Errorf("attachment scan request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return result, fmt.Errorf("attachment scan rejected with status %d", resp.StatusCode)
	}
	var payloadResp struct {
		Verdict string `json:"verdict"`
		Details string `json:"details"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payloadResp); err != nil {
		return result, fmt.Errorf("decode attachment scan response: %w", err)
	}
	result.Verdict = parseScanVerdict(payloadResp.Verdict)
	result.Details = strings.TrimSpace(payloadResp.Details)
	return result, nil
}

func (s *httpAttachmentScanner) signatureFor(body []byte) string {
	if s == nil || len(s.signingKey) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, s.signingKey)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func parseScanVerdict(value string) AttachmentScanVerdict {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(AttachmentScanVerdictClean):
		return AttachmentScanVerdictClean
	case string(AttachmentScanVerdictInfected):
		return AttachmentScanVerdictInfected
	default:
		return AttachmentScanVerdictUnknown
	}
}
