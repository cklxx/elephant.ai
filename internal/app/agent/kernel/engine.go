package kernel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"

	"github.com/robfig/cron/v3"
)

// cronParser is the standard 5-field cron parser used throughout the kernel.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

const (
	kernelRuntimeSectionStart = "<!-- KERNEL_RUNTIME:START -->"
	kernelRuntimeSectionEnd   = "<!-- KERNEL_RUNTIME:END -->"
)

// ValidateSchedule checks whether the given cron expression is valid.
// Called at build time to fail fast on misconfiguration.
func ValidateSchedule(expr string) error {
	_, err := cronParser.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", expr, err)
	}
	return nil
}

// Engine runs the kernel agent loop: a cron-driven cycle that reads STATE.md,
// plans dispatches, and executes them via the AgentCoordinator.
type Engine struct {
	config               KernelConfig
	stateFile            *StateFile
	store                kerneldomain.Store
	planner              Planner
	executor             Executor
	logger               logging.Logger
	notifier             CycleNotifier // optional; called after non-empty cycles
	systemPromptProvider func() string

	stopped  chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup // tracks in-flight RunCycle goroutines
}

type staleDispatchRecoverer interface {
	RecoverStaleRunning(ctx context.Context, kernelID string) (int, error)
}

// SetNotifier registers an optional callback invoked after each non-empty cycle.
func (e *Engine) SetNotifier(fn func(ctx context.Context, result *kerneldomain.CycleResult, err error)) {
	e.notifier = fn
}

// SetSystemPromptProvider registers an optional provider to refresh
// SYSTEM_PROMPT.md snapshots each cycle.
func (e *Engine) SetSystemPromptProvider(fn func() string) {
	e.systemPromptProvider = fn
}

// NewEngine creates a new kernel engine.
func NewEngine(
	config KernelConfig,
	stateFile *StateFile,
	store kerneldomain.Store,
	planner Planner,
	executor Executor,
	logger logging.Logger,
) *Engine {
	return &Engine{
		config:    config,
		stateFile: stateFile,
		store:     store,
		planner:   planner,
		executor:  executor,
		logger:    logging.OrNop(logger),
		stopped:   make(chan struct{}),
	}
}

// RunCycle executes one PERCEIVE-ORIENT-DECIDE-ACT-UPDATE cycle.
func (e *Engine) RunCycle(ctx context.Context) (result *kerneldomain.CycleResult, err error) {
	start := time.Now()
	cycleID := id.NewRunID()
	defer func() {
		e.persistCycleRuntimeState(result, err)
		e.persistSystemPromptSnapshot()
	}()

	// 1. Read STATE.md (opaque text).
	stateContent, err := e.stateFile.Read()
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if stateContent == "" {
		if seedErr := e.stateFile.Seed(e.config.SeedState); seedErr != nil {
			return nil, fmt.Errorf("seed state: %w", seedErr)
		}
		stateContent = e.config.SeedState
	}

	// 2. Recover stale running dispatches from previous interrupted cycles.
	if recoverer, ok := e.store.(staleDispatchRecoverer); ok {
		recovered, recoverErr := recoverer.RecoverStaleRunning(ctx, e.config.KernelID)
		if recoverErr != nil {
			e.logger.Warn("Kernel: recover stale dispatches failed: %v", recoverErr)
		} else if recovered > 0 {
			e.logger.Warn("Kernel: recovered %d stale running dispatch(es)", recovered)
		}
	}

	// 3. Query each agent's most recent dispatch status.
	recentByAgent, err := e.store.ListRecentByAgent(ctx, e.config.KernelID)
	if err != nil {
		e.logger.Warn("Kernel: list recent dispatches failed: %v", err)
		recentByAgent = map[string]kerneldomain.Dispatch{}
	}

	// 4. Plan — generate dispatch specs (STATE content injected into prompts).
	specs, err := e.planner.Plan(ctx, stateContent, recentByAgent)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}
	if len(specs) == 0 {
		return &kerneldomain.CycleResult{
			CycleID:  cycleID,
			KernelID: e.config.KernelID,
			Status:   kerneldomain.CycleSuccess,
			Duration: time.Since(start),
		}, nil
	}

	// 5. Enqueue dispatches to Postgres.
	dispatches, err := e.store.EnqueueDispatches(ctx, e.config.KernelID, cycleID, specs)
	if err != nil {
		return nil, fmt.Errorf("enqueue: %w", err)
	}

	// 6. Execute dispatches in parallel (bounded by MaxConcurrent).
	result = e.executeDispatches(ctx, cycleID, dispatches)
	result.Duration = time.Since(start)

	return result, nil
}

