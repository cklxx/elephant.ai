package framework

import (
	"context"
	"fmt"

	"alex/internal/core/channel"
	"alex/internal/core/envelope"
	"alex/internal/core/hook"
	"alex/internal/core/tape"
)

// turnExecutor handles a single turn lifecycle.
type turnExecutor struct {
	hooks    *hook.HookRuntime
	tapes    *tape.TapeManager
	channels *channel.Manager
}

// execute runs the 7 lifecycle steps:
//  1. resolve_session — CallFirst on SessionResolver
//  2. load_state — CallMany on StateLoader
//  3. build_prompt — CallFirst on PromptBuilder
//  4. run_model — CallFirst on ModelRunner
//  5. save_state — CallMany on StateSaver
//  6. render_outbound — CallFirst on OutboundRenderer
//  7. dispatch_outbound — CallMany on OutboundDispatcher
//
// PreTaskHook runs before step 4, PostTaskHook runs after step 5.
// ErrorHandler is called on any step error.
func (t *turnExecutor) execute(ctx context.Context, env envelope.Envelope) (*hook.TurnResult, error) {
	state := &hook.TurnState{
		Input:    env.ContentOf(),
		Channel:  envelope.FieldOf[string](env, "channel"),
		UserID:   envelope.FieldOf[string](env, "user_id"),
		Metadata: env.Fields(),
	}

	result := &hook.TurnResult{
		Input:    state.Input,
		Metadata: make(map[string]any),
	}

	// Step 1: resolve_session
	if err := t.handleError(ctx, state, "resolve_session", t.resolveSession(ctx, state)); err != nil {
		result.Error = err
		return result, err
	}
	result.SessionID = state.SessionID
	result.RunID = state.RunID

	// Record turn_start after session/run IDs are known; turn_end on exit.
	t.recordAnchor(ctx, "turn_start", state)
	defer t.recordAnchor(ctx, "turn_end", state)

	// Step 2: load_state
	if err := t.handleError(ctx, state, "load_state", t.loadState(ctx, state)); err != nil {
		result.Error = err
		return result, err
	}

	// Step 3: build_prompt
	prompt, err := t.buildPrompt(ctx, state)
	if err != nil {
		if herr := t.handleError(ctx, state, "build_prompt", err); herr != nil {
			result.Error = herr
			return result, herr
		}
	}
	result.Prompt = prompt

	// PreTaskHook: runs before model execution
	if err := t.handleError(ctx, state, "pre_task", t.preTask(ctx, state)); err != nil {
		result.Error = err
		return result, err
	}

	// Step 4: run_model
	modelOutput, err := t.runModel(ctx, state, prompt)
	if err != nil {
		if herr := t.handleError(ctx, state, "run_model", err); herr != nil {
			result.Error = herr
			return result, herr
		}
	}
	result.ModelOutput = modelOutput

	// Step 5: save_state
	if err := t.handleError(ctx, state, "save_state", t.saveState(ctx, state)); err != nil {
		result.Error = err
		return result, err
	}

	// PostTaskHook: runs after save_state
	if err := t.handleError(ctx, state, "post_task", t.postTask(ctx, state, result)); err != nil {
		result.Error = err
		return result, err
	}

	// Step 6: render_outbound
	outbounds, err := t.renderOutbound(ctx, state, modelOutput)
	if err != nil {
		if herr := t.handleError(ctx, state, "render_outbound", err); herr != nil {
			result.Error = herr
			return result, herr
		}
	}
	result.Outbounds = outbounds

	// Step 7: dispatch_outbound
	if err := t.handleError(ctx, state, "dispatch_outbound", t.dispatchOutbound(ctx, outbounds)); err != nil {
		result.Error = err
		return result, err
	}

	return result, nil
}

func (t *turnExecutor) resolveSession(ctx context.Context, state *hook.TurnState) error {
	_, err := hook.CallFirst[any](ctx, t.hooks, func(p hook.Plugin) (any, bool, error) {
		if sr, ok := p.(hook.SessionResolver); ok {
			return nil, true, sr.ResolveSession(ctx, state)
		}
		return nil, false, nil
	})
	return err
}

func (t *turnExecutor) loadState(ctx context.Context, state *hook.TurnState) error {
	_, errs := hook.CallMany[any](ctx, t.hooks, func(p hook.Plugin) (any, bool, error) {
		if sl, ok := p.(hook.StateLoader); ok {
			return nil, true, sl.LoadState(ctx, state)
		}
		return nil, false, nil
	})
	return firstError(errs)
}

func (t *turnExecutor) buildPrompt(ctx context.Context, state *hook.TurnState) (*hook.Prompt, error) {
	return hook.CallFirst[*hook.Prompt](ctx, t.hooks, func(p hook.Plugin) (*hook.Prompt, bool, error) {
		if pb, ok := p.(hook.PromptBuilder); ok {
			prompt, err := pb.BuildPrompt(ctx, state)
			if err != nil {
				return nil, false, err
			}
			return prompt, true, nil
		}
		return nil, false, nil
	})
}

