package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"alex/internal/delivery/server/app"
	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/infra/observability"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"

	"go.opentelemetry.io/otel/attribute"
)

const (
	sseSeenEventIDsCacheSize = 10000
	sseRunSeqCacheSize       = 2048
	sseHeartbeatInterval     = 30 * time.Second
)

type sessionSSERequest struct {
	sessionID             string
	debugMode             bool
	includeSessionHistory bool
	includeGlobalHistory  bool
}

type taskSSERequest struct {
	taskID    string
	sessionID string
}

type sseEventSenderConfig struct {
	request               *http.Request
	writer                http.ResponseWriter
	flusher               http.Flusher
	logger                logging.Logger
	debugMode             bool
	filter                func(agent.AgentEvent) bool
	recordMetrics         bool
	serializeFailureLabel string
	writeFailureLabel     string
}

type sessionStreamLifecycle struct {
	ctx               context.Context
	closeReason       string
	spanSetAttributes func(...attribute.KeyValue)
	finish            func(error)
	finished          bool
}

func (l *sessionStreamLifecycle) Finish(err error) {
	if l == nil || l.finished {
		return
	}
	l.finished = true
	if l.finish != nil {
		l.finish(err)
	}
}

// HandleSSEStream handles SSE connection for real-time event streaming
func (h *SSEHandler) HandleSSEStream(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context(), h.logger)
	req, err := parseSessionSSERequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lifecycle := h.startSessionStreamLifecycle(r.Context(), req.sessionID)
	r = r.WithContext(lifecycle.ctx)
	defer lifecycle.Finish(nil)

	flusher, err := prepareSSEStream(w, logger, "SSE")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info("SSE connection established for session: %s", req.sessionID)
	clientChan := make(chan agent.AgentEvent, 100)
	h.broadcaster.RegisterClient(req.sessionID, clientChan)
	defer h.broadcaster.UnregisterClient(req.sessionID, clientChan)

	if err := h.writeSessionConnectedEvent(w, r, flusher, req.sessionID, lifecycle.spanSetAttributes); err != nil {
		lifecycle.closeReason = "connected_write_failed"
		lifecycle.Finish(err)
		return
	}

	sendEvent := h.newEventSender(sseEventSenderConfig{
		request:               r,
		writer:                w,
		flusher:               flusher,
		logger:                logger,
		debugMode:             req.debugMode,
		recordMetrics:         true,
		serializeFailureLabel: "Failed to serialize event",
		writeFailureLabel:     "Failed to send SSE message",
	})

	if req.includeGlobalHistory {
		if err := h.replayHistory(r.Context(), app.EventHistoryFilter{SessionID: ""}, sendEvent); err != nil {
			logger.Warn("Failed to replay global events: %v", err)
		}
	}

	if req.includeSessionHistory {
		if err := h.replayHistory(r.Context(), app.EventHistoryFilter{SessionID: req.sessionID}, sendEvent); err != nil {
			logger.Warn("Failed to replay historical events for session %s: %v", req.sessionID, err)
		} else {
			logger.Info("Completed replaying historical events for session: %s", req.sessionID)
		}
	}
	drainQueuedEvents(clientChan, sendEvent)
	lifecycle.closeReason = h.runSessionStreamLoop(r.Context(), w, flusher, clientChan, logger, req.sessionID, sendEvent)
}

func shouldSendEvent(event agent.AgentEvent, seenEventIDs *stringLRU, lastSeqByRun *runSeqLRU) bool {
	if event == nil {
		return false
	}
	if seq := event.GetSeq(); seq > 0 {
		runID := strings.TrimSpace(event.GetRunID())
		if runID != "" {
			if last, ok := lastSeqByRun.Get(runID); ok && seq <= last {
				return false
			}
			lastSeqByRun.Set(runID, seq)
		}
	}
	if eventID := strings.TrimSpace(event.GetEventID()); eventID != "" {
		if _, ok := seenEventIDs.Get(eventID); ok {
			return false
		}
		seenEventIDs.Set(eventID, "")
	}
	return true
}

