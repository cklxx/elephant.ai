package react

import (
	"context"
	"strings"
	"sync"
	"time"

	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
)

// reactRuntime wraps the ReAct loop with explicit lifecycle bookkeeping so the
// engine can focus on business rules while the runtime owns iteration control,
// workflow transitions, and cancellation handling.
type reactRuntime struct {
	engine       *ReactEngine
	ctx          context.Context
	task         string
	state        *TaskState
	services     Services
	tracker      *reactWorkflow
	startTime    time.Time
	resultOnce   sync.Once
	workflowOnce sync.Once
	prepare      func()

	// UI orchestration state (Plan → Clarify → ReAct → Finalize).
	runID                 string
	planEmitted           bool
	planVersion           int
	planComplexity        string
	planReviewEnabled     bool
	lastPlanReviewVersion int
	currentTaskID         string
	clarifyEmitted        map[string]bool
	pendingTaskID         string
	nextTaskSeq           int
	pauseRequested        bool

	// Background task manager for async agent execution.
	bgManager      *BackgroundTaskManager
	bgManagerOwned bool
	// Track emitted completion events to avoid duplicates.
	bgCompletionEmitted map[string]bool
	// External input requests from interactive external agents.
	externalInputCh      <-chan agent.InputRequest
	externalInputEmitted map[string]bool

	// User input channel for live message injection from chat gateways.
	userInputCh <-chan agent.UserInput
	// Consecutive non-recoverable tool failures used to prevent retry loops.
	lastNonRetryableToolFailure  string
	consecutiveNonRetryableFails int
}

const (
	planStatusPending    = "pending"
	planStatusInProgress = "in_progress"
	planStatusBlocked    = "blocked"
	planStatusCompleted  = "completed"
)

const repeatedNonRetryableToolFailureThreshold = 3

func newReactRuntime(engine *ReactEngine, ctx context.Context, task string, state *TaskState, services Services, prepare func()) *reactRuntime {
	runtime := &reactRuntime{
		engine:    engine,
		ctx:       ctx,
		task:      task,
		state:     state,
		services:  services,
		tracker:   newReactWorkflow(engine.workflow),
		startTime: engine.clock.Now(),
		prepare:   prepare,
	}
	if state != nil {
		runtime.runID = strings.TrimSpace(state.RunID)
		runtime.planReviewEnabled = state.PlanReviewEnabled
	}
	runtime.clarifyEmitted = make(map[string]bool)
	runtime.nextTaskSeq = 1
	runtime.userInputCh = agent.UserInputChFromContext(ctx)

	// Initialize background task manager when executor is available.
	if engine.backgroundExecutor != nil || engine.backgroundManager != nil {
		sessionID := ""
		runID := ""
		parentRunID := ""
		if state != nil {
			sessionID = state.SessionID
			runID = state.RunID
			parentRunID = state.ParentRunID
		}
		if engine.backgroundManager != nil {
			runtime.bgManager = engine.backgroundManager
			runtime.bgManagerOwned = false
		} else {
			runtime.bgManager = newBackgroundTaskManagerWithDeps(
				ctx,
				engine.logger,
				engine.clock,
				engine.idGenerator,
				engine.idContextReader,
				engine.goRunner,
				engine.workingDirResolver,
				engine.workspaceMgrFactory,
				engine.backgroundExecutor,
				engine.externalExecutor,
				engine.emitEvent,
				func(ctx context.Context) domain.BaseEvent {
					return engine.newBaseEvent(ctx, sessionID, runID, parentRunID)
				},
				sessionID,
				engine.eventListener,
				0,
			)
			runtime.bgManagerOwned = true
		}
		runtime.bgCompletionEmitted = make(map[string]bool)
		if runtime.bgManager != nil {
			runtime.ctx = withBackgroundEventSink(runtime.ctx, backgroundEventSink{
				emitEvent: engine.emitEvent,
				baseEvent: func(ctx context.Context) domain.BaseEvent {
					return engine.newBaseEvent(ctx, sessionID, runID, parentRunID)
				},
				parentListener: engine.eventListener,
			})
			runtime.externalInputCh = runtime.bgManager.InputRequests()
			runtime.externalInputEmitted = make(map[string]bool)
		}
	}

	return runtime
}

