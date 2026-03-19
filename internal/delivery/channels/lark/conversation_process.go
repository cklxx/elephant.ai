package lark

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"alex/internal/app/agent/llmclient"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

const (
	defaultConversationTimeout = 8 * time.Second
	conversationMaxTok         = 300
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

// fastPathIntent classifies a user message for the fast-path router.
type fastPathIntent int

const (
	fastPathNone     fastPathIntent = iota // route to Chat LLM
	fastPathDispatch                        // clearly a task — skip LLM, spawn worker
	fastPathStatus                          // status query — skip LLM
	fastPathStop                            // stop intent — skip LLM
)

var dispatchWorkerTool = ports.ToolDefinition{
	Name:        dispatchWorkerToolName,
	Description: "Launch a background agent to execute a task asynchronously. The agent will notify the user when done. Returns task_id of the spawned worker.",
	Parameters: ports.ParameterSchema{
		Type: "object",
		Properties: map[string]ports.Property{
			"task": {
				Type:        "string",
				Description: "Task description preserving the user's original intent",
			},
		},
		Required: []string{"task"},
	},
}

var stopWorkerTool = ports.ToolDefinition{
	Name:        stopWorkerToolName,
	Description: "Stop a running background agent task.",
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

var conversationSystemPrompt = strings.TrimSpace(`
You are a conversation router. Your only job is to reply and dispatch.

Decision (pick one):
- Reply directly: greetings, chitchat, progress queries, pure Q&A — anything that requires no action.
- dispatch_worker: everything else — whenever the user wants something done, dispatch it.

IMPORTANT: When calling dispatch_worker, you MUST also include a short text reply acknowledging the request (e.g. "好，我来看一下", "收到，马上处理"). Never call dispatch_worker with an empty text response.
Keep all replies short and natural. Match the user's language.

Task control:
- User sends a follow-up or correction for a running task → dispatch_worker (it injects into the running task).
- User asks to stop ("stop", "cancel", "never mind") and a task is running → stop_worker.

Safety:
- Never fabricate information, tool outputs, or task status.
- Never include secrets, API keys, or credentials in replies.
`)

// classifyFastPath returns a fastPathIntent for the given message content.
// This runs before the Chat LLM call and can bypass it for common patterns,
// saving ~40-60% of LLM calls.
func classifyFastPath(content string) fastPathIntent {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return fastPathNone
	}

	// Stop intent keywords — must be a standalone intent, not embedded in another word.
	// "/stop" commands are handled before reaching this function; these catch natural-language stops.
	stopKws := []string{"取消任务", "停止任务", "/stop"}
	for _, kw := range stopKws {
		if strings.Contains(lower, kw) {
			return fastPathStop
		}
	}

	// Status query keywords — must be specific enough not to match dispatch messages.
	// Bare words like "task" and "status" are intentionally excluded because they
	// appear frequently in dispatch requests (e.g. "task one", "check status of PR").
	statusKws := []string{"任务状态", "任务进展", "#1", "#2", "#3", "#4", "#5", "task status", "怎么样了"}
	for _, kw := range statusKws {
		if strings.Contains(lower, kw) {
			return fastPathStatus
		}
	}

	// Dispatch keywords — message clearly wants something done.
	dispatchKws := []string{
		"帮我", "research", "分析", "写", "draft", "find", "搜索", "查", "做", "生成", "创建",
		"translate", "翻译", "summarize", "总结", "run", "execute", "fix", "repair",
	}
	// Only fast-path dispatch if the message is substantive (>5 runes).
	if utf8.RuneCountInString(content) > 5 {
		for _, kw := range dispatchKws {
			if strings.Contains(lower, kw) {
				return fastPathDispatch
			}
		}
	}

	return fastPathNone
}

// handleViaConversationProcess sends the user message to the gateway's LLM
// with a single tool (dispatch_worker). The text reply goes to the user
// immediately; a dispatch_worker call spawns a background worker.
// Always returns true — fully owns the message lifecycle when enabled.
func (g *Gateway) handleViaConversationProcess(ctx context.Context, msg *incomingMessage) bool {
	lang := detectLang(msg.content)
	slotMap := g.getOrCreateSlotMap(msg.chatID)

	// Fast-path classifier: bypass Chat LLM for common patterns.
	switch classifyFastPath(msg.content) {
	case fastPathDispatch:
		taskID := g.spawnWorkerInSlotMap(ctx, msg, slotMap, msg.content)
		var ack string
		if lang == "en" {
			ack = fmt.Sprintf("On it, starting %s.", taskID)
		} else {
			ack = fmt.Sprintf("好，开始执行 %s。", taskID)
		}
		g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), ack)
		return true
	case fastPathStatus:
		g.handleNaturalTaskStatusQuery(msg)
		return true
	case fastPathStop:
		slotMap.stopAll(true)
		var ack string
		if lang == "en" {
			ack = "Stopped all running tasks."
		} else {
			ack = "已停止所有运行中的任务。"
		}
		g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), ack)
		return true
	}

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

	reply, toolCalls, llmErr := g.conversationLLMWithList(ctx, msg.senderID, msg.content, allWorkers, chatHistory)

	// Wait for reaction goroutine to finish so we have the ID for removal.
	<-reactionDone
	g.removeProcessingReaction(ctx, msg.messageID, processingReactionID)

	// Fallback: if LLM failed, dispatch a worker directly.
	if llmErr != nil {
		g.logger.Warn("conversationLLM failed, falling back to direct dispatch: %v", llmErr)
		taskID := g.spawnWorkerInSlotMap(ctx, msg, slotMap, msg.content)
		var ack string
		if lang == "en" {
			ack = fmt.Sprintf("Starting %s.", taskID)
		} else {
			ack = fmt.Sprintf("好，开始执行 %s。", taskID)
		}
		g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), ack)
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

	// When no worker is dispatched, send the reply directly.
	if reply != "" && !hasDispatchWorker {
		g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply)
	}

	for _, tc := range toolCalls {
		switch tc.Name {
		case dispatchWorkerToolName:
			taskArg, _ := tc.Arguments["task"].(string)
			if taskArg == "" {
				taskArg = msg.content
			}
			taskID, injected := g.spawnOrInjectWorker(ctx, msg, slotMap, taskArg)
			if injected {
				logger.Info("conversation: INJECTED msg=%s into running worker", msg.messageID)
				g.dispatchFormattedReply(ctx, msg.chatID, "", "你的消息已加入当前任务")
			} else {
				logger.Info("conversation: SPAWNED new worker msg=%s task=%s taskID=%s", msg.messageID, utils.Truncate(taskArg, 60, "..."), taskID)
				if reply != "" {
					g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), reply)
				}
			}
		case stopWorkerToolName:
			taskIDArg, _ := tc.Arguments["task_id"].(string)
			taskIDArg = strings.TrimSpace(taskIDArg)
			if taskIDArg == "" {
				slotMap.stopAll(true)
			} else {
				slotMap.stopByTaskID(taskIDArg)
			}
		}
	}

	return true
}

