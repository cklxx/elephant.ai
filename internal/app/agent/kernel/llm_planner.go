package kernel

import (
	"alex/internal/shared/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	core "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/domain/agent/taskfile"
	kerneldomain "alex/internal/domain/kernel"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// LLMPlannerConfig controls the LLM-driven planner behavior.
type LLMPlannerConfig struct {
	Profile              runtimeconfig.LLMProfile
	// ProfileFunc, when set, is called on every Plan() invocation and overrides
	// the static Profile field. Use this to resolve the LLM profile dynamically
	// (e.g. from a subscription store that may change at runtime) rather than
	// relying on the value captured at process startup.
	ProfileFunc          func() runtimeconfig.LLMProfile
	Refresher            llmclient.CredentialRefresher // optional; refreshes credentials for long-running processes
	MaxDispatches        int
	GoalFilePath         string
	Timeout              time.Duration
	TeamDispatchEnabled  bool
	MaxTeamsPerCycle     int
	TeamTimeoutSeconds   int
	AllowedTeamTemplates []string
	// StateDir is the kernel-specific state directory (e.g. ~/.alex/kernel/{kernel_id}).
	// When set, team status sidecars are read from StateDir/tasks/ instead of .elephant/tasks/.
	StateDir             string
}

// planningDecision is the unit of LLM planning output.
type planningDecision struct {
	AgentID        string            `json:"agent_id"`
	Dispatch       bool              `json:"dispatch"`
	Priority       int               `json:"priority"`
	Prompt         string            `json:"prompt"`
	Reason         string            `json:"reason"`
	Kind           string            `json:"kind"`
	TeamTemplate   string            `json:"team_template"`
	TeamGoal       string            `json:"team_goal"`
	TeamPrompts    map[string]string `json:"team_prompts"`
	TimeoutSeconds int               `json:"timeout_seconds"`
}

// LLMPlanner uses a small LLM to dynamically decide what to dispatch.
// It reads STATE.md, optionally GOAL.md, and recent dispatch history,
// then calls an LLM to produce a structured dispatch plan.
type LLMPlanner struct {
	kernelID             string
	factory              portsllm.LLMClientFactory
	config               LLMPlannerConfig
	staticAgents         []AgentConfig
	configuredAgentSet   map[string]struct{}
	agentCooldownMinutes map[string]int
	allowedTemplateSet   map[string]struct{}
	defaultBucketAgentID string
	logger               logging.Logger
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
	if config.MaxTeamsPerCycle <= 0 {
		config.MaxTeamsPerCycle = DefaultKernelMaxTeamsPerCycle
	}
	if config.TeamTimeoutSeconds <= 0 {
		config.TeamTimeoutSeconds = DefaultKernelTeamTimeoutSeconds
	}
	templates := uniqueTrimmed(config.AllowedTeamTemplates)
	config.AllowedTeamTemplates = templates
	allowedTemplateSet := make(map[string]struct{}, len(templates))
	for _, template := range templates {
		allowedTemplateSet[strings.ToLower(template)] = struct{}{}
	}
	configuredAgentSet := make(map[string]struct{}, len(staticAgents))
	agentCooldownMinutes := make(map[string]int, len(staticAgents))
	defaultBucketAgentID := ""
	for _, agentCfg := range staticAgents {
		agentID := strings.TrimSpace(agentCfg.AgentID)
		if agentID == "" {
			continue
		}
		lowerAgentID := strings.ToLower(agentID)
		configuredAgentSet[lowerAgentID] = struct{}{}
		if agentCfg.CooldownMinutes > 0 {
			agentCooldownMinutes[lowerAgentID] = agentCfg.CooldownMinutes
		}
		if defaultBucketAgentID == "" && strings.HasSuffix(agentID, "-executor") {
			defaultBucketAgentID = agentID
		}
	}
	return &LLMPlanner{
		kernelID:             kernelID,
		factory:              factory,
		config:               config,
		staticAgents:         staticAgents,
		configuredAgentSet:   configuredAgentSet,
		agentCooldownMinutes: agentCooldownMinutes,
		allowedTemplateSet:   allowedTemplateSet,
		defaultBucketAgentID: defaultBucketAgentID,
		logger:               logging.OrNop(logger),
	}
}

// Plan calls the LLM to decide which agents to dispatch and with what prompts.
func (p *LLMPlanner) Plan(ctx context.Context, stateContent string, recentByAgent map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
	// Resolve the LLM profile dynamically if a ProfileFunc is provided; otherwise
	// fall back to the static Profile captured at construction time.
	baseProfile := p.config.Profile
	if p.config.ProfileFunc != nil {
		baseProfile = p.config.ProfileFunc()
	}
	profile := runtimeconfig.LLMProfile{
		Provider:       strings.TrimSpace(baseProfile.Provider),
		Model:          strings.TrimSpace(baseProfile.Model),
		APIKey:         strings.TrimSpace(baseProfile.APIKey),
		BaseURL:        strings.TrimSpace(baseProfile.BaseURL),
		Headers:        llmclient.CloneHeaders(baseProfile.Headers),
		TimeoutSeconds: baseProfile.TimeoutSeconds,
	}
	p.logger.Info("LLMPlanner: starting plan (provider=%s model=%s timeout=%s)", profile.Provider, profile.Model, p.config.Timeout)

	goalContent, goalContextStatus := p.readGoalFile()
	planningPrompt := p.buildPlanningPrompt(stateContent, goalContent, goalContextStatus, recentByAgent)

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

func (p *LLMPlanner) readGoalFile() (content string, status string) {
	if p.config.GoalFilePath == "" {
		return "", "goal_context_not_configured"
	}
	path := p.config.GoalFilePath
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = home + path[1:]
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		p.logger.Warn("LLMPlanner: failed to read GOAL file at %s: %v", path, err)
		return "", "goal_context_unreadable"
	}
	content = strings.TrimSpace(string(data))
	if content == "" {
		p.logger.Warn("LLMPlanner: GOAL file at %s is empty", path)
		return "", "goal_context_empty"
	}
	// Cap at 3000 chars to keep planning prompt concise.
	if len([]rune(content)) > 3000 {
		content = string([]rune(content)[:3000]) + "\n...(truncated)"
		status = "goal_context_loaded_truncated"
	} else {
		status = "goal_context_loaded"
	}
	p.logger.Info("LLMPlanner: goal context status=%s chars=%d", status, len([]rune(content)))
	return content, status
}

func (p *LLMPlanner) buildPlanningPrompt(stateContent, goalContent, goalContextStatus string, recentByAgent map[string]kerneldomain.Dispatch) string {
	var b strings.Builder

	b.WriteString("## Current Time\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")

	if strings.TrimSpace(goalContextStatus) == "" {
		goalContextStatus = "goal_context_unknown"
	}
	b.WriteString("## Goal Context Status\n")
	b.WriteString(goalContextStatus)
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
			// For team dispatches, annotate with role-level results.
			if d.Kind == kerneldomain.DispatchKindTeam && d.Team != nil {
				if roleAnnotation := p.summarizeTeamRoles(d.Team.Template); roleAnnotation != "" {
					summary += " " + roleAnnotation
				}
			}
			summary = compactSummary(summary, 100)
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", agentID, d.Status, age, summary))
		}
	}
	b.WriteString("\n")

	// List enabled agents as reference (disabled agents can never be dispatched).
	if len(p.staticAgents) > 0 {
		b.WriteString("## Configured Agents (reuse these agent_ids; avoid creating ad-hoc ids)\n")
		for _, a := range p.staticAgents {
			if !a.Enabled {
				continue
			}
			b.WriteString(fmt.Sprintf("- `%s` (priority=%d)\n", a.AgentID, a.Priority))
		}
		b.WriteString("\n")
	}

	if p.config.TeamDispatchEnabled {
		b.WriteString("## Team Dispatch Constraints\n")
		b.WriteString(fmt.Sprintf("- Max team dispatches this cycle: %d\n", p.config.MaxTeamsPerCycle))
		b.WriteString(fmt.Sprintf("- Team timeout seconds: %d\n", p.config.TeamTimeoutSeconds))
		if len(p.config.AllowedTeamTemplates) == 0 {
			b.WriteString("- Allowed team templates: (none)\n")
			b.WriteString("- Do NOT emit kind=team decisions when no templates are available.\n\n")
		} else {
			b.WriteString("- Allowed team templates:\n")
			for _, template := range p.config.AllowedTeamTemplates {
				b.WriteString(fmt.Sprintf("  - %s\n", template))
			}
			b.WriteString("- Team decisions MUST use one of the allowed templates exactly.\n\n")
		}
	}

	b.WriteString(fmt.Sprintf("## Max dispatches this cycle: %d agents\n", p.config.MaxDispatches))

	return b.String()
}

