package execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/acp"
	jsonrpc "alex/internal/mcp"
)

const (
	acpRetryBaseDelay  = 200 * time.Millisecond
	acpRetryMaxDelay   = 2 * time.Second
	acpRetryMaxElapsed = 10 * time.Second
)

func callInitialize(ctx context.Context, client *acp.Client) error {
	resp, err := callWithRetry(ctx, client, "initialize", map[string]any{
		"protocolVersion": 1,
	})
	if err != nil {
		return fmt.Errorf("acp_executor initialize failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("acp_executor initialize error: %s", resp.Error.Message)
	}
	return nil
}

func callSetMode(ctx context.Context, client *acp.Client, sessionID, mode string) error {
	resp, err := callWithRetry(ctx, client, "session/set_mode", map[string]any{
		"sessionId": sessionID,
		"modeId":    mode,
	})
	if err != nil {
		return fmt.Errorf("acp_executor set_mode failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("acp_executor set_mode error: %s", resp.Error.Message)
	}
	return nil
}

func (t *acpExecutorTool) ensureSession(ctx context.Context, client *acp.Client, sessionID, cwd string) (string, map[string]bool, error) {
	t.mu.Lock()
	remoteID := t.sessions[sessionID]
	t.mu.Unlock()

	mcpServers := []any{}

	if remoteID != "" {
		resp, err := callWithRetry(ctx, client, "session/load", map[string]any{
			"sessionId":  remoteID,
			"cwd":        cwd,
			"mcpServers": mcpServers,
		})
		if err == nil && resp != nil && resp.Error == nil {
			return remoteID, parseAvailableModes(resp.Result), nil
		}
	}

	resp, err := callWithRetry(ctx, client, "session/new", map[string]any{
		"cwd":        cwd,
		"mcpServers": mcpServers,
	})
	if err != nil {
		return "", nil, fmt.Errorf("acp_executor session/new failed: %w", err)
	}
	if resp.Error != nil {
		return "", nil, fmt.Errorf("acp_executor session/new error: %s", resp.Error.Message)
	}

	newID := ""
	if resp.Result != nil {
		if raw, ok := resp.Result.(map[string]any); ok {
			if val, ok := raw["sessionId"].(string); ok {
				newID = strings.TrimSpace(val)
			}
		}
	}
	if newID == "" {
		return "", nil, fmt.Errorf("acp_executor session/new missing sessionId")
	}
	t.mu.Lock()
	t.sessions[sessionID] = newID
	t.mu.Unlock()
	return newID, parseAvailableModes(resp.Result), nil
}

func parseAvailableModes(result any) map[string]bool {
	raw, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	rawModes, ok := raw["modes"].(map[string]any)
	if !ok {
		return nil
	}
	rawList, ok := rawModes["availableModes"].([]any)
	if !ok {
		return nil
	}
	out := make(map[string]bool, len(rawList))
	for _, entry := range rawList {
		mode, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id, ok := mode["id"].(string)
		if !ok {
			continue
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out[id] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func modeSupported(available map[string]bool, mode string) bool {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return false
	}
	if available == nil {
		return true
	}
	return available[mode]
}

func isUnsupportedModeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unsupported mode") || strings.Contains(msg, "invalid params")
}

func callWithRetry(ctx context.Context, client *acp.Client, method string, params map[string]any) (*jsonrpc.Response, error) {
	if client == nil {
		return nil, fmt.Errorf("acp client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	delay := acpRetryBaseDelay

	for {
		resp, err := client.Call(ctx, method, params)
		if err == nil || !acp.IsRetryableError(err) {
			return resp, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Since(start) >= acpRetryMaxElapsed {
			return nil, err
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		if delay < acpRetryMaxDelay {
			delay *= 2
			if delay > acpRetryMaxDelay {
				delay = acpRetryMaxDelay
			}
		}
	}
}
