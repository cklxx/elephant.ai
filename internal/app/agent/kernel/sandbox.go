package kernel

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// IsSandboxPathRestriction reports whether an error indicates a sandbox/permission path restriction.
func IsSandboxPathRestriction(err error) bool {
	return isSandboxPathRestriction(err)
}

func isSandboxPathRestriction(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fs.ErrPermission) || os.IsPermission(err) {
		return true
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "permission denied") {
		return true
	}
	if strings.Contains(lower, "operation not permitted") {
		return true
	}
	if strings.Contains(lower, "read-only file system") {
		return true
	}
	if strings.Contains(lower, "path must stay within the working directory") {
		return true
	}
	if strings.Contains(lower, "sandbox") && strings.Contains(lower, "restrict") {
		return true
	}
	return false
}
