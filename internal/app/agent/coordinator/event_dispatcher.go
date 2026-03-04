package coordinator

import (
	"context"
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
	toolspolicy "alex/internal/infra/tools"
)

// EventDispatcher provides a single assembly point for the coordinator event
// pipeline (envelope translation, SLA enrichment, plan-title extraction, and
// per-run serialization).
type EventDispatcher interface {
	Listener() agent.EventListener
	Flush(ctx context.Context, runID string)
	Title() string
}

// EventDispatcherOptions controls optional pipeline stages.
type EventDispatcherOptions struct {
	EnablePlanTitle bool
	OnPlanTitle     func(string)
}

type eventStage interface {
	Wrap(listener agent.EventListener) agent.EventListener
}

type workflowEnvelopeStage struct{}

func (workflowEnvelopeStage) Wrap(listener agent.EventListener) agent.EventListener {
	return wrapWithWorkflowEnvelope(listener)
}

type slaEnrichmentStage struct {
	collector *toolspolicy.SLACollector
}

func (s slaEnrichmentStage) Wrap(listener agent.EventListener) agent.EventListener {
	return wrapWithSLAEnrichment(listener, s.collector)
}

type defaultEventDispatcher struct {
	listener      agent.EventListener
	serializing   *SerializingEventListener
	titleRecorder *planSessionTitleRecorder
}

// NewEventDispatcher builds the coordinator event pipeline as a single unit.
func NewEventDispatcher(listener agent.EventListener, collector *toolspolicy.SLACollector, opts EventDispatcherOptions) EventDispatcher {
	if listener == nil {
		return &defaultEventDispatcher{}
	}

	sink := listener
	stages := []eventStage{
		slaEnrichmentStage{collector: collector},
		workflowEnvelopeStage{},
	}
	for _, stage := range stages {
		sink = stage.Wrap(sink)
	}

	var titleRecorder *planSessionTitleRecorder
	if opts.EnablePlanTitle {
		titleRecorder = &planSessionTitleRecorder{
			sink:    sink,
			onTitle: opts.OnPlanTitle,
		}
		sink = titleRecorder
	}

	serializing := NewSerializingEventListener(sink)
	if serializing != nil {
		sink = serializing
	}

	return &defaultEventDispatcher{
		listener:      sink,
		serializing:   serializing,
		titleRecorder: titleRecorder,
	}
}

func (d *defaultEventDispatcher) Listener() agent.EventListener {
	if d == nil {
		return nil
	}
	return d.listener
}

func (d *defaultEventDispatcher) Flush(ctx context.Context, runID string) {
	if d == nil || d.serializing == nil {
		return
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		runID = "unknown"
	}
	d.serializing.Flush(ctx, runID)
}

func (d *defaultEventDispatcher) Title() string {
	if d == nil || d.titleRecorder == nil {
		return ""
	}
	return d.titleRecorder.Title()
}
