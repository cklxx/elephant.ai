package rl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ports "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
)

// LLMJudge implements the Judge interface using an LLM provider to score
// borderline RL trajectories. It sends the trajectory as structured context
// and asks the model to return a quality score on the same 0-100 scale.
type LLMJudge struct {
	client portsllm.LLMClient
}

// NewLLMJudge creates a Judge backed by the given LLM client.
func NewLLMJudge(client portsllm.LLMClient) *LLMJudge {
	return &LLMJudge{client: client}
}

// Score evaluates the trajectory quality and returns a score from 0-100.
func (j *LLMJudge) Score(ctx context.Context, traj *RLTrajectory) (float64, error) {
	prompt, err := buildJudgePrompt(traj)
	if err != nil {
		return 0, fmt.Errorf("build judge prompt: %w", err)
	}

	resp, err := j.client.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role:    "system",
				Content: judgeSystemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.0,
		MaxTokens:   512,
	})
	if err != nil {
		return 0, fmt.Errorf("llm completion: %w", err)
	}

	score, err := parseJudgeResponse(resp.Content)
	if err != nil {
		return 0, fmt.Errorf("parse judge response: %w", err)
	}

	return score, nil
}

const judgeSystemPrompt = `You are an RL trajectory quality judge. You evaluate agent task trajectories for training data quality.

Given a trajectory with steps (thought → action → observation), evaluate:
1. Step efficiency: Did the agent take a reasonable number of steps?
2. Tool usage quality: Were tool calls appropriate and well-formed?
3. Reasoning quality: Were the agent's thoughts logical and progressive?
4. Outcome quality: Did the agent achieve or make progress toward the goal?
5. Training signal: Would this trajectory provide useful learning signal for RL?

Respond with ONLY a JSON object:
{"score": <number 0-100>, "reasoning": "<brief explanation>"}`

func buildJudgePrompt(traj *RLTrajectory) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Evaluate this RL trajectory (auto_score=%.1f, grade=%s, outcome=%s):\n\n",
		traj.AutoScore, traj.Grade, traj.Metadata.Outcome))

	sb.WriteString(fmt.Sprintf("Task: %s\n", traj.TaskID))
	sb.WriteString(fmt.Sprintf("Total steps: %d\n", traj.Metadata.TotalSteps))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", traj.Metadata.Duration))
	sb.WriteString(fmt.Sprintf("Tokens used: %d\n", traj.Metadata.TokensUsed))

	if len(traj.Metadata.ToolsUsed) > 0 {
		sb.WriteString(fmt.Sprintf("Tools used: %s\n", strings.Join(traj.Metadata.ToolsUsed, ", ")))
	}

	sb.WriteString("\n--- Steps ---\n")

	// Limit to first 20 steps to stay within context limits
	maxSteps := len(traj.Steps)
	if maxSteps > 20 {
		maxSteps = 20
	}

	for _, step := range traj.Steps[:maxSteps] {
		sb.WriteString(fmt.Sprintf("\n[Step %d] (reward=%.2f)\n", step.StepIndex, step.Reward))
		if step.Thought != "" {
			sb.WriteString(fmt.Sprintf("  Thought: %s\n", truncate(step.Thought, 200)))
		}
		sb.WriteString(fmt.Sprintf("  Action: %s\n", truncate(step.Action, 200)))
		if step.Observation != "" {
			sb.WriteString(fmt.Sprintf("  Observation: %s\n", truncate(step.Observation, 200)))
		}
		if step.ToolCall != nil {
			sb.WriteString(fmt.Sprintf("  Tool: %s\n", step.ToolCall.Name))
		}
	}

	if len(traj.Steps) > 20 {
		sb.WriteString(fmt.Sprintf("\n... (%d more steps omitted)\n", len(traj.Steps)-20))
	}

	return sb.String(), nil
}

// judgeResponse is the expected JSON structure from the LLM judge.
type judgeResponse struct {
	Score     float64 `json:"score"`
	Reasoning string  `json:"reasoning"`
}

func parseJudgeResponse(content string) (float64, error) {
	content = strings.TrimSpace(content)

	// Try to extract JSON from potential markdown code blocks
	if idx := strings.Index(content, "{"); idx >= 0 {
		if end := strings.LastIndex(content, "}"); end > idx {
			content = content[idx : end+1]
		}
	}

	var resp judgeResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return 0, fmt.Errorf("invalid JSON in judge response: %w", err)
	}

	if resp.Score < 0 || resp.Score > 100 {
		return 0, fmt.Errorf("judge score %f out of range [0, 100]", resp.Score)
	}

	return resp.Score, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
