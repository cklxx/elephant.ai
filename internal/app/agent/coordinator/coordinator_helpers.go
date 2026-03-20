package coordinator

import (
	"fmt"

	appconfig "alex/internal/app/agent/config"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	react "alex/internal/domain/agent/react"
)

func buildCompletionDefaultsFromConfig(cfg appconfig.Config) react.CompletionDefaults {
	defaults := react.CompletionDefaults{}

	if cfg.TemperatureProvided {
		temp := cfg.Temperature
		defaults.Temperature = &temp
	}
	if cfg.MaxTokens > 0 {
		maxTokens := cfg.MaxTokens
		defaults.MaxTokens = &maxTokens
	}
	if cfg.TopP > 0 {
		topP := cfg.TopP
		defaults.TopP = &topP
	}
	if len(cfg.StopSequences) > 0 {
		defaults.StopSequences = append([]string(nil), cfg.StopSequences...)
	}

	return defaults
}

func attachWorkflowSnapshot(result *agent.TaskResult, wf *agentWorkflow, sessionID, runID, parentRunID string) *agent.TaskResult {
	if result == nil {
		result = &agent.TaskResult{}
	}
	result.SessionID = defaultString(result.SessionID, sessionID)
	result.RunID = defaultString(result.RunID, runID)
	result.ParentRunID = defaultString(result.ParentRunID, parentRunID)

	if wf != nil {
		snapshot := wf.snapshot()
		result.Workflow = &snapshot
	}

	return result
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// obfuscateSessionID masks session identifiers when logging to avoid leaking
// potentially sensitive values. It retains a short prefix and suffix to keep
// logs useful for correlation while hiding the majority of the identifier.
func obfuscateSessionID(id string) string {
	if id == "" {
		return ""
	}

	if len(id) <= 8 {
		return "****"
	}

	return fmt.Sprintf("%s...%s", id[:4], id[len(id)-4:])
}

// ensureSessionMetadata sets a session metadata key if the value is non-empty
// and no existing value is present.
func ensureSessionMetadata(session *storage.Session, key string, value string) {
	if value == "" {
		return
	}
	if session.Metadata == nil {
		session.Metadata = make(map[string]string)
	}
	if session.Metadata[key] == "" {
		session.Metadata[key] = value
	}
}
