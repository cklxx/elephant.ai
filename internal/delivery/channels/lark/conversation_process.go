package lark

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

const (
	defaultConversationTimeout = 8 * time.Second
	conversationMaxTok = 10240
	conversationTemp           = 0.3

	respondToolName        = "respond"
	stopWorkerToolName     = "stop_worker"

	// Legacy tool name kept for reference during migration.
	dispatchWorkerToolName = "dispatch_worker"

	// Brain response mode constants.
	modeDirect   = "direct"
	modeThink    = "think"
	modeDelegate = "delegate"
	modeStream   = "stream"

	conversationChatHistoryMaxRounds = 5
	conversationPromptCacheTTL       = 60 * time.Second
)

// memoryCacheEntry holds a cached memory context string with an expiry.
type memoryCacheEntry struct {
	context   string
	expiresAt time.Time
}

// respondModes enumerates valid brain response modes.
var respondModes = map[string]bool{
	modeDirect:   true,
	modeThink:    true,
	modeDelegate: true,
	modeStream:   true, // placeholder — falls back to direct
}

// buildRespondTool constructs the unified respond tool definition.
// The brain uses this single tool to choose a response mode and generate a reply.
func (g *Gateway) buildRespondTool() ports.ToolDefinition {
	desc := "Choose how to respond to the user. Modes:\n" +
		"- direct: you know the answer → reply immediately\n" +
		"- think: you need to look something up or reason (5-15s) → give a quick take, then a deeper follow-up\n" +
		"- delegate: this needs real work (coding, research, multi-step) → describe what you'll do, then launch a worker\n" +
		"- stream: (reserved, behaves like direct for now)"
	if g.cfg.ConversationWorkerCapabilities != "" {
		desc += "\n\nWorker capabilities:\n" + g.cfg.ConversationWorkerCapabilities
	}
	return ports.ToolDefinition{
		Name:        respondToolName,
		Description: desc,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"mode": {
					Type:        "string",
					Description: "Response mode: direct | think | delegate | stream",
					Enum:        []any{modeDirect, modeThink, modeDelegate, modeStream},
				},
				"reply": {
					Type:        "string",
					Description: "Your reply to the user. In direct mode this is the final answer. In think mode this is a quick take. In delegate mode this describes what you'll do.",
				},
				"task": {
					Type:        "string",
					Description: "Task description for the background worker (required for delegate mode, ignored otherwise). Preserve user's original intent. When a skill matches, include 'skill:<name>'.",
				},
			},
			Required: []string{"mode", "reply"},
		},
	}
}

var stopWorkerTool = ports.ToolDefinition{
	Name:        stopWorkerToolName,
	Description: "When the user wants to cancel or abort a running task → stop the specified worker. Empty task_id stops all running tasks; specific task_id (e.g. '#1') stops only that task.",
	Parameters: ports.ParameterSchema{
		Type: "object",
		Properties: map[string]ports.Property{
			"task_id": {
				Type:        "string",
				Description: "Task ID to stop (e.g. '#1'). Empty string stops all running tasks.",
			},
			"ack": {
				Type:        "string",
				Description: "One-sentence reply shown to the user describing the action. E.g. '正在停止任务 #1' / 'stopping all tasks'. Max 20 chars in Chinese, 30 chars in English. No period at end.",
			},
		},
		Required: []string{"ack"},
	},
}

// defaultConversationReplyRules is the fallback when Config.ConversationReplyRules is empty.
const defaultConversationReplyRules = `## Reply rules
Length adapts to content:
- Simple ack: 1 sentence (好，查一下)
- Status answer: 2-3 sentences with key data
- Opinion/pushback: as long as needed, stay concise
- Never pad. If 5 chars is enough, use 5 chars.

Voice:
- 中文口语化，同事语气
- English: direct, casual, no corporate speak
- 禁: 请/您/好的收到/可以的/其实/然后/的话
- Allowed: 我觉得/不太对/你确定吗/等我看看`

