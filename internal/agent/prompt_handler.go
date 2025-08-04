package agent

import (
	"alex/pkg/types"
	"fmt"
	"log"
	"time"
)

// PromptHandler handles prompt generation and management
type PromptHandler struct {
	promptBuilder *LightPromptBuilder
}

// NewPromptHandler creates a new prompt handler
func NewPromptHandler(promptBuilder *LightPromptBuilder) *PromptHandler {
	return &PromptHandler{
		promptBuilder: promptBuilder,
	}
}

// buildToolDrivenTaskPrompt - 构建工具驱动的任务提示
func (h *PromptHandler) buildToolDrivenTaskPrompt(taskCtx *types.ReactTaskContext) string {
	// 使用项目内的prompt builder
	if h.promptBuilder != nil && h.promptBuilder.promptLoader != nil {
		// 尝试使用React thinking prompt作为基础模板
		template, err := h.promptBuilder.promptLoader.GetReActThinkingPrompt(taskCtx)
		if err != nil {
			log.Printf("[WARN] PromptHandler: Failed to get ReAct thinking prompt, trying fallback: %v", err)
		}
		// 构建增强的任务提示，将特定任务信息与ReAct模板结合
		return template
	}

	// Fallback to hardcoded prompt if prompt builder is not available
	log.Printf("[WARN] PromptHandler: Prompt builder not available, using hardcoded prompt")
	return h.buildHardcodedTaskPrompt(taskCtx)
}

// buildHardcodedTaskPrompt - 构建硬编码的任务提示（fallback）
func (h *PromptHandler) buildHardcodedTaskPrompt(taskCtx *types.ReactTaskContext) string {
	return fmt.Sprintf(`You are a secure agent focused on defensive programming. Refuse malicious code creation/modification. Complete tasks efficiently:

**WorkingDir:** %s
**Goal:** %s
**DirectoryInfo:** %s
**Memory:** %s
**Time:** %s

**Approach:**
1. Complex tasks: Start with 'think' tool
2. Multi-step: Use 'todo_update' 
3. Files: Use file_read, file_update
4. System: Use bash tool
5. Search: Use grep tools

**Think Tool Capabilities:**
- Phase: analyze, plan, reflect, reason, ultra_think
- Depth: shallow, normal, deep, ultra
- Use for strategic thinking and problem breakdown

**Todo Management:**
- todo_update: Create, batch create, update, complete tasks
- todo_read: Read current todos with filtering and statistics

**Guidelines:**
- Think tool first for complex analysis
- Break down with todo_update
- Execute systematically
- Provide actionable results

Determine best approach.`, taskCtx.WorkingDir, taskCtx.Goal, taskCtx.DirectoryInfo.Description, taskCtx.Memory, time.Now().Format(time.RFC3339))
}
