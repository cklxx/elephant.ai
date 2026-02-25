package react

import (
	"context"

	id "alex/internal/shared/utils/id"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	traceScopeReact = "alex.react"

	traceSpanReactIteration = "alex.react.iteration"
	traceSpanLLMGenerate    = "alex.llm.generate"
	traceSpanToolExecute    = "alex.tool.execute"

	traceAttrSessionID   = "alex.session_id"
	traceAttrRunID       = "alex.run_id"
	traceAttrParentRunID = "alex.parent_run_id"
	traceAttrIteration   = "alex.iteration"
	traceAttrStatus      = "alex.status"
	traceAttrToolName    = "alex.tool_name"
	traceAttrModel       = "alex.llm.model"
)

func startReactSpan(ctx context.Context, spanName string, state *TaskState, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ids := id.IDsFromContext(ctx)
	if state != nil {
		if ids.SessionID == "" {
			ids.SessionID = state.SessionID
		}
		if ids.RunID == "" {
			ids.RunID = state.RunID
		}
		if ids.ParentRunID == "" {
			ids.ParentRunID = state.ParentRunID
		}
	}

	spanAttrs := make([]attribute.KeyValue, 0, len(attrs)+3)
	if ids.SessionID != "" {
		spanAttrs = append(spanAttrs, attribute.String(traceAttrSessionID, ids.SessionID))
	}
	if ids.RunID != "" {
		spanAttrs = append(spanAttrs, attribute.String(traceAttrRunID, ids.RunID))
	}
	if ids.ParentRunID != "" {
		spanAttrs = append(spanAttrs, attribute.String(traceAttrParentRunID, ids.ParentRunID))
	}
	spanAttrs = append(spanAttrs, attrs...)

	return otel.Tracer(traceScopeReact).Start(ctx, spanName, trace.WithAttributes(spanAttrs...))
}

func markSpanResult(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String(traceAttrStatus, "error"))
		return
	}
	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.String(traceAttrStatus, "success"))
}
