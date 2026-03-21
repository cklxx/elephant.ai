package lark

import (
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

const (
	defaultConversationTimeout = 8 * time.Second
	conversationMaxTok         = 120
	conversationMaxTokQuery    = 400
	conversationTemp           = 0.3

	dispatchWorkerToolName = "dispatch_worker"
	stopWorkerToolName     = "stop_worker"

	conversationChatHistoryMaxRounds = 5
	conversationPromptCacheTTL       = 60 * time.Second

	imMaxRunes = 20
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
			},
			Required: []string{"task"},
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
		},
	},
}

// defaultConversationReplyRules is the fallback when Config.ConversationReplyRules is empty.
const defaultConversationReplyRules = `## Reply rules (HARD CONSTRAINTS)
- Chinese: ≤12 chars, NO 句号, drop 主语/我, casual
- English: ≤15 chars, lowercase, fragments, no period
- NEVER use 其实/然后/的话/非常/请/您/好的/可以的
- One short sentence only. No explanations.`

// conversationSystemPromptBase is the core prompt without reply rules.
// Reply rules are injected from config (or defaults) at runtime.
// Unified personality: absorbs task status, usage, notice, and narration voices.
const conversationSystemPromptBase = `You are an IM chatbot. Reply ultra-short or use tools.

## Decision flow
Pick the right action:
- reply only: greetings, chitchat, simple Q&A.
- dispatch_worker: user wants something done (coding, research, analysis, writing).
- stop_worker: user wants to cancel a task.
- query_tasks: user asks about tasks, progress, status, what's running.
- query_usage: user asks about usage, cost, stats, model info.
- manage_notice: user wants to bind/check/clear notification group.

When dispatching, also reply with a short ack (4-8 chars).
When a skill matches, include its name in the dispatch task.

%REPLY_RULES%

## Narration voice
When reporting tool results (query_tasks, query_usage, manage_notice):
- Conclusion first, 2-5 sentences.
- **Bold** key data (counts, costs, durations).
- Drop technical fields (task_id, status codes, chat_id).
- Preserve actionable info (links, paths, errors).
- No Markdown headings, no emoji.

## Task control
- Follow-up for running task → dispatch_worker (inject).
- Stop intent → stop_worker.
- Cross-task: when user says "use #1's result" or "based on what #1 found", include "#1" reference in the dispatch task description. The worker will receive the referenced result as context.

## Safety
- Never fabricate info or status.
- Never include secrets.`

// resolveConversationSystemPrompt resolves the %REPLY_RULES% placeholder
// in conversationSystemPromptBase using config or defaults.
func (g *Gateway) resolveConversationSystemPrompt() string {
	replyRules := g.cfg.ConversationReplyRules
	if replyRules == "" {
		replyRules = defaultConversationReplyRules
	}
	return strings.Replace(conversationSystemPromptBase, "%REPLY_RULES%", replyRules, 1)
}

// imBaseReplacer covers formal→casual substitutions that are always safe to apply.
var imBaseReplacer = strings.NewReplacer(
	"请稍等", "等下",
	"请稍等一下", "等下",
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
	"我已经", "",
	"因为", "",
	"所以", "",
	"其实", "",
	"然后", "",
	"的话", "",
	"非常", "很",
	"可以", "行",
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

// splitIMFragments splits a reply into IM-sized fragments at clause boundaries.
// Max 3 fragments. Each fragment capped at imMaxRunes. Tail merged into last.
// Empty/punctuation-only shards are filtered.
func splitIMFragments(s string) []string {
	if s == "" {
		return nil
	}
	if utf8.RuneCountInString(s) <= imMaxRunes {
		return []string{s}
	}

	// Split on clause boundaries.
	var segments []string
	start := 0
	for i, r := range s {
		if i > 0 && isClauseBoundary(r) {
			seg := strings.TrimSpace(s[start:i])
			if seg != "" && !isPunctOnly(seg) {
				segments = append(segments, seg)
			}
			start = i + utf8.RuneLen(r)
		}
	}
	// Remaining tail.
	tail := strings.TrimSpace(s[start:])
	if tail != "" && !isPunctOnly(tail) {
		segments = append(segments, tail)
	}

	if len(segments) == 0 {
		return []string{capFragment(s)} // no valid segments — return capped original
	}
	if len(segments) == 1 {
		return []string{capFragment(segments[0])}
	}

	// Cap at 3: merge remaining into last fragment.
	const maxFragments = 3
	if len(segments) > maxFragments {
		merged := make([]string, maxFragments)
		copy(merged, segments[:maxFragments-1])
		merged[maxFragments-1] = strings.Join(segments[maxFragments-1:], "，")
		segments = merged
	}

	// Per-fragment length cap.
	for i, seg := range segments {
		segments[i] = capFragment(seg)
	}
	return segments
}

func isClauseBoundary(r rune) bool {
	return r == '，' || r == '、' || r == '！' || r == '？' || r == '；' || r == '\n' || r == ','
}

func isPunctOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) && !unicode.IsSymbol(r) {
			return false
		}
	}
	return true
}

