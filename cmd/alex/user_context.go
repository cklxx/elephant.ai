package main

import (
	"os"
	"os/user"
	"strings"
	"unicode"
)

const defaultCLIUserID = "cli-user"

func resolveCLIUserID() string {
	candidates := []string{
		lookupEnv("ALEX_USER_ID"),
		lookupEnv("ALEX_CLI_USER_ID"),
		lookupEnv("USER"),
		lookupEnv("USERNAME"),
	}

	for _, candidate := range candidates {
		if sanitized := sanitizeUserID(candidate); sanitized != "" {
			return sanitized
		}
	}

	if current, err := user.Current(); err == nil {
		if sanitized := sanitizeUserID(current.Username); sanitized != "" {
			return sanitized
		}
	}

	return defaultCLIUserID
}

func sanitizeUserID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
			lastDash = false
		case r == '-' || r == '_':
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}

	sanitized := builder.String()
	sanitized = strings.Trim(sanitized, "-")
	sanitized = strings.TrimLeft(sanitized, ".")
	sanitized = strings.TrimRight(sanitized, ".")

	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	return sanitized
}

func lookupEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return ""
}
