package adapters

import (
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/coding"
)

func toExternalRequest(req coding.TaskRequest, progress func(agent.ExternalAgentProgress)) agent.ExternalAgentRequest {
	return agent.ExternalAgentRequest{
		TaskID:      req.TaskID,
		Prompt:      req.Prompt,
		AgentType:   req.AgentType,
		WorkingDir:  req.WorkingDir,
		Config:      req.Config,
		SessionID:   req.SessionID,
		CausationID: req.CausationID,
		OnProgress:  progress,
	}
}

func toTaskResult(taskID string, result *agent.ExternalAgentResult) *coding.TaskResult {
	if result == nil {
		return &coding.TaskResult{TaskID: taskID}
	}
	return &coding.TaskResult{
		TaskID:     taskID,
		Answer:     result.Answer,
		Iterations: result.Iterations,
		TokensUsed: result.TokensUsed,
		Error:      result.Error,
		Metadata:   result.Metadata,
	}
}

func wrapProgress(cb coding.ProgressCallback) func(agent.ExternalAgentProgress) {
	if cb == nil {
		return nil
	}
	return func(p agent.ExternalAgentProgress) {
		cb(coding.TaskProgress{
			Iteration:    p.Iteration,
			TokensUsed:   p.TokensUsed,
			CostUSD:      p.CostUSD,
			CurrentTool:  p.CurrentTool,
			CurrentArgs:  p.CurrentArgs,
			FilesTouched: p.FilesTouched,
			LastActivity: p.LastActivity,
		})
	}
}