// conversationSystemPromptBase is the personality-first brain prompt.
// %REPLY_RULES% and %NARRATION% are replaced at runtime.
// %URGENCY% is replaced with urgency context when detected.
const conversationSystemPromptBase = `You are Alex, a sharp engineering colleague. You think before you speak, you have opinions, and you push back when something seems off.

## Response modes (use the respond tool)
- DIRECT: You know the answer. Say it. Be substantive, not telegraphic.
  "CI green，但 auth test 又 flaky 了，这周第三次了" not "CI 没问题"
- THINK: You need 5-10 seconds to look something up or reason through it. Give a quick take first, then I'll think deeper.
  "应该是 cost 模块的问题，等我查一下具体哪个 case fail 了"
- DELEGATE: This needs real work (coding, research, multi-step tasks). Say what you're going to do, not "收到处理中".
  "重构 cost 模块，我先看看现在的结构再动手" not "收到，处理中"

## Decision examples
- "你好" → respond(mode=direct, reply="你好")
- "重构 auth 模块" → respond(mode=delegate, reply="重构 auth，我先看看现在的结构", task="重构 auth 模块")
- "帮我看看为什么 CI 挂了" → respond(mode=delegate, reply="查 CI，看看是哪个 case 挂的", task="检查 CI 失败原因")
- "停" / "cancel" / "算了" → stop_worker
- "昨天改了什么" → respond(mode=think, reply="等我查一下 git log")
- "Go 的 goroutine 和 thread 有什么区别" → respond(mode=direct, reply="goroutine 是用户态轻量线程...")
- "用 PostgreSQL 不要 MySQL" → respond(mode=delegate, reply="好，换成 PostgreSQL", task="将数据库从 MySQL 迁移到 PostgreSQL")
- "👍" → respond(mode=direct, reply="👍")

## Decision rules
1. Stop/cancel intent → stop_worker
2. If the answer is in your context (worker status, chat history, memory) → DIRECT
3. If you need to fetch/compute something (git log, analysis) → THINK
4. Action request (coding, research, multi-step) → DELEGATE
5. Follow-up to running task → DELEGATE (inject)
6. Everything else → DIRECT

Cross-task: include "#N" in task description to reference task N's result.

%REPLY_RULES%
%URGENCY%
%NARRATION%
## Voice
- 中文口语化，像同事聊天，不像客服
- Have opinions. "我觉得这样更好" > "有几种方案"
- Push back when warranted. "这个方案 edge case 没覆盖" > "好的"
- Match the user's energy. Short question → short answer. Deep question → real analysis.

## Safety
- Never fabricate info or status.
- Never include secrets.`

// conversationNarrationVoice is injected into the system prompt only when
// query tools (query_tasks, query_usage, manage_notice) are present.
const conversationNarrationVoice = `## Narration voice (for query tool ack)
- Conclusion first, 2-5 sentences.
- **Bold** key data (counts, costs, durations).
- Drop technical fields (task_id, status codes, chat_id).
- Preserve actionable info (links, paths, errors).
- No Markdown headings, no emoji.

`

// resolveConversationSystemPrompt resolves placeholders in
// conversationSystemPromptBase. hasQueryTools controls whether narration
// voice instructions are included. urgencyCtx is optional urgency context.
func (g *Gateway) resolveConversationSystemPrompt(hasQueryTools bool, urgencyCtx ...string) string {
	replyRules := g.cfg.ConversationReplyRules
	if replyRules == "" {
		replyRules = defaultConversationReplyRules
	}
	narration := ""
	if hasQueryTools {
		narration = conversationNarrationVoice
	}
	urgency := ""
	if len(urgencyCtx) > 0 && urgencyCtx[0] != "" {
		urgency = urgencyCtx[0]
	}
	prompt := strings.Replace(conversationSystemPromptBase, "%REPLY_RULES%", replyRules, 1)
	prompt = strings.Replace(prompt, "%URGENCY%", urgency, 1)
	return strings.Replace(prompt, "%NARRATION%", narration, 1)
}

// imBaseReplacer covers formal→casual substitutions that are always safe to apply.
// Only includes replacements that cannot break mid-sentence meaning.
var imBaseReplacer = strings.NewReplacer(
	"请稍等一下", "等下",
	"请稍等", "等下",
	"您好", "你好",
	"您", "你",
	"非常感谢", "谢了",
	"非常抱歉", "抱歉",
	"不好意思", "抱歉",
	"好的，", "好，",
	"可以的", "行",
	"收到了", "收到",
	"没有问题", "没问题",
	"需要我帮", "要我帮",
	"非常", "很",
)

// imCasualReplacer covers additional aggressive substitutions used when
// the sender relationship is known to be informal (level >= 1).
var imCasualReplacer = strings.NewReplacer(
	"好的", "好",
	"是的，", "对，",
	"是的", "对",
	"知道了", "知道",
	"正在处理中", "处理中",
	"需要的话", "要的话",
	"请稍等", "稍等",
	"等一下", "等下",
	"没问题的", "没问题",
	"当然可以", "行",
)

// detectFormalityLevel returns 0 (neutral) or 1 (casual) based on chat context
// and optional memory context from SOUL.md/USER.md.
//
// Priority: memory relationship keywords > chat type heuristic.
// Memory keywords (case-insensitive):
//   - neutral (0): "外部客户", "client", "external", "合作方", "partner"
//   - casual  (1): "同事", "colleague", "朋友", "friend", "teammate", "队友"
func detectFormalityLevel(chatType string, memoryCtx string) int {
	if memoryCtx != "" {
		lower := strings.ToLower(memoryCtx)
		for _, kw := range formalityNeutralKeywords {
			if strings.Contains(lower, kw) {
				return 0
			}
		}
		for _, kw := range formalityCasualKeywords {
			if strings.Contains(lower, kw) {
				return 1
			}
		}
	}
	if chatType == "p2p" {
		return 1
	}
	return 0
}

var (
	formalityNeutralKeywords = []string{"外部客户", "client", "external", "合作方", "partner"}
	formalityCasualKeywords  = []string{"同事", "colleague", "朋友", "friend", "teammate", "队友"}
)

