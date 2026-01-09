package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

const (
	shareTokenMetadataKey   = "share_token"
	shareEnabledMetadataKey = "share_enabled"
)

// ErrShareTokenInvalid signals an invalid or missing share token.
var ErrShareTokenInvalid = errors.New("share token invalid")

// EnsureSessionShareToken returns an existing share token or creates one.
func (s *ServerCoordinator) EnsureSessionShareToken(ctx context.Context, sessionID string, reset bool) (string, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return "", fmt.Errorf("session id required")
	}

	session, err := s.sessionStore.Get(ctx, trimmedID)
	if err != nil {
		return "", err
	}

	metadata := session.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
		session.Metadata = metadata
	}

	if !reset {
		if existing := strings.TrimSpace(metadata[shareTokenMetadataKey]); existing != "" {
			return existing, nil
		}
	}

	token := fmt.Sprintf("share-%s", id.NewKSUID())
	if token == "share-" {
		return "", fmt.Errorf("failed to generate share token")
	}

	metadata[shareTokenMetadataKey] = token
	metadata[shareEnabledMetadataKey] = "true"

	if err := s.sessionStore.Save(ctx, session); err != nil {
		return "", err
	}

	return token, nil
}

// ValidateShareToken returns the session if the token matches.
func (s *ServerCoordinator) ValidateShareToken(ctx context.Context, sessionID string, token string) (*ports.Session, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return nil, fmt.Errorf("session id required")
	}

	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return nil, ErrShareTokenInvalid
	}

	session, err := s.sessionStore.Get(ctx, trimmedID)
	if err != nil {
		return nil, err
	}

	expected := ""
	if session.Metadata != nil {
		expected = strings.TrimSpace(session.Metadata[shareTokenMetadataKey])
	}
	if expected == "" || expected != trimmedToken {
		return nil, ErrShareTokenInvalid
	}

	return session, nil
}
