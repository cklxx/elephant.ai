package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// mockChannel is a test double implementing Channel.
type mockChannel struct {
	name       string
	sent       []Notification
	mu         sync.Mutex
	sendErr    error
	supportsFn func(NotificationPriority) bool
}

func newMockChannel(name string) *mockChannel {
	return &mockChannel{name: name}
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Send(_ context.Context, n Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, n)
	return nil
}

func (m *mockChannel) Supports(p NotificationPriority) bool {
	if m.supportsFn != nil {
		return m.supportsFn(p)
	}
	return true
}

func (m *mockChannel) sentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

// ---------------------------------------------------------------------------
// Center tests
// ---------------------------------------------------------------------------

func TestRegisterChannelAndListChannels(t *testing.T) {
	c := NewCenter()
	ch1 := newMockChannel("email")
	ch2 := newMockChannel("sms")

	c.RegisterChannel(ch1, ChannelConfig{Enabled: true, MinPriority: PriorityNormal})
	c.RegisterChannel(ch2, ChannelConfig{Enabled: true, MinPriority: PriorityHigh, IsDefault: true})

	channels := c.ListChannels()
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}

	found := make(map[string]ChannelConfig)
	for _, cfg := range channels {
		found[cfg.Name] = cfg
	}

	if cfg, ok := found["email"]; !ok || !cfg.Enabled {
		t.Error("email channel not found or disabled")
	}
	if cfg, ok := found["sms"]; !ok || !cfg.IsDefault {
		t.Error("sms channel not found or not default")
	}
}

func TestSendToSpecificChannel(t *testing.T) {
	c := NewCenter()
	ch := newMockChannel("webhook")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	n := Notification{
		UserID:   "user-1",
		Title:    "Test",
		Body:     "Hello",
		Priority: PriorityNormal,
		Channel:  "webhook",
	}

	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusDelivered {
		t.Errorf("expected delivered, got %s", result.Status)
	}
	if result.Channel != "webhook" {
		t.Errorf("expected channel webhook, got %s", result.Channel)
	}
	if ch.sentCount() != 1 {
		t.Errorf("expected 1 send, got %d", ch.sentCount())
	}
}

func TestSendToDefaultChannel(t *testing.T) {
	c := NewCenter(WithDefaultChannel("log"))
	ch := newMockChannel("log")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	n := Notification{
		UserID:   "user-1",
		Title:    "Default",
		Body:     "Goes to log",
		Priority: PriorityNormal,
	}

	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusDelivered {
		t.Errorf("expected delivered, got %s", result.Status)
	}
	if result.Channel != "log" {
		t.Errorf("expected channel log, got %s", result.Channel)
	}
}

func TestCriticalNotificationsGoToAllChannels(t *testing.T) {
	c := NewCenter()

	ch1 := newMockChannel("primary")
	ch2 := newMockChannel("backup")
	ch3 := newMockChannel("lowonly")
	ch3.supportsFn = func(p NotificationPriority) bool {
		return p <= PriorityNormal // does NOT support critical
	}

	c.RegisterChannel(ch1, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})
	c.RegisterChannel(ch2, ChannelConfig{Enabled: true, MinPriority: PriorityLow})
	c.RegisterChannel(ch3, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	n := Notification{
		Title:    "URGENT",
		Body:     "System down",
		Priority: PriorityCritical,
	}

	_, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// primary gets it as default; backup gets it via critical fan-out;
	// lowonly does NOT support critical so it should not receive the fan-out.
	if ch1.sentCount() != 1 {
		t.Errorf("primary: expected 1 send, got %d", ch1.sentCount())
	}
	if ch2.sentCount() != 1 {
		t.Errorf("backup: expected 1 send, got %d", ch2.sentCount())
	}
	if ch3.sentCount() != 0 {
		t.Errorf("lowonly: expected 0 sends (doesn't support critical), got %d", ch3.sentCount())
	}
}