// naturalizeReply post-processes LLM output to match IM casual register.
// level 0 = neutral (base rules only); level 1 = casual (base + aggressive).
// aggressiveCasual controls whether the aggressive casual replacer runs.
// Length is controlled via the system prompt, not code-level truncation.
func naturalizeReply(s string, level int, aggressiveCasual ...bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Remove sentence-ending Chinese full-stop — IM almost never uses it.
	s = strings.TrimSuffix(s, "。")
	// Remove surrounding quotes that LLM sometimes adds.
	s = strings.Trim(s, "\"")
	s = imBaseReplacer.Replace(s)
	// Aggressive casual replacer is gated — only apply when explicitly enabled.
	aggressive := len(aggressiveCasual) > 0 && aggressiveCasual[0]
	if level >= 1 && aggressive {
		s = imCasualReplacer.Replace(s)
	}
	return s
}


// sendReply naturalizes the reply and dispatches it as a single message.
func (g *Gateway) sendReply(ctx context.Context, chatID, replyToID, reply string, level int) {
	aggressive := g.cfg.AggressiveCasualRewrite != nil && *g.cfg.AggressiveCasualRewrite
	text := naturalizeReply(reply, level, aggressive)
	if text == "" {
		return
	}
	g.dispatchFormattedReply(ctx, chatID, replyToID, text)
}

// fallbackAck returns a short default ack when the LLM did not produce one.
func fallbackAck(lang string) string {
	if lang == "en" {
		return "on it"
	}
	return "收到，处理中"
}

// taskRefPattern matches cross-task references like #1, #2, #12.
var taskRefPattern = regexp.MustCompile(`#(\d+)`)

// resolveTaskReferences scans taskContent for #N references and prepends
// the referenced task results as context. Returns the enriched content.
func resolveTaskReferences(taskContent string, slotMap *chatSlotMap) string {
	matches := taskRefPattern.FindAllString(taskContent, -1)
	if len(matches) == 0 {
		return taskContent
	}
	seen := make(map[string]bool)
	var refs []string
	for _, ref := range matches {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		preview := slotMap.resultPreview(ref)
		if preview == "" {
			continue
		}
		refs = append(refs, fmt.Sprintf("[%s result]: %s", ref, preview))
	}
	if len(refs) == 0 {
		return taskContent
	}
	return strings.Join(refs, "\n") + "\n\n" + taskContent
}

// prefetchResult holds pre-fetched query data and tool list decisions.
type prefetchResult struct {
	extraContext  string                 // pre-fetched data to inject in user message
	tools         []ports.ToolDefinition // full tool list for this request
	maxTokens     int                    // dynamic token budget
	hasQueryTools bool                   // whether any query tools are included
}

// taskQueryKeywords triggers query_tasks tool inclusion + pre-fetch.
var taskQueryKeywords = []string{
	"tasks", "task", "status", "running", "进度", "任务",
	"/tasks", "/task",
}

// usageQueryKeywords triggers query_usage tool inclusion + pre-fetch.
var usageQueryKeywords = []string{
	"usage", "stats", "cost", "用量", "统计", "费用",
	"/usage", "/stats",
}

// noticeQueryKeywords triggers manage_notice tool inclusion.
var noticeQueryKeywords = []string{
	"notice", "通知", "/notice",
}

// prefetchQueryContext scans the user message for query intent keywords
// and pre-fetches relevant data concurrently. Returns the tool list
// and token budget to use for the LLM call.
func (g *Gateway) prefetchQueryContext(ctx context.Context, msg *incomingMessage) prefetchResult {
	lower := strings.ToLower(msg.content)

	hasTask := containsAnyKeyword(lower, taskQueryKeywords)
	hasUsage := containsAnyKeyword(lower, usageQueryKeywords)
	hasNotice := containsAnyKeyword(lower, noticeQueryKeywords)
	hasQuery := hasTask || hasUsage || hasNotice

	// Always include base tools.
	tools := []ports.ToolDefinition{g.buildRespondTool(), stopWorkerTool}

	if !hasQuery {
		return prefetchResult{
			tools:     tools,
			maxTokens: conversationMaxTok,
		}
	}

	// Include query tools.
	if hasTask {
		tools = append(tools, buildQueryTasksTool())
	}
	if hasUsage {
		tools = append(tools, buildQueryUsageTool())
	}
	if hasNotice {
		tools = append(tools, buildManageNoticeTool())
	}

	// Pre-fetch data concurrently. Count sources first so the channel
	// buffer matches the number of goroutines (avoids deadlock if extended).
	type result struct {
		label string
		data  string
	}
	pending := 0
	if hasTask && g.taskStore != nil {
		pending++
	}
	if hasUsage && g.costTracker != nil {
		pending++
	}
	ch := make(chan result, pending)

	if hasTask && g.taskStore != nil {
		go func() {
			tasks, err := g.taskStore.ListByChat(ctx, msg.chatID, true, 10)
			if err != nil || len(tasks) == 0 {
				ch <- result{"tasks", ""}
				return
			}
			ch <- result{"tasks", g.formatActiveTaskList(tasks)}
		}()
	}
	if hasUsage && g.costTracker != nil {
		go func() {
			now := g.currentTime()
			today, err := g.costTracker.GetDailyCost(ctx, now)
			if err == nil && today != nil && today.RequestCount > 0 {
				ch <- result{"usage", formatCostSummaryBlock(today)}
				return
			}
			ch <- result{"usage", ""}
		}()
	}

	var extraParts []string
collect:
	for range pending {
		select {
		case r := <-ch:
			if r.data != "" {
				extraParts = append(extraParts, fmt.Sprintf("[Pre-fetched %s data]\n%s", r.label, r.data))
			}
		case <-ctx.Done():
			break collect // use whatever was collected so far
		}
	}

	return prefetchResult{
		extraContext:  strings.Join(extraParts, "\n\n"),
		tools:         tools,
		maxTokens:     conversationMaxTok,
		hasQueryTools: true,
	}
}

