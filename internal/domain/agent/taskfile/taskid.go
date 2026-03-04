package taskfile

import "strings"

// BaseTaskID strips all -retry-N suffixes from a task ID, returning the
// original base identifier. For example, "slow-task-retry-1-retry-2"
// returns "slow-task".
func BaseTaskID(id string) string {
	base := id
	for {
		idx := strings.LastIndex(base, "-retry-")
		if idx <= 0 {
			return base
		}
		suffix := base[idx+len("-retry-"):]
		if suffix == "" {
			return base
		}
		for _, r := range suffix {
			if r < '0' || r > '9' {
				return base
			}
		}
		base = base[:idx]
	}
}

// ExtractRoleID extracts the role name from a team task ID by stripping
// the "team-" prefix, "-debate" suffix, and any "-retry-N" suffixes.
// Returns empty string for non-team task IDs.
func ExtractRoleID(id string) string {
	trimmed := strings.TrimSpace(id)
	if !strings.HasPrefix(trimmed, "team-") {
		return ""
	}
	result := strings.TrimPrefix(trimmed, "team-")
	// Strip retry suffixes first, then debate suffix.
	result = BaseTaskID(result)
	result = strings.TrimSuffix(result, "-debate")
	return strings.TrimSpace(result)
}