func (h *SSEHandler) shouldStreamEvent(event agent.AgentEvent, debugMode bool) bool {
	if event == nil {
		return false
	}
	base := app.BaseAgentEvent(event)
	if base == nil {
		return false
	}

	// Context snapshots are stored for debugging and analytics but contain
	// sensitive/internal details that don't need to be pushed to clients in
	// real time.
	if base.EventType() == types.EventDiagnosticContextSnapshot {
		return false
	}

	// Only stream events that are meaningful to the frontend experience.
	if !sseAllowlist[base.EventType()] {
		if !debugMode || !sseDebugAllowlist[base.EventType()] {
			return false
		}
	}

	// Only stream workflow envelopes and explicit user task submissions.
	switch e := base.(type) {
	case *domain.Event:
		if e.Kind == types.EventInputReceived {
			return true
		}
		return false
	case *domain.WorkflowEventEnvelope:
		if env, ok := event.(*domain.WorkflowEventEnvelope); ok && !debugMode {
			if blockedNodeIDs[env.NodeID] {
				return false
			}
			for _, prefix := range blockedNodePrefixes {
				if strings.HasPrefix(env.NodeID, prefix) {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

type responseWriterUnwrapper interface {
	Unwrap() http.ResponseWriter
}

// resolveHTTPFlusher unwraps middleware layers until it finds a writer
// that supports http.Flusher so SSE streaming can proceed.
func resolveHTTPFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	checked := 0
	current := w
	for current != nil && checked < 16 {
		if flusher, ok := current.(http.Flusher); ok {
			return flusher, true
		}
		unwrapper, ok := current.(responseWriterUnwrapper)
		if !ok {
			break
		}
		next := unwrapper.Unwrap()
		if next == nil || next == current {
			break
		}
		current = next
		checked++
	}
	return nil, false
}

// HandleTaskSSEStream handles SSE connection scoped to a single task (run_id).
// It subscribes to the session's event stream and filters for events matching
// the requested task_id.
func (h *SSEHandler) HandleTaskSSEStream(w http.ResponseWriter, r *http.Request) {
	logger := logging.FromContext(r.Context(), h.logger)
	req, err := parseTaskSSERequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, err := prepareSSEStream(w, logger, "task SSE")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info("Task SSE connection established: task=%s session=%s", req.taskID, req.sessionID)

	clientChan := make(chan agent.AgentEvent, 100)
	h.broadcaster.RegisterClient(req.sessionID, clientChan)
	defer h.broadcaster.UnregisterClient(req.sessionID, clientChan)

	if err := writeAndFlushSSEEvent(w, flusher, "connected", fmt.Sprintf(
		"{\"task_id\":\"%s\",\"session_id\":\"%s\"}",
		req.taskID, req.sessionID,
	)); err != nil {
		logger.Error("Failed to send task SSE connection message: %v", err)
		return
	}

	matchesTask := func(event agent.AgentEvent) bool {
		return event != nil && strings.TrimSpace(event.GetRunID()) == req.taskID
	}
	sendEvent := h.newEventSender(sseEventSenderConfig{
		request:               r,
		writer:                w,
		flusher:               flusher,
		logger:                logger,
		filter:                matchesTask,
		serializeFailureLabel: "Failed to serialize task event",
		writeFailureLabel:     "Failed to send task SSE message",
	})

	_ = h.replayHistory(r.Context(), app.EventHistoryFilter{SessionID: req.sessionID}, sendEvent)
	drainQueuedEvents(clientChan, sendEvent)
	h.runTaskStreamLoop(r.Context(), w, flusher, clientChan, logger, req.taskID, matchesTask, sendEvent)
}

func parseSessionSSERequest(r *http.Request) (sessionSSERequest, error) {
	sessionID, err := extractRequiredSessionIDFromQuery(r)
	if err != nil {
		return sessionSSERequest{}, err
	}

	replayMode := utils.TrimLower(r.URL.Query().Get("replay"))
	req := sessionSSERequest{
		sessionID:             sessionID,
		debugMode:             strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("debug")), "1") || strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("debug")), "true"),
		includeSessionHistory: true,
		includeGlobalHistory:  true,
	}
	switch replayMode {
	case "", "full":
	case "session":
		req.includeGlobalHistory = false
	case "none":
		req.includeSessionHistory = false
		req.includeGlobalHistory = false
	default:
		return sessionSSERequest{}, fmt.Errorf("invalid replay mode")
	}
	return req, nil
}

