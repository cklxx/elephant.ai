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

	dispatchWorkerToolName = "dispatch_worker"
	stopWorkerToolName     = "stop_worker"

	conversationChatHistoryMaxRounds = 5
	conversationPromptCacheTTL       = 60 * time.Second
)

// memoryCacheEntry holds a cached memory context string with an expiry.
type memoryCacheEntry struct {
	context   string
	expiresAt time.Time
}

// buildDispatchWorkerTool constructs the dispatch_worker tool definition,
// embedding the skills catalog in the tool description so the LLM can
// accurately match user requests to worker capabilities.
func (g *Gateway) buildDispatchWorkerTool() ports.ToolDefinition {
	desc := "When the user wants something done (coding, research, analysis, writing) → launch a background agent. Also use when injecting follow-up context into a running task. The agent notifies the user on completion. Returns task_id."
	if g.cfg.ConversationWorkerCapabilities != "" {
		desc += "\n\nScenario → skill mapping:\n" + g.cfg.ConversationWorkerCapabilities
	}
	return ports.ToolDefinition{
		Name:        dispatchWorkerToolName,
		Description: desc,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"task": {
					Type:        "string",
					Description: "Task description preserving the user's original intent. When a skill matches, include 'skill:<name>' in the task.",
				},
				"ack": {
					Type:        "string",
					Description: "One-sentence reply shown to the user describing what you're about to do. Be specific about the action, not generic. E.g. '正在检查 kaku panel 状态' / '搜索最新 agent 记忆论文' / 'checking kaku panel status'. Max 20 chars in Chinese, 30 chars in English. No period at end.",
				},
			},
			Required: []string{"task", "ack"},
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
const defaultConversationReplyRules = `## Reply rules (HARD CONSTRAINTS)
- 中文: ≤12字, 禁句号, 省略主语/我, 口语化
- English: ≤15 words, lowercase, fragments, no period
- NEVER use 其实/然后/的话/非常/请/您/好的/可以的
- One short sentence only. No explanations.`

// conversationSystemPromptBase is the core prompt without reply rules.
// Reply rules are injected from config (or defaults) at runtime.
// Narration voice is injected only when query tools are present (%NARRATION%).
const conversationSystemPromptBase = `You are an IM chatbot. Reply ultra-short or use tools.

## Decision examples
- "你好" → reply directly
- "重构 auth 模块" → dispatch_worker
- "帮我看看为什么 CI 挂了" → dispatch_worker
- "停" / "cancel that" / "算了不用了" / "停掉现在的" → stop_worker
- "用 PostgreSQL 不要 MySQL" → dispatch_worker (inject follow-up)
- "lint" → dispatch_worker

## Decision rules
1. Stop/cancel intent (停/取消/别做了/算了/cancel/stop/nevermind) → stop_worker
2. Action request (anything that takes time) → dispatch_worker
3. Follow-up to running task → dispatch_worker (inject)
4. Everything else → reply directly

Every tool call MUST include an "ack" parameter — the reply shown to the user.
Cross-task: include "#N" in task description to reference task N's result.

%REPLY_RULES%
%NARRATION%
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
// voice instructions are included.
func (g *Gateway) resolveConversationSystemPrompt(hasQueryTools bool) string {
	replyRules := g.cfg.ConversationReplyRules
	if replyRules == "" {
		replyRules = defaultConversationReplyRules
	}
	narration := ""
	if hasQueryTools {
		narration = conversationNarrationVoice
	}
	prompt := strings.Replace(conversationSystemPromptBase, "%REPLY_RULES%", replyRules, 1)
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
// Length is controlled via the system prompt, not code-level truncation.
func naturalizeReply(s string, level int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Remove sentence-ending Chinese full-stop — IM almost never uses it.
	s = strings.TrimSuffix(s, "。")
	// Remove surrounding quotes that LLM sometimes adds.
	s = strings.Trim(s, "\"")
	s = imBaseReplacer.Replace(s)
	if level >= 1 {
		s = imCasualReplacer.Replace(s)
	}
	return s
}


// sendReply naturalizes the reply and dispatches it as a single message.
func (g *Gateway) sendReply(ctx context.Context, chatID, replyToID, reply string, level int) {
	text := naturalizeReply(reply, level)
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
	tools := []ports.ToolDefinition{g.buildDispatchWorkerTool(), stopWorkerTool}

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

// handleViaConversationProcess sends the user message to the gateway's LLM
// with a single tool (dispatch_worker). The text reply goes to the user
// immediately; a dispatch_worker call spawns a background worker.
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

	// Kick off processing reaction concurrently while calling the Chat LLM.
	var processingReactionID string
	reactionDone := make(chan struct{})
	go func() {
		defer close(reactionDone)
		processingReactionID = g.addProcessingReaction(ctx, msg.messageID)
	}()

	reply, toolCalls, llmErr := g.conversationLLMDynamic(ctx, msg, allWorkers, chatHistory, pf)

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

	// Check if any tool call is dispatch_worker.
	hasDispatchWorker := false
	for _, tc := range toolCalls {
		if tc.Name == dispatchWorkerToolName {
			hasDispatchWorker = true
			break
		}
	}

	logger := logging.FromContext(ctx, g.logger)
	logger.Info("conversation: decision msg=%s hasDispatchWorker=%t reply_len=%d tool_calls=%d",
		msg.messageID, hasDispatchWorker, len(reply), len(toolCalls))

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

	// ackSent tracks whether we already replied for this message.
	ackSent := false
	// sendToolAck sends the ack from tool args; never falls back to hardcoded text.
	sendToolAck := func(tc ports.ToolCall) {
		if ackSent {
			return
		}
		ack, _ := tc.Arguments["ack"].(string)
		if ack == "" {
			ack = reply
		}
		if ack != "" {
			g.sendReply(ctx, msg.chatID, replyTarget(msg.messageID, true), ack, level)
			ackSent = true
		}
	}

	for _, tc := range toolCalls {
		// Record tool invocation in sliding context.
		g.recordToolContext(msg.chatID, tc.Name, tc.Arguments)

		switch tc.Name {
		case dispatchWorkerToolName:
			taskArg, _ := tc.Arguments["task"].(string)
			if taskArg == "" {
				taskArg = msg.content
			}
			taskID, injected := g.spawnOrInjectWorker(ctx, msg, slotMap, taskArg)
			if injected {
				logger.Info("conversation: INJECTED msg=%s into running worker", msg.messageID)
			} else {
				logger.Info("conversation: SPAWNED new worker msg=%s task=%s taskID=%s", msg.messageID, utils.Truncate(taskArg, 60, "..."), taskID)
			}
			sendToolAck(tc)
		case stopWorkerToolName:
			taskIDArg, _ := tc.Arguments["task_id"].(string)
			g.executeStopWorkerExtended(ctx, slotMap, taskIDArg)
			sendToolAck(tc)
		case queryTasksToolName:
			result := g.executeQueryTasks(ctx, msg, tc.Arguments)
			logger.Info("conversation: query_tasks result_len=%d", len(result))
			sendToolAck(tc)
		case queryUsageToolName:
			result := g.executeQueryUsage(ctx, msg, tc.Arguments)
			logger.Info("conversation: query_usage result_len=%d", len(result))
			sendToolAck(tc)
		case manageNoticeToolName:
			result := g.executeManageNotice(msg, tc.Arguments)
			logger.Info("conversation: manage_notice result=%s", utils.Truncate(result, 60, "..."))
			sendToolAck(tc)
		}
	}

	return true
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

	systemPrompt := g.buildConversationSystemPrompt(ctx, opts.senderID, opts.hasQueryTools)

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
// based on the prefetch result. Injects sliding context and pre-fetched data.
func (g *Gateway) conversationLLMDynamic(ctx context.Context, msg *incomingMessage, workers workerSnapshotList, chatHistory string, pf prefetchResult) (string, []ports.ToolCall, error) {
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
		tools:       []ports.ToolDefinition{g.buildDispatchWorkerTool(), stopWorkerTool},
		maxTokens:   conversationMaxTok,
	})
}

// buildConversationSystemPrompt composes the conversation router's system
// prompt by prepending memory context (SOUL.md, USER.md, long-term) and
// appending date/timezone when available. Uses a 60-second TTL cache per senderID.
// hasQueryTools controls whether narration voice instructions are included.
func (g *Gateway) buildConversationSystemPrompt(ctx context.Context, senderID string, hasQueryTools bool) string {
	var sections []string

	if g.conversationPromptLoader != nil {
		memoryCtx := g.loadCachedConversationPrompt(ctx, senderID)
		if memoryCtx != "" {
			sections = append(sections, memoryCtx)
		}
	}

	sections = append(sections, strings.TrimSpace(g.resolveConversationSystemPrompt(hasQueryTools)))

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