// thinkModeTimeout is the extended timeout for think mode's secondary LLM call.
const thinkModeTimeout = 15 * time.Second

// handleViaConversationProcess sends the user message to the conversation brain.
// The brain chooses a response mode (direct/think/delegate) via the respond tool.
// Always returns true — fully owns the message lifecycle when enabled.
func (g *Gateway) handleViaConversationProcess(ctx context.Context, msg *incomingMessage) bool {
	lang := detectLang(msg.content)

	var memoryCtx string
	if g.conversationPromptLoader != nil {
		memoryCtx = g.loadCachedConversationPrompt(ctx, msg.senderID)
	}
	level := detectFormalityLevel(msg.chatType, memoryCtx)

	slotMap := g.getOrCreateSlotMap(msg.chatID)

	// Pre-fetch query context based on keyword detection.
	pf := g.prefetchQueryContext(ctx, msg)

	// Snapshot all active workers for LLM context.
	allWorkers := g.snapshotAllWorkers(msg.chatID, lang)

	// Fetch recent chat history for conversational context (12 messages).
	chatHistory := g.fetchConversationChatHistory(ctx, msg)

	// Detect urgency/energy level from the message.
	urgencyCtx := detectUrgency(msg.content, g.currentTime())

	// Kick off processing reaction concurrently while calling the Chat LLM.
	var processingReactionID string
	reactionDone := make(chan struct{})
	go func() {
		defer close(reactionDone)
		processingReactionID = g.addProcessingReaction(ctx, msg.messageID)
	}()

	reply, toolCalls, llmErr := g.conversationLLMDynamic(ctx, msg, allWorkers, chatHistory, pf, urgencyCtx)

	// Wait for reaction goroutine to finish so we have the ID for removal.
	<-reactionDone
	g.removeProcessingReaction(ctx, msg.messageID, processingReactionID)

	// Fallback: if LLM failed, dispatch a worker directly with a short ack.
	if llmErr != nil {
		g.logger.Warn("conversationLLM failed, falling back to direct dispatch: %v", llmErr)
		ack := fallbackAck(lang)
		g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), ack, level)
		g.spawnWorkerInSlotMap(ctx, msg, slotMap, msg.content)
		return true
	}

	logger := logging.FromContext(ctx, g.logger)

	// Fallback: LLM returned empty reply with no tool calls — treat as dispatch.
	if reply == "" && len(toolCalls) == 0 {
		logger.Warn("conversation: empty LLM response, falling back to dispatch for msg=%s", msg.messageID)
		ack := fallbackAck(lang)
		g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), ack, level)
		g.spawnWorkerInSlotMap(ctx, msg, slotMap, msg.content)
		return true
	}

	// When there are no tool calls, send the LLM text reply directly.
	if len(toolCalls) == 0 && reply != "" {
		g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply, level)
	}

	// replySent tracks whether we already replied for this message.
	replySent := false
	// trySendAck extracts ack from tool args and sends it if not already sent.
	trySendAck := func(tc ports.ToolCall) {
		if replySent {
			return
		}
		ack, _ := tc.Arguments["ack"].(string)
		if ack == "" {
			ack = reply
		}
		if ack != "" {
			g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), ack, level)
			replySent = true
		}
	}

	for _, tc := range toolCalls {
		// Record tool invocation in sliding context (skip trivial direct replies).
		if tc.Name != respondToolName || tc.Arguments["mode"] != modeDirect {
			g.recordToolContext(msg.chatID, tc.Name, tc.Arguments)
		}

		switch tc.Name {
		case respondToolName:
			mode, _ := tc.Arguments["mode"].(string)
			replyArg, _ := tc.Arguments["reply"].(string)
			taskArg, _ := tc.Arguments["task"].(string)

			// Validate mode — invalid mode falls back to direct.
			if !respondModes[mode] {
				logger.Warn("conversation: invalid mode=%q, falling back to direct for msg=%s", mode, msg.messageID)
				mode = modeDirect
			}

			// Log brain decision for mode analytics.
			g.logBrainDecision(msg, mode, lang, urgencyCtx)

			logger.Info("conversation: brain mode=%s msg=%s reply_len=%d", mode, msg.messageID, len(replyArg))

			switch mode {
			case modeDirect, modeStream:
				// Send the reply immediately. Stream mode falls back to direct.
				if replyArg != "" && !replySent {
					g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), replyArg, level)
					replySent = true
				}

			case modeThink:
				// Phase 1: send quick take immediately.
				if replyArg != "" && !replySent {
					g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), replyArg, level)
					replySent = true
				}
				// Phase 2: spawn goroutine for secondary LLM call.
				g.spawnThinkMode(ctx, msg, replyArg, pf, allWorkers, chatHistory, urgencyCtx, level)

			case modeDelegate:
				// Send informative ack, then spawn or inject worker.
				if replyArg != "" && !replySent {
					g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), replyArg, level)
					replySent = true
				}
				if taskArg == "" {
					taskArg = msg.content
				}
				taskID, injected := g.spawnOrInjectWorker(ctx, msg, slotMap, taskArg)
				if injected {
					logger.Info("conversation: INJECTED msg=%s into running worker", msg.messageID)
				} else {
					logger.Info("conversation: SPAWNED worker msg=%s task=%s taskID=%s", msg.messageID, utils.Truncate(taskArg, 60, "..."), taskID)
				}
			}

		case stopWorkerToolName:
			taskIDArg, _ := tc.Arguments["task_id"].(string)
			g.executeStopWorkerExtended(ctx, slotMap, taskIDArg)
			trySendAck(tc)
			g.cancelThinkMode(msg.chatID)

		case queryTasksToolName:
			result := g.executeQueryTasks(ctx, msg, tc.Arguments)
			logger.Info("conversation: query_tasks result_len=%d", len(result))
			trySendAck(tc)
		case queryUsageToolName:
			result := g.executeQueryUsage(ctx, msg, tc.Arguments)
			logger.Info("conversation: query_usage result_len=%d", len(result))
			trySendAck(tc)
		case manageNoticeToolName:
			result := g.executeManageNotice(msg, tc.Arguments)
			logger.Info("conversation: manage_notice result=%s", utils.Truncate(result, 60, "..."))
			trySendAck(tc)

		// Legacy: handle dispatch_worker tool calls from cached prompts.
		case dispatchWorkerToolName:
			taskArg, _ := tc.Arguments["task"].(string)
			if taskArg == "" {
				taskArg = msg.content
			}
			trySendAck(tc)
			g.spawnOrInjectWorker(ctx, msg, slotMap, taskArg)
		}
	}

	return true
}