func (e *Engine) persistCycleRuntimeState(result *kerneldomain.CycleResult, cycleErr error) {
	ctx := context.Background()

	// 1. Commit pre-cycle snapshot (if versioned store is available).
	cycleLabel := "(none)"
	if result != nil {
		cycleLabel = result.CycleID
	}
	if err := e.stateFile.CommitCycleBoundary(ctx, fmt.Sprintf("pre-cycle %s", cycleLabel)); err != nil {
		e.logger.Debug("Kernel: pre-cycle commit: %v", err)
	}

	// 2. Read current STATE.md.
	stateContent, err := e.stateFile.Read()
	if err != nil {
		e.logger.Warn("Kernel: read state for runtime persistence failed: %v", err)
		return
	}
	if strings.TrimSpace(stateContent) == "" {
		stateContent = e.config.SeedState
	}

	// 3. Parse existing cycle history and prepend new entry.
	now := time.Now()
	history := parseCycleHistory(stateContent)
	newEntry := buildCycleHistoryEntry(result, cycleErr, now)
	history = append([]cycleHistoryEntry{newEntry}, history...)

	maxHistory := e.config.MaxCycleHistory
	if maxHistory <= 0 {
		maxHistory = 5
	}
	if len(history) > maxHistory {
		history = history[:maxHistory]
	}

	// 4. Render runtime block with rolling history.
	runtimeBlock := renderKernelRuntimeBlockWithHistory(result, cycleErr, now, history)
	updated := upsertKernelRuntimeBlock(stateContent, runtimeBlock)
	if err := e.stateFile.Write(updated); err != nil {
		e.logger.Warn("Kernel: persist runtime state failed: %v", err)
		return
	}

	// 5. Commit post-cycle state so the final cycle's data is captured.
	if err := e.stateFile.CommitCycleBoundary(ctx, fmt.Sprintf("post-cycle %s", cycleLabel)); err != nil {
		e.logger.Debug("Kernel: post-cycle commit: %v", err)
	}
}

func (e *Engine) persistSystemPromptSnapshot() {
	if e.systemPromptProvider == nil {
		return
	}
	prompt := strings.TrimSpace(e.systemPromptProvider())
	if prompt == "" {
		return
	}
	if err := e.stateFile.WriteSystemPrompt(RenderSystemPromptMarkdown(prompt, time.Now())); err != nil {
		e.logger.Warn("Kernel: persist system prompt snapshot failed: %v", err)
	}
}

// cycleHistoryEntry represents one row in the rolling cycle history table.
type cycleHistoryEntry struct {
	CycleID    string
	Status     string
	Dispatched string
	Succeeded  string
	Failed     string
	Summary    string
	UpdatedAt  string
}

func renderKernelRuntimeBlockWithHistory(result *kerneldomain.CycleResult, cycleErr error, now time.Time, history []cycleHistoryEntry) string {
	lines := renderKernelRuntimeLines(result, cycleErr, now)
	lines = append(lines, "")
	lines = append(lines, renderCycleHistoryTable(history))
	return strings.Join(lines, "\n")
}