func (r *reactRuntime) run() (*TaskResult, error) {
	r.tracker.startContext(r.task)
	resumed, err := r.engine.ResumeFromCheckpoint(r.ctx, r.state.SessionID, r.state, r.services)
	if err != nil {
		r.engine.logger.Warn("Failed to resume from checkpoint: %v", err)
	}
	if resumed {
		r.tracker.completeContext(workflowContextOutput(r.state))
	} else {
		r.prepareContext()
	}

	// Inject orchestration context (dispatcher + team defs + recorder) in a
	// single context.WithValue call for tools.
	{
		oc := agent.OrchestrationContext{
			TeamDefinitions: r.engine.teamDefinitions,
			TeamRunRecorder: r.engine.teamRunRecorder,
		}
		if r.bgManager != nil {
			oc.Dispatcher = newBackgroundDispatcherWithEvents(r, r.bgManager)
		}
		r.ctx = agent.WithOrchestrationContext(r.ctx, oc)
	}

	for r.state.Iterations < r.engine.maxIterations {
		// Inject background completion notifications before each iteration.
		r.injectBackgroundNotifications()
		r.injectExternalInputRequests()
		r.injectUserInput()

		if result, stop, err := r.handleCancellation(); stop || err != nil {
			r.cleanupBackgroundTasks()
			return result, err
		}

		r.state.Iterations++
		r.engine.logger.Debug("Iteration %d/%d", r.state.Iterations, r.engine.maxIterations)

		result, done, err := r.runIteration()

		// Async persist session state after successful iteration (best-effort).
		r.persistSessionAfterIteration()

		if err != nil {
			r.cleanupBackgroundTasks()
			return nil, err
		}
		if done {
			r.cleanupBackgroundTasks()
			return result, nil
		}
	}

	r.cleanupBackgroundTasks()
	return r.handleMaxIterations()
}

func (r *reactRuntime) prepareContext() {
	if r.prepare != nil {
		r.prepare()
	} else {
		r.engine.prepareUserTaskContext(r.ctx, r.task, r.state)
	}
	r.tracker.completeContext(workflowContextOutput(r.state))
}

func (r *reactRuntime) handleCancellation() (*TaskResult, bool, error) {
	if r.ctx.Err() == nil {
		return nil, false, nil
	}

	r.engine.logger.Debug("Context cancelled, stopping execution: %v", r.ctx.Err())
	finalResult := r.finalizeResult("cancelled", nil, true, nil)
	return finalResult, true, r.ctx.Err()
}

func (r *reactRuntime) handleMaxIterations() (*TaskResult, error) {
	r.engine.logger.Warn("Max iterations (%d) reached, requesting final answer", r.engine.maxIterations)
	finalResult := r.engine.finalize(r.state, "max_iterations", r.engine.clock.Now().Sub(r.startTime))

	if utils.IsBlank(finalResult.Answer) {
		r.state.Messages = append(r.state.Messages, Message{
			Role:    "user",
			Content: "Please provide your final answer to the user's question now.",
			Source:  ports.MessageSourceSystemPrompt,
		})

		finalThought, err := r.engine.think(r.ctx, r.state, r.services)
		if err == nil && finalThought.Content != "" {
			if att := resolveContentAttachments(finalThought.Content, r.state); len(att) > 0 {
				finalThought.Attachments = att
			}
			r.state.Messages = append(r.state.Messages, finalThought)
			if registerMessageAttachments(r.ctx, r.state, &r.state.Messages[len(r.state.Messages)-1], r.engine.attachmentPersister) {
				r.engine.updateAttachmentCatalogMessage(r.state)
			}
			finalResult.Answer = finalThought.Content
			r.engine.logger.Debug("Final answer retry produced %d chars", len(finalResult.Answer))
		}
	}

	return r.finalizeResult("max_iterations", finalResult, true, nil), nil
}

func (r *reactRuntime) emitWorkflowToolStartedEvents(calls []ToolCall) {
	if len(calls) == 0 {
		return
	}

	state := r.state
	for idx := range calls {
		call := calls[idx]
		r.engine.emitEvent(domain.NewToolStartedEvent(
			r.engine.newBaseEvent(r.ctx, state.SessionID, state.RunID, state.ParentRunID),
			state.Iterations, call.ID, call.Name, call.Arguments,
		))
	}
}

func (r *reactRuntime) finishWorkflow(stopReason string, result *TaskResult, err error) {
	r.workflowOnce.Do(func() {
		if r.tracker != nil {
			r.tracker.finalize(stopReason, result, err)
		}
	})
}
