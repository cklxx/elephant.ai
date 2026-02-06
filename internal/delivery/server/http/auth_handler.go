package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	authapp "alex/internal/auth/app"
	"alex/internal/auth/domain"
	"alex/internal/logging"
)

const (
	maxAuthBodySize   = 1 << 16
	refreshCookieName = "alex_refresh_token"
	accessCookieName  = "alex_access_token"
)

// AuthHandler manages authentication endpoints.
type AuthHandler struct {
	service *authapp.Service
	logger  logging.Logger
	secure  bool
}

// NewAuthHandler builds a new authentication handler.
func NewAuthHandler(service *authapp.Service, secure bool) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  logging.NewComponentLogger("AuthHandler"),
		secure:  secure,
	}
}

// registerRequest holds incoming registration payload.
type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type adjustPointsRequest struct {
	Delta int64 `json:"delta"`
}

type updateSubscriptionRequest struct {
	Tier      string  `json:"tier"`
	ExpiresAt *string `json:"expires_at"`
}

type tokenResponse struct {
	AccessToken    string    `json:"access_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	RefreshExpires time.Time `json:"refresh_expires_at"`
	User           userDTO   `json:"user"`
}

type userDTO struct {
	ID            string          `json:"id"`
	Email         string          `json:"email"`
	DisplayName   string          `json:"display_name"`
	PointsBalance int64           `json:"points_balance"`
	Subscription  subscriptionDTO `json:"subscription"`
}

type subscriptionDTO struct {
	Tier              string     `json:"tier"`
	MonthlyPriceCents int        `json:"monthly_price_cents"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

type subscriptionPlanDTO struct {
	Tier              string `json:"tier"`
	MonthlyPriceCents int    `json:"monthly_price_cents"`
}

type plansResponse struct {
	Plans []subscriptionPlanDTO `json:"plans"`
}

type oauthStartResponse struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// HandleRegister processes POST /api/auth/register.
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req registerRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		h.writeError(w, err)
		return
	}
	user, err := h.service.RegisterLocal(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUserDTO(user))
}

// HandleLogin processes POST /api/auth/login.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req loginRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		h.writeError(w, err)
		return
	}
	tokens, err := h.service.LoginWithPassword(r.Context(), req.Email, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	claims, err := h.service.ParseAccessToken(r.Context(), tokens.AccessToken)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	user, err := h.service.GetUser(r.Context(), claims.Subject)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	h.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiry)
	h.setAccessCookie(w, tokens.AccessToken, tokens.AccessExpiry)
	resp := tokenResponse{AccessToken: tokens.AccessToken, ExpiresAt: tokens.AccessExpiry, RefreshExpires: tokens.RefreshExpiry, User: toUserDTO(user)}
	writeJSON(w, http.StatusOK, resp)
}

// HandleRefresh processes POST /api/auth/refresh.
func (h *AuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	refresh := h.readRefreshToken(r)
	if refresh == "" {
		var req refreshRequest
		if err := decodeJSONBody(w, r, &req); err != nil {
			if !errors.Is(err, io.EOF) {
				h.writeError(w, err)
				return
			}
		} else {
			refresh = req.RefreshToken
		}
	}
	if refresh == "" {
		http.Error(w, "refresh token required", http.StatusBadRequest)
		return
	}
	tokens, err := h.service.RefreshAccessToken(r.Context(), refresh, r.UserAgent(), clientIP(r))
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	claims, err := h.service.ParseAccessToken(r.Context(), tokens.AccessToken)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	user, err := h.service.GetUser(r.Context(), claims.Subject)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	h.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiry)
	h.setAccessCookie(w, tokens.AccessToken, tokens.AccessExpiry)
	resp := tokenResponse{AccessToken: tokens.AccessToken, ExpiresAt: tokens.AccessExpiry, RefreshExpires: tokens.RefreshExpiry, User: toUserDTO(user)}
	writeJSON(w, http.StatusOK, resp)
}