func renderKernelRuntimeLines(result *kerneldomain.CycleResult, cycleErr error, now time.Time) []string {
	lines := []string{
		"## kernel_runtime",
		fmt.Sprintf("- updated_at: %s", now.UTC().Format(time.RFC3339)),
	}
	if result == nil {
		lines = append(lines,
			"- latest_cycle_id: (none)",
			"- latest_status: error",
			"- latest_dispatched: 0",
			"- latest_succeeded: 0",
			"- latest_failed: 0",
			"- latest_failed_agents: (none)",
			"- latest_agent_summary: (none)",
			"- latest_duration_ms: 0",
		)
	} else {
		failedAgents := "(none)"
		if len(result.FailedAgents) > 0 {
			failedAgents = strings.Join(result.FailedAgents, ", ")
		}
		lines = append(lines,
			fmt.Sprintf("- latest_cycle_id: %s", result.CycleID),
			fmt.Sprintf("- latest_status: %s", result.Status),
			fmt.Sprintf("- latest_dispatched: %d", result.Dispatched),
			fmt.Sprintf("- latest_succeeded: %d", result.Succeeded),
			fmt.Sprintf("- latest_failed: %d", result.Failed),
			fmt.Sprintf("- latest_failed_agents: %s", failedAgents),
			fmt.Sprintf("- latest_agent_summary: %s", renderStateAgentSummary(result.AgentSummary)),
			fmt.Sprintf("- latest_duration_ms: %d", result.Duration.Milliseconds()),
		)
	}
	if cycleErr != nil {
		lines = append(lines, fmt.Sprintf("- latest_error: %s", cycleErr.Error()))
	} else {
		lines = append(lines, "- latest_error: (none)")
	}
	return lines
}

func buildCycleHistoryEntry(result *kerneldomain.CycleResult, cycleErr error, now time.Time) cycleHistoryEntry {
	if result == nil {
		errMsg := "(none)"
		if cycleErr != nil {
			errMsg = compactSummary(cycleErr.Error(), 80)
		}
		return cycleHistoryEntry{
			CycleID:    "(none)",
			Status:     "error",
			Dispatched: "0",
			Succeeded:  "0",
			Failed:     "0",
			Summary:    errMsg,
			UpdatedAt:  now.UTC().Format(time.RFC3339),
		}
	}
	summary := renderStateAgentSummary(result.AgentSummary)
	// Compact for table cells.
	summary = compactSummary(summary, 120)
	// Replace pipe characters to avoid breaking the markdown table.
	summary = strings.ReplaceAll(summary, "|", "/")
	return cycleHistoryEntry{
		CycleID:    result.CycleID,
		Status:     string(result.Status),
		Dispatched: fmt.Sprintf("%d", result.Dispatched),
		Succeeded:  fmt.Sprintf("%d", result.Succeeded),
		Failed:     fmt.Sprintf("%d", result.Failed),
		Summary:    summary,
		UpdatedAt:  now.UTC().Format(time.RFC3339),
	}
}

func renderCycleHistoryTable(entries []cycleHistoryEntry) string {
	lines := []string{
		"### cycle_history",
		"| cycle_id | status | dispatched | succeeded | failed | summary | updated_at |",
		"|----------|--------|------------|-----------|--------|---------|------------|",
	}
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |",
			e.CycleID, e.Status, e.Dispatched, e.Succeeded, e.Failed, e.Summary, e.UpdatedAt))
	}
	return strings.Join(lines, "\n")
}

