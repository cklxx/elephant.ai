package adapters

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"

	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
)

// Argon2 parameters tuned for server side hashing. Keeping moderate cost for tests.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
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
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(token), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s", argonTime, argonMemory, argonThreads, b64Salt, b64Hash), nil
}

// VerifyRefreshToken compares a plain token against an encoded hash.
func (m *JWTTokenManager) VerifyRefreshToken(token, encodedHash string) (bool, error) {
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}
	computed := argon2.IDKey([]byte(token), salt, params.time, params.memory, params.threads, uint32(len(hash)))
	if len(computed) != len(hash) {
		return false, nil
	}
	var diff byte
	for i := range computed {
		diff |= computed[i] ^ hash[i]
	}
	return diff == 0, nil
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

type argonParams struct {
	time    uint32
	memory  uint32
	threads uint8
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return argonParams{}, nil, nil, fmt.Errorf("invalid hash format")
	}
	var params argonParams
	var err error
	if params.time, err = parseUint32(parts[1]); err != nil {
		return argonParams{}, nil, nil, err
	}
	if params.memory, err = parseUint32(parts[2]); err != nil {
		return argonParams{}, nil, nil, err
	}
	threads, err := parseUint32(parts[3])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	// Bounds check: threads must be between 1 and 255 to fit in uint8
	if threads == 0 || threads > 255 {
		return argonParams{}, nil, nil, fmt.Errorf("invalid thread count: must be between 1 and 255")
	}
	params.threads = uint8(threads)
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	return params, salt, hash, nil
}

func parseUint32(value string) (uint32, error) {
	v, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

var _ ports.TokenManager = (*JWTTokenManager)(nil)