// spawnThinkMode launches a goroutine for think mode's secondary LLM call.
// Sends an enriched reply as a follow-up message, prefixed with a reference
// to the original question for context.
func (g *Gateway) spawnThinkMode(ctx context.Context, msg *incomingMessage, quickTake string, pf prefetchResult, workers workerSnapshotList, chatHistory, urgencyCtx string, level int) {
	thinkCtx, thinkCancel := context.WithTimeout(context.WithoutCancel(ctx), thinkModeTimeout)

	// Store cancel func so stop_worker can cancel think mode.
	g.setThinkCancel(msg.chatID, thinkCancel)

	go func() {
		defer thinkCancel()
		defer g.clearThinkCancel(msg.chatID)

		// Build enriched prompt with the quick take as context.
		systemPrompt := g.buildConversationSystemPrompt(ctx, msg.senderID, pf.hasQueryTools, urgencyCtx)
		enrichedPrompt := systemPrompt + "\n\nYou already gave a quick take: \"" + quickTake + "\"\nNow provide a more detailed, substantive answer. Include specific data, analysis, or reasoning."

		var sb strings.Builder
		sb.WriteString("Worker status: ")
		sb.WriteString(workers.StatusSummary())
		if chatHistory != "" {
			sb.WriteString("\n\nRecent chat:\n")
			sb.WriteString(chatHistory)
		}
		if pf.extraContext != "" {
			sb.WriteString("\n\n")
			sb.WriteString(pf.extraContext)
		}
		sb.WriteString("\n\nUser message: ")
		sb.WriteString(strings.TrimSpace(msg.content))

		client, _, err := llmclient.GetClientFromProfile(g.llmFactory, g.llmProfile, nil, false)
		if err != nil {
			g.logger.Warn("think mode: client error: %v", err)
			g.sendThinkFallback(ctx, msg, level)
			return
		}

		resp, err := client.Complete(thinkCtx, ports.CompletionRequest{
			Messages: []ports.Message{
				{Role: "system", Content: enrichedPrompt},
				{Role: "user", Content: sb.String()},
			},
			Temperature: conversationTemp,
			MaxTokens:   2048,
		})
		if err != nil {
			g.logger.Warn("think mode: secondary LLM failed: %v", err)
			g.sendThinkFallback(ctx, msg, level)
			return
		}

		enriched := strings.TrimSpace(resp.Content)
		if enriched == "" {
			g.sendThinkFallback(ctx, msg, level)
			return
		}

		// Prepend quote reference to link back to original question.
		enriched = "关于「" + utils.Truncate(strings.TrimSpace(msg.content), 30, "...") + "」：" + enriched

		g.sendReply(ctx, msg.chatID, "", enriched, level)
	}()
}

