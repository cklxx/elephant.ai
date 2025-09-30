package agent

import (
	"fmt"
	"math/rand"
	"time"

	"alex/pkg/types"
)

// generateTaskID - 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d_%d", time.Now().UnixNano(), rand.Intn(1000))
}

// buildFinalResult - 构建最终结果
func buildFinalResult(taskCtx *types.ReactTaskContext, answer string, confidence float64, success bool) *types.ReactTaskResult {
	totalDuration := time.Since(taskCtx.StartTime)

	return &types.ReactTaskResult{
		Success:          success,
		Answer:           answer,
		Confidence:       confidence,
		Steps:            taskCtx.History,
		Duration:         totalDuration,
		TokensUsed:       taskCtx.TokensUsed,
		PromptTokens:     taskCtx.PromptTokens,
		CompletionTokens: taskCtx.CompletionTokens,
	}
}