// conversationLLMWithList calls the gateway's shared LLM with multi-worker status.
// Returns (reply, toolCalls, error). An error means the LLM call itself failed.
func (g *Gateway) conversationLLMWithList(ctx context.Context, senderID, userMsg string, workers workerSnapshotList, chatHistory string) (string, []ports.ToolCall, error) {
	if g.llmFactory == nil {
		return "", nil, fmt.Errorf("no LLM factory configured")
	}

	client, _, err := llmclient.GetClientFromProfile(g.llmFactory, g.llmProfile, nil, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get LLM client: %w", err)
	}

	llmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultConversationTimeout)
	defer cancel()

	systemPrompt := g.buildConversationSystemPrompt(ctx, senderID)

	var sb strings.Builder
	sb.WriteString("Worker status: ")
	sb.WriteString(workers.StatusSummary())
	if chatHistory != "" {
		sb.WriteString("\n\nRecent chat:\n")
		sb.WriteString(chatHistory)
	}
	sb.WriteString("\n\nUser message: ")
	sb.WriteString(strings.TrimSpace(userMsg))

	resp, err := client.Complete(llmCtx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: sb.String()},
		},
		Tools:       []ports.ToolDefinition{dispatchWorkerTool, stopWorkerTool},
		Temperature: conversationTemp,
		MaxTokens:   conversationMaxTok,
	})
	if err != nil {
		return "", nil, err
	}

	g.logger.Info("conversation LLM: reply=%q tool_calls=%d", utils.Truncate(resp.Content, 80, "…"), len(resp.ToolCalls))
	return strings.TrimSpace(resp.Content), resp.ToolCalls, nil
}