// sendThinkFallback sends a brief error follow-up when think mode's secondary call fails.
func (g *Gateway) sendThinkFallback(ctx context.Context, msg *incomingMessage, level int) {
	lang := detectLang(msg.content)
	var fallback string
	if lang == "en" {
		fallback = "couldn't find more details, try asking again"
	} else {
		fallback = "没查到更多细节，你可以再问一次"
	}
	g.sendReply(ctx, msg.chatID, "", fallback, level)
}

// setThinkCancel stores a think mode cancel function for a chat.
// Cancels any previously active think goroutine for this chat first.
func (g *Gateway) setThinkCancel(chatID string, cancel context.CancelFunc) {
	if prev, ok := g.thinkCancels.Swap(chatID, cancel); ok {
		if prevCancel, ok := prev.(context.CancelFunc); ok {
			prevCancel()
		}
	}
}

// clearThinkCancel removes the think cancel for a chat.
func (g *Gateway) clearThinkCancel(chatID string) {
	g.thinkCancels.Delete(chatID)
}

// cancelThinkMode cancels any active think mode secondary call for a chat.
func (g *Gateway) cancelThinkMode(chatID string) {
	if v, ok := g.thinkCancels.LoadAndDelete(chatID); ok {
		if cancel, ok := v.(context.CancelFunc); ok {
			cancel()
		}
	}
}

// conversationLLMOpts configures the conversation LLM call.
type conversationLLMOpts struct {
	senderID       string
	userMsg        string
	workers        workerSnapshotList
	chatHistory    string
	tools          []ports.ToolDefinition
	maxTokens      int
	extraContext   string // pre-fetched data injected before user message
	chatID         string // when set, inject sliding context from chatConversationContext
	hasQueryTools  bool   // when true, inject narration voice section into system prompt
	urgencyCtx     string // urgency context to inject into system prompt
}

// conversationLLMCall is the unified conversation LLM entry point.
func (g *Gateway) conversationLLMCall(ctx context.Context, opts conversationLLMOpts) (string, []ports.ToolCall, error) {
	if g.llmFactory == nil {
		return "", nil, fmt.Errorf("no LLM factory configured")
	}

	client, _, err := llmclient.GetClientFromProfile(g.llmFactory, g.llmProfile, nil, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get LLM client: %w", err)
	}

	llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultConversationTimeout)
	defer cancel()

	systemPrompt := g.buildConversationSystemPrompt(ctx, opts.senderID, opts.hasQueryTools, opts.urgencyCtx)

	var sb strings.Builder
	sb.WriteString("Worker status: ")
	sb.WriteString(opts.workers.StatusSummary())

	// Inject sliding context when chatID is available.
	if opts.chatID != "" {
		chatCtx := g.getOrCreateChatContext(opts.chatID)
		if rendered := chatCtx.render(); rendered != "" {
			sb.WriteString("\n\n")
			sb.WriteString(rendered)
		}
	}

	if opts.chatHistory != "" {
		sb.WriteString("\n\nRecent chat:\n")
		sb.WriteString(opts.chatHistory)
	}

	if opts.extraContext != "" {
		sb.WriteString("\n\n")
		sb.WriteString(opts.extraContext)
	}

	sb.WriteString("\n\nUser message: ")
	sb.WriteString(strings.TrimSpace(opts.userMsg))

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: sb.String()},
		},
		Tools:       opts.tools,
		Temperature: conversationTemp,
		MaxTokens:   opts.maxTokens,
	})
	if err != nil {
		return "", nil, err
	}

	g.logger.Info("conversation LLM: reply=%q tool_calls=%d maxTok=%d", utils.Truncate(resp.Content, 80, "…"), len(resp.ToolCalls), opts.maxTokens)
	return strings.TrimSpace(resp.Content), resp.ToolCalls, nil
}

// conversationLLMDynamic calls the conversation LLM with dynamic tools and token budget
// based on the prefetch result. Injects sliding context, pre-fetched data, and urgency.
func (g *Gateway) conversationLLMDynamic(ctx context.Context, msg *incomingMessage, workers workerSnapshotList, chatHistory string, pf prefetchResult, urgencyCtx string) (string, []ports.ToolCall, error) {
	return g.conversationLLMCall(ctx, conversationLLMOpts{
		senderID:      msg.senderID,
		userMsg:       msg.content,
		workers:       workers,
		chatHistory:   chatHistory,
		tools:         pf.tools,
		maxTokens:     pf.maxTokens,
		extraContext:  pf.extraContext,
		chatID:        msg.chatID,
		hasQueryTools: pf.hasQueryTools,
		urgencyCtx:    urgencyCtx,
	})
}

// conversationLLMWithList calls the gateway's shared LLM with multi-worker status.
// Returns (reply, toolCalls, error). An error means the LLM call itself failed.
func (g *Gateway) conversationLLMWithList(ctx context.Context, senderID, userMsg string, workers workerSnapshotList, chatHistory string) (string, []ports.ToolCall, error) {
	return g.conversationLLMCall(ctx, conversationLLMOpts{
		senderID:    senderID,
		userMsg:     userMsg,
		workers:     workers,
		chatHistory: chatHistory,
		tools:       []ports.ToolDefinition{g.buildRespondTool(), stopWorkerTool},
		maxTokens:   conversationMaxTok,
	})
}

