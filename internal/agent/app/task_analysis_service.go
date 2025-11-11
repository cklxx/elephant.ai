package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

// TaskAnalysis contains the structured result of task pre-analysis.
type TaskAnalysis struct {
	ActionName  string
	Goal        string
	Approach    string
	RawAnalysis string
}

// TaskAnalysisService performs lightweight LLM analysis prior to execution.
type TaskAnalysisService struct {
	logger  ports.Logger
	timeout time.Duration
}

// NewTaskAnalysisService constructs a task analysis service with sensible defaults.
func NewTaskAnalysisService(logger ports.Logger) *TaskAnalysisService {
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	return &TaskAnalysisService{
		logger:  logger,
		timeout: 5 * time.Second,
	}
}

// Analyze runs a short LLM prompt to classify the requested task.
func (s *TaskAnalysisService) Analyze(ctx context.Context, task string, llmClient ports.LLMClient) *TaskAnalysis {
	if llmClient == nil {
		return nil
	}

	s.logger.Debug("Starting task pre-analysis")

	prompt := fmt.Sprintf(`Analyze this task and provide a concise structured response:

Task: %s

Respond in this exact format:
Action: [single verb phrase, e.g., "Analyzing codebase", "Implementing feature", "Debugging issue"]
Goal: [what needs to be achieved]
Approach: [brief strategy]

Keep each line under 80 characters. Be specific and actionable.`, task)

	requestID := id.NewRequestID()

	req := ports.CompletionRequest{
		Messages: []ports.Message{{
			Role:    "user",
			Content: prompt,
			Source:  ports.MessageSourceSystemPrompt,
		}},
		Temperature: 0.2,
		MaxTokens:   150,
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}

	analyzeCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	resp, err := llmClient.Complete(analyzeCtx, req)
	if err != nil {
		s.logger.Warn("Task pre-analysis failed (request_id=%s): %v", requestID, err)
		return nil
	}

	if resp == nil || resp.Content == "" {
		s.logger.Warn("Task pre-analysis returned empty response (request_id=%s)", requestID)
		return nil
	}

	s.logger.Debug("Task pre-analysis LLM response (request_id=%s): %s", requestID, resp.Content)
	analysis := parseTaskAnalysis(resp.Content)
	s.logger.Debug("Task pre-analysis completed (request_id=%s): action=%s, goal=%s", requestID, analysis.ActionName, analysis.Goal)
	return analysis
}

func fallbackTaskAnalysis(task string) *TaskAnalysis {
	trimmed := strings.TrimSpace(task)
	if trimmed == "" {
		return nil
	}

	goal := trimmed
	if len(goal) > 160 {
		goal = goal[:160] + "..."
	}

	action := inferActionFromTask(trimmed)
	approach := "Outline a plan and execute the request step by step."

	return &TaskAnalysis{
		ActionName:  action,
		Goal:        goal,
		Approach:    approach,
		RawAnalysis: fmt.Sprintf("Action: %s\nGoal: %s\nApproach: %s", action, goal, approach),
	}
}

func inferActionFromTask(task string) string {
	sentence := task
	if idx := strings.IndexAny(sentence, ".!?"); idx >= 0 {
		sentence = sentence[:idx]
	}
	sentence = strings.TrimSpace(sentence)
	if sentence == "" {
		return "Processing request"
	}
	if len(sentence) > 60 {
		sentence = sentence[:60] + "..."
	}
	if strings.HasPrefix(strings.ToLower(sentence), "please") {
		sentence = strings.TrimSpace(sentence[6:])
	}
	if sentence == "" {
		return "Processing request"
	}
	return fmt.Sprintf("Working on %s", strings.ToLower(sentence))
}

func parseTaskAnalysis(content string) *TaskAnalysis {
	analysis := &TaskAnalysis{RawAnalysis: content}
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Action:"):
			analysis.ActionName = strings.TrimSpace(strings.TrimPrefix(line, "Action:"))
		case strings.HasPrefix(line, "Goal:"):
			analysis.Goal = strings.TrimSpace(strings.TrimPrefix(line, "Goal:"))
		case strings.HasPrefix(line, "Approach:"):
			analysis.Approach = strings.TrimSpace(strings.TrimPrefix(line, "Approach:"))
		}
	}

	if analysis.ActionName == "" {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && len(line) < 80 {
				analysis.ActionName = line
				break
			}
		}
		if analysis.ActionName == "" {
			analysis.ActionName = "Processing request"
		}
	}

	return analysis
}