func capFragment(s string) string {
	if utf8.RuneCountInString(s) <= imMaxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:imMaxRunes])
}

// defaultIMDelay is the production delay function used between IM fragments.
func defaultIMDelay(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// sendFragmentedReply naturalizes the reply, splits into IM fragments, and
// dispatches each with a delay between them to simulate natural typing rhythm.
// NOTE: dispatchFormattedReply has its own inner splitMessage for markdown-based
// splitting, but short IM fragments (≤20 runes) never trigger it. No delay stacking.
func (g *Gateway) sendFragmentedReply(ctx context.Context, chatID, replyToID, reply string, level int) {
	text := naturalizeReply(reply, level)
	if text == "" {
		return
	}
	fragments := splitIMFragments(text)
	if len(fragments) == 0 {
		return
	}
	for i, frag := range fragments {
		if i > 0 {
			delay := randomIMDelay()
			if !g.imDelayFn(ctx, delay) {
				return // context cancelled
			}
		}
		g.dispatchFormattedReply(ctx, chatID, replyToID, frag)
	}
}

// randomIMDelay returns a random duration between 400ms and 800ms.
func randomIMDelay() time.Duration {
	return time.Duration(400+rand.IntN(401)) * time.Millisecond
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
	extraContext string                 // pre-fetched data to inject in user message
	tools        []ports.ToolDefinition // full tool list for this request
	maxTokens    int                    // dynamic token budget
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
				ch <- result{"tasks", "No active tasks."}
				return
			}
			ch <- result{"tasks", g.formatActiveTaskList(tasks)}
		}()
	}
	if hasUsage && g.costTracker != nil {
		go func() {
			now := g.currentTime()
			today, err := g.costTracker.GetDailyCost(ctx, now)
			if err != nil || today == nil || today.RequestCount == 0 {
				ch <- result{"usage", "No usage data for today."}
				return
			}
			ch <- result{"usage", formatCostSummaryBlock(today)}
		}()
	}

	var extraParts []string
	for range pending {
		select {
		case r := <-ch:
			extraParts = append(extraParts, fmt.Sprintf("[Pre-fetched %s data]\n%s", r.label, r.data))
		case <-ctx.Done():
			return prefetchResult{tools: tools, maxTokens: conversationMaxTokQuery}
		}
	}

	return prefetchResult{
		extraContext: strings.Join(extraParts, "\n\n"),
		tools:       tools,
		maxTokens:   conversationMaxTokQuery,
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

	// Fallback: if LLM failed, dispatch a worker directly (no hardcoded ack).
	if llmErr != nil {
		g.logger.Warn("conversationLLM failed, falling back to direct dispatch: %v", llmErr)
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
	fragments := splitIMFragments(naturalizeReply(reply, level))
	logger.Info("conversation: decision msg=%s hasDispatchWorker=%t reply_len=%d fragments=%d tool_calls=%d",
		msg.messageID, hasDispatchWorker, len(reply), len(fragments), len(toolCalls))

	// Fallback: LLM returned empty reply with no tool calls — treat as dispatch.
	if reply == "" && len(toolCalls) == 0 {
		logger.Warn("conversation: empty LLM response, falling back to dispatch for msg=%s", msg.messageID)
		g.spawnWorkerInSlotMap(ctx, msg, slotMap, msg.content)
		return true
	}

	// When no worker is dispatched, send the reply as fragmented IM messages.
	if reply != "" && !hasDispatchWorker {
		g.sendFragmentedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply, level)
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
				if reply != "" {
					g.sendFragmentedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply, level)
				}
			} else {
				logger.Info("conversation: SPAWNED new worker msg=%s task=%s taskID=%s", msg.messageID, utils.Truncate(taskArg, 60, "..."), taskID)
				if reply != "" {
					g.sendFragmentedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply, level)
				}
			}
		case stopWorkerToolName:
			taskIDArg, _ := tc.Arguments["task_id"].(string)
			g.executeStopWorkerExtended(ctx, slotMap, taskIDArg)
		case queryTasksToolName:
			result := g.executeQueryTasks(ctx, msg, tc.Arguments)
			// Result is already in the LLM response via pre-fetch; log for debugging.
			logger.Info("conversation: query_tasks result_len=%d", len(result))
		case queryUsageToolName:
			result := g.executeQueryUsage(ctx, msg, tc.Arguments)
			logger.Info("conversation: query_usage result_len=%d", len(result))
		case manageNoticeToolName:
			result := g.executeManageNotice(msg, tc.Arguments)
			logger.Info("conversation: manage_notice result=%s", utils.Truncate(result, 60, "..."))
		}
	}

	return true
}