// buildConversationSystemPrompt composes the conversation brain's system
// prompt by prepending memory context (SOUL.md, USER.md, long-term) and
// appending date/timezone when available. Uses a 60-second TTL cache per senderID.
// hasQueryTools controls whether narration voice instructions are included.
// urgencyCtx is optional urgency context injected into the prompt.
func (g *Gateway) buildConversationSystemPrompt(ctx context.Context, senderID string, hasQueryTools bool, urgencyCtx ...string) string {
	var sections []string

	if g.conversationPromptLoader != nil {
		memoryCtx := g.loadCachedConversationPrompt(ctx, senderID)
		if memoryCtx != "" {
			sections = append(sections, memoryCtx)
		}
	}

	sections = append(sections, strings.TrimSpace(g.resolveConversationSystemPrompt(hasQueryTools, urgencyCtx...)))

	now := g.currentTime()
	sections = append(sections, fmt.Sprintf("Current date: %s (%s)", now.Format("2006-01-02"), now.Location().String()))

	return strings.Join(sections, "\n\n")
}

// loadCachedConversationPrompt returns the memory context for senderID,
// loading it fresh at most once per 60-second window per sender.
func (g *Gateway) loadCachedConversationPrompt(ctx context.Context, senderID string) string {
	now := g.currentTime()
	if v, ok := g.conversationPromptCache.Load(senderID); ok {
		entry := v.(*memoryCacheEntry)
		if now.Before(entry.expiresAt) {
			return entry.context
		}
	}
	memoryCtx := g.conversationPromptLoader(ctx, senderID)
	g.conversationPromptCache.Store(senderID, &memoryCacheEntry{
		context:   memoryCtx,
		expiresAt: now.Add(conversationPromptCacheTTL),
	})
	return memoryCtx
}

// evictExpiredPromptCache removes stale entries from conversationPromptCache.
// Called by the state cleanup goroutine.
func (g *Gateway) evictExpiredPromptCache() {
	now := g.currentTime()
	g.conversationPromptCache.Range(func(k, v any) bool {
		entry := v.(*memoryCacheEntry)
		if now.After(entry.expiresAt) {
			g.conversationPromptCache.Delete(k)
		}
		return true
	})
}

// ---------------------------------------------------------------------------
// Urgency detection — heuristic-based context injection
// ---------------------------------------------------------------------------

// urgencyFrustrationMarkers are message patterns indicating user frustration.
// Avoid single-character punctuation (!, ！) — too broad, matches normal messages.
var urgencyFrustrationMarkers = []string{
	"怎么又", "又挂了", "又出问题", "又失败", "又报错",
	"wtf", "again", "still broken", "not working",
	"??", "？？", "!!!", "！！！",
}

// detectUrgency returns an urgency context string to inject into the brain prompt.
// Returns empty string for neutral energy (no injection needed).
func detectUrgency(content string, now time.Time) string {
	lower := strings.ToLower(content)

	// Check frustration markers.
	frustrated := false
	for _, marker := range urgencyFrustrationMarkers {
		if strings.Contains(lower, marker) {
			frustrated = true
			break
		}
	}

	// Check late night (22:00 - 06:00).
	hour := now.Hour()
	lateNight := hour >= 22 || hour < 6

	if frustrated {
		return "[context: user seems frustrated — respond with empathy + immediate action]"
	}
	if lateNight {
		return "[context: late night — keep it brief, prioritize action over explanation]"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Mode analytics — structured brain decision logging
// ---------------------------------------------------------------------------

// logBrainDecision logs a structured event for each brain mode decision.
func (g *Gateway) logBrainDecision(msg *incomingMessage, mode, lang, urgencyCtx string) {
	g.logger.Info("brain_decision: mode=%s chat=%s sender=%s msg_len=%d lang=%s urgency=%q",
		mode, msg.chatID, msg.senderID, len([]rune(msg.content)), lang, urgencyCtx)
}

// getOrCreateSlotMap returns (or lazily creates) the chatSlotMap for a chat.
func (g *Gateway) getOrCreateSlotMap(chatID string) *chatSlotMap {
	v, _ := g.activeChatSlots.LoadOrStore(chatID, &chatSlotMap{slots: make(map[string]*sessionSlot)})
	return v.(*chatSlotMap)
}

// spawnWorkerInSlotMap allocates a new slot in slotMap and launches a worker.
// Returns the assigned task ID (e.g. "#1") and the number of active tasks
// (including this one). taskID is empty when at capacity.
func (g *Gateway) spawnWorkerInSlotMap(ctx context.Context, msg *incomingMessage, slotMap *chatSlotMap, taskContent string) (string, int) {
	max := g.cfg.MaxConcurrentWorkers
	if max <= 0 {
		max = 5
	}
	slot, taskID, activeCount, ok := slotMap.allocateSlotIfCapacity(max, g.currentTime())
	if !ok {
		lang := detectLang(msg.content)
		var notice string
		if lang == "en" {
			notice = fmt.Sprintf("All %d worker slots are busy. Please wait for a task to finish before starting a new one.", max)
		} else {
			notice = fmt.Sprintf("当前已有 %d 个任务在运行，请等待任务完成后再启动新任务。", max)
		}
		g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), notice)
		return "", 0
	}

	// Resolve cross-task references (#1, #2, …) in the task content.
	enrichedContent := resolveTaskReferences(taskContent, slotMap)

	sessionID, isResume := g.resolveSessionForNewTask(ctx, msg.chatID, slot)
	inputCh := make(chan agent.UserInput, 16)
	taskCtx, taskCancel := context.WithCancel(context.Background())

	slot.mu.Lock()
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.taskCancel = taskCancel
	slot.taskToken++
	taskToken := slot.taskToken
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.taskDesc = strings.TrimSpace(taskContent) // store original desc (without context injection)
	slot.recentProgress = slot.recentProgress[:0]
	slot.lastTouched = g.currentTime()
	slot.taskStartTime = g.currentTime()
	slot.lastProgressAt = g.currentTime()
	slot.mu.Unlock()

	workerMsg := *msg
	workerMsg.content = enrichedContent

	g.launchWorkerGoroutineForSlotMap(&workerMsg, slot, slotMap, sessionID, inputCh, taskCancel, taskCtx, taskToken, isResume)
	return taskID, activeCount
}

