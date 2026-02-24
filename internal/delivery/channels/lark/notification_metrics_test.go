package lark

import (
	"context"
	"testing"
	"time"

	"alex/internal/delivery/channels"
)

type capturedNotificationMetric struct {
	surface string
	action  string
	mode    string
	status  string
	latency time.Duration
}

type capturedBlockingMetric struct {
	source       string
	optionsCount int
}

type fakeLarkMetrics struct {
	notifications []capturedNotificationMetric
	blocking      []capturedBlockingMetric
	statusProxy   []string
}

func (m *fakeLarkMetrics) RecordLarkNotification(_ context.Context, surface, action, mode, status string, latency time.Duration) {
	m.notifications = append(m.notifications, capturedNotificationMetric{
		surface: surface,
		action:  action,
		mode:    mode,
		status:  status,
		latency: latency,
	})
}

func (m *fakeLarkMetrics) RecordLarkBlockingPrompt(_ context.Context, source string, optionsCount int) {
	m.blocking = append(m.blocking, capturedBlockingMetric{
		source:       source,
		optionsCount: optionsCount,
	})
}

func (m *fakeLarkMetrics) RecordLarkStatusProxyQuery(_ context.Context, source string) {
	m.statusProxy = append(m.statusProxy, source)
}

func TestGatewayNotificationMetricsDispatchAndUpdate(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&stubExecutor{}, rec, channels.BaseConfig{AllowDirect: true, SessionPrefix: "lark"})
	gw.cfg.NotificationMetricsV2 = true

	metrics := &fakeLarkMetrics{}
	gw.SetNotificationMetrics(metrics)

	ctx := context.Background()
	if _, err := gw.dispatchNotification(ctx, "oc_chat", "", "text", textContent("hello"), "task_result", notificationModeMilestone.String()); err != nil {
		t.Fatalf("dispatch send failed: %v", err)
	}
	if _, err := gw.dispatchNotification(ctx, "oc_chat", "om_parent", "text", textContent("reply"), "task_result", notificationModeMilestone.String()); err != nil {
		t.Fatalf("dispatch reply failed: %v", err)
	}
	if err := gw.updateNotification(ctx, "om_edit", "updated", "task_result", notificationModeMilestone.String()); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if len(metrics.notifications) != 3 {
		t.Fatalf("expected 3 notification metrics, got %d", len(metrics.notifications))
	}
	if metrics.notifications[0].action != "send" || metrics.notifications[1].action != "reply" || metrics.notifications[2].action != "update" {
		t.Fatalf("unexpected actions: %+v", metrics.notifications)
	}
}

func TestGatewayNotificationMetricsBlockingAndStatusProxy(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&stubExecutor{}, rec, channels.BaseConfig{AllowDirect: true, SessionPrefix: "lark"})
	gw.cfg.NotificationMetricsV2 = true

	metrics := &fakeLarkMetrics{}
	gw.SetNotificationMetrics(metrics)

	gw.recordBlockingPrompt(context.Background(), "await_user_input", 2)
	gw.recordStatusProxyQuery(context.Background(), "natural_language")

	if len(metrics.blocking) != 1 {
		t.Fatalf("expected 1 blocking metric, got %d", len(metrics.blocking))
	}
	if metrics.blocking[0].source != "await_user_input" || metrics.blocking[0].optionsCount != 2 {
		t.Fatalf("unexpected blocking metric: %+v", metrics.blocking[0])
	}
	if len(metrics.statusProxy) != 1 || metrics.statusProxy[0] != "natural_language" {
		t.Fatalf("unexpected status proxy metrics: %+v", metrics.statusProxy)
	}
}

func TestHandleNaturalTaskStatusQueryRecordsMetrics(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&stubExecutor{}, rec, channels.BaseConfig{AllowDirect: true, SessionPrefix: "lark"})
	gw.cfg.NotificationMetricsV2 = true

	metrics := &fakeLarkMetrics{}
	gw.SetNotificationMetrics(metrics)

	msg := &incomingMessage{
		chatID:    "oc_chat",
		messageID: "om_source",
		senderID:  "ou_user",
		content:   "代码助手现在在做什么",
	}
	gw.handleNaturalTaskStatusQuery(msg)

	if len(metrics.statusProxy) != 1 {
		t.Fatalf("expected 1 status proxy metric, got %d", len(metrics.statusProxy))
	}
	if len(metrics.notifications) != 1 {
		t.Fatalf("expected 1 notification metric, got %d", len(metrics.notifications))
	}
	if metrics.notifications[0].surface != "task_status_proxy" {
		t.Fatalf("unexpected notification surface: %+v", metrics.notifications[0])
	}
}