// conversationLLM is the legacy single-worker variant kept for backward compat
// with tests that call it directly. It delegates to conversationLLMWithList.
func (g *Gateway) conversationLLM(ctx context.Context, senderID, userMsg string, snap workerSnapshot, chatHistory string) (string, []ports.ToolCall) {
	list := workerSnapshotList{Snapshots: []workerSnapshot{snap}, Lang: snap.Lang}
	if snap.Phase == slotIdle {
		list.Snapshots = nil
	}
	reply, calls, _ := g.conversationLLMWithList(ctx, senderID, userMsg, list, chatHistory)
	return reply, calls
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

	sections = append(sections, conversationSystemPrompt)

	if g.cfg.ConversationWorkerCapabilities != "" {
		sections = append(sections, "Worker capabilities:\n"+g.cfg.ConversationWorkerCapabilities)
	}

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
// Returns the assigned task ID (e.g. "#1").
func (g *Gateway) spawnWorkerInSlotMap(ctx context.Context, msg *incomingMessage, slotMap *chatSlotMap, taskContent string) string {
	max := g.cfg.MaxConcurrentWorkers
	if max <= 0 {
		max = 5
	}
	slot, taskID, ok := slotMap.allocateSlotIfCapacity(max, g.currentTime())
	if !ok {
		lang := detectLang(msg.content)
		var notice string
		if lang == "en" {
			notice = fmt.Sprintf("All %d worker slots are busy. Please wait for a task to finish before starting a new one.", max)
		} else {
			notice = fmt.Sprintf("当前已有 %d 个任务在运行，请等待任务完成后再启动新任务。", max)
		}
		g.dispatchFormattedReply(ctx, msg.chatID, replyTarget(msg.messageID, true), notice)
		return ""
	}

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
	slot.taskDesc = strings.TrimSpace(taskContent)
	slot.recentProgress = slot.recentProgress[:0]
	slot.lastTouched = g.currentTime()
	slot.taskStartTime = g.currentTime()
	slot.lastProgressAt = g.currentTime()
	slot.mu.Unlock()

	workerMsg := *msg
	workerMsg.content = taskContent

	g.launchWorkerGoroutineForSlotMap(&workerMsg, slot, slotMap, sessionID, inputCh, taskCancel, taskCtx, taskToken, isResume)
	return taskID
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

	taskID := g.spawnWorkerInSlotMap(ctx, msg, slotMap, taskContent)
	return taskID, false
}

// launchWorkerGoroutineForSlotMap is like launchWorkerGoroutine but cleans up
// the slot from the chatSlotMap rather than from activeSlots.
func (g *Gateway) launchWorkerGoroutineForSlotMap(msg *incomingMessage, slot *sessionSlot, slotMap *chatSlotMap, sessionID string, inputCh chan agent.UserInput, taskCancel context.CancelFunc, taskCtx context.Context, taskToken uint64, isResume bool) {
	g.taskWG.Add(1)
	go func(taskCtx context.Context, taskCancel context.CancelFunc, taskToken uint64) {
		defer g.taskWG.Done()
		defer taskCancel()

		awaitingInput := g.runTask(taskCtx, msg, sessionID, inputCh, isResume, taskToken)

		slot.mu.Lock()
		if slot.taskToken == taskToken {
			if slot.intentionalCancelToken == taskToken {
				slot.intentionalCancelToken = 0
			}
			slot.inputCh = nil
			slot.taskCancel = nil
			slot.taskStartTime = time.Time{}
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
