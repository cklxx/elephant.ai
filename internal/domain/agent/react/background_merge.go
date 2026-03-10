package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
)

func (m *BackgroundTaskManager) tryAutoMerge(ctx context.Context, bt *backgroundTask, result *agent.TaskResult) error {
	bt.mu.Lock()
	cfg := core.CloneStringMap(bt.config)
	alloc := bt.workspace
	if bt.mergeStatus == "" {
		bt.mergeStatus = agent.MergeStatusNotMerged
	}
	bt.mu.Unlock()

	if !strings.EqualFold(strings.TrimSpace(cfg["task_kind"]), "coding") {
		return nil
	}
	if !parseConfigBoolDefault(cfg["merge_on_success"], true) {
		return nil
	}
	if !parseConfigBoolDefault(cfg["verify"], true) {
		bt.mu.Lock()
		bt.mergeStatus = agent.MergeStatusFailed
		bt.mu.Unlock()
		return fmt.Errorf("auto merge requires verify=true for coding tasks")
	}
	if alloc == nil || alloc.Mode == agent.WorkspaceModeShared {
		return nil
	}
	if m.workspaceMgr == nil {
		bt.mu.Lock()
		bt.mergeStatus = agent.MergeStatusFailed
		bt.mu.Unlock()
		return fmt.Errorf("auto merge requested but workspace manager is not available")
	}

	strategy := parseMergeStrategy(cfg["merge_strategy"])
	mergeResult, err := m.workspaceMgr.Merge(ctx, alloc, strategy)
	if err != nil {
		// For resolve strategy, attempt agent-based conflict resolution.
		if strategy == agent.MergeStrategyResolve && mergeResult != nil && len(mergeResult.Conflicts) > 0 {
			if resolveErr := m.tryResolveConflicts(ctx, bt, mergeResult, result); resolveErr != nil {
				bt.mu.Lock()
				bt.mergeStatus = agent.MergeStatusFailed
				bt.mu.Unlock()
				return fmt.Errorf("auto merge failed: %w (resolve also failed: %v)", err, resolveErr)
			}
			if result != nil {
				result.Answer = strings.TrimSpace(result.Answer + "\n\n[Conflict Resolved] branch=" + mergeResult.Branch + " strategy=resolve")
			}
			bt.mu.Lock()
			bt.mergeStatus = agent.MergeStatusMerged
			bt.mu.Unlock()
			return nil
		}
		bt.mu.Lock()
		bt.mergeStatus = agent.MergeStatusFailed
		bt.mu.Unlock()
		return fmt.Errorf("auto merge failed: %w", err)
	}
	if mergeResult != nil && !mergeResult.Success {
		bt.mu.Lock()
		bt.mergeStatus = agent.MergeStatusFailed
		bt.mu.Unlock()
		return fmt.Errorf("auto merge failed for branch %q", mergeResult.Branch)
	}
	if result != nil && mergeResult != nil && utils.HasContent(mergeResult.Branch) {
		result.Answer = strings.TrimSpace(result.Answer + "\n\n[Auto Merge] branch=" + mergeResult.Branch + " strategy=" + string(strategy))
	}
	bt.mu.Lock()
	bt.mergeStatus = agent.MergeStatusMerged
	bt.mu.Unlock()
	return nil
}

// tryResolveConflicts dispatches a sub-agent to resolve merge conflict markers
// left in the working tree by a failed auto merge. Returns nil if the resolver
// successfully committed the merge.
func (m *BackgroundTaskManager) tryResolveConflicts(
	ctx context.Context,
	bt *backgroundTask,
	mergeResult *agent.MergeResult,
	result *agent.TaskResult,
) error {
	resolverID := bt.id + "-conflict-resolver"
	prompt := m.buildConflictResolverPrompt(bt, mergeResult)

	bt.mu.Lock()
	resolverConfig := core.CloneStringMap(bt.config)
	bt.mu.Unlock()
	resolverConfig["merge_on_success"] = "false"
	resolverConfig["verify"] = "false"
	resolverConfig["autonomy_level"] = "full"

	if err := m.Dispatch(ctx, agent.BackgroundDispatchRequest{
		TaskID:        resolverID,
		Description:   "resolve merge conflicts for " + bt.id,
		Prompt:        prompt,
		AgentType:     bt.agentType,
		WorkspaceMode: agent.WorkspaceModeShared,
		Config:        resolverConfig,
		CausationID:   bt.causationID,
	}); err != nil {
		return fmt.Errorf("dispatch conflict resolver: %w", err)
	}

	results := m.Collect([]string{resolverID}, true, 10*time.Minute)
	if len(results) == 0 {
		return fmt.Errorf("conflict resolver %q produced no result", resolverID)
	}
	if results[0].Status != agent.BackgroundTaskStatusCompleted {
		errMsg := results[0].Error
		if errMsg == "" {
			errMsg = "unknown failure"
		}
		return fmt.Errorf("conflict resolver %q failed: %s", resolverID, errMsg)
	}
	return nil
}

// buildConflictResolverPrompt constructs the prompt for the conflict resolver agent.
func (m *BackgroundTaskManager) buildConflictResolverPrompt(bt *backgroundTask, mergeResult *agent.MergeResult) string {
	bt.mu.Lock()
	description := bt.description
	taskResult := ""
	if bt.result != nil {
		taskResult = bt.result.Answer
	}
	baseBranch := ""
	if bt.workspace != nil {
		baseBranch = bt.workspace.BaseBranch
	}
	bt.mu.Unlock()

	conflictFiles := strings.Join(mergeResult.Conflicts, "\n")
	var sb strings.Builder
	sb.WriteString("[MERGE CONFLICT RESOLUTION]\n\n")
	sb.WriteString("Branch: " + mergeResult.Branch + " → " + baseBranch + "\n")
	sb.WriteString("Conflicting files:\n" + conflictFiles + "\n\n")
	sb.WriteString("Original task: " + description + "\n")
	sb.WriteString("Task result:\n" + taskResult + "\n\n")
	if mergeResult.ConflictDiff != "" {
		sb.WriteString("Conflict diff (files with markers):\n" + mergeResult.ConflictDiff + "\n\n")
	}
	sb.WriteString("Instructions:\n")
	sb.WriteString("1. Read each conflicting file — they contain <<<<<<, =======, >>>>>>> markers\n")
	sb.WriteString("2. The HEAD side is the current main branch; the task-branch side contains the task's changes\n")
	sb.WriteString("3. Resolve each conflict intelligently, preserving the intent of both sides\n")
	sb.WriteString("4. git add " + strings.Join(mergeResult.Conflicts, " ") + "\n")
	sb.WriteString("5. git commit --no-edit   ← completes the in-progress merge, do not create a new message\n")
	return sb.String()
}

func parseConfigBoolDefault(raw string, fallback bool) bool {
	trimmed := utils.TrimLower(raw)
	if trimmed == "" {
		return fallback
	}
	switch trimmed {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseMergeStrategy(raw string) agent.MergeStrategy {
	switch utils.TrimLower(raw) {
	case string(agent.MergeStrategySquash):
		return agent.MergeStrategySquash
	case string(agent.MergeStrategyRebase):
		return agent.MergeStrategyRebase
	case string(agent.MergeStrategyReview):
		return agent.MergeStrategyReview
	case string(agent.MergeStrategyResolve):
		return agent.MergeStrategyResolve
	default:
		return agent.MergeStrategyAuto
	}
}