// parseCycleHistory extracts cycle_history table rows from existing STATE.md content.
// Returns empty slice if no history table is found (backward compatible).
func parseCycleHistory(content string) []cycleHistoryEntry {
	// Find "### cycle_history" section.
	idx := strings.Index(content, "### cycle_history")
	if idx < 0 {
		return nil
	}
	rest := content[idx:]
	// Stop at the runtime section end marker to avoid parsing content outside the block.
	if endIdx := strings.Index(rest, kernelRuntimeSectionEnd); endIdx > 0 {
		rest = rest[:endIdx]
	}

	var entries []cycleHistoryEntry
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		// Skip header and separator rows.
		if strings.Contains(line, "cycle_id") || strings.Contains(line, "--------") {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 7 {
			continue
		}
		entries = append(entries, cycleHistoryEntry{
			CycleID:    cells[0],
			Status:     cells[1],
			Dispatched: cells[2],
			Succeeded:  cells[3],
			Failed:     cells[4],
			Summary:    cells[5],
			UpdatedAt:  cells[6],
		})
	}
	return entries
}

// splitTableRow splits a markdown table row "|a|b|c|" into trimmed cell values.
func splitTableRow(line string) []string {
	// Trim leading/trailing pipe.
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func renderStateAgentSummary(entries []kerneldomain.AgentCycleSummary) string {
	if len(entries) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		status := string(entry.Status)
		if status == "" {
			status = string(kerneldomain.DispatchDone)
		}
		summary := strings.TrimSpace(entry.Summary)
		if summary == "" {
			summary = strings.TrimSpace(entry.Error)
		}
		if summary == "" {
			summary = "(none)"
		}
		summary = compactSummary(summary, 180)
		parts = append(parts, fmt.Sprintf("%s[%s]: %s", entry.AgentID, status, summary))
	}
	return strings.Join(parts, " | ")
}

func upsertKernelRuntimeBlock(content, runtimeBlock string) string {
	block := kernelRuntimeSectionStart + "\n" + runtimeBlock + "\n" + kernelRuntimeSectionEnd

	start := strings.Index(content, kernelRuntimeSectionStart)
	end := strings.Index(content, kernelRuntimeSectionEnd)
	if start >= 0 && end > start {
		end += len(kernelRuntimeSectionEnd)
		prefix := strings.TrimRight(content[:start], "\n")
		suffix := strings.TrimLeft(content[end:], "\n")

		var out strings.Builder
		if prefix != "" {
			out.WriteString(prefix)
			out.WriteString("\n\n")
		}
		out.WriteString(block)
		if suffix != "" {
			out.WriteString("\n\n")
			out.WriteString(suffix)
		}
		out.WriteString("\n")
		return out.String()
	}

	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return block + "\n"
	}
	return trimmed + "\n\n" + block + "\n"
}

