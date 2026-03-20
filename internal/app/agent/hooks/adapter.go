package hooks

import (
	"context"

	"alex/internal/core/hook"
)

// proactiveHookAdapter wraps a legacy ProactiveHook as a core/hook Plugin
// implementing PreTaskHook and PostTaskHook.
type proactiveHookAdapter struct {
	legacy ProactiveHook
}

// AdaptProactiveHook wraps a legacy ProactiveHook so it can be registered
// with a core/hook.HookRuntime.
func AdaptProactiveHook(h ProactiveHook) hook.Plugin {
	return &proactiveHookAdapter{legacy: h}
}

func (a *proactiveHookAdapter) Name() string  { return a.legacy.Name() }
func (a *proactiveHookAdapter) Priority() int { return 50 }

// PreTask delegates to the legacy hook's OnTaskStart.
// Injections are collected but not applied here — application-level code is
// responsible for incorporating injections into the conversation context.
func (a *proactiveHookAdapter) PreTask(ctx context.Context, state *hook.TurnState) error {
	task := TaskInfo{
		TaskInput: state.Input,
		SessionID: state.SessionID,
		RunID:     state.RunID,
		UserID:    state.UserID,
	}
	injections := a.legacy.OnTaskStart(ctx, task)
	if len(injections) > 0 {
		// Store injections in TurnState so downstream code can consume them.
		state.Set("legacy_injections", injections)
	}
	return nil
}

// PostTask delegates to the legacy hook's OnTaskCompleted.
func (a *proactiveHookAdapter) PostTask(ctx context.Context, state *hook.TurnState, result *hook.TurnResult) error {
	ri := TaskResultInfo{
		TaskInput: state.Input,
		SessionID: state.SessionID,
		RunID:     state.RunID,
		UserID:    state.UserID,
	}
	if result != nil && result.ModelOutput != nil {
		ri.Answer = result.ModelOutput.Text
		ri.StopReason = result.ModelOutput.StopReason
	}
	return a.legacy.OnTaskCompleted(ctx, ri)
}

// Compile-time interface assertions.
var (
	_ hook.Plugin      = (*proactiveHookAdapter)(nil)
	_ hook.PreTaskHook = (*proactiveHookAdapter)(nil)
	_ hook.PostTaskHook = (*proactiveHookAdapter)(nil)
)
