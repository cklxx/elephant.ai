package kernel

import (
	"alex/internal/shared/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	core "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	kerneldomain "alex/internal/domain/kernel"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// LLMPlannerConfig controls the LLM-driven planner behavior.
type LLMPlannerConfig struct {
	Profile       runtimeconfig.LLMProfile
	Refresher     llmclient.CredentialRefresher // optional; refreshes credentials for long-running processes
	MaxDispatches int
	GoalFilePath  string
	Timeout       time.Duration
}

// planningDecision is the unit of LLM planning output.
type planningDecision struct {
	AgentID  string `json:"agent_id"`
	Dispatch bool   `json:"dispatch"`
	Priority int    `json:"priority"`
	Prompt   string `json:"prompt"`
	Reason   string `json:"reason"`
}

// LLMPlanner uses a small LLM to dynamically decide what to dispatch.
// It reads STATE.md, optionally GOAL.md, and recent dispatch history,
// then calls an LLM to produce a structured dispatch plan.
type LLMPlanner struct {
	kernelID     string
	factory      portsllm.LLMClientFactory
	config       LLMPlannerConfig
	staticAgents []AgentConfig
	logger       logging.Logger
}

// NewLLMPlanner creates a new LLM-driven planner.
func NewLLMPlanner(
	kernelID string,
	factory portsllm.LLMClientFactory,
	config LLMPlannerConfig,
	staticAgents []AgentConfig,
	logger logging.Logger,
) *LLMPlanner {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxDispatches <= 0 {
		config.MaxDispatches = 5
	}
	return &LLMPlanner{
		kernelID:     kernelID,
		factory:      factory,
		config:       config,
		staticAgents: staticAgents,
		logger:       logging.OrNop(logger),
	}
}

// Plan calls the LLM to decide which agents to dispatch and with what prompts.
func (p *LLMPlanner) Plan(ctx context.Context, stateContent string, recentByAgent map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
	profile := runtimeconfig.LLMProfile{
		Provider:       strings.TrimSpace(p.config.Profile.Provider),
		Model:          strings.TrimSpace(p.config.Profile.Model),
		APIKey:         strings.TrimSpace(p.config.Profile.APIKey),
		BaseURL:        strings.TrimSpace(p.config.Profile.BaseURL),
		Headers:        llmclient.CloneHeaders(p.config.Profile.Headers),
		TimeoutSeconds: p.config.Profile.TimeoutSeconds,
	}
	p.logger.Info("LLMPlanner: starting plan (provider=%s model=%s timeout=%s)", profile.Provider, profile.Model, p.config.Timeout)

	goalContent := p.readGoalFile()
	planningPrompt := p.buildPlanningPrompt(stateContent, goalContent, recentByAgent)

	client, _, err := llmclient.GetClientFromProfile(p.factory, profile, p.config.Refresher, p.config.Refresher != nil)
	if err != nil {
		return nil, fmt.Errorf("llm planner: get client: %w", err)
	}

	planCtx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	resp, err := client.Complete(planCtx, core.CompletionRequest{
		Messages: []core.Message{
			{Role: "system", Content: llmPlannerSystemPrompt},
			{Role: "user", Content: planningPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	})
	if err != nil {
		return nil, fmt.Errorf("llm planner: complete: %w", err)
	}

	responseText := resp.Content
	// Some models (e.g. kimi-for-coding) put output in thinking/reasoning; fallback.
	if utils.IsBlank(responseText) && len(resp.Thinking.Parts) > 0 {
		for _, part := range resp.Thinking.Parts {
			if strings.Contains(part.Text, "[") {
				responseText = part.Text
				p.logger.Info("LLMPlanner: using thinking content as response (%d chars)", len(responseText))
				break
			}
		}
	}
	p.logger.Info("LLMPlanner: LLM response (%d chars, stop=%s): %.200s", len(responseText), resp.StopReason, responseText)

	decisions, err := parsePlanningDecisions(responseText)
	if err != nil {
		p.logger.Warn("LLMPlanner: parse failed (%v); raw response: %.500s", err, responseText)
		return nil, fmt.Errorf("llm planner: parse: %w", err)
	}

	specs := p.toDispatchSpecs(decisions, stateContent, recentByAgent)
	p.logger.Info("LLMPlanner: planned %d dispatch(es) from %d decision(s)", len(specs), len(decisions))
	return specs, nil
}

func (p *LLMPlanner) readGoalFile() string {
	if p.config.GoalFilePath == "" {
		return ""
	}
	path := p.config.GoalFilePath
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = home + path[1:]
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	// Cap at 3000 chars to keep planning prompt concise.
	if len([]rune(content)) > 3000 {
		content = string([]rune(content)[:3000]) + "\n...(truncated)"
	}
	return content
}

func (p *LLMPlanner) buildPlanningPrompt(stateContent, goalContent string, recentByAgent map[string]kerneldomain.Dispatch) string {
	var b strings.Builder

	b.WriteString("## Current Time\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")

	if goalContent != "" {
		b.WriteString("## GOAL.md (Objectives & Opportunities)\n")
		b.WriteString(goalContent)
		b.WriteString("\n\n")
	}

	b.WriteString("## STATE.md (Current State)\n")
	b.WriteString(stateContent)
	b.WriteString("\n\n")

	b.WriteString("## Recent Dispatch History\n")
	if len(recentByAgent) == 0 {
		b.WriteString("(no history)\n")
	} else {
		b.WriteString("| agent_id | status | time | summary |\n")
		b.WriteString("|----------|--------|------|---------|\n")
		for agentID, d := range recentByAgent {
			age := "(unknown)"
			if !d.UpdatedAt.IsZero() {
				dur := time.Since(d.UpdatedAt).Round(time.Minute)
				age = fmt.Sprintf("%v ago", dur)
			}
			summary := d.TaskID
			if d.Error != "" {
				summary = d.Error
			}
			summary = compactSummary(summary, 60)
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", agentID, d.Status, age, summary))
		}
	}
	b.WriteString("\n")

	// List statically configured agents as reference.
	if len(p.staticAgents) > 0 {
		b.WriteString("## Configured Agents (use directly or create new agent_id)\n")
		for _, a := range p.staticAgents {
			status := "available"
			if !a.Enabled {
				status = "disabled"
			}
			b.WriteString(fmt.Sprintf("- `%s` (priority=%d, %s)\n", a.AgentID, a.Priority, status))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("## Max dispatches this cycle: %d agents\n", p.config.MaxDispatches))

	return b.String()
}

func (p *LLMPlanner) toDispatchSpecs(decisions []planningDecision, stateContent string, recentByAgent map[string]kerneldomain.Dispatch) []kerneldomain.DispatchSpec {
	var specs []kerneldomain.DispatchSpec
	for _, d := range decisions {
		if !d.Dispatch {
			continue
		}
		if len(specs) >= p.config.MaxDispatches {
			break
		}
		agentID := strings.TrimSpace(d.AgentID)
		if agentID == "" {
			continue
		}
		// Skip agents that are still running.
		if recent, ok := recentByAgent[agentID]; ok && recent.Status == kerneldomain.DispatchRunning {
			p.logger.Debug("LLMPlanner: skipping %s (still running)", agentID)
			continue
		}
		prompt := strings.TrimSpace(d.Prompt)
		if prompt == "" {
			// Try to use a static agent's prompt template.
			for _, a := range p.staticAgents {
				if a.AgentID == agentID {
					prompt = strings.ReplaceAll(a.Prompt, "{STATE}", stateContent)
					break
				}
			}
		}
		if prompt == "" {
			prompt = fmt.Sprintf("Execute task: %s\n\nCurrent state:\n%s", d.Reason, stateContent)
		}
		// Always inject STATE into dynamic prompts.
		prompt = strings.ReplaceAll(prompt, "{STATE}", stateContent)

		priority := d.Priority
		if priority <= 0 {
			priority = 5
		}
		specs = append(specs, kerneldomain.DispatchSpec{
			AgentID:  agentID,
			Prompt:   prompt,
			Priority: priority,
			Metadata: map[string]string{
				"planner": "llm",
				"reason":  compactSummary(d.Reason, 100),
			},
		})
	}
	return specs
}

// parsePlanningDecisions extracts a JSON array of planningDecision from raw LLM output.
func parsePlanningDecisions(raw string) ([]planningDecision, error) {
	raw = strings.TrimSpace(raw)
	// Strip markdown code fences if present.
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		raw = raw[idx+7:]
	} else if idx := strings.Index(raw, "```"); idx >= 0 {
		raw = raw[idx+3:]
	}
	if idx := strings.LastIndex(raw, "```"); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)
	// Find the JSON array bounds.
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array found in planning response")
	}
	raw = raw[start : end+1]
	var decisions []planningDecision
	if err := json.Unmarshal([]byte(raw), &decisions); err != nil {
		return nil, fmt.Errorf("unmarshal planning decisions: %w", err)
	}
	return decisions, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// HybridPlanner
// ─────────────────────────────────────────────────────────────────────────────

// HybridPlanner combines StaticPlanner (fallback/baseline) with LLMPlanner (intelligence).
// LLMPlanner runs first; on error or empty result, StaticPlanner provides the baseline.
type HybridPlanner struct {
	static *StaticPlanner
	llm    *LLMPlanner
	logger logging.Logger
}

// NewHybridPlanner creates a planner that merges static and LLM-driven decisions.
func NewHybridPlanner(static *StaticPlanner, llm *LLMPlanner, logger logging.Logger) *HybridPlanner {
	return &HybridPlanner{static: static, llm: llm, logger: logging.OrNop(logger)}
}

// Plan executes LLMPlanner first, falls back to StaticPlanner if empty or error.
func (p *HybridPlanner) Plan(ctx context.Context, stateContent string, recentByAgent map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
	llmSpecs, err := p.llm.Plan(ctx, stateContent, recentByAgent)
	if err != nil {
		p.logger.Warn("HybridPlanner: LLMPlanner error (%v); falling back to static", err)
		return p.static.Plan(ctx, stateContent, recentByAgent)
	}
	if len(llmSpecs) == 0 {
		p.logger.Info("HybridPlanner: LLMPlanner returned empty plan; falling back to static")
		return p.static.Plan(ctx, stateContent, recentByAgent)
	}
	p.logger.Info("HybridPlanner: using LLM plan (%d specs)", len(llmSpecs))
	return llmSpecs, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Planning System Prompt
// ─────────────────────────────────────────────────────────────────────────────

const llmPlannerSystemPrompt = `You are the task scheduler for the elephant.ai kernel. Your role is to decide which agent tasks to dispatch this cycle, based on GOAL.md + STATE.md + dispatch history.

## Core Principles
- **Action over research**: If GOAL contains concrete actionable opportunities, prioritize execution tasks (build, apply, publish) over further investigation.
- **Start new tasks immediately**: When you see a high-value opportunity, dispatch the corresponding agent right away.
- **No task type restrictions**: You may dispatch any type of task:
  - Build websites (shell_exec + write_file to create project structure and code)
  - Apply for APIs (browser tool to visit registration pages)
  - Post messages (send_message / shell_exec curl)
  - Send emails (mail_manage)
  - Write code / develop features (shell_exec + write_file)
  - Contact clients / external communication (send_message)
  - Data analysis / reports (web_search + write_file)
  - Deployment / operations (shell_exec)
- **Do not repeat completed work**: If history shows a task was recently completed successfully with no new requirements, skip it.
- **Every cycle must produce real output**: Each dispatched agent must execute at least one real tool action.

## Agent ID Naming Convention
- You may use pre-configured agent_ids (see the "Configured Agents" section)
- You may also create entirely new ad-hoc agent_ids
- Naming convention: ` + "`{action}-{target}`" + `, e.g.:
  - ` + "`website-builder`" + ` — build websites
  - ` + "`api-applicant`" + ` — apply for various APIs
  - ` + "`lark-poster`" + ` — post messages in Lark groups
  - ` + "`client-outreach`" + ` — contact potential clients
  - ` + "`mvp-developer`" + ` — develop MVP products
  - ` + "`content-writer`" + ` — write content/copy
  - ` + "`deploy-operator`" + ` — deployment and operations

## Output Format (strict JSON only, no other text)

` + "```json" + `
[
  {
    "agent_id": "string — agent identifier",
    "dispatch": true,
    "priority": 8,
    "prompt": "detailed task instructions...",
    "reason": "one-sentence explanation of why this task is dispatched"
  }
]
` + "```" + `

## Prompt Writing Guidelines
Each agent's prompt must include:
1. **Clear task objective** (what to do, why)
2. **Available tools hint** (browser / shell_exec / write_file / web_search / send_message, etc.)
3. **Output path** (write under the current working directory using relative paths, e.g. ./artifacts/...)
4. **Completion criteria** (what counts as successful completion)
5. **Action directive**: "Do not ask questions, do not explain — start executing immediately."

## Scheduling Rules
1. Never re-dispatch an agent that is currently running (status=running)
2. priority >= 8: urgent / high-value tasks, must be dispatched immediately
3. Agents that completed successfully within the last 30 minutes with no new requirements may be skipped
4. Immediately actionable opportunities in GOAL → priority >= 8
5. Clear "next steps" in STATE → dispatch the corresponding task directly

## Empty Dispatch
If no tasks need to be dispatched (all completed, running, or no actionable goals), output an empty array:
` + "```json" + `
[]
` + "```"
