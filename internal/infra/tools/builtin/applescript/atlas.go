package applescript

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

const (
	atlasProcessName          = "ChatGPT Atlas"
	defaultConversationLimit  = 20
)

type atlasTool struct {
	shared.BaseTool
	runner Runner
}

// NewAtlas creates an Atlas (ChatGPT Atlas) AppleScript tool using the real osascript runner.
func NewAtlas() tools.ToolExecutor {
	return newAtlas(ExecRunner{})
}

func newAtlas(r Runner) *atlasTool {
	return &atlasTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "atlas",
				Description: `Control ChatGPT Atlas desktop app via AppleScript accessibility API (macOS only).

Actions:
- list_conversations: list sidebar conversation items
- switch_conversation: click a conversation by name
- read_bookmarks: read bookmarks section
- view_history: read recent history items

Note: Atlas has no scripting dictionary; this tool uses System Events UI automation,
which requires Accessibility permissions.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: "Action to perform.",
							Enum:        []any{"list_conversations", "switch_conversation", "read_bookmarks", "view_history"},
						},
						"conversation": {
							Type:        "string",
							Description: "Conversation name to switch to (required for switch_conversation).",
						},
						"limit": {
							Type:        "number",
							Description: "Maximum number of items to return (default 20).",
						},
					},
					Required: []string{"action"},
				},
			},
			ports.ToolMetadata{
				Name:        "atlas",
				Version:     "0.1.0",
				Category:    "automation",
				Tags:        []string{"atlas", "chatgpt", "macos", "applescript"},
				Dangerous:   false,
				SafetyLevel: ports.SafetyLevelReversible,
			},
		),
		runner: r,
	}
}

func (t *atlasTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action := shared.StringArg(call.Arguments, "action")
	if action == "" {
		return shared.ToolError(call.ID, "missing required parameter 'action'")
	}

	running, err := isAppRunning(ctx, t.runner, atlasProcessName)
	if err != nil {
		return shared.ToolError(call.ID, "failed to check if Atlas is running: %v", err)
	}
	if !running {
		return shared.ToolError(call.ID, "ChatGPT Atlas is not running")
	}

	switch action {
	case "list_conversations":
		return t.listConversations(ctx, call)
	case "switch_conversation":
		return t.switchConversation(ctx, call)
	case "read_bookmarks":
		return t.readBookmarks(ctx, call)
	case "view_history":
		return t.viewHistory(ctx, call)
	default:
		return shared.ToolError(call.ID, "unsupported action: %s", action)
	}
}

func (t *atlasTool) listConversations(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	limit := defaultConversationLimit
	if v, ok := shared.IntArg(call.Arguments, "limit"); ok && v > 0 {
		limit = v
	}

	// Use System Events to enumerate UI elements in the sidebar.
	// Atlas UI structure may vary; we attempt to get static text elements
	// from the first scroll area (sidebar) of window 1.
	script := `tell application "System Events"
	tell process "ChatGPT Atlas"
		set frontmost to true
		set uiItems to {}
		try
			set sidebarGroup to group 1 of splitter group 1 of window 1
			set uiElems to every static text of every UI element of sidebarGroup
			repeat with elemList in uiElems
				repeat with elem in elemList
					set end of uiItems to value of elem
				end repeat
			end repeat
		on error
			-- Fallback: try direct static texts in window
			set allTexts to every static text of window 1
			repeat with t in allTexts
				set end of uiItems to value of t
			end repeat
		end try
		set output to ""
		repeat with item_ in uiItems
			if item_ is not missing value and item_ is not "" then
				set output to output & item_ & linefeed
			end if
		end repeat
		return output
	end tell
end tell`

	out, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "list_conversations: %v", err)
	}

	items := parseLines(out, limit)
	data, _ := json.Marshal(items)
	return &ports.ToolResult{CallID: call.ID, Content: string(data)}, nil
}

func (t *atlasTool) switchConversation(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	name, errResult := shared.RequireStringArg(call.Arguments, call.ID, "conversation")
	if errResult != nil {
		return errResult, nil
	}

	escaped := escapeAppleScriptString(name)
	script := fmt.Sprintf(`tell application "System Events"
	tell process "ChatGPT Atlas"
		set frontmost to true
		set found to false
		try
			set sidebarGroup to group 1 of splitter group 1 of window 1
			set allElems to every UI element of sidebarGroup
			repeat with elem in allElems
				try
					set texts to every static text of elem
					repeat with t in texts
						if value of t is "%s" then
							click elem
							set found to true
							exit repeat
						end if
					end repeat
				end try
				if found then exit repeat
			end repeat
		end try
		return found
	end tell
end tell`, escaped)

	out, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "switch_conversation: %v", err)
	}

	if strings.TrimSpace(out) != "true" {
		return shared.ToolError(call.ID, "conversation %q not found in sidebar", name)
	}
	return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("Switched to conversation: %s", name)}, nil
}

func (t *atlasTool) readBookmarks(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	limit := defaultConversationLimit
	if v, ok := shared.IntArg(call.Arguments, "limit"); ok && v > 0 {
		limit = v
	}

	script := `tell application "System Events"
	tell process "ChatGPT Atlas"
		set frontmost to true
		set uiItems to {}
		try
			set bookmarkGroup to group 2 of splitter group 1 of window 1
			set uiElems to every static text of every UI element of bookmarkGroup
			repeat with elemList in uiElems
				repeat with elem in elemList
					set end of uiItems to value of elem
				end repeat
			end repeat
		on error
			return "bookmarks section not found"
		end try
		set output to ""
		repeat with item_ in uiItems
			if item_ is not missing value and item_ is not "" then
				set output to output & item_ & linefeed
			end if
		end repeat
		return output
	end tell
end tell`

	out, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "read_bookmarks: %v", err)
	}

	items := parseLines(out, limit)
	data, _ := json.Marshal(items)
	return &ports.ToolResult{CallID: call.ID, Content: string(data)}, nil
}

func (t *atlasTool) viewHistory(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	limit := defaultConversationLimit
	if v, ok := shared.IntArg(call.Arguments, "limit"); ok && v > 0 {
		limit = v
	}

	script := `tell application "System Events"
	tell process "ChatGPT Atlas"
		set frontmost to true
		set uiItems to {}
		try
			set mainGroup to group 1 of splitter group 1 of window 1
			set allElems to every UI element of mainGroup
			repeat with elem in allElems
				try
					set texts to every static text of elem
					repeat with t in texts
						if value of t is not missing value and value of t is not "" then
							set end of uiItems to value of t
						end if
					end repeat
				end try
			end repeat
		on error
			return "history section not found"
		end try
		set output to ""
		repeat with item_ in uiItems
			set output to output & item_ & linefeed
		end repeat
		return output
	end tell
end tell`

	out, err := t.runner.RunScript(ctx, script)
	if err != nil {
		return shared.ToolError(call.ID, "view_history: %v", err)
	}

	items := parseLines(out, limit)
	data, _ := json.Marshal(items)
	return &ports.ToolResult{CallID: call.ID, Content: string(data)}, nil
}

// parseLines splits newline-separated output into a string slice, respecting the limit.
func parseLines(output string, limit int) []string {
	var items []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		items = append(items, line)
		if len(items) >= limit {
			break
		}
	}
	return items
}