func (p *LLMPlanner) toDispatchSpecs(decisions []planningDecision, stateContent string, recentByAgent map[string]kerneldomain.Dispatch) []kerneldomain.DispatchSpec {
	compactState := compactStateForDispatch(stateContent)
	var specs []kerneldomain.DispatchSpec
	teamDispatches := 0
	seenAgentIDs := make(map[string]struct{}, len(decisions))
	for _, d := range decisions {
		if !d.Dispatch {
			continue
		}
		if len(specs) >= p.config.MaxDispatches {
			break
		}
		kind := normalizePlanningDecisionKind(d)
		if kind == kerneldomain.DispatchKindTeam {
			if !p.config.TeamDispatchEnabled {
				continue
			}
			if teamDispatches >= p.config.MaxTeamsPerCycle {
				continue
			}
			template := strings.TrimSpace(d.TeamTemplate)
			if !p.isAllowedTeamTemplate(template) {
				continue
			}
			teamGoal := strings.TrimSpace(d.TeamGoal)
			if teamGoal == "" {
				teamGoal = strings.TrimSpace(d.Reason)
			}
			if teamGoal == "" {
				continue
			}
			agentID := "team:" + template
			agentKey := utils.TrimLower(agentID)
			if _, exists := seenAgentIDs[agentKey]; exists {
				p.logger.Debug("LLMPlanner: skipping duplicate decision for %s in same cycle", agentID)
				continue
			}
			if p.shouldSkipInFlightDispatch(agentID, recentByAgent) {
				continue
			}
			priority := d.Priority
			if priority <= 0 {
				priority = 8
			}
			timeoutSeconds := d.TimeoutSeconds
			if timeoutSeconds <= 0 {
				timeoutSeconds = p.config.TeamTimeoutSeconds
			}
			teamSpec := kerneldomain.TeamDispatchSpec{
				Template:       template,
				Goal:           teamGoal,
				Prompts:        copyStringMap(d.TeamPrompts),
				TimeoutSeconds: timeoutSeconds,
				Wait:           true,
			}
			specs = append(specs, kerneldomain.DispatchSpec{
				AgentID:  agentID,
				Prompt:   buildKernelTeamDispatchPrompt(teamSpec),
				Priority: priority,
				Kind:     kerneldomain.DispatchKindTeam,
				Team:     &teamSpec,
				Metadata: map[string]string{
					"planner":       "llm",
					"dispatch_kind": string(kerneldomain.DispatchKindTeam),
					"team_template": template,
					"reason":        compactSummary(d.Reason, 100),
				},
			})
			seenAgentIDs[agentKey] = struct{}{}
			teamDispatches++
			continue
		}
		agentID := strings.TrimSpace(d.AgentID)
		agentID = p.normalizeAgentID(agentID, d.Reason)
		if agentID == "" {
			continue
		}
		agentKey := utils.TrimLower(agentID)
		if _, exists := seenAgentIDs[agentKey]; exists {
			p.logger.Debug("LLMPlanner: skipping duplicate decision for %s in same cycle", agentID)
			continue
		}
		if p.shouldSkipInFlightDispatch(agentID, recentByAgent) {
			continue
		}
		if p.shouldSkipAgentCooldown(agentID, recentByAgent) {
			continue
		}
		prompt := strings.TrimSpace(d.Prompt)
		if prompt == "" {
			// Try to use a static agent's prompt template.
			for _, a := range p.staticAgents {
				if a.AgentID == agentID {
					prompt = strings.ReplaceAll(a.Prompt, "{STATE}", compactState)
					break
				}
			}
		}
		if prompt == "" {
			prompt = fmt.Sprintf("Execute task: %s\n\nCurrent state:\n%s", d.Reason, compactState)
		}
		// Always inject STATE into dynamic prompts (compact version for dispatched agents).
		prompt = strings.ReplaceAll(prompt, "{STATE}", compactState)
		if reject, reason := shouldRejectAutonomyViolatingPrompt(prompt); reject {
			p.logger.Debug("LLMPlanner: rejecting %s by autonomy gate (%s)", agentID, reason)
			continue
		}

		priority := d.Priority
		if priority <= 0 {
			priority = 5
		}
		specs = append(specs, kerneldomain.DispatchSpec{
			AgentID:  agentID,
			Prompt:   prompt,
			Priority: priority,
			Kind:     kerneldomain.DispatchKindAgent,
			Metadata: map[string]string{
				"planner":       "llm",
				"dispatch_kind": string(kerneldomain.DispatchKindAgent),
				"reason":        compactSummary(d.Reason, 100),
			},
		})
		seenAgentIDs[agentKey] = struct{}{}
	}
	return specs
}

