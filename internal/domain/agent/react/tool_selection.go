package react

import (
	"strings"

	"alex/internal/domain/agent/ports"
)

// Tool groups for dynamic tool selection. On turn 2+, only the core group
// and the groups containing recently-used tools are sent to the LLM, which
// reduces the tool definition token overhead by ~50-70%.

// coreTools are always included regardless of turn.
var coreTools = map[string]bool{
	"plan":                true,
	"ask_user":            true,
	"context_checkpoint":  true,
	"skills":              true,
	"reply_agent":         true,
	"run_tasks":           true,
	"artifacts_write":     true,
	"memory_search":       true,
	"memory_get":          true,
	"memory_set":          true,
	"memory_related":      true,
}

// toolGroups maps each tool to a functional group. Tools in the same group
// are included together when any member was recently used.
var toolGroups = map[string]string{
	"read_file":       "file",
	"write_file":      "file",
	"replace_in_file": "file",
	"list_dir":        "file",
	"shell_exec":      "file",
	"web_search":      "search",
	"web_fetch":       "search",
	"channel":         "lark",
	"lark_chat_history": "lark",
	"lark_calendar_create": "lark",
}

const recentToolLookbackTurns = 4

// selectToolsForTurn filters tool definitions based on the current iteration.
// Turn 1 (Iterations==0): all tools.
// Turn 2+: core tools + groups of recently used tools.
// Safety valve: if ToolSelectionRecovery is set, all tools are returned.
func selectToolsForTurn(allTools []ports.ToolDefinition, state *TaskState) []ports.ToolDefinition {
	if state == nil || state.Iterations == 0 || state.ToolSelectionRecovery {
		// First turn or recovery mode: return all tools.
		if state != nil {
			state.ToolSelectionRecovery = false
		}
		return allTools
	}

	// Determine which groups are active based on recently used tools.
	activeGroups := recentlyUsedGroups(state.Messages, recentToolLookbackTurns)

	selected := make([]ports.ToolDefinition, 0, len(allTools))
	for _, tool := range allTools {
		name := strings.TrimSpace(tool.Name)
		if shouldIncludeTool(name, activeGroups) {
			selected = append(selected, tool)
		}
	}

	return selected
}

// shouldIncludeTool returns true if the tool should be included in the
// current turn's tool definitions.
func shouldIncludeTool(name string, activeGroups map[string]bool) bool {
	// Core tools always included.
	if coreTools[name] {
		return true
	}
	// MCP tools always included (dynamic, user-configured).
	if strings.HasPrefix(name, "mcp__") {
		return true
	}
	// Check if this tool's group is active.
	if group, ok := toolGroups[name]; ok {
		return activeGroups[group]
	}
	// Tools not in any group are included by default (e.g. custom tools).
	return true
}

// recentlyUsedGroups scans recent messages for tool calls and returns the
// set of tool groups that were used.
func recentlyUsedGroups(messages []Message, lookbackTurns int) map[string]bool {
	groups := make(map[string]bool)
	turnsScanned := 0

	for i := len(messages) - 1; i >= 0 && turnsScanned < lookbackTurns; i-- {
		msg := messages[i]
		if strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
			turnsScanned++
		}
		for _, tc := range msg.ToolCalls {
			name := strings.TrimSpace(tc.Name)
			if group, ok := toolGroups[name]; ok {
				groups[group] = true
			}
		}
	}
	return groups
}

// markToolNotFoundRecovery sets the safety valve flag so the next turn
// restores all tool definitions.
func markToolNotFoundRecovery(state *TaskState) {
	if state != nil {
		state.ToolSelectionRecovery = true
	}
}
