package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "alex/internal/auth/app"
	"alex/internal/auth/domain"
	"alex/internal/utils"
)

const (
	maxAuthBodySize   = 1 << 16
	refreshCookieName = "alex_refresh_token"
)

// AuthHandler manages authentication endpoints.
type AuthHandler struct {
	service *authapp.Service
	logger  *utils.Logger
	secure  bool
}

// NewAuthHandler builds a new authentication handler.
func NewAuthHandler(service *authapp.Service, secure bool) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  utils.NewComponentLogger("AuthHandler"),
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

type tokenResponse struct {
	AccessToken    string    `json:"access_token"`
	ExpiresAt      time.Time `json:"expires_at"`
	RefreshExpires time.Time `json:"refresh_expires_at"`
	User           userDTO   `json:"user"`
}

type userDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
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
	writeJSON(w, http.StatusCreated, userDTO{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName})
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
			h.writeError(w, err)
			return
		}
		refresh = req.RefreshToken
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
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "", Path: "/", Expires: time.Unix(0, 0), MaxAge: -1, HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

// HandleMe returns the current user from the Authorization header.
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}
	claims, err := h.service.ParseAccessToken(r.Context(), token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	user, err := h.service.GetUser(r.Context(), claims.Subject)
	if err != nil {
		h.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(user))
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
	resp := tokenResponse{AccessToken: tokens.AccessToken, ExpiresAt: tokens.AccessExpiry, RefreshExpires: tokens.RefreshExpiry, User: toUserDTO(user)}
	writeJSON(w, http.StatusOK, resp)
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
		SameSite: http.SameSiteLaxMode,
	})
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
	defer r.Body.Close()
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
	return userDTO{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName}
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
	host := strings.Split(r.RemoteAddr, ":")
	if len(host) > 0 {
		return host[0]
	}
	return ""
}