// compactStateForDispatch strips the runtime block and truncates recent_actions
// so dispatched agents get a lighter STATE context. The planner still sees full STATE.
func compactStateForDispatch(stateContent string) string {
	// Remove runtime block (only planner needs cycle stats).
	if start := strings.Index(stateContent, kernelRuntimeSectionStart); start >= 0 {
		if end := strings.Index(stateContent, kernelRuntimeSectionEnd); end > start {
			stateContent = stateContent[:start] + stateContent[end+len(kernelRuntimeSectionEnd):]
		}
	}
	stateContent = truncateRecentActions(stateContent, 3)
	return strings.TrimSpace(stateContent)
}

// truncateRecentActions keeps only the last N entries in a "## recent_actions" section.
// Entries are lines starting with "- " under the section header.
func truncateRecentActions(content string, maxEntries int) string {
	const header = "## recent_actions"
	idx := strings.Index(strings.ToLower(content), header)
	if idx < 0 {
		return content
	}
	sectionStart := idx + len(header)
	// Find the end of this section (next ## header or EOF).
	rest := content[sectionStart:]
	sectionEnd := sectionStart + len(rest)
	if nextHeader := strings.Index(rest, "\n## "); nextHeader >= 0 {
		sectionEnd = sectionStart + nextHeader
	}

	sectionBody := content[sectionStart:sectionEnd]
	lines := strings.Split(sectionBody, "\n")
	var kept []string
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			count++
			if count > maxEntries {
				continue
			}
		}
		kept = append(kept, line)
	}
	return content[:sectionStart] + strings.Join(kept, "\n") + content[sectionEnd:]
}

