package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/textutil"
	"alex/internal/logging"
	"alex/internal/memory"
	"alex/internal/skills"
)

const (
	memoryCaptureHookName = "memory_capture"
	// minToolCallsForCapture is the minimum number of tool calls in a task
	// before auto-capture triggers. Pure conversations are not captured.
	minToolCallsForCapture = 1
	// maxCaptureContentLen caps the memory content length to avoid bloating the store.
	maxCaptureContentLen = 1000
)

// MemoryCaptureHook automatically captures key decisions and outcomes from completed
// tasks and writes them to the memory store for future recall.
type MemoryCaptureHook struct {
	memoryService   memory.Service
	enabled         bool
	captureMessages bool
	dedupeThreshold float64
	logger          logging.Logger
}

// MemoryCaptureConfig controls auto-capture behaviour.
type MemoryCaptureConfig struct {
	Enabled         bool
	AutoCapture     bool
	CaptureMessages bool
	DedupeThreshold float64
}

// NewMemoryCaptureHook creates a new memory capture hook.
func NewMemoryCaptureHook(svc memory.Service, logger logging.Logger, cfg MemoryCaptureConfig) *MemoryCaptureHook {
	enabled := true
	if !cfg.Enabled || !cfg.AutoCapture {
		enabled = false
	}
	dedupe := cfg.DedupeThreshold
	if dedupe <= 0 {
		dedupe = 0.85
	}
	return &MemoryCaptureHook{
		memoryService:   svc,
		enabled:         enabled,
		captureMessages: cfg.CaptureMessages,
		dedupeThreshold: dedupe,
		logger:          logging.OrNop(logger),
	}
}

func (h *MemoryCaptureHook) Name() string { return memoryCaptureHookName }

// OnTaskStart is a no-op for the capture hook.
func (h *MemoryCaptureHook) OnTaskStart(_ context.Context, _ TaskInfo) []Injection {
	return nil
}

// OnTaskCompleted extracts a summary from the task result and writes it to memory.
// Only tasks with tool calls are captured to avoid noise from pure conversations.
func (h *MemoryCaptureHook) OnTaskCompleted(ctx context.Context, result TaskResultInfo) error {
	if h.memoryService == nil || !h.enabled {
		return nil
	}
	policy, hasPolicy := appcontext.MemoryPolicyFromContext(ctx)
	if hasPolicy {
		if !policy.Enabled || !policy.AutoCapture {
			return nil
		}
	} else {
		policy = appcontext.ResolveMemoryPolicy(ctx)
	}

	// Filter: only capture tasks that involved tool calls
	if len(result.ToolCalls) < minToolCallsForCapture {
		return nil
	}

	// Filter: must have a non-empty answer
	if strings.TrimSpace(result.Answer) == "" {
		return nil
	}

	userID := result.UserID
	if userID == "" {
		userID = "default"
	}

	summary := buildCaptureSummary(result)
	keywords := extractCaptureKeywords(result)
	slots := buildCaptureSlots(ctx, result)

	entry := memory.Entry{
		UserID:   userID,
		Content:  summary,
		Keywords: keywords,
		Slots:    slots,
	}

	if h.isDuplicate(ctx, entry, userID) {
		h.logger.Debug("Skipped auto-capture due to similarity threshold (user=%s)", userID)
	} else {
		saved, err := h.memoryService.Save(ctx, entry)
		if err != nil {
			h.logger.Warn("Memory capture failed: %v", err)
			return fmt.Errorf("memory capture: %w", err)
		}
		h.logger.Info("Auto-captured memory %s (keywords: %v, tools: %d)",
			saved.Key, keywords, len(result.ToolCalls))
	}

	h.captureWorkflowTrace(ctx, result, userID)

	return nil
}

// buildCaptureSummary creates a concise memory entry from the task result.
func buildCaptureSummary(result TaskResultInfo) string {
	var sb strings.Builder

	// Task input (truncated)
	input := strings.TrimSpace(result.TaskInput)
	if len(input) > 200 {
		input = input[:200] + "..."
	}
	sb.WriteString(fmt.Sprintf("Task: %s\n", input))

	// Tools used
	if len(result.ToolCalls) > 0 {
		toolNames := make([]string, 0, len(result.ToolCalls))
		seen := make(map[string]bool)
		for _, tc := range result.ToolCalls {
			if !seen[tc.ToolName] {
				toolNames = append(toolNames, tc.ToolName)
				seen[tc.ToolName] = true
			}
		}
		sb.WriteString(fmt.Sprintf("Tools: %s\n", strings.Join(toolNames, ", ")))
	}

	// Outcome
	sb.WriteString(fmt.Sprintf("Outcome: %s\n", result.StopReason))

	// Answer summary (truncated)
	answer := strings.TrimSpace(result.Answer)
	if answer != "" {
		if len(answer) > maxCaptureContentLen {
			answer = answer[:maxCaptureContentLen] + "..."
		}
		sb.WriteString(fmt.Sprintf("Result: %s\n", answer))
	}

	return sb.String()
}