func TestSendMultiToMultipleChannels(t *testing.T) {
	c := NewCenter()

	ch1 := newMockChannel("a")
	ch2 := newMockChannel("b")
	ch3 := newMockChannel("c")

	c.RegisterChannel(ch1, ChannelConfig{Enabled: true, MinPriority: PriorityLow})
	c.RegisterChannel(ch2, ChannelConfig{Enabled: true, MinPriority: PriorityLow})
	c.RegisterChannel(ch3, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	n := Notification{
		Title:    "Multi",
		Body:     "To a and c",
		Priority: PriorityNormal,
	}

	results, err := c.SendMulti(context.Background(), n, []string{"a", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != StatusDelivered {
			t.Errorf("channel %s: expected delivered, got %s (err=%s)", r.Channel, r.Status, r.Error)
		}
	}
	if ch1.sentCount() != 1 {
		t.Errorf("ch a: expected 1 send, got %d", ch1.sentCount())
	}
	if ch2.sentCount() != 0 {
		t.Errorf("ch b: expected 0 sends, got %d", ch2.sentCount())
	}
	if ch3.sentCount() != 1 {
		t.Errorf("ch c: expected 1 send, got %d", ch3.sentCount())
	}
}

func TestChannelNotFoundError(t *testing.T) {
	c := NewCenter()

	n := Notification{
		Title:    "Lost",
		Body:     "No channel",
		Priority: PriorityNormal,
		Channel:  "nonexistent",
	}

	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("expected not found error, got %q", result.Error)
	}
}

func TestNoDefaultChannelError(t *testing.T) {
	c := NewCenter()

	n := Notification{
		Title:    "No default",
		Body:     "Should fail",
		Priority: PriorityNormal,
	}

	_, err := c.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error when no channel and no default")
	}
	if !strings.Contains(err.Error(), "no channel specified") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHistoryTracking(t *testing.T) {
	c := NewCenter(WithHistorySize(5))
	ch := newMockChannel("log")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})

	ctx := context.Background()
	for i := 0; i < 7; i++ {
		n := Notification{
			Title:    fmt.Sprintf("Notif-%d", i),
			Body:     "body",
			Priority: PriorityNormal,
		}
		_, _ = c.Send(ctx, n)
	}

	// History size is 5, so only the last 5 should be retained.
	history := c.History("", 10)
	if len(history) != 5 {
		t.Fatalf("expected 5 history entries, got %d", len(history))
	}

	// Most recent should be first.
	if history[0].Status != StatusDelivered {
		t.Error("most recent should be delivered")
	}

	// Limit works.
	limited := c.History("", 2)
	if len(limited) != 2 {
		t.Fatalf("expected 2 limited entries, got %d", len(limited))
	}
}

func TestUnregisterChannel(t *testing.T) {
	c := NewCenter()
	ch := newMockChannel("temp")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})

	channels := c.ListChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}

	c.UnregisterChannel("temp")

	channels = c.ListChannels()
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels after unregister, got %d", len(channels))
	}

	// Default should be cleared.
	n := Notification{Title: "x", Body: "y", Priority: PriorityNormal}
	_, err := c.Send(context.Background(), n)
	if err == nil {
		t.Error("expected error after unregistering default channel")
	}
}

