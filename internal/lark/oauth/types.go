package oauth

import "time"

// Token holds a user's OAuth credentials for Lark Open API calls.
//
// Note: OpenID is tenant-scoped. Persist tokens keyed by OpenID.
type Token struct {
	OpenID           string    `json:"open_id"`
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	Scope            string    `json:"scope"`
	TokenType        string    `json:"token_type"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (t Token) AccessValidAt(now time.Time, leeway time.Duration) bool {
	if t.AccessToken == "" || t.ExpiresAt.IsZero() {
		return false
	}
	if leeway < 0 {
		leeway = 0
	}
	return now.Add(leeway).Before(t.ExpiresAt)
}