// HandleLogout revokes the current refresh token.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	refresh := h.readRefreshToken(r)
	if refresh != "" {
		if err := h.service.Logout(r.Context(), refresh); err != nil && !errors.Is(err, domain.ErrSessionNotFound) {
			h.writeDomainError(w, err)
			return
		}
	}
	h.clearRefreshCookie(w)
	h.clearAccessCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// HandleMe returns the current user from the Authorization header.
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// HandleAdjustPoints processes POST /api/auth/points.
func (h *AuthHandler) HandleAdjustPoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	var req adjustPointsRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		h.writeError(w, err)
		return
	}
	updated, err := h.service.AdjustPoints(r.Context(), user.ID, req.Delta)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(updated))
}

// HandleUpdateSubscription processes POST /api/auth/subscription.
func (h *AuthHandler) HandleUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, ok := h.requireAuthenticatedUser(w, r)
	if !ok {
		return
	}
	var req updateSubscriptionRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		h.writeError(w, err)
		return
	}
	tier := domain.SubscriptionTier(strings.TrimSpace(strings.ToLower(req.Tier)))
	var expiresAt *time.Time
	if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ExpiresAt))
		if err != nil {
			http.Error(w, "invalid expires_at", http.StatusBadRequest)
			return
		}
		expiresAt = &parsed
	}
	updated, err := h.service.UpdateSubscription(r.Context(), user.ID, tier, expiresAt)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(updated))
}

// HandleListPlans returns the subscription catalog.
func (h *AuthHandler) HandleListPlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	plans := domain.SubscriptionPlans()
	dtos := make([]subscriptionPlanDTO, 0, len(plans))
	for _, plan := range plans {
		dtos = append(dtos, toSubscriptionPlanDTO(plan))
	}
	writeJSON(w, http.StatusOK, plansResponse{Plans: dtos})
}

// HandleOAuthStart handles GET /api/auth/{provider}/login.
func (h *AuthHandler) HandleOAuthStart(provider domain.ProviderType, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, state, err := h.service.StartOAuth(r.Context(), provider)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, oauthStartResponse{URL: url, State: state})
}

// HandleOAuthCallback handles GET /api/auth/{provider}/callback.
func (h *AuthHandler) HandleOAuthCallback(provider domain.ProviderType, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing code/state", http.StatusBadRequest)
		return
	}
	tokens, err := h.service.CompleteOAuth(r.Context(), provider, code, state, r.UserAgent(), clientIP(r))
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	claims, err := h.service.ParseAccessToken(r.Context(), tokens.AccessToken)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	user, err := h.service.GetUser(r.Context(), claims.Subject)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	h.setRefreshCookie(w, tokens.RefreshToken, tokens.RefreshExpiry)
	h.setAccessCookie(w, tokens.AccessToken, tokens.AccessExpiry)
	resp := tokenResponse{AccessToken: tokens.AccessToken, ExpiresAt: tokens.AccessExpiry, RefreshExpires: tokens.RefreshExpiry, User: toUserDTO(user)}
	if prefersHTML(r.Header.Get("Accept")) {
		h.writeOAuthSuccessPage(w)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) writeOAuthSuccessPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, oauthCallbackSuccessHTML)
}

func prefersHTML(accept string) bool {
	if accept == "" {
		return false
	}
	accept = strings.ToLower(accept)
	for _, part := range strings.Split(accept, ",") {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if idx := strings.Index(value, ";"); idx >= 0 {
			value = value[:idx]
		}
		if value == "text/html" || value == "application/xhtml+xml" {
			return true
		}
	}
	return false
}

const oauthCallbackSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Login complete · Alex Console</title>
  <style>
    :root {
      color-scheme: light dark;
    }
    body {
      margin: 0;
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
      background: #f3f4f6;
    }
    main {
      background: #fff;
      padding: 32px;
      max-width: 420px;
      border-radius: 16px;
      box-shadow: 0 20px 45px rgba(15, 23, 42, 0.15);
      text-align: center;
    }
    h1 {
      font-size: 1.5rem;
      margin-bottom: 0.75rem;
      color: #111827;
    }
    p {
      margin: 0 0 1.25rem;
      color: #4b5563;
      line-height: 1.5;
    }
    button {
      background: #111827;
      color: #fff;
      border: none;
      border-radius: 999px;
      padding: 0.65rem 1.5rem;
      font-size: 1rem;
      cursor: pointer;
    }
    #fallback[hidden] {
      display: none;
    }
  </style>
  <script>
    (function () {
      function notifyParent() {
        try {
          if (window.opener && !window.opener.closed) {
            window.opener.postMessage({ source: "alex-auth", status: "success" }, "*");
          }
        } catch (err) {
          // ignore cross-origin restrictions
        }
      }

      function closeWindow() {
        notifyParent();
        try {
          window.close();
        } catch (err) {
          // ignore
        }
      }

      window.addEventListener("load", function () {
        closeWindow();
        window.setTimeout(function () {
          var fallback = document.getElementById("fallback");
          if (fallback) {
            fallback.hidden = false;
            var button = document.getElementById("close-window");
            if (button) {
              try { button.focus(); } catch (err) {}
            }
          }
        }, 800);
      });
    })();
  </script>
</head>
<body>
  <main>
    <h1>Login complete</h1>
    <p>You're signed in and this window will close automatically. You can return to Alex Console at any time.</p>
    <div id="fallback" hidden>
      <p>If this window didn't close, you can do it manually now.</p>
      <button id="close-window" type="button" onclick="window.close()">Close this window</button>
    </div>
  </main>
</body>
</html>`

func (h *AuthHandler) requireAuthenticatedUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return domain.User{}, false
	}
	claims, err := h.service.ParseAccessToken(r.Context(), token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return domain.User{}, false
	}
	user, err := h.service.GetUser(r.Context(), claims.Subject)
	if err != nil {
		h.writeDomainError(w, err)
		return domain.User{}, false
	}
	return user, true
}

func (h *AuthHandler) writeError(w http.ResponseWriter, err error) {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		http.Error(w, fmt.Sprintf("invalid json at %d", syntaxErr.Offset), http.StatusBadRequest)
	case errors.As(err, &typeErr):
		http.Error(w, fmt.Sprintf("invalid value for %s", typeErr.Field), http.StatusBadRequest)
	default:
		http.Error(w, "invalid request body", http.StatusBadRequest)
	}
}

func (h *AuthHandler) writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrUserExists):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, domain.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, domain.ErrProviderNotConfigured):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, domain.ErrStateMismatch):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrSessionExpired):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, domain.ErrSessionNotFound):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, domain.ErrInsufficientPoints):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidSubscriptionTier):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrSubscriptionExpiryRequired):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrSubscriptionExpiryInPast):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    base64.StdEncoding.EncodeToString([]byte(token)),
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: h.sameSiteMode(),
	})
}

func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: h.sameSiteMode(),
	})
}

func (h *AuthHandler) setAccessCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessCookieName,
		Value:    base64.StdEncoding.EncodeToString([]byte(token)),
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: h.sameSiteMode(),
	})
}

func (h *AuthHandler) clearAccessCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: h.sameSiteMode(),
	})
}

func (h *AuthHandler) sameSiteMode() http.SameSite {
	if h.secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func (h *AuthHandler) readRefreshToken(r *http.Request) string {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	defer func() {
		_ = r.Body.Close()
	}()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	if decoder.More() {
		return fmt.Errorf("multiple json values")
	}
	return nil
}

func toUserDTO(user domain.User) userDTO {
	plan := user.SubscriptionTier.Plan()
	return userDTO{
		ID:            user.ID,
		Email:         user.Email,
		DisplayName:   user.DisplayName,
		PointsBalance: user.PointsBalance,
		Subscription: subscriptionDTO{
			Tier:              string(plan.Tier),
			MonthlyPriceCents: plan.MonthlyPriceCents,
			ExpiresAt:         user.SubscriptionExpiresAt,
		},
	}
}

func toSubscriptionPlanDTO(plan domain.SubscriptionPlan) subscriptionPlanDTO {
	return subscriptionPlanDTO{
		Tier:              string(plan.Tier),
		MonthlyPriceCents: plan.MonthlyPriceCents,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func clientIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Forwarded-For"); realIP != "" {
		parts := strings.Split(realIP, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.Trim(r.RemoteAddr, "[]")
}