func TestSetDefault(t *testing.T) {
	c := NewCenter()
	ch1 := newMockChannel("first")
	ch2 := newMockChannel("second")
	c.RegisterChannel(ch1, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})
	c.RegisterChannel(ch2, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	if err := c.SetDefault("second"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify via ListChannels.
	for _, cfg := range c.ListChannels() {
		if cfg.Name == "second" && !cfg.IsDefault {
			t.Error("second should be default")
		}
		if cfg.Name == "first" && cfg.IsDefault {
			t.Error("first should no longer be default")
		}
	}

	// Setting non-existent channel should error.
	if err := c.SetDefault("nope"); err == nil {
		t.Error("expected error for non-existent channel")
	}
}

func TestSetDefaultRoutesCorrectly(t *testing.T) {
	c := NewCenter()
	ch1 := newMockChannel("first")
	ch2 := newMockChannel("second")
	c.RegisterChannel(ch1, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})
	c.RegisterChannel(ch2, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	_ = c.SetDefault("second")

	n := Notification{Title: "Routed", Body: "to second", Priority: PriorityNormal}
	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channel != "second" {
		t.Errorf("expected channel second, got %s", result.Channel)
	}
	if ch2.sentCount() != 1 {
		t.Errorf("second: expected 1 send, got %d", ch2.sentCount())
	}
	if ch1.sentCount() != 0 {
		t.Errorf("first: expected 0 sends, got %d", ch1.sentCount())
	}
}

// ---------------------------------------------------------------------------
// LogChannel tests
// ---------------------------------------------------------------------------

func TestLogChannelOutput(t *testing.T) {
	var buf bytes.Buffer
	ch := NewLogChannel("console", &buf)

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	n := Notification{
		ID:        "test-id",
		Title:     "Server Alert",
		Body:      "CPU usage high",
		Priority:  PriorityHigh,
		CreatedAt: now,
	}

	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	expected := "[2026-01-15T10:30:00Z] [HIGH] Server Alert: CPU usage high\n"
	if output != expected {
		t.Errorf("unexpected output:\ngot:  %q\nwant: %q", output, expected)
	}
}

func TestLogChannelName(t *testing.T) {
	ch := NewLogChannel("mylog", &bytes.Buffer{})
	if ch.Name() != "mylog" {
		t.Errorf("expected name mylog, got %s", ch.Name())
	}
}

func TestLogChannelSupportsAllPriorities(t *testing.T) {
	ch := NewLogChannel("log", &bytes.Buffer{})
	for _, p := range []NotificationPriority{PriorityLow, PriorityNormal, PriorityHigh, PriorityCritical} {
		if !ch.Supports(p) {
			t.Errorf("LogChannel should support priority %v", p)
		}
	}
}

// ---------------------------------------------------------------------------
// LogChannel integration with Center
// ---------------------------------------------------------------------------

func TestLogChannelViaCenterIntegration(t *testing.T) {
	var buf bytes.Buffer
	ch := NewLogChannel("console", &buf)
	c := NewCenter(WithDefaultChannel("console"))
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	n := Notification{
		Title:     "Hello",
		Body:      "World",
		Priority:  PriorityNormal,
		CreatedAt: time.Date(2026, 2, 1, 8, 0, 0, 0, time.UTC),
	}

	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusDelivered {
		t.Errorf("expected delivered, got %s", result.Status)
	}
	if !strings.Contains(buf.String(), "Hello: World") {
		t.Errorf("log output missing expected content: %s", buf.String())
	}
}

// ---------------------------------------------------------------------------
// WebhookChannel tests
// ---------------------------------------------------------------------------

func TestWebhookChannelSend(t *testing.T) {
	var received webhookPayload
	var receivedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewWebhookChannel("hook", srv.URL,
		WithTimeout(5*time.Second),
		WithHeaders(map[string]string{"X-Token": "secret123"}),
	)

	n := Notification{
		ID:       "wh-001",
		Title:    "Deploy",
		Body:     "v1.2.3 deployed",
		Priority: PriorityHigh,
		Metadata: map[string]string{"env": "prod"},
	}

	if err := ch.Send(context.Background(), n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify payload.
	if received.ID != "wh-001" {
		t.Errorf("expected id wh-001, got %s", received.ID)
	}
	if received.Title != "Deploy" {
		t.Errorf("expected title Deploy, got %s", received.Title)
	}
	if received.Body != "v1.2.3 deployed" {
		t.Errorf("expected body 'v1.2.3 deployed', got %s", received.Body)
	}
	if received.Priority != 3 {
		t.Errorf("expected priority 3, got %d", received.Priority)
	}
	if received.Metadata["env"] != "prod" {
		t.Errorf("expected metadata env=prod, got %v", received.Metadata)
	}

	// Verify custom header.
	if receivedHeaders.Get("X-Token") != "secret123" {
		t.Errorf("expected X-Token header, got %s", receivedHeaders.Get("X-Token"))
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedHeaders.Get("Content-Type"))
	}
}

func TestWebhookChannelServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ch := NewWebhookChannel("hook", srv.URL)
	n := Notification{
		Title:    "Fail",
		Body:     "Should error",
		Priority: PriorityNormal,
	}

	err := ch.Send(context.Background(), n)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status 500 in error, got: %v", err)
	}
}

func TestWebhookChannelName(t *testing.T) {
	ch := NewWebhookChannel("myhook", "http://example.com")
	if ch.Name() != "myhook" {
		t.Errorf("expected name myhook, got %s", ch.Name())
	}
}

func TestWebhookChannelSupportsAllPriorities(t *testing.T) {
	ch := NewWebhookChannel("hook", "http://example.com")
	for _, p := range []NotificationPriority{PriorityLow, PriorityNormal, PriorityHigh, PriorityCritical} {
		if !ch.Supports(p) {
			t.Errorf("WebhookChannel should support priority %v", p)
		}
	}
}

