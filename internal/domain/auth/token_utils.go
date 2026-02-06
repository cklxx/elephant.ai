package domain

import (
	"crypto/sha256"
	"encoding/base64"
)

// FingerprintRefreshToken returns a deterministic fingerprint for a refresh token.
// The fingerprint is safe to store in databases and can be used for indexed lookups
// without revealing the original token value.
func FingerprintRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