func shouldRejectAutonomyViolatingPrompt(prompt string) (bool, string) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return true, "empty_prompt"
	}
	lower := strings.ToLower(trimmed)
	if containsAny(lower, "ask_user(", "needs_user_input", `action="clarify"`, `action="request"`,
		"request_user", "awaiting user confirmation", "ask the user", "wait for user", "wait for confirmation") {
		return true, "requires_user_confirmation"
	}
	if containsAny(lower, "without tool action", "no tool action", "do not use tools", "analysis only") {
		return true, "no_concrete_tool_action"
	}
	return false, ""
}

func (p *LLMPlanner) shouldSkipInFlightDispatch(agentID string, recentByAgent map[string]kerneldomain.Dispatch) bool {
	recent, ok := recentDispatchByAgentID(recentByAgent, agentID)
	if !ok {
		return false
	}
	if recent.Status == kerneldomain.DispatchRunning || recent.Status == kerneldomain.DispatchPending {
		p.logger.Debug("LLMPlanner: skipping %s (%s dispatch already in flight)", agentID, recent.Status)
		return true
	}
	return false
}

func (p *LLMPlanner) shouldSkipAgentCooldown(agentID string, recentByAgent map[string]kerneldomain.Dispatch) bool {
	recent, ok := recentDispatchByAgentID(recentByAgent, agentID)
	if !ok || recent.Status != kerneldomain.DispatchDone || recent.UpdatedAt.IsZero() {
		return false
	}
	cooldownMinutes, hasCooldown := p.agentCooldownMinutes[utils.TrimLower(agentID)]
	if !hasCooldown || cooldownMinutes <= 0 {
		return false
	}
	cooldown := time.Duration(cooldownMinutes) * time.Minute
	since := time.Since(recent.UpdatedAt)
	if since < 0 || since >= cooldown {
		return false
	}
	p.logger.Debug("LLMPlanner: skipping %s (cooldown active, remaining=%s)", agentID, (cooldown - since).Round(time.Second))
	return true
}

