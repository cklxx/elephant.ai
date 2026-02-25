package http

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"alex/internal/shared/utils"
)

const maxSessionIDLength = 128

var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validateSessionID(id string) error {
	if utils.IsBlank(id) {
		return errors.New("session_id is required")
	}
	if len(id) > maxSessionIDLength {
		return fmt.Errorf("session_id too long (max %d characters)", maxSessionIDLength)
	}
	if !sessionIDPattern.MatchString(id) {
		return errors.New("session_id contains invalid characters")
	}
	return nil
}

func isValidOptionalSessionID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", nil
	}
	if err := validateSessionID(trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}
