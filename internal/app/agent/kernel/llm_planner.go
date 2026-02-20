package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	core "alex/internal/domain/agent/ports"
	kerneldomain "alex/internal/domain/kernel"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/shared/logging"
)

// LLMPlannerConfig controls the LLM-driven planner behavior.
type LLMPlannerConfig struct {
	Provider      string
	Model         string
	APIKey        string
	BaseURL       string
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
	goalContent := p.readGoalFile()
	planningPrompt := p.buildPlanningPrompt(stateContent, goalContent, recentByAgent)

	client, err := p.factory.GetClient(p.config.Provider, p.config.Model, portsllm.LLMConfig{
		APIKey:  p.config.APIKey,
		BaseURL: p.config.BaseURL,
	})
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
		MaxTokens:   2000,
	})
	if err != nil {
		return nil, fmt.Errorf("llm planner: complete: %w", err)
	}

	decisions, err := parsePlanningDecisions(resp.Content)
	if err != nil {
		p.logger.Warn("LLMPlanner: parse failed (%v); returning empty", err)
		return nil, nil
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

	b.WriteString("## 当前时间\n")
	b.WriteString(time.Now().Format(time.RFC3339))
	b.WriteString("\n\n")

	if goalContent != "" {
		b.WriteString("## GOAL.md（目标与机会）\n")
		b.WriteString(goalContent)
		b.WriteString("\n\n")
	}

	b.WriteString("## STATE.md（当前状态）\n")
	b.WriteString(stateContent)
	b.WriteString("\n\n")

	b.WriteString("## 最近 dispatch 记录\n")
	if len(recentByAgent) == 0 {
		b.WriteString("(无历史记录)\n")
	} else {
		b.WriteString("| agent_id | status | 时间 | 摘要 |\n")
		b.WriteString("|----------|--------|------|------|\n")
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
		b.WriteString("## 已配置的 agent（可直接派发或创建新 agent_id）\n")
		for _, a := range p.staticAgents {
			status := "可用"
			if !a.Enabled {
				status = "禁用"
			}
			b.WriteString(fmt.Sprintf("- `%s` (priority=%d, %s)\n", a.AgentID, a.Priority, status))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("## 本轮最多派发: %d 个 agent\n", p.config.MaxDispatches))

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
			prompt = fmt.Sprintf("执行任务: %s\n\n当前状态:\n%s", d.Reason, stateContent)
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
		p.logger.Debug("HybridPlanner: LLMPlanner returned empty plan; falling back to static")
		return p.static.Plan(ctx, stateContent, recentByAgent)
	}
	return llmSpecs, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Planning System Prompt
// ─────────────────────────────────────────────────────────────────────────────

const llmPlannerSystemPrompt = `你是 elephant.ai kernel 的任务调度器。你的职责是：基于 GOAL.md + STATE.md + 历史记录，决定本轮应该派发哪些 agent 任务。

## 核心原则
- **行动优于调研**：如果 GOAL 中有具体可执行的机会，优先派发执行任务（搭建、申请、发布），不是继续研究。
- **新任务立即开始**：看到高价值机会 → 立即派发相应 agent 开始做。
- **任务类型无限制**：可以派发任何类型的任务：
  - 搭建网站（shell_exec + write_file 创建项目结构和代码）
  - 申请 API（browser 工具访问注册页面）
  - 发帖/发消息（send_message / shell_exec curl）
  - 发邮件（mail_manage）
  - 写代码/开发功能（shell_exec + write_file）
  - 联系客户/外部沟通（send_message）
  - 数据分析/报告（web_search + write_file）
  - 部署/运维（shell_exec）
- **不要重复已完成的工作**：如果历史记录显示某任务最近已成功完成且无新需求，跳过。
- **每轮必须有真实产出**：每个被派发的 agent 必须执行至少一个真实工具动作。

## Agent ID 命名规则
- 可以使用已配置的 agent_id（见"已配置的 agent"列表）
- 也可以创建全新的 ad-hoc agent_id
- 命名惯例：` + "`{action}-{target}`" + `，如：
  - ` + "`website-builder`" + ` — 搭建网站
  - ` + "`api-applicant`" + ` — 申请各种 API
  - ` + "`lark-poster`" + ` — 在 Lark 群发消息/帖子
  - ` + "`client-outreach`" + ` — 联系潜在客户
  - ` + "`mvp-developer`" + ` — 开发 MVP 产品
  - ` + "`content-writer`" + ` — 撰写内容/文案
  - ` + "`deploy-operator`" + ` — 部署和运维

## 输出格式（严格 JSON，不要任何其他文字）

` + "```json" + `
[
  {
    "agent_id": "string — agent 标识符",
    "dispatch": true,
    "priority": 8,
    "prompt": "详细的任务指令...",
    "reason": "一句话说明为什么派发这个任务"
  }
]
` + "```" + `

## Prompt 编写规范
为每个 agent 生成的 prompt 必须包含：
1. **明确任务目标**（做什么、为什么做）
2. **可用工具提示**（browser / shell_exec / write_file / web_search / send_message 等）
3. **输出路径**（写入 ~/.alex/kernel/default/ 下对应目录）
4. **完成标准**（什么算成功完成）
5. **行动指令**：「不要询问、不要解释、直接开始做。」

## 调度规则
1. 正在运行的 agent（status=running）绝对不要重复派发
2. priority >= 8：紧急/高价值任务，必须立即派发
3. 最近 30 分钟内成功完成且无新需求的 agent 可以跳过
4. GOAL 中有"立即可执行"的机会 → priority >= 8
5. STATE 中有明确的"下一步" → 直接派发相应任务

## 空调度
如果当前无需派发任何任务（全部已完成、正在运行、或无可执行目标），输出空数组：
` + "```json" + `
[]
` + "```"
