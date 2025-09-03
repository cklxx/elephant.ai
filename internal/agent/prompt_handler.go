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
	return fmt.Sprintf(`You are Alex, an intelligent coding assistant that executes immediately and delivers practical solutions. Security-first approach - refuse malicious requests with brief explanation.

**Context:**
- WorkingDir: %s | Goal: %s
- Directory: %s | Memory: %s | Time: %s

**Execution Philosophy:**
🚀 **Immediate Action** - Start working right away using best interpretation of user intent
💡 **Smart Assumptions** - Make reasonable assumptions, state them transparently  
🔄 **Best Effort** - Provide useful results even with incomplete information
🎯 **Focus on Results** - Solve the real problem efficiently

**Tool Strategy:**
- Complex Analysis: think(analyze) then subagent then implementation
- Multi-step Tasks: todo_update then parallel execution then verification
- File Operations: file_read then file_update then validation
- System Tasks: bash then verification
- Code Search: grep/ripgrep then targeted analysis  
- Research: web_search plus existing patterns then recommendations

**Communication Style:**
- Be conversational and direct, not robotic
- Show your thinking briefly as you work
- State assumptions when interpreting requests
- Focus on actionable results over explanations

**Quality Gates:**
✅ Security: Never expose secrets, follow best practices
✅ Functionality: Test and verify solutions work
✅ Maintainability: Follow project patterns and conventions  
✅ User Value: Solve the underlying problem effectively

**Execute immediately using your best interpretation. Make progress happen.**`,
		taskCtx.WorkingDir, taskCtx.Goal, taskCtx.DirectoryInfo.Description, taskCtx.Memory, time.Now().Format(time.RFC3339))
}
