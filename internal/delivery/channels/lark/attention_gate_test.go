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

// ---------- Bug fix: blank keyword filtering ----------

func TestAttentionGate_BlankKeywordsFiltered(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0", "", "  ", "\t", "hotfix", " "},
	})
	// Only "p0" and "hotfix" should survive.
	if len(gate.lowerKeywords) != 2 {
		t.Fatalf("lowerKeywords = %v, want 2 entries", gate.lowerKeywords)
	}
	if gate.lowerKeywords[0] != "p0" || gate.lowerKeywords[1] != "hotfix" {
		t.Errorf("lowerKeywords = %v, want [p0 hotfix]", gate.lowerKeywords)
	}
}

func TestAttentionGate_BlankKeywordDoesNotMatchEverything(t *testing.T) {
	// Before fix: an empty keyword "" would match every message via
	// strings.Contains(lower, ""), making everything UrgencyHigh.
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"", "  "},
	})
	// A routine message must NOT be classified as UrgencyHigh.
	if got := gate.ClassifyUrgency("just a normal message"); got != UrgencyLow {
		t.Errorf("blank keywords should not match, got urgency %d", got)
	}
}

func TestAttentionGate_KeywordTrimmedWhitespace(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"  hotfix  "},
	})
	if got := gate.ClassifyUrgency("please apply the hotfix"); got != UrgencyHigh {
		t.Errorf("trimmed keyword should match, got urgency %d", got)
	}
}

// ---------- Bug fix: stale budget GC ----------

func TestAttentionGate_BudgetGC_RemovesStaleChatIDs(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    10,
	})

	t0 := time.Now()

	// Populate more than budgetGCThreshold chat entries.
	for i := 0; i < budgetGCThreshold+10; i++ {
		chatID := "chat-" + time.Duration(i).String()
		gate.RecordDispatch(chatID, t0)
	}

	if len(gate.budgets) != budgetGCThreshold+10 {
		t.Fatalf("budgets = %d, want %d", len(gate.budgets), budgetGCThreshold+10)
	}

	// Advance time past the budget window so all entries are stale.
	t1 := t0.Add(10 * time.Minute)

	// A new dispatch triggers GC since we're above threshold.
	gate.RecordDispatch("fresh-chat", t1)

	// All old entries should be GC'd; only "fresh-chat" remains.
	if len(gate.budgets) != 1 {
		t.Errorf("after GC, budgets = %d, want 1 (only fresh-chat)", len(gate.budgets))
	}
	if _, ok := gate.budgets["fresh-chat"]; !ok {
		t.Error("fresh-chat should survive GC")
	}
}

func TestAttentionGate_BudgetGC_KeepsActiveChatIDs(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    10,
	})

	t0 := time.Now()

	// Populate entries above threshold.
	for i := 0; i < budgetGCThreshold+5; i++ {
		chatID := "chat-" + time.Duration(i).String()
		gate.RecordDispatch(chatID, t0)
	}

	// Dispatch within the same window — entries are still active.
	t1 := t0.Add(2 * time.Minute)
	gate.RecordDispatch("trigger", t1)

	// All chats still have recent activity, so none should be GC'd.
	// We expect threshold+5 original + 1 trigger = threshold+6.
	if len(gate.budgets) != budgetGCThreshold+6 {
		t.Errorf("active entries should survive GC, budgets = %d, want %d",
			len(gate.budgets), budgetGCThreshold+6)
	}
}

func TestAttentionGate_BudgetGC_EmptyTimestamps(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    10,
	})

	// Manually inject an entry with empty timestamps.
	gate.mu.Lock()
	for i := 0; i < budgetGCThreshold+1; i++ {
		gate.budgets["empty-"+time.Duration(i).String()] = &chatBudget{}
	}
	gate.mu.Unlock()

	now := time.Now()
	gate.RecordDispatch("live", now)

	// All empty entries should be GC'd.
	if len(gate.budgets) != 1 {
		t.Errorf("empty-timestamp entries should be GC'd, budgets = %d, want 1", len(gate.budgets))
	}
}
