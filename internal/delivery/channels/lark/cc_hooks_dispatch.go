package lark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/app/workdir"
)

// runCCHooksSetup writes Claude Code hooks directly into .claude/settings.local.json.
// No agent or Python script involved — just a deterministic file write.
func (g *Gateway) runCCHooksSetup(msg *incomingMessage) {
	cfg := g.cfg.CCHooksAutoConfig
	if cfg == nil {
		return
	}

	settingsPath := ccHooksSettingsPath(g.cfg.WorkspaceDir)
	err := writeCCHooks(settingsPath, cfg.ServerURL, cfg.Token)

	var reply string
	if err != nil {
		g.logger.Warn("cc-hooks-setup: write failed: %v", err)
		reply = fmt.Sprintf("Claude Code hooks 自动配置失败：%v\n请参考 scripts/cc_hooks/settings.example.json 手动配置", err)
	} else {
		reply = fmt.Sprintf("Claude Code hooks 配置完成。\npath: %s", settingsPath)
	}

	execCtx := g.buildTaskCommandContext(msg)
	g.dispatch(execCtx, msg.chatID, replyTarget("", msg.isGroup), "text", textContent(reply))
}

// ccHooksSettingsPath returns the path to .claude/settings.local.json
// relative to the project working directory.
func ccHooksSettingsPath(workspaceDir string) string {
	dir := strings.TrimSpace(workspaceDir)
	if dir == "" {
		dir = workdir.DefaultWorkingDir()
	}
	return filepath.Join(dir, ".claude", "settings.local.json")
}

// buildHookCommand builds the shell command for the hook entry.
func buildHookCommand(serverURL, token string) string {
	env := fmt.Sprintf("ELEPHANT_HOOKS_URL=%s", serverURL)
	if token != "" {
		env += fmt.Sprintf(" ELEPHANT_HOOKS_TOKEN=%s", token)
	}
	return fmt.Sprintf(`%s "$CLAUDE_PROJECT_DIR"/scripts/cc_hooks/notify_lark.sh`, env)
}

// writeCCHooks writes or merges hooks into the settings file.
func writeCCHooks(path, serverURL, token string) error {
	command := buildHookCommand(serverURL, token)

	hookEntry := map[string]interface{}{
		"type":    "command",
		"command": command,
		"async":   true,
		"timeout": 10,
	}
	hookRule := []interface{}{
		map[string]interface{}{
			"hooks": []interface{}{hookEntry},
		},
	}
	hooks := map[string]interface{}{
		"PostToolUse": hookRule,
		"Stop":        hookRule,
	}

	// Read existing settings if present.
	existing := make(map[string]interface{})
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	existing["hooks"] = hooks

	return atomicWriteJSON(path, existing)
}

// removeCCHooks removes the hooks key from the settings file.
func removeCCHooks(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil // nothing to remove
	}
	if err != nil {
		return err
	}

	var existing map[string]interface{}
	if err := json.Unmarshal(data, &existing); err != nil {
		return err
	}

	delete(existing, "hooks")

	if len(existing) == 0 {
		return os.Remove(path)
	}
	return atomicWriteJSON(path, existing)
}

// atomicWriteJSON writes data as formatted JSON to path via a temp file rename.
func atomicWriteJSON(path string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