// executeDispatches runs dispatches concurrently with a bounded semaphore.
func (e *Engine) executeDispatches(ctx context.Context, cycleID string, dispatches []kerneldomain.Dispatch) *kerneldomain.CycleResult {
	result := &kerneldomain.CycleResult{
		CycleID:    cycleID,
		KernelID:   e.config.KernelID,
		Dispatched: len(dispatches),
	}

	maxConcurrent := e.config.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	sem := make(chan struct{}, maxConcurrent)

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, d := range dispatches {
		wg.Add(1)
		go func(d kerneldomain.Dispatch) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if err := e.store.MarkDispatchRunning(ctx, d.DispatchID); err != nil {
				e.logger.Warn("Kernel: mark running %s: %v", d.DispatchID, err)
			}

			// Copy metadata to avoid concurrent mutation of the shared map.
			meta := make(map[string]string, len(d.Metadata)+2)
			for k, v := range d.Metadata {
				meta[k] = v
			}
			if e.config.UserID != "" {
				meta["user_id"] = e.config.UserID
			}
			if e.config.Channel != "" {
				meta["channel"] = e.config.Channel
			}
			if e.config.ChatID != "" {
				meta["chat_id"] = e.config.ChatID
			}

			execResult, execErr := e.executor.Execute(ctx, d.AgentID, d.Prompt, meta)

			mu.Lock()
			defer mu.Unlock()

			if execErr != nil {
				result.Failed++
				result.FailedAgents = append(result.FailedAgents, d.AgentID)
				result.AgentSummary = append(result.AgentSummary, kerneldomain.AgentCycleSummary{
					AgentID: d.AgentID,
					Status:  kerneldomain.DispatchFailed,
					Error:   execErr.Error(),
				})
				if markErr := e.store.MarkDispatchFailed(ctx, d.DispatchID, execErr.Error()); markErr != nil {
					e.logger.Warn("Kernel: mark failed %s: %v", d.DispatchID, markErr)
				}
				e.logger.Warn("Kernel: dispatch %s (agent=%s) failed: %v", d.DispatchID, d.AgentID, execErr)
			} else {
				result.Succeeded++
				result.AgentSummary = append(result.AgentSummary, kerneldomain.AgentCycleSummary{
					AgentID: d.AgentID,
					TaskID:  execResult.TaskID,
					Status:  kerneldomain.DispatchDone,
					Summary: execResult.Summary,
				})
				if markErr := e.store.MarkDispatchDone(ctx, d.DispatchID, execResult.TaskID); markErr != nil {
					e.logger.Warn("Kernel: mark done %s: %v", d.DispatchID, markErr)
				}
			}
		}(d)
	}

	wg.Wait()
	sort.Slice(result.AgentSummary, func(i, j int) bool {
		if result.AgentSummary[i].AgentID == result.AgentSummary[j].AgentID {
			return result.AgentSummary[i].Status < result.AgentSummary[j].Status
		}
		return result.AgentSummary[i].AgentID < result.AgentSummary[j].AgentID
	})

	switch {
	case result.Failed == 0:
		result.Status = kerneldomain.CycleSuccess
	case result.Succeeded > 0:
		result.Status = kerneldomain.CyclePartialSuccess
	default:
		result.Status = kerneldomain.CycleFailed
	}

	return result
}

// Run starts the main loop, scheduling RunCycle according to the cron expression.
// It blocks until the context is cancelled or Stop is called.
func (e *Engine) Run(ctx context.Context) {
	sched, err := cronParser.Parse(e.config.Schedule)
	if err != nil {
		// Schedule was already validated at build time; this is defensive.
		e.logger.Warn("Kernel: invalid schedule %q: %v", e.config.Schedule, err)
		return
	}

	e.logger.Info("Kernel[%s] starting (schedule=%s)", e.config.KernelID, e.config.Schedule)

	for {
		nextRun := sched.Next(time.Now())
		timer := time.NewTimer(time.Until(nextRun))

		select {
		case <-ctx.Done():
			timer.Stop()
			e.logger.Info("Kernel[%s] stopped (context cancelled)", e.config.KernelID)
			return
		case <-e.stopped:
			timer.Stop()
			e.logger.Info("Kernel[%s] stopped", e.config.KernelID)
			return
		case <-timer.C:
			e.wg.Add(1)
			func() {
				defer e.wg.Done()
				result, cycleErr := e.RunCycle(ctx)
				if cycleErr != nil {
					e.logger.Warn("Kernel[%s] RunCycle error: %v", e.config.KernelID, cycleErr)
				} else {
					e.logger.Info("Kernel[%s] cycle %s: %s (dispatched=%d ok=%d fail=%d %s)",
						e.config.KernelID, result.CycleID, result.Status,
						result.Dispatched, result.Succeeded, result.Failed, result.Duration)
				}
				// Notify on non-empty cycles or errors.
				if e.notifier != nil {
					if cycleErr != nil || (result != nil && result.Dispatched > 0) {
						e.notifier(ctx, result, cycleErr)
					}
				}
			}()
		}
	}
}

// Stop signals the engine to exit the Run loop.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() { close(e.stopped) })
}

// Name returns the subsystem name for lifecycle management.
func (e *Engine) Name() string { return "kernel" }

// Drain gracefully stops the engine and waits for any in-flight cycle to finish.
func (e *Engine) Drain(_ context.Context) error {
	e.Stop()
	e.wg.Wait()
	return nil
}
