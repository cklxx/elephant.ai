package coordinator

import (
	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	toolspolicy "alex/internal/infra/tools"
)

// slaEventDecorator enriches tool.completed workflow envelopes with SLA
// metrics from the tool policy collector. It wraps an EventListener and
// passes all other events through unmodified.
type slaEventDecorator struct {
	sink      agent.EventListener
	collector *toolspolicy.SLACollector
}

// wrapWithSLAEnrichment returns a listener that enriches tool.completed
// events with SLA data. If collector is nil, returns the listener unchanged.
func wrapWithSLAEnrichment(listener agent.EventListener, collector *toolspolicy.SLACollector) agent.EventListener {
	if listener == nil || collector == nil {
		return listener
	}
	return &slaEventDecorator{sink: listener, collector: collector}
}

func (d *slaEventDecorator) OnEvent(evt agent.AgentEvent) {
	if env, ok := evt.(*domain.WorkflowEventEnvelope); ok && env.Event == types.EventToolCompleted {
		if toolName, _ := env.Payload["tool_name"].(string); toolName != "" {
			sla := d.collector.GetSLA(toolName)
			env.Payload["tool_sla"] = map[string]any{
				"tool_name":      sla.ToolName,
				"p50_latency_ms": sla.P50Latency.Milliseconds(),
				"p95_latency_ms": sla.P95Latency.Milliseconds(),
				"p99_latency_ms": sla.P99Latency.Milliseconds(),
				"error_rate":     sla.ErrorRate,
				"call_count":     sla.CallCount,
				"success_rate":   sla.SuccessRate,
				"cost_usd_total": sla.CostUSDTotal,
				"cost_usd_avg":   sla.CostUSDAvg,
			}
		}
	}
	d.sink.OnEvent(evt)
}
