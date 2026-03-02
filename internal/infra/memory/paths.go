package memory

import (
	"strings"

	"alex/internal/shared/utils"
)

var reservedUserDirNames = map[string]struct{}{
	strings.ToLower(dailyDirName):      {},
	strings.ToLower(memoryFileName):    {},
	strings.ToLower(indexFileName):     {},
	strings.ToLower(legacyUserDirName): {},
}

func isReservedUserDirName(name string) bool {
	trimmed := utils.TrimLower(name)
	if trimmed == "" {
		return false
	}
	_, ok := reservedUserDirNames[trimmed]
	return ok
}

// ResolveUserRoot resolves the memory root for a specific user.
// Local agents share a single memory root; userID is ignored.
func ResolveUserRoot(rootDir, _ string) string {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		return ""
	}
	return root
}
