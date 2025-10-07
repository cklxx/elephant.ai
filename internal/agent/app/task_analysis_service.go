package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
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

	req := ports.CompletionRequest{
		Messages:    []ports.Message{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   150,
	}

	analyzeCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	resp, err := llmClient.Complete(analyzeCtx, req)
	if err != nil {
		s.logger.Warn("Task pre-analysis failed: %v", err)
		return nil
	}

	if resp == nil || resp.Content == "" {
		s.logger.Warn("Task pre-analysis returned empty response")
		return nil
	}

	s.logger.Debug("Task pre-analysis LLM response: %s", resp.Content)
	analysis := parseTaskAnalysis(resp.Content)
	s.logger.Debug("Task pre-analysis completed: action=%s, goal=%s", analysis.ActionName, analysis.Goal)
	return analysis
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