// ---------------------------------------------------------------------------
// WebhookChannel integration with Center
// ---------------------------------------------------------------------------

func TestWebhookChannelViaCenterIntegration(t *testing.T) {
	var received webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewWebhookChannel("wh", srv.URL)
	c := NewCenter(WithDefaultChannel("wh"))
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	n := Notification{
		Title:    "Webhook Center",
		Body:     "Integration test",
		Priority: PriorityNormal,
	}

	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusDelivered {
		t.Errorf("expected delivered, got %s (err=%s)", result.Status, result.Error)
	}
	if received.Title != "Webhook Center" {
		t.Errorf("unexpected payload title: %s", received.Title)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestSendAssignsIDIfEmpty(t *testing.T) {
	c := NewCenter()
	ch := newMockChannel("ch")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})

	n := Notification{Title: "NoID", Body: "x", Priority: PriorityNormal}
	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NotificationID == "" {
		t.Error("expected auto-assigned notification ID")
	}
}

func TestDisabledChannelReturnsFailure(t *testing.T) {
	c := NewCenter()
	ch := newMockChannel("disabled")
	c.RegisterChannel(ch, ChannelConfig{Enabled: false, MinPriority: PriorityLow})

	n := Notification{Title: "x", Body: "y", Priority: PriorityNormal, Channel: "disabled"}
	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed for disabled channel, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "disabled") {
		t.Errorf("error should mention disabled, got: %s", result.Error)
	}
}

func TestMinPriorityFiltering(t *testing.T) {
	c := NewCenter()
	ch := newMockChannel("highonly")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityHigh})

	// Low priority should be rejected.
	n := Notification{Title: "Low", Body: "x", Priority: PriorityLow, Channel: "highonly"}
	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed for low priority, got %s", result.Status)
	}

	// High priority should succeed.
	n2 := Notification{Title: "High", Body: "x", Priority: PriorityHigh, Channel: "highonly"}
	result2, err := c.Send(context.Background(), n2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.Status != StatusDelivered {
		t.Errorf("expected delivered for high priority, got %s (err=%s)", result2.Status, result2.Error)
	}
}

func TestSendMultiWithNonexistentChannel(t *testing.T) {
	c := NewCenter()
	ch := newMockChannel("real")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow})

	n := Notification{Title: "Multi", Body: "x", Priority: PriorityNormal}
	results, err := c.SendMulti(context.Background(), n, []string{"real", "fake"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != StatusDelivered {
		t.Errorf("real: expected delivered, got %s", results[0].Status)
	}
	if results[1].Status != StatusFailed {
		t.Errorf("fake: expected failed, got %s", results[1].Status)
	}
}

func TestChannelSendError(t *testing.T) {
	c := NewCenter()
	ch := newMockChannel("failing")
	ch.sendErr = fmt.Errorf("connection refused")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})

	n := Notification{Title: "Err", Body: "x", Priority: PriorityNormal}
	result, err := c.Send(context.Background(), n)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "connection refused") {
		t.Errorf("expected connection refused in error, got: %s", result.Error)
	}
}

func TestNotificationPriorityString(t *testing.T) {
	tests := []struct {
		p    NotificationPriority
		want string
	}{
		{PriorityLow, "LOW"},
		{PriorityNormal, "NORMAL"},
		{PriorityHigh, "HIGH"},
		{PriorityCritical, "CRITICAL"},
		{NotificationPriority(99), "PRIORITY(99)"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("Priority(%d).String() = %q, want %q", int(tt.p), got, tt.want)
		}
	}
}

func TestConcurrentSend(t *testing.T) {
	c := NewCenter(WithHistorySize(100))
	ch := newMockChannel("concurrent")
	c.RegisterChannel(ch, ChannelConfig{Enabled: true, MinPriority: PriorityLow, IsDefault: true})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n := Notification{
				Title:    fmt.Sprintf("Concurrent-%d", i),
				Body:     "body",
				Priority: PriorityNormal,
			}
			_, _ = c.Send(context.Background(), n)
		}(i)
	}
	wg.Wait()

	if ch.sentCount() != 50 {
		t.Errorf("expected 50 sends, got %d", ch.sentCount())
	}
	history := c.History("", 100)
	if len(history) != 50 {
		t.Errorf("expected 50 history entries, got %d", len(history))
	}
}