func parseTaskSSERequest(r *http.Request) (taskSSERequest, error) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		return taskSSERequest{}, fmt.Errorf("task_id is required")
	}
	sessionID, err := extractRequiredSessionIDFromQuery(r)
	if err != nil {
		return taskSSERequest{}, err
	}
	return taskSSERequest{taskID: taskID, sessionID: sessionID}, nil
}

func prepareSSEStream(w http.ResponseWriter, logger logging.Logger, streamName string) (http.Flusher, error) {
	setSSEHeaders(w)
	clearSSEWriteDeadline(w, logger, streamName)

	flusher, ok := resolveHTTPFlusher(w)
	if !ok {
		logger.Error("Response writer does not support streaming (type=%T)", w)
		return nil, fmt.Errorf("Streaming unsupported")
	}
	return flusher, nil
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

func clearSSEWriteDeadline(w http.ResponseWriter, logger logging.Logger, streamName string) {
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		logger.Warn("Failed to clear write deadline for %s: %v", streamName, err)
	}
}

func writeSSEEvent(w io.Writer, eventType, data string) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
	return err
}

func writeAndFlushSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType, data string) error {
	if err := writeSSEEvent(w, eventType, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func writeSSEComment(w io.Writer, comment string) error {
	_, err := fmt.Fprintf(w, ": %s\n\n", comment)
	return err
}

func (h *SSEHandler) startSessionStreamLifecycle(ctx context.Context, sessionID string) *sessionStreamLifecycle {
	lifecycle := &sessionStreamLifecycle{
		ctx:         ctx,
		closeReason: "client_closed",
		finish:      func(error) {},
	}
	if h.obs != nil && h.obs.Tracer != nil {
		tracedCtx, span := h.obs.Tracer.StartSpan(ctx, observability.SpanSSEConnection,
			attribute.String("http.route", "/api/sse"),
		)
		span.SetAttributes(attribute.String(observability.AttrSessionID, sessionID))
		lifecycle.ctx = tracedCtx
		lifecycle.spanSetAttributes = span.SetAttributes
		lifecycle.finish = func(err error) {
			if err != nil {
				span.RecordError(err)
			}
			span.SetAttributes(attribute.String("alex.sse.close_reason", lifecycle.closeReason))
			span.End()
		}
	}
	if h.obs != nil {
		startedAt := time.Now()
		h.obs.Metrics.IncrementSSEConnections(lifecycle.ctx)
		prevFinish := lifecycle.finish
		lifecycle.finish = func(err error) {
			h.obs.Metrics.DecrementSSEConnections(lifecycle.ctx)
			h.obs.Metrics.RecordSSEConnectionDuration(lifecycle.ctx, time.Since(startedAt))
			prevFinish(err)
		}
	}
	return lifecycle
}

func (h *SSEHandler) writeSessionConnectedEvent(
	w http.ResponseWriter,
	r *http.Request,
	flusher http.Flusher,
	sessionID string,
	spanSetAttributes func(...attribute.KeyValue),
) error {
	activeRunID := ""
	if h.runTracker != nil {
		activeRunID = h.runTracker.GetActiveRunID(sessionID)
	}
	if spanSetAttributes != nil && activeRunID != "" {
		spanSetAttributes(attribute.String(observability.AttrRunID, activeRunID))
	}
	streamRunID := id.RunIDFromContext(r.Context())
	if streamRunID == "" {
		streamRunID = activeRunID
	}
	payload := fmt.Sprintf(
		"{\"session_id\":\"%s\",\"run_id\":\"%s\",\"parent_run_id\":\"%s\",\"active_run_id\":\"%s\"}",
		sessionID,
		streamRunID,
		id.ParentRunIDFromContext(r.Context()),
		activeRunID,
	)
	if err := writeAndFlushSSEEvent(w, flusher, "connected", payload); err != nil {
		if h.obs != nil {
			h.obs.Metrics.RecordSSEMessage(r.Context(), "connected", "write_error", 0)
		}
		return err
	}
	if h.obs != nil {
		h.obs.Metrics.RecordSSEMessage(r.Context(), "connected", "ok", int64(len(payload)))
	}
	return nil
}

func (h *SSEHandler) newEventSender(cfg sseEventSenderConfig) func(agent.AgentEvent) bool {
	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)
	finalAnswerCache := newStringLRU(sseFinalAnswerCacheSize)
	seenEventIDs := newStringLRU(sseSeenEventIDsCacheSize)
	lastSeqByRun := newRunSeqLRU(sseRunSeqCacheSize)

	return func(event agent.AgentEvent) bool {
		if cfg.filter != nil && !cfg.filter(event) {
			return true
		}
		if !h.shouldStreamEvent(event, cfg.debugMode) {
			return true
		}
		if !shouldSendEvent(event, seenEventIDs, lastSeqByRun) {
			return true
		}

		data, err := h.serializeEvent(event, sentAttachments, finalAnswerCache)
		if err != nil {
			cfg.logger.Error("%s: %v", cfg.serializeFailureLabel, err)
			h.recordSSEEventMetric(cfg, event.EventType(), "serialization_error", 0)
			return false
		}

		payload := fmt.Sprintf("event: %s\ndata: %s\n\n", event.EventType(), data)
		if err := writeAndFlushSSEEvent(cfg.writer, cfg.flusher, event.EventType(), data); err != nil {
			cfg.logger.Error("%s: %v", cfg.writeFailureLabel, err)
			h.recordSSEEventMetric(cfg, event.EventType(), "write_error", 0)
			return false
		}

		h.recordSSEEventMetric(cfg, event.EventType(), "ok", int64(len(payload)))
		return true
	}
}

func (h *SSEHandler) recordSSEEventMetric(cfg sseEventSenderConfig, eventType, status string, size int64) {
	if !cfg.recordMetrics || h.obs == nil {
		return
	}
	h.obs.Metrics.RecordSSEMessage(cfg.request.Context(), eventType, status, size)
}

func (h *SSEHandler) replayHistory(ctx context.Context, filter app.EventHistoryFilter, send func(agent.AgentEvent) bool) error {
	return h.broadcaster.StreamHistory(ctx, filter, func(event agent.AgentEvent) error {
		send(event)
		return nil
	})
}

func drainQueuedEvents(clientChan <-chan agent.AgentEvent, send func(agent.AgentEvent) bool) {
	for {
		select {
		case event := <-clientChan:
			send(event)
		default:
			return
		}
	}
}

func (h *SSEHandler) runSessionStreamLoop(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	clientChan <-chan agent.AgentEvent,
	logger logging.Logger,
	sessionID string,
	sendEvent func(agent.AgentEvent) bool,
) string {
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case event := <-clientChan:
			if !sendEvent(event) {
				continue
			}
		case <-ticker.C:
			if err := writeSSEComment(w, "heartbeat"); err != nil {
				logger.Error("Failed to send heartbeat: %v", err)
				if h.obs != nil {
					h.obs.Metrics.RecordSSEMessage(ctx, "heartbeat", "write_error", 0)
				}
				return "heartbeat_failed"
			}
			flusher.Flush()
		case <-ctx.Done():
			logger.Info("SSE connection closed for session: %s", sessionID)
			return "context_cancelled"
		}
	}
}

func (h *SSEHandler) runTaskStreamLoop(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	clientChan <-chan agent.AgentEvent,
	logger logging.Logger,
	taskID string,
	matchesTask func(agent.AgentEvent) bool,
	sendEvent func(agent.AgentEvent) bool,
) {
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case event := <-clientChan:
			if !sendEvent(event) {
				continue
			}
			if matchesTask(event) {
				switch event.EventType() {
				case types.EventResultFinal, types.EventResultCancelled:
					logger.Info("Task SSE: task %s completed, closing stream", taskID)
					return
				}
			}
		case <-ticker.C:
			if err := writeSSEComment(w, "heartbeat"); err != nil {
				return
			}
			flusher.Flush()
		case <-ctx.Done():
			logger.Info("Task SSE connection closed: task=%s", taskID)
			return
		}
	}
}