// extractCaptureKeywords extracts keywords from both the task input and tool names.
func extractCaptureKeywords(result TaskResultInfo) []string {
	// Start with task input keywords
	keywords := extractKeywords(result.TaskInput)

	// Add unique tool names as keywords
	seen := make(map[string]bool, len(keywords))
	for _, kw := range keywords {
		seen[kw] = true
	}
	for _, tc := range result.ToolCalls {
		name := strings.ToLower(tc.ToolName)
		if !seen[name] {
			seen[name] = true
			keywords = append(keywords, name)
		}
	}

	return keywords
}

// buildCaptureSlots creates structured metadata slots for the captured memory.
func buildCaptureSlots(ctx context.Context, result TaskResultInfo) map[string]string {
	slots := map[string]string{
		"type":    "auto_capture",
		"scope":   "user",
		"source":  "memory_capture",
		"outcome": result.StopReason,
	}

	if len(result.ToolCalls) > 0 {
		// Record the tool sequence for pattern recognition
		toolSeq := make([]string, 0, len(result.ToolCalls))
		for _, tc := range result.ToolCalls {
			toolSeq = append(toolSeq, tc.ToolName)
		}
		slots["tool_sequence"] = strings.Join(toolSeq, "→")
	}

	if result.SessionID != "" {
		slots["session_id"] = result.SessionID
	}
	if senderID := strings.TrimSpace(result.UserID); senderID != "" && senderID != "default" {
		slots["sender_id"] = senderID
	}
	if channel := appcontext.ChannelFromContext(ctx); channel != "" {
		slots["channel"] = channel
	}
	if chatID := appcontext.ChatIDFromContext(ctx); chatID != "" {
		slots["chat_id"] = chatID
	}

	return slots
}

func (h *MemoryCaptureHook) isDuplicate(ctx context.Context, entry memory.Entry, userID string) bool {
	if h.memoryService == nil || h.dedupeThreshold <= 0 {
		return false
	}
	query := memory.Query{
		UserID:   userID,
		Text:     entry.Content,
		Keywords: entry.Keywords,
		Limit:    5,
	}
	existing, err := h.memoryService.Recall(ctx, query)
	if err != nil || len(existing) == 0 {
		return false
	}
	for _, prev := range existing {
		if textutil.SimilarityScore(entry.Content, prev.Content) >= h.dedupeThreshold {
			return true
		}
	}
	return false
}

func (h *MemoryCaptureHook) captureWorkflowTrace(ctx context.Context, result TaskResultInfo, userID string) {
	if h.memoryService == nil || len(result.ToolCalls) < 2 {
		return
	}

	trace := skills.WorkflowTrace{
		TaskID:    result.RunID,
		UserID:    userID,
		Outcome:   result.StopReason,
		CreatedAt: time.Now(),
	}
	for _, call := range result.ToolCalls {
		trace.Tools = append(trace.Tools, skills.ToolStep{
			Name:    call.ToolName,
			Success: call.Success,
		})
	}

	payload, err := json.Marshal(trace)
	if err != nil {
		h.logger.Warn("Workflow trace marshal failed: %v", err)
		return
	}

	entry := memory.Entry{
		UserID:   userID,
		Content:  string(payload),
		Keywords: append([]string{"workflow_trace"}, trace.ToolNames()...),
		Slots: map[string]string{
			"type":    "workflow_trace",
			"scope":   "user",
			"source":  "memory_capture",
			"task_id": result.RunID,
			"outcome": result.StopReason,
		},
		CreatedAt: trace.CreatedAt,
	}
	if result.SessionID != "" {
		entry.Slots["session_id"] = result.SessionID
	}
	if senderID := strings.TrimSpace(result.UserID); senderID != "" && senderID != "default" {
		entry.Slots["sender_id"] = senderID
	}
	if channel := appcontext.ChannelFromContext(ctx); channel != "" {
		entry.Slots["channel"] = channel
	}
	if chatID := appcontext.ChatIDFromContext(ctx); chatID != "" {
		entry.Slots["chat_id"] = chatID
	}

	if _, err := h.memoryService.Save(ctx, entry); err != nil {
		h.logger.Warn("Workflow trace save failed: %v", err)
	}
}