// spawnOrInjectWorker tries to inject into the most recently active worker in
// the slotMap; if no running worker exists, spawns a new one.
// Returns (taskID, injected). When injected=true, taskID is the existing task's ID.
func (g *Gateway) spawnOrInjectWorker(ctx context.Context, msg *incomingMessage, slotMap *chatSlotMap, taskContent string) (string, bool) {
	// Look for a running slot to inject into.
	var injected bool
	var injectedTaskID string
	slotMap.forEachSlot(func(taskID string, s *sessionSlot) {
		if injected {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.phase == slotRunning && s.inputCh != nil {
			select {
			case s.inputCh <- agent.UserInput{Content: taskContent, SenderID: msg.senderID, MessageID: msg.messageID}:
				injected = true
				injectedTaskID = taskID
			default:
				lang := detectLang(msg.content)
				if lang == "en" {
					g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), "Message received, will be processed after current task.")
				} else {
					g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), "消息已收到，等待当前任务处理完毕后执行")
				}
			}
		}
	})
	if injected {
		return injectedTaskID, true
	}

	taskID, _ := g.spawnWorkerInSlotMap(ctx, msg, slotMap, taskContent)
	return taskID, false
}

// launchWorkerGoroutineForSlotMap is like launchWorkerGoroutine but cleans up
// the slot from the chatSlotMap rather than from activeSlots.
func (g *Gateway) launchWorkerGoroutineForSlotMap(msg *incomingMessage, slot *sessionSlot, slotMap *chatSlotMap, sessionID string, inputCh chan agent.UserInput, taskCancel context.CancelFunc, taskCtx context.Context, taskToken uint64, isResume bool) {
	g.taskWG.Add(1)
	go func(taskCtx context.Context, taskCancel context.CancelFunc, taskToken uint64) {
		defer g.taskWG.Done()
		defer taskCancel()

		awaitingInput, answerPreview := g.runTask(taskCtx, msg, sessionID, inputCh, isResume, taskToken)

		slot.mu.Lock()
		if slot.taskToken == taskToken {
			if slot.intentionalCancelToken == taskToken {
				slot.intentionalCancelToken = 0
			}
			slot.inputCh = nil
			slot.taskCancel = nil
			slot.taskStartTime = time.Time{}
			if answerPreview != "" {
				slot.lastResultPreview = answerPreview
			}
			if awaitingInput {
				slot.phase = slotAwaitingInput
				slot.lastSessionID = slot.sessionID
			} else {
				slot.phase = slotIdle
				slot.sessionID = ""
			}
			slot.lastTouched = g.currentTime()
		}
		slot.mu.Unlock()

		if awaitingInput {
			g.drainAndReprocess(inputCh, msg.chatID, msg.chatType, taskToken)
		} else {
			g.discardPendingInputs(inputCh, msg.chatID)
		}
	}(taskCtx, taskCancel, taskToken)
}

// fetchConversationChatHistory retrieves recent chat rounds for the
// conversation process LLM context. Returns empty string on failure.
// Fetches 12 messages (reduced from 50 for token efficiency).
func (g *Gateway) fetchConversationChatHistory(ctx context.Context, msg *incomingMessage) string {
	if g.messenger == nil {
		return ""
	}
	history, err := g.fetchRecentChatRounds(ctx, msg.chatID, msg.messageID, 12, conversationChatHistoryMaxRounds)
	if err != nil {
		g.logger.Warn("conversation: chat history fetch failed: %v", err)
		return ""
	}
	return history
}

// conversationProcessEnabled reports whether the conversation process is on.
func (g *Gateway) conversationProcessEnabled() bool {
	if g.cfg.ConversationProcessEnabled == nil {
		return false
	}
	return *g.cfg.ConversationProcessEnabled
}
