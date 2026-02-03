package memory

import (
	"path/filepath"
	"strings"
)

var reservedUserDirNames = map[string]struct{}{
	strings.ToLower(dailyDirName):      {},
	strings.ToLower(memoryFileName):    {},
	strings.ToLower(indexFileName):     {},
	strings.ToLower(legacyUserDirName): {},
}

func isReservedUserDirName(name string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return false
	}
	_, ok := reservedUserDirNames[trimmed]
	return ok
}

func normalizeUserDirName(userID string) string {
	safe := sanitizeSegment(userID)
	if safe == "" {
		return "user"
	}
	if isReservedUserDirName(safe) {
		return "user-" + safe
	}
	return safe
}

// ResolveUserRoot resolves the memory root for a specific user.
func ResolveUserRoot(rootDir, userID string) string {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		return ""
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return root
	}
	safe := normalizeUserDirName(userID)
	return filepath.Join(root, safe)
}
