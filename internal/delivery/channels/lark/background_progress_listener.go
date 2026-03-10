package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
)

const (
	defaultBackgroundProgressInterval       = 10 * time.Minute
	defaultBackgroundProgressWindow         = 10 * time.Minute
	codeBackgroundProgressInterval          = 3 * time.Minute
	maxBackgroundListenerLifetime           = 4 * time.Hour
	defaultTeamCompletionSummaryLLMTimeout  = 10 * time.Second
	teamCompletionSummaryMaxPromptChars     = 3000
	teamCompletionSummaryMaxReplyChars      = 1200
	teamCompletionSummaryMinTasks           = 2
)

// completedTaskRecord captures the final state of a completed background task
// for team-level summary generation.
type completedTaskRecord struct {
	taskID      string
	description string
	status      string
	answer      string
	errText     string
	duration    time.Duration
}

type progressRecord struct {
	ts          time.Time
	currentTool string
	currentArgs string
	tokensUsed  int
	files       []string
	activity    time.Time
}

type bgTaskTracker struct {
	mu sync.Mutex

	taskID      string
	description string
	agentType   string
	startedAt   time.Time

	status         string
	progressMsgID  string
	pendingSummary string
	mergeStatus    string

	interval time.Duration
	window   time.Duration

	lastProgress progressRecord
	recent       []progressRecord

	stopCh chan struct{}
	doneCh chan struct{}
}

func (t *bgTaskTracker) stop() {
	select {
	case <-t.doneCh:
		return
	default:
	}
	select {
	case <-t.stopCh:
		// already closed
	default:
		close(t.stopCh)
	}
	<-t.doneCh
}

type backgroundProgressListener struct {
	inner     agent.EventListener
	ctx       context.Context
	g         *Gateway
	chatID    string
	replyToID string
	logger    logging.Logger
	now       func() time.Time
	interval  time.Duration
	window    time.Duration

	mu             sync.Mutex
	tasks          map[string]*bgTaskTracker
	completedTasks []completedTaskRecord
	closed         bool
	released       bool
	pollerInterval time.Duration // configurable for testing; defaults to 30s
	doneCh         chan struct{} // closed in Close() to stop the poller
	doneOnce       sync.Once
}

func newBackgroundProgressListener(ctx context.Context, inner agent.EventListener, g *Gateway, chatID, replyToID string, logger logging.Logger, interval, window time.Duration) *backgroundProgressListener {
	if interval <= 0 {
		interval = defaultBackgroundProgressInterval
	}
	if window <= 0 {
		window = defaultBackgroundProgressWindow
	}
	return &backgroundProgressListener{
		inner:          inner,
		ctx:            context.WithoutCancel(ctx),
		g:              g,
		chatID:         chatID,
		replyToID:      replyToID,
		logger:         logging.OrNop(logger),
		now:            time.Now,
		interval:       interval,
		window:         window,
		tasks:          make(map[string]*bgTaskTracker),
		pollerInterval: 30 * time.Second,
		doneCh:         make(chan struct{}),
	}
}

func (l *backgroundProgressListener) Close() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	tasks := make([]*bgTaskTracker, 0, len(l.tasks))
	for _, t := range l.tasks {
		tasks = append(tasks, t)
	}
	l.mu.Unlock()

	// Signal the completion poller to stop.
	l.doneOnce.Do(func() { close(l.doneCh) })

	for _, t := range tasks {
		t.stop()
	}
}

// Release marks the foreground caller as done. If no background tasks are
// tracked, it closes the listener immediately. Otherwise, it defers closing
// until the last tracked task completes. A safety-net timer prevents leaks
// if a completion event is lost.
func (l *backgroundProgressListener) Release() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.released = true
	shouldClose := len(l.tasks) == 0
	l.mu.Unlock()

	if shouldClose {
		l.Close()
		return
	}

	// Start the completion poller — polls TaskStore as a safety net in case
	// both the normal and direct event paths fail (e.g. process crash/OOM).
	go l.pollForCompletions()

	// Safety net: prevent leaks if completion event is lost.
	go func() {
		t := time.NewTimer(maxBackgroundListenerLifetime)
		defer t.Stop()
		select {
		case <-t.C:
			l.logger.Warn("backgroundProgressListener force-closing after max lifetime")
			l.Close()
		case <-l.doneCh:
		}
	}()
}

func (l *backgroundProgressListener) OnEvent(event agent.AgentEvent) {
	if l.inner != nil {
		l.inner.OnEvent(event)
	}

	switch e := event.(type) {
	case *domain.WorkflowEventEnvelope:
		l.onEnvelope(e)
	case *domain.Event:
		// Direct bypass path: BackgroundTaskManager sends completion events
		// directly here when the SerializingEventListener queue may be dead.
		// Dedup is safe: getTask returns nil after the first handler deletes the task.
		if e.Kind == types.EventBackgroundTaskCompleted {
			l.onRawCompleted(e)
		}
	}
}

func (l *backgroundProgressListener) onEnvelope(env *domain.WorkflowEventEnvelope) {
	if env == nil {
		return
	}

	switch strings.TrimSpace(env.Event) {
	case types.EventBackgroundTaskDispatched:
		l.onBackgroundDispatched(env)
	case types.EventExternalAgentProgress:
		l.onExternalProgress(env)
	case types.EventExternalInputRequested:
		l.onExternalInputRequested(env)
	case types.EventBackgroundTaskCompleted:
		l.onBackgroundCompleted(env)
	}
}

func (l *backgroundProgressListener) getTask(taskID string) *bgTaskTracker {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	return l.tasks[taskID]
}

// buildHumanHeader returns a human-friendly initial message for a background task.
func (l *backgroundProgressListener) buildHumanHeader(t *bgTaskTracker, title string) string {
	desc := strings.TrimSpace(t.description)
	if desc != "" {
		return fmt.Sprintf("%s — %s", title, truncateForLark(desc, 100))
	}
	return title
}

func (l *backgroundProgressListener) clock() time.Time {
	if l.now != nil {
		return l.now()
	}
	return time.Now()
}

func (l *backgroundProgressListener) taskInterval(agentType string) time.Duration {
	interval := l.interval
	if isCodeAgentType(agentType) {
		interval = minDuration(interval, codeBackgroundProgressInterval)
	}
	if interval <= 0 {
		return defaultBackgroundProgressInterval
	}
	return interval
}

func (l *backgroundProgressListener) taskWindow(_ string) time.Duration {
	if l.window <= 0 {
		return defaultBackgroundProgressWindow
	}
	return l.window
}
