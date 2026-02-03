package http

import (
	"fmt"
	nethttp "net/http"
	"strings"

	larkoauth "alex/internal/lark/oauth"
	"alex/internal/logging"
)

type LarkOAuthHandler struct {
	svc    *larkoauth.Service
	logger logging.Logger
}

func NewLarkOAuthHandler(svc *larkoauth.Service, logger logging.Logger) *LarkOAuthHandler {
	if svc == nil {
		return nil
	}
	return &LarkOAuthHandler{
		svc:    svc,
		logger: logging.OrNop(logger),
	}
}

func (h *LarkOAuthHandler) HandleStart(w nethttp.ResponseWriter, r *nethttp.Request) {
	if h == nil || h.svc == nil {
		nethttp.Error(w, "Lark OAuth not configured", nethttp.StatusServiceUnavailable)
		return
	}
	_, authURL, err := h.svc.StartAuth(r.Context())
	if err != nil {
		h.logger.Warn("Lark OAuth start failed: %v", err)
		nethttp.Error(w, "Failed to start Lark OAuth", nethttp.StatusInternalServerError)
		return
	}
	nethttp.Redirect(w, r, authURL, nethttp.StatusFound)
}

func (h *LarkOAuthHandler) HandleCallback(w nethttp.ResponseWriter, r *nethttp.Request) {
	if h == nil || h.svc == nil {
		nethttp.Error(w, "Lark OAuth not configured", nethttp.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	code := strings.TrimSpace(q.Get("code"))
	state := strings.TrimSpace(q.Get("state"))

	token, err := h.svc.HandleCallback(r.Context(), code, state)
	if err != nil {
		h.logger.Warn("Lark OAuth callback failed: %v", err)
		nethttp.Error(w, "Lark OAuth failed (state may be expired). Please retry authorization.", nethttp.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width,initial-scale=1">
    <title>Lark OAuth Success</title>
    <style>
      body{font-family:ui-sans-serif,system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial;max-width:720px;margin:40px auto;padding:0 16px;color:#111}
      .card{border:1px solid #e5e7eb;border-radius:12px;padding:16px}
      .muted{color:#6b7280;font-size:14px}
      code{background:#f3f4f6;border-radius:6px;padding:2px 6px}
    </style>
  </head>
  <body>
    <div class="card">
      <h2>Authorization complete</h2>
      <p class="muted">You can close this page and return to Lark.</p>
      <p class="muted">OpenID: <code>%s</code></p>
    </div>
  </body>
</html>`, token.OpenID)
}
