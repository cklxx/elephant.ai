package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

type TaskPlanner struct {
	logger ports.Logger
	clock  ports.Clock
}

func NewTaskPlanner(logger ports.Logger, clock ports.Clock) *TaskPlanner {
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &TaskPlanner{logger: logger, clock: clock}
}

type plannerResponse struct {
	Steps []string `json:"steps"`
}

func (p *TaskPlanner) Plan(ctx context.Context, llm ports.LLMClient, sessionID, taskID, parentTaskID, task string) ([]string, error) {
	if strings.TrimSpace(task) == "" {
		return []string{"总结"}, nil
	}
	if llm == nil {
		// Minimal fallback that still preserves the Planner + ReAct structure.
		return []string{"执行", "总结"}, nil
	}

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: plannerSystemPrompt},
			{Role: "user", Content: task},
		},
		Temperature: 0.2,
		MaxTokens:   300,
		TopP:        1.0,
		Metadata: map[string]any{
			"intent":     "task_planning",
			"session_id": sessionID,
			"task_id":    taskID,
			"ts":         p.clock.Now().Format(time.RFC3339Nano),
		},
	}

	resp, err := llm.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("planner llm call failed: %w", err)
	}

	steps, parseErr := parsePlannerSteps(resp.Content)
	if parseErr != nil {
		p.logger.Warn("planner parse failed, falling back to single summary step: %v", parseErr)
		return []string{"总结"}, nil
	}

	normalized := normalizePlannerSteps(steps)
	if len(normalized) == 0 {
		return []string{"总结"}, nil
	}

	// Ensure the plan always ends with a summary step so the UI can render the
	// Planner + ReAct contract deterministically.
	last := strings.TrimSpace(normalized[len(normalized)-1])
	if last == "" || (!strings.Contains(last, "总结") && !strings.Contains(strings.ToLower(last), "summary")) {
		normalized = append(normalized, "总结")
	}

	return normalized, nil
}

func normalizePlannerSteps(steps []string) []string {
	seen := make(map[string]bool, len(steps))
	out := make([]string, 0, len(steps))
	for _, raw := range steps {
		step := strings.TrimSpace(raw)
		step = strings.TrimPrefix(step, "-")
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		if len([]rune(step)) > 48 {
			r := []rune(step)
			step = strings.TrimSpace(string(r[:48]))
		}
		if seen[step] {
			continue
		}
		seen[step] = true
		out = append(out, step)
	}
	return out
}

func parsePlannerSteps(content string) ([]string, error) {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil, fmt.Errorf("empty planner content")
	}

	var parsed plannerResponse
	if err := json.Unmarshal([]byte(text), &parsed); err == nil && len(parsed.Steps) > 0 {
		return parsed.Steps, nil
	}

	// Attempt to recover JSON if the model added extra wrapper text.
	if obj := extractJSONObject(text); obj != "" {
		if err := json.Unmarshal([]byte(obj), &parsed); err == nil && len(parsed.Steps) > 0 {
			return parsed.Steps, nil
		}
	}

	// Fallback: accept newline-delimited steps.
	lines := strings.Split(text, "\n")
	var steps []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "0123456789. )\t-")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		steps = append(steps, line)
	}
	if len(steps) > 0 {
		return steps, nil
	}

	return nil, fmt.Errorf("unable to parse steps")
}

func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(text[start : end+1])
}

const plannerSystemPrompt = "" +
	"You are a planner. Output ONLY valid JSON with the shape {\"steps\":[\"...\"]}.\n" +
	"Constraints:\n" +
	"- The output must be JSON only. No markdown. No commentary.\n" +
	"- 3 to 7 steps.\n" +
	"- Each step is a short task title (<= 12 Chinese chars or <= 8 English words).\n" +
	"- The last step MUST be \"总结\".\n"

