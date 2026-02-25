package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	agent "alex/internal/domain/agent/ports/agent"
	id "alex/internal/shared/utils/id"
)

// BaseConfig holds the config fields shared by every channel gateway.
type BaseConfig struct {
	SessionPrefix string
	ReplyPrefix   string
	AllowGroups   bool
	AllowDirect   bool
	AgentPreset   string
	ToolPreset    string
	ReplyTimeout  time.Duration
	MemoryEnabled bool
}

// BaseGateway provides helpers shared by every channel gateway.
type BaseGateway struct {
	SessionLocks sync.Map
}

// SessionLock returns or creates a per-session mutex. An empty sessionID
// returns a fresh (unshared) mutex so callers never need a nil check.
func (g *BaseGateway) SessionLock(sessionID string) *sync.Mutex {
	if sessionID == "" {
		return &sync.Mutex{}
	}
	value, _ := g.SessionLocks.LoadOrStore(sessionID, &sync.Mutex{})
	return value.(*sync.Mutex)
}

// BuildBaseContext assembles the context keys common to every channel gateway
// (session ID, user ID, log ID, channel name, chat ID, group flag, memory policy).
func BuildBaseContext(cfg BaseConfig, channel, sessionID, userID, chatID string, isGroup bool) context.Context {
	ctx := context.Background()
	ctx = id.WithSessionID(ctx, sessionID)
	ctx = id.WithUserID(ctx, userID)
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	ctx = appcontext.WithChannel(ctx, channel)
	ctx = appcontext.WithChatID(ctx, chatID)
	ctx = appcontext.WithIsGroup(ctx, isGroup)
	if cfg.MemoryEnabled {
		ctx = appcontext.WithMemoryPolicy(ctx, appcontext.MemoryPolicy{
			Enabled:         true,
			AutoRecall:      true,
			AutoCapture:     true,
			CaptureMessages: true,
			RefreshEnabled:  true,
		})
	} else {
		ctx = appcontext.WithMemoryPolicy(ctx, appcontext.MemoryPolicy{
			Enabled:         false,
			AutoRecall:      false,
			AutoCapture:     false,
			CaptureMessages: false,
			RefreshEnabled:  false,
		})
	}
	return ctx
}

// ApplyPresets adds agent/tool preset values to the context if configured.
func ApplyPresets(ctx context.Context, cfg BaseConfig) context.Context {
	if cfg.AgentPreset != "" || cfg.ToolPreset != "" {
		ctx = context.WithValue(ctx, appcontext.PresetContextKey{}, appcontext.PresetConfig{
			AgentPreset: cfg.AgentPreset,
			ToolPreset:  cfg.ToolPreset,
		})
	}
	return ctx
}

// ApplyTimeout wraps ctx with a deadline if cfg.ReplyTimeout > 0.
// The returned cancel function must be deferred by the caller.
func ApplyTimeout(ctx context.Context, cfg BaseConfig) (context.Context, context.CancelFunc) {
	if cfg.ReplyTimeout > 0 {
		return context.WithTimeout(ctx, cfg.ReplyTimeout)
	}
	return ctx, func() {}
}

// BuildReplyCore constructs the reply string from the agent result, applying
// the reply prefix. Channel-specific decoration (e.g. mentions) is left to
// the caller.
func BuildReplyCore(cfg BaseConfig, result *agent.TaskResult, execErr error) string {
	reply := ""
	if execErr != nil {
		reply = fmt.Sprintf("执行失败：%v", execErr)
	} else if result != nil {
		reply = result.Answer
	}
	reply = ShapeReply7C(reply)
	if reply == "" {
		return ""
	}
	if cfg.ReplyPrefix != "" {
		reply = cfg.ReplyPrefix + reply
	}
	return reply
}
