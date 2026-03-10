package lark

import (
	"testing"
	"time"
)

func TestAttentionGate_ClassifyUrgency_Disabled(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: false})
	if got := gate.ClassifyUrgency("hello"); got != UrgencyNormal {
		t.Errorf("disabled gate should return UrgencyNormal, got %d", got)
	}
}

func TestAttentionGate_ClassifyUrgency_EmptyContent(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	if got := gate.ClassifyUrgency(""); got != UrgencyLow {
		t.Errorf("empty content should be UrgencyLow, got %d", got)
	}
}

func TestAttentionGate_ClassifyUrgency_ConfiguredKeyword(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0", "hotfix"},
	})
	if got := gate.ClassifyUrgency("We need a P0 fix now"); got != UrgencyHigh {
		t.Errorf("configured keyword P0 should be UrgencyHigh, got %d", got)
	}
	if got := gate.ClassifyUrgency("deploy the HOTFIX branch"); got != UrgencyHigh {
		t.Errorf("case-insensitive keyword hotfix should be UrgencyHigh, got %d", got)
	}
}

func TestAttentionGate_ClassifyUrgency_BuiltinPatterns(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	tests := []struct {
		content string
		want    UrgencyLevel
	}{
		{"服务紧急需要修复", UrgencyHigh},
		{"this is urgent please help", UrgencyHigh},
		{"the build failed again", UrgencyHigh},
		{"server is down", UrgencyHigh},
		{"I'm blocked on this", UrgencyHigh},
		{"系统出错了", UrgencyHigh},
		{"请帮忙看一下这个文件", UrgencyLow},
		{"can you review this PR", UrgencyLow},
	}
	for _, tt := range tests {
		if got := gate.ClassifyUrgency(tt.content); got != tt.want {
			t.Errorf("ClassifyUrgency(%q) = %d, want %d", tt.content, got, tt.want)
		}
	}
}

func TestAttentionGate_ClassifyUrgency_Exclamations(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	// Short message with 3+ exclamation marks → UrgencyHigh
	if got := gate.ClassifyUrgency("help!!!"); got != UrgencyHigh {
		t.Errorf("triple exclamation should be UrgencyHigh, got %d", got)
	}
	// Long message with exclamations → UrgencyLow (length >= 50 runes, no urgency keywords)
	long := "this is a very long message that should not be considered particularly noteworthy at all!!!"
	if got := gate.ClassifyUrgency(long); got != UrgencyLow {
		t.Errorf("long message with exclamations should be UrgencyLow, got %d", got)
	}
}

func TestAttentionGate_RecordDispatch_WithinBudget(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 10 * time.Minute,
		BudgetMax:    3,
	})
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !gate.RecordDispatch("chat-1", now) {
			t.Errorf("dispatch %d should be within budget", i)
		}
	}
	// 4th should be over budget
	if gate.RecordDispatch("chat-1", now) {
		t.Error("4th dispatch should be over budget")
	}
}

func TestAttentionGate_RecordDispatch_WindowExpiry(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    2,
	})
	t0 := time.Now()
	gate.RecordDispatch("chat-1", t0)
	gate.RecordDispatch("chat-1", t0)
	// Over budget at t0
	if gate.RecordDispatch("chat-1", t0) {
		t.Error("should be over budget at t0")
	}
	// After window expires, budget resets
	t1 := t0.Add(6 * time.Minute)
	if !gate.RecordDispatch("chat-1", t1) {
		t.Error("should be within budget after window expiry")
	}
}

func TestAttentionGate_RecordDispatch_NoBudgetLimit(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:   true,
		BudgetMax: 0, // disabled
	})
	now := time.Now()
	for i := 0; i < 100; i++ {
		if !gate.RecordDispatch("chat-1", now) {
			t.Errorf("dispatch %d should always pass with no budget limit", i)
		}
	}
}

func TestAttentionGate_AutoAckMessage_Default(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	if got := gate.AutoAckMessage(); got != defaultAutoAckMessage {
		t.Errorf("default auto-ack = %q, want %q", got, defaultAutoAckMessage)
	}
}

func TestAttentionGate_AutoAckMessage_Custom(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		AutoAckMessage: "Got it!",
	})
	if got := gate.AutoAckMessage(); got != "Got it!" {
		t.Errorf("custom auto-ack = %q, want %q", got, "Got it!")
	}
}
