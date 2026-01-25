package tools

import "context"

// ToolProgressEmitter streams incremental tool output back to listeners.
// The boolean marks the final chunk when true.
type ToolProgressEmitter func(chunk string, isComplete bool)

type toolProgressEmitterKey struct{}

// WithToolProgressEmitter attaches a tool progress emitter to the context.
func WithToolProgressEmitter(ctx context.Context, emitter ToolProgressEmitter) context.Context {
	if ctx == nil || emitter == nil {
		return ctx
	}
	return context.WithValue(ctx, toolProgressEmitterKey{}, emitter)
}

// GetToolProgressEmitter retrieves a tool progress emitter from the context.
func GetToolProgressEmitter(ctx context.Context) ToolProgressEmitter {
	if ctx == nil {
		return nil
	}
	emitter, _ := ctx.Value(toolProgressEmitterKey{}).(ToolProgressEmitter)
	return emitter
}

// EmitToolProgress forwards a tool progress update when an emitter is available.
func EmitToolProgress(ctx context.Context, chunk string, isComplete bool) {
	if emitter := GetToolProgressEmitter(ctx); emitter != nil {
		emitter(chunk, isComplete)
	}
}