func (t *turnExecutor) preTask(ctx context.Context, state *hook.TurnState) error {
	_, errs := hook.CallMany[any](ctx, t.hooks, func(p hook.Plugin) (any, bool, error) {
		if pt, ok := p.(hook.PreTaskHook); ok {
			return nil, true, pt.PreTask(ctx, state)
		}
		return nil, false, nil
	})
	return firstError(errs)
}

func (t *turnExecutor) runModel(ctx context.Context, state *hook.TurnState, prompt *hook.Prompt) (*hook.ModelOutput, error) {
	return hook.CallFirst[*hook.ModelOutput](ctx, t.hooks, func(p hook.Plugin) (*hook.ModelOutput, bool, error) {
		if mr, ok := p.(hook.ModelRunner); ok {
			output, err := mr.RunModel(ctx, state, prompt)
			if err != nil {
				return nil, false, err
			}
			return output, true, nil
		}
		return nil, false, nil
	})
}

func (t *turnExecutor) saveState(ctx context.Context, state *hook.TurnState) error {
	_, errs := hook.CallMany[any](ctx, t.hooks, func(p hook.Plugin) (any, bool, error) {
		if ss, ok := p.(hook.StateSaver); ok {
			return nil, true, ss.SaveState(ctx, state)
		}
		return nil, false, nil
	})
	return firstError(errs)
}

func (t *turnExecutor) postTask(ctx context.Context, state *hook.TurnState, result *hook.TurnResult) error {
	_, errs := hook.CallMany[any](ctx, t.hooks, func(p hook.Plugin) (any, bool, error) {
		if pt, ok := p.(hook.PostTaskHook); ok {
			return nil, true, pt.PostTask(ctx, state, result)
		}
		return nil, false, nil
	})
	return firstError(errs)
}

func (t *turnExecutor) renderOutbound(ctx context.Context, state *hook.TurnState, output *hook.ModelOutput) ([]hook.Outbound, error) {
	return hook.CallFirst[[]hook.Outbound](ctx, t.hooks, func(p hook.Plugin) ([]hook.Outbound, bool, error) {
		if or, ok := p.(hook.OutboundRenderer); ok {
			outbounds, err := or.RenderOutbound(ctx, state, output)
			if err != nil {
				return nil, false, err
			}
			return outbounds, true, nil
		}
		return nil, false, nil
	})
}

func (t *turnExecutor) dispatchOutbound(ctx context.Context, outbounds []hook.Outbound) error {
	_, errs := hook.CallMany[any](ctx, t.hooks, func(p hook.Plugin) (any, bool, error) {
		if od, ok := p.(hook.OutboundDispatcher); ok {
			return nil, true, od.DispatchOutbound(ctx, outbounds)
		}
		return nil, false, nil
	})
	return firstError(errs)
}

func (t *turnExecutor) handleError(ctx context.Context, state *hook.TurnState, step string, err error) error {
	if err == nil {
		return nil
	}

	// Record the error to tape before calling error handlers.
	t.recordError(ctx, step, err, state)

	// Call error handlers but still return the original error if no handler overrides.
	_, handlerErrs := hook.CallMany[any](ctx, t.hooks, func(p hook.Plugin) (any, bool, error) {
		if eh, ok := p.(hook.ErrorHandler); ok {
			return nil, true, eh.HandleError(ctx, state, step, err)
		}
		return nil, false, nil
	})

	// If any error handler itself failed, wrap both.
	if herr := firstError(handlerErrs); herr != nil {
		return fmt.Errorf("%s: %w (error handler also failed: %v)", step, err, herr)
	}

	return fmt.Errorf("%s: %w", step, err)
}

// recordAnchor writes a lifecycle boundary anchor to the tape.
func (t *turnExecutor) recordAnchor(ctx context.Context, label string, state *hook.TurnState) {
	if t.tapes == nil {
		return
	}
	meta := tape.EntryMeta{
		SessionID: state.SessionID,
		RunID:     state.RunID,
	}
	_ = t.tapes.Append(ctx, tape.NewAnchor(label, meta))
}

// recordError writes a step error to the tape.
func (t *turnExecutor) recordError(ctx context.Context, step string, err error, state *hook.TurnState) {
	if t.tapes == nil {
		return
	}
	meta := tape.EntryMeta{
		SessionID: state.SessionID,
		RunID:     state.RunID,
	}
	_ = t.tapes.Append(ctx, tape.NewError(err.Error(), step, meta))
}

// firstError returns the first non-nil error from a slice, or nil.
func firstError(errs []error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

