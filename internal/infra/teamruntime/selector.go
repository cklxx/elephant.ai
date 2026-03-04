package teamruntime

import (
	"sort"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/coding"
	"alex/internal/shared/utils"
)

var profilePreference = map[string][]string{
	"execution": {
		agent.AgentTypeCodex, "opencode", agent.AgentTypeKimi, agent.AgentTypeClaudeCode, "gemini",
	},
	"planning": {
		agent.AgentTypeClaudeCode, agent.AgentTypeCodex, "gemini", agent.AgentTypeKimi, "opencode",
	},
	"long_context": {
		"gemini", agent.AgentTypeClaudeCode, agent.AgentTypeKimi, agent.AgentTypeCodex, "opencode",
	},
}

// SelectRoleBinding chooses the best available CLI for a role and constructs
// a deterministic fallback chain.
func SelectRoleBinding(
	roleID string,
	profile string,
	targetCLI string,
	caps []coding.DiscoveredCLICapability,
	roleLogPath string,
) RoleBinding {
	binding := RoleBinding{
		RoleID:            strings.TrimSpace(roleID),
		CapabilityProfile: normalizeProfile(profile),
		TargetCLI:         strings.TrimSpace(targetCLI),
		RoleLogPath:       strings.TrimSpace(roleLogPath),
	}
	if len(caps) == 0 {
		return binding
	}

	available := make([]coding.DiscoveredCLICapability, 0, len(caps))
	for _, cap := range caps {
		if cap.Executable {
			available = append(available, cap)
		}
	}
	if len(available) == 0 {
		return binding
	}

	// Target CLI hard preference.
	if target := strings.TrimSpace(targetCLI); target != "" {
		for _, cap := range available {
			if equalCLI(cap, target) {
				binding.SelectedCLI = cap.ID
				binding.SelectedPath = cap.Path
				binding.SelectedAgentType = selectedAgentType(cap)
				break
			}
		}
	}

	// Profile-driven selection.
	ranked := rankByProfile(available, binding.CapabilityProfile)
	if binding.SelectedCLI == "" && len(ranked) > 0 {
		top := ranked[0]
		binding.SelectedCLI = top.ID
		binding.SelectedPath = top.Path
		binding.SelectedAgentType = selectedAgentType(top)
	}

	for _, cap := range ranked {
		if cap.ID == binding.SelectedCLI {
			continue
		}
		binding.FallbackCLIs = append(binding.FallbackCLIs, cap.ID)
	}
	return binding
}

func selectedAgentType(cap coding.DiscoveredCLICapability) string {
	if strings.TrimSpace(cap.AgentType) != "" {
		return cap.AgentType
	}
	return agent.AgentTypeGenericCLI
}

func rankByProfile(caps []coding.DiscoveredCLICapability, profile string) []coding.DiscoveredCLICapability {
	preferences := profilePreference[normalizeProfile(profile)]
	if len(preferences) == 0 {
		preferences = []string{agent.AgentTypeCodex, agent.AgentTypeClaudeCode, agent.AgentTypeKimi, "gemini", "opencode"}
	}

	index := make(map[string]int, len(preferences))
	for i, id := range preferences {
		index[id] = i
	}

	out := append([]coding.DiscoveredCLICapability(nil), caps...)
	sort.Slice(out, func(i, j int) bool {
		li := indexScore(out[i], index)
		lj := indexScore(out[j], index)
		if li != lj {
			return li < lj
		}
		if out[i].AdapterSupport != out[j].AdapterSupport {
			return out[i].AdapterSupport
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func indexScore(cap coding.DiscoveredCLICapability, index map[string]int) int {
	if score, ok := index[cap.ID]; ok {
		return score
	}
	// Unknown CLIs are still valid fallback candidates.
	return 1000
}

func normalizeProfile(profile string) string {
	trimmed := utils.TrimLower(profile)
	switch trimmed {
	case "execution", "execute", "coding":
		return "execution"
	case "planning", "plan":
		return "planning"
	case "long_context", "long-context", "longtext", "long_text":
		return "long_context"
	default:
		return trimmed
	}
}

func equalCLI(cap coding.DiscoveredCLICapability, raw string) bool {
	target := utils.TrimLower(raw)
	if target == "" {
		return false
	}
	if utils.TrimLower(cap.ID) == target {
		return true
	}
	if utils.TrimLower(cap.Binary) == target {
		return true
	}
	return utils.TrimLower(filepathBase(cap.Path)) == target
}

func filepathBase(path string) string {
	parts := strings.Split(strings.ReplaceAll(path, "\\", "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