func (p *LLMPlanner) normalizeAgentID(agentID, reason string) string {
	trimmed := strings.TrimSpace(agentID)
	if trimmed != "" {
		if _, ok := p.configuredAgentSet[strings.ToLower(trimmed)]; ok {
			return trimmed
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "team:") {
			return trimmed
		}
	}
	if p.defaultBucketAgentID != "" {
		mapped := p.selectBucketAgent(reason)
		if mapped != "" {
			if trimmed != "" {
				p.logger.Debug("LLMPlanner: remapped unknown agent_id %q to %q", trimmed, mapped)
			} else {
				p.logger.Debug("LLMPlanner: assigned bucket agent_id %q for reason", mapped)
			}
			return mapped
		}
	}
	return trimmed
}

func (p *LLMPlanner) selectBucketAgent(reason string) string {
	bucket := classifyReasonBucket(reason)
	if bucket == "" {
		return p.defaultBucketAgentID
	}
	candidate := utils.TrimLower(bucket + "-executor")
	if _, ok := p.configuredAgentSet[candidate]; ok {
		return candidate
	}
	return p.defaultBucketAgentID
}

func classifyReasonBucket(reason string) string {
	text := utils.TrimLower(reason)
	if text == "" {
		return ""
	}
	if containsAny(text, "build", "implement", "fix", "code", "deploy", "release") {
		return "build"
	}
	if containsAny(text, "research", "investigate", "analyze", "benchmark", "compare") {
		return "research"
	}
	if containsAny(text, "outreach", "message", "email", "notify", "contact", "sync") {
		return "outreach"
	}
	if containsAny(text, "data", "state", "record", "snapshot", "log", "artifact", "file") {
		return "data"
	}
	if containsAny(text, "audit", "validate", "verify", "review", "check", "risk") {
		return "audit"
	}
	return ""
}