// conversationLLMOpts configures the conversation LLM call.
type conversationLLMOpts struct {
	senderID     string
	userMsg      string
	workers      workerSnapshotList
	chatHistory  string
	tools        []ports.ToolDefinition
	maxTokens    int
	extraContext string // pre-fetched data injected before user message
	chatID       string // when set, inject sliding context from chatConversationContext
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

	systemPrompt := g.buildConversationSystemPrompt(ctx, opts.senderID)

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
		senderID:     msg.senderID,
		userMsg:      msg.content,
		workers:      workers,
		chatHistory:  chatHistory,
		tools:        pf.tools,
		maxTokens:    pf.maxTokens,
		extraContext: pf.extraContext,
		chatID:       msg.chatID,
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
func (g *Gateway) buildConversationSystemPrompt(ctx context.Context, senderID string) string {
	var sections []string

	if g.conversationPromptLoader != nil {
		memoryCtx := g.loadCachedConversationPrompt(ctx, senderID)
		if memoryCtx != "" {
			sections = append(sections, memoryCtx)
		}
	}

	sections = append(sections, strings.TrimSpace(g.resolveConversationSystemPrompt()))

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

// spawnWorker is the legacy single-slot spawn for backward compat with
// the handleMessage non-conversation-process path and tests.
// Returns true if injected into existing, false if spawned new.
func (g *Gateway) spawnWorker(ctx context.Context, msg *incomingMessage, slot *sessionSlot, _ workerSnapshot, taskContent string) bool {
	slot.mu.Lock()
	if slot.phase == slotRunning {
		ch := slot.inputCh
		sessionID := slot.sessionID
		slot.mu.Unlock()
		if ch != nil {
			select {
			case ch <- agent.UserInput{Content: taskContent, SenderID: msg.senderID, MessageID: msg.messageID}:
				g.logger.Info("conversation: injected into running worker session %s", sessionID)
			default:
				g.logger.Warn("conversation: worker inputCh full for session %s", sessionID)
				g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), "消息已收到，等待当前任务处理完毕后执行")
			}
		}
		return true
	}

	sessionID, isResume := g.resolveSessionForNewTask(ctx, msg.chatID, slot)
	inputCh := make(chan agent.UserInput, 16)
	taskCtx, taskCancel := context.WithCancel(context.Background())
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.taskCancel = taskCancel
	slot.taskToken++
	taskToken := slot.taskToken
	slot.sessionID = sessionID
	slot.lastSessionID = sessionID
	slot.taskDesc = strings.TrimSpace(taskContent)
	slot.recentProgress = slot.recentProgress[:0]
	slot.lastTouched = g.currentTime()
	slot.taskStartTime = g.currentTime()
	slot.mu.Unlock()

	workerMsg := *msg
	workerMsg.content = taskContent

	g.launchWorkerGoroutine(&workerMsg, slot, sessionID, inputCh, taskCancel, taskCtx, taskToken, isResume)
	return false
}

// stopWorkerFromConversation cancels all running workers for the given chat.
// Used by the legacy stop path (non-slotMap).
func (g *Gateway) stopWorkerFromConversation(chatID string, slot *sessionSlot) {
	slot.mu.Lock()
	cancel := slot.taskCancel
	running := slot.phase == slotRunning && cancel != nil
	if running {
		slot.intentionalCancelToken = slot.taskToken
	}
	slot.mu.Unlock()

	if running {
		cancel()
		g.logger.Info("conversation: stopped worker for chat %s", chatID)
	}
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
