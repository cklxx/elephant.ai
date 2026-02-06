package adapters

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"alex/internal/domain/auth"
	"alex/internal/domain/auth/ports"
	"alex/internal/infra/auth/crypto"
)

// JWTTokenManager issues JWT access tokens and hashes refresh tokens using Argon2id.
type JWTTokenManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

// NewJWTTokenManager creates a new token manager.
func NewJWTTokenManager(secret, issuer string, accessTTL time.Duration) *JWTTokenManager {
	if accessTTL == 0 {
		accessTTL = 15 * time.Minute
	}
	return &JWTTokenManager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL}
}

// GenerateAccessToken implements ports.TokenManager.
func (m *JWTTokenManager) GenerateAccessToken(_ context.Context, user domain.User, sessionID string) (string, time.Time, error) {
	if len(m.secret) == 0 {
		return "", time.Time{}, errors.New("jwt secret not configured")
	}
	expiresAt := time.Now().Add(m.accessTTL)
	claims := jwt.MapClaims{
		"sub":        user.ID,
		"email":      user.Email,
		"session_id": sessionID,
		"exp":        expiresAt.Unix(),
		"iss":        m.issuer,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// GenerateRefreshToken generates a random token and returns the plain and hashed values.
func (m *JWTTokenManager) GenerateRefreshToken(context.Context) (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain := base64.RawURLEncoding.EncodeToString(buf)
	hash, err := m.HashRefreshToken(plain)
	if err != nil {
		return "", "", err
	}
	return plain, hash, nil
}

// HashRefreshToken encodes the provided string using Argon2id.
func (m *JWTTokenManager) HashRefreshToken(token string) (string, error) {
	return crypto.HashValue(token)
}

// VerifyRefreshToken compares a plain token against an encoded hash.
func (m *JWTTokenManager) VerifyRefreshToken(token, encodedHash string) (bool, error) {
	return crypto.VerifyArgon2id(token, encodedHash)
}

// ParseAccessToken parses a JWT access token.
func (m *JWTTokenManager) ParseAccessToken(_ context.Context, token string) (domain.Claims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return domain.Claims{}, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return domain.Claims{}, errors.New("invalid token claims")
	}
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	sessionID, _ := claims["session_id"].(string)
	expValue, _ := claims["exp"].(float64)
	expiresAt := time.Unix(int64(expValue), 0)
	return domain.Claims{Subject: sub, Email: email, SessionID: sessionID, ExpiresAt: expiresAt}, nil
}

var _ ports.TokenManager = (*JWTTokenManager)(nil)
