package memory

import (
	"path/filepath"
	"strings"
)

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
	safe := sanitizeSegment(userID)
	return filepath.Join(root, userDirName, safe)
}
