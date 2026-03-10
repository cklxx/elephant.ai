package leader

import "time"

// HandoffContext carries structured information about why a session
// is being escalated to a human operator.
type HandoffContext struct {
	SessionID         string   `json:"session_id"`
	Member            string   `json:"member"`
	Goal              string   `json:"goal"`
	Reason            string   `json:"reason"`
	StallCount        int      `json:"stall_count"`
	Elapsed           string   `json:"elapsed"`
	RecommendedAction string   `json:"recommended_action"` // "provide_input", "retry", "abort"
	CreatedAt         time.Time `json:"created_at"`
}

// ToPayload converts the HandoffContext to a map suitable for hooks.Event.Payload.
func (c HandoffContext) ToPayload() map[string]any {
	return map[string]any{
		"reason":             c.Reason,
		"session_id":         c.SessionID,
		"member":             c.Member,
		"goal":               c.Goal,
		"stall_count":        c.StallCount,
		"elapsed":            c.Elapsed,
		"recommended_action": c.RecommendedAction,
		"created_at":         c.CreatedAt,
	}
}

// ParseHandoffContext extracts a HandoffContext from an event payload
// with safe type assertions and zero-value fallbacks.
func ParseHandoffContext(payload map[string]any) HandoffContext {
	ctx := HandoffContext{}
	if payload == nil {
		return ctx
	}
	if v, ok := payload["reason"].(string); ok {
		ctx.Reason = v
	}
	if v, ok := payload["session_id"].(string); ok {
		ctx.SessionID = v
	}
	if v, ok := payload["member"].(string); ok {
		ctx.Member = v
	}
	if v, ok := payload["goal"].(string); ok {
		ctx.Goal = v
	}
	if v, ok := payload["stall_count"].(int); ok {
		ctx.StallCount = v
	}
	if v, ok := payload["elapsed"].(string); ok {
		ctx.Elapsed = v
	}
	if v, ok := payload["recommended_action"].(string); ok {
		ctx.RecommendedAction = v
	}
	if v, ok := payload["created_at"].(time.Time); ok {
		ctx.CreatedAt = v
	}
	return ctx
}

// buildHandoffContext constructs a HandoffContext from runtime state.
func (a *Agent) buildHandoffContext(sessionID, reason string) HandoffContext {
	ctx := HandoffContext{
		SessionID: sessionID,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	snap, ok := a.rt.GetSession(sessionID)
	if ok {
		ctx.Member = string(snap.Member)
		ctx.Goal = snap.Goal
		if snap.StartedAt != nil {
			ctx.Elapsed = time.Since(*snap.StartedAt).Round(time.Second).String()
		}
	}
	a.stallCountsMu.Lock()
	ctx.StallCount = a.stallCounts[sessionID]
	a.stallCountsMu.Unlock()

	ctx.RecommendedAction = recommendAction(ctx)
	return ctx
}

// recommendAction suggests what the human operator should do.
func recommendAction(ctx HandoffContext) string {
	if ctx.StallCount >= maxStallAttempts {
		return "abort"
	}
	return "provide_input"
}
