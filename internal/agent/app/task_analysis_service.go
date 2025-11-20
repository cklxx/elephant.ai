package app

import (
	"context"
	"encoding/xml"
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
	Steps       []ports.TaskAnalysisStep
	Criteria    []string
	Retrieval   ports.TaskRetrievalPlan
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

	prompt := fmt.Sprintf(`You are a planning agent focused on "Info check + defaults + actionable template". Produce a concise, immediately usable plan that surfaces missing inputs, applied defaults, and the next executable steps.

Task:
"""%s"""

Return ONLY well-formed XML (no prose) following this schema:
<task_analysis>
  <action>string</action>                      <!-- concise verb phrase summarizing the plan -->
  <goal>string</goal>                          <!-- concrete outcome, including assumed defaults -->
  <approach>string</approach>                  <!-- short strategy that calls out missing info + defaults -->
  <success_criteria>
    <criterion>string</criterion>              <!-- 2-5 checks, including resolving key gaps -->
  </success_criteria>
  <task_breakdown>
    <step requires_external_research="bool" requires_retrieval="bool" requires_discovery="bool">
      <description>string</description>        <!-- high-level step with clear owner action -->
      <reason>string</reason>                  <!-- why this step matters (e.g., fill defaults, confirm inputs) -->
    </step>
  </task_breakdown>
  <retrieval_plan should_retrieve="bool">
    <local_queries><query>string</query></local_queries>      <!-- internal doc/code queries for context -->
    <search_queries><query>string</query></search_queries>    <!-- web search queries if needed -->
    <crawl_urls><url>string</url></crawl_urls>                <!-- specific URLs to fetch -->
    <knowledge_gaps><gap>string</gap></knowledge_gaps>        <!-- missing facts or decisions to confirm -->
    <notes>string</notes>                                     <!-- defaults/assumptions applied -->
  </retrieval_plan>
</task_analysis>

Guidelines:
- Lead with what information is missing and the defaults you will apply; make those explicit in <approach> and <notes>.
- Keep lists small (<=4 items) and omit duplicates.
- If retrieval is unnecessary, set should_retrieve="false" and leave lists empty.
- Use clear, executable language in the same language as the task.
- Always include at least one step that checks/collects inputs before execution and one step that outputs the actionable template/plan.
`, task)

	requestID := id.NewRequestID()

	req := ports.CompletionRequest{
		Messages: []ports.Message{{
			Role:    "user",
			Content: prompt,
			Source:  ports.MessageSourceSystemPrompt,
		}},
		Temperature: 0.15,
		MaxTokens:   450,
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

	goal := truncateRunes(trimmed, 160)

	action := inferActionFromTask(trimmed)
	approach := "Gather required details, fill gaps with explicit defaults, then provide the final, ready-to-use output without extra analysis."

	return &TaskAnalysis{
		ActionName:  action,
		Goal:        goal,
		Approach:    approach,
		RawAnalysis: fmt.Sprintf("Action: %s\nGoal: %s\nApproach: %s", action, goal, approach),
		Criteria: []string{
			"Call out assumptions or defaults applied to missing inputs",
			"Provide a concise, directly usable response",
			"Avoid superfluous analysis in the final output",
		},
		Steps: []ports.TaskAnalysisStep{
			{
				Description:          "Identify missing inputs (requirements, constraints) and propose safe defaults",
				NeedsExternalContext: false,
				Rationale:            "Align the response with user intent while making assumptions explicit",
			},
			{
				Description:          "Produce the final output or template directly, summarizing assumptions instead of extra analysis",
				NeedsExternalContext: false,
				Rationale:            "Keep the answer actionable and focused on deliverables",
			},
		},
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
	if strings.HasPrefix(strings.ToLower(sentence), "please") {
		sentence = strings.TrimSpace(sentence[6:])
	}
	if sentence == "" {
		return "Processing request"
	}
	sentence = truncateRunes(sentence, 60)
	return sentence
}

func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	return string(runes[:limit]) + "..."
}

func parseTaskAnalysis(content string) *TaskAnalysis {
	if structured := parseStructuredTaskAnalysis(content); structured != nil {
		structured.RawAnalysis = content
		return structured
	}

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

type llmTaskAnalysis struct {
	XMLName   xml.Name              `xml:"task_analysis"`
	Action    string                `xml:"action"`
	Goal      string                `xml:"goal"`
	Approach  string                `xml:"approach"`
	Success   []string              `xml:"success_criteria>criterion"`
	Steps     []llmTaskAnalysisStep `xml:"task_breakdown>step"`
	Retrieval llmRetrievalPlan      `xml:"retrieval_plan"`
}

type llmTaskAnalysisStep struct {
	Description          string `xml:"description"`
	Reason               string `xml:"reason"`
	RequiresExternal     bool   `xml:"requires_external_research,attr"`
	RequiresRetrieval    bool   `xml:"requires_retrieval,attr"`
	RequiresDiscovery    bool   `xml:"requires_discovery,attr"`
	NeedsExternalContext bool   `xml:"needs_external_context,attr"`
}

type llmRetrievalPlan struct {
	ShouldRetrieve bool     `xml:"should_retrieve,attr"`
	LocalQueries   []string `xml:"local_queries>query"`
	SearchQueries  []string `xml:"search_queries>query"`
	CrawlURLs      []string `xml:"crawl_urls>url"`
	KnowledgeGaps  []string `xml:"knowledge_gaps>gap"`
	Notes          string   `xml:"notes"`
}

func parseStructuredTaskAnalysis(content string) *TaskAnalysis {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	fragment := extractTaskAnalysisFragment(trimmed)
	if fragment == "" {
		return nil
	}

	decoder := xml.NewDecoder(strings.NewReader(fragment))
	decoder.Strict = false

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "task_analysis" {
			continue
		}

		var payload llmTaskAnalysis
		if err := decoder.DecodeElement(&payload, &start); err != nil {
			return nil
		}

		analysis := &TaskAnalysis{
			ActionName: strings.TrimSpace(payload.Action),
			Goal:       strings.TrimSpace(payload.Goal),
			Approach:   strings.TrimSpace(payload.Approach),
		}
		analysis.Criteria = normalizeList(payload.Success)

		if len(payload.Steps) > 0 {
			steps := make([]ports.TaskAnalysisStep, 0, len(payload.Steps))
			for _, step := range payload.Steps {
				desc := strings.TrimSpace(step.Description)
				if desc == "" {
					continue
				}
				needsExternal := step.RequiresExternal || step.RequiresRetrieval || step.RequiresDiscovery || step.NeedsExternalContext
				steps = append(steps, ports.TaskAnalysisStep{
					Description:          desc,
					NeedsExternalContext: needsExternal,
					Rationale:            strings.TrimSpace(step.Reason),
				})
			}
			if len(steps) > 0 {
				analysis.Steps = steps
			}
		}

		retrieval := payload.Retrieval
		analysis.Retrieval = ports.TaskRetrievalPlan{
			ShouldRetrieve: retrieval.ShouldRetrieve,
			LocalQueries:   normalizeList(retrieval.LocalQueries),
			SearchQueries:  normalizeList(retrieval.SearchQueries),
			CrawlURLs:      normalizeList(retrieval.CrawlURLs),
			KnowledgeGaps:  normalizeList(retrieval.KnowledgeGaps),
			Notes:          strings.TrimSpace(retrieval.Notes),
		}
		if !analysis.Retrieval.ShouldRetrieve {
			analysis.Retrieval.ShouldRetrieve = len(analysis.Retrieval.LocalQueries) > 0 || len(analysis.Retrieval.SearchQueries) > 0 || len(analysis.Retrieval.CrawlURLs) > 0
		}

		return analysis
	}
}

func extractTaskAnalysisFragment(content string) string {
	lower := strings.ToLower(content)
	start := strings.Index(lower, "<task_analysis")
	if start < 0 {
		return ""
	}

	fragment := content[start:]
	lowerFragment := lower[start:]

	endStart := strings.Index(lowerFragment, "</task_analysis")
	if endStart < 0 {
		return ""
	}

	closing := fragment[endStart:]
	gt := strings.Index(closing, ">")
	if gt < 0 {
		return ""
	}

	end := start + endStart + gt + 1
	return strings.TrimSpace(content[start:end])
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