func containsAny(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

// summarizeTeamRoles reads the team status sidecar and returns a compact role summary.
// Returns empty string if the status file cannot be read.
func (p *LLMPlanner) summarizeTeamRoles(template string) string {
	var tasksDir string
	if p.config.StateDir != "" {
		tasksDir = filepath.Join(p.config.StateDir, "tasks")
	} else {
		tasksDir = filepath.Join(".elephant", "tasks")
	}
	statusPath := filepath.Join(tasksDir, "team-"+template+".status.yaml")
	sf, err := taskfile.ReadStatusFile(statusPath)
	if err != nil {
		return ""
	}
	total := len(sf.Tasks)
	if total == 0 {
		return ""
	}
	done, failed := 0, 0
	var failedRoles []string
	for _, ts := range sf.Tasks {
		switch ts.Status {
		case "completed":
			done++
		case "failed":
			failed++
			reason := ts.Error
			if reason == "" {
				reason = "error"
			}
			failedRoles = append(failedRoles, fmt.Sprintf("%s: %s", ts.ID, compactSummary(reason, 20)))
		}
	}
	annotation := fmt.Sprintf("roles: %d/%d done", done, total)
	if failed > 0 {
		annotation += fmt.Sprintf(", %d failed (%s)", failed, strings.Join(failedRoles, "; "))
	}
	return annotation
}

func (p *LLMPlanner) isAllowedTeamTemplate(template string) bool {
	trimmed := strings.TrimSpace(template)
	if trimmed == "" {
		return false
	}
	if len(p.allowedTemplateSet) == 0 {
		return false
	}
	_, ok := p.allowedTemplateSet[strings.ToLower(trimmed)]
	return ok
}

func normalizePlanningDecisionKind(d planningDecision) kerneldomain.DispatchKind {
	kind := utils.TrimLower(d.Kind)
	switch kind {
	case string(kerneldomain.DispatchKindTeam):
		return kerneldomain.DispatchKindTeam
	case "", string(kerneldomain.DispatchKindAgent):
		if strings.TrimSpace(d.TeamTemplate) != "" {
			return kerneldomain.DispatchKindTeam
		}
		return kerneldomain.DispatchKindAgent
	default:
		return kerneldomain.DispatchKindAgent
	}
}

func uniqueTrimmed(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func copyStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func recentDispatchByAgentID(recentByAgent map[string]kerneldomain.Dispatch, agentID string) (kerneldomain.Dispatch, bool) {
	if len(recentByAgent) == 0 {
		return kerneldomain.Dispatch{}, false
	}
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		return kerneldomain.Dispatch{}, false
	}
	if dispatch, ok := recentByAgent[trimmed]; ok {
		return dispatch, true
	}
	lower := strings.ToLower(trimmed)
	for key, dispatch := range recentByAgent {
		if utils.TrimLower(key) == lower {
			return dispatch, true
		}
	}
	return kerneldomain.Dispatch{}, false
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

const llmPlannerSystemPrompt = `You are the task scheduler for the elephant.ai kernel. Decide which agent tasks to dispatch this cycle using GOAL.md, STATE.md, and dispatch history.

Core principles:
- Prioritize execution tasks over repeated research when actionable work exists.
- Start high-value tasks immediately.
- Reuse configured agent IDs; do not invent ad-hoc IDs.
- Never redispatch agents that are running or pending.
- Skip recently successful work when no new requirement exists.
- Each dispatched task must produce at least one real tool action.

Agent ID policy:
- Prefer pre-configured agent_ids listed in the prompt context.
- If no exact fit exists, choose one closest bucket agent:
  - build-executor
  - research-executor
  - outreach-executor
  - data-executor
  - audit-executor

Output format:
Return strict JSON array only. No prose.
Each item fields:
- kind: "agent" or "team"
- agent_id: string
- dispatch: boolean
- priority: integer
- prompt: string
- reason: string
Optional for team:
- team_template
- team_goal
- team_prompts
- timeout_seconds

Scheduling rules:
1) Never re-dispatch status=running or status=pending.
2) priority >= 8 for urgent/high-value actionable work.
3) Respect explicit next steps from STATE.
4) For recent [awaiting_input] failures, redesign prompt to be fully autonomous.
5) Return [] when nothing should dispatch.
`
