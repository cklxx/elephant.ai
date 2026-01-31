package memory

import (
	"context"
	"testing"
	"time"
)

func TestServiceSaveAndRecall(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store)

	created, err := svc.Save(context.Background(), Entry{
		UserID:   "user-1",
		Content:  "修复登录 bug，并总结 root cause。",
		Keywords: []string{"登录", "bug"},
		Slots: map[string]string{
			"intent": "triage",
			"area":   "auth",
		},
	})
	if err != nil {
		t.Fatalf("save returned error: %v", err)
	}
	if created.Key == "" {
		t.Fatalf("expected generated key")
	}
	if created.CreatedAt.IsZero() {
		t.Fatalf("expected created_at set")
	}

	results, err := svc.Recall(context.Background(), Query{
		UserID:   "user-1",
		Keywords: []string{"Bug"},
		Slots: map[string]string{
			"intent": "triage",
		},
		Limit: 3,
	})
	if err != nil {
		t.Fatalf("recall returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key != created.Key {
		t.Fatalf("unexpected key: %s", results[0].Key)
	}
	if results[0].Content != created.Content {
		t.Fatalf("unexpected content: %s", results[0].Content)
	}
}

func TestServiceRecallRespectsSlots(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store)

	_, err := svc.Save(context.Background(), Entry{
		UserID:  "user-1",
		Content: "triage auth issue",
		Slots:   map[string]string{"intent": "triage"},
	})
	if err != nil {
		t.Fatalf("save triage failed: %v", err)
	}
	_, err = svc.Save(context.Background(), Entry{
		UserID:  "user-1",
		Content: "write summary",
		Slots:   map[string]string{"intent": "write"},
	})
	if err != nil {
		t.Fatalf("save write failed: %v", err)
	}

	results, err := svc.Recall(context.Background(), Query{
		UserID: "user-1",
		Slots:  map[string]string{"intent": "triage"},
	})
	if err != nil {
		t.Fatalf("recall returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for matching slot, got %d", len(results))
	}
	if results[0].Slots["intent"] != "triage" {
		t.Fatalf("unexpected slot match: %+v", results[0].Slots)
	}
}

func TestServiceRejectsMissingUserOrContent(t *testing.T) {
	svc := NewService(NewInMemoryStore())
	if _, err := svc.Save(context.Background(), Entry{}); err == nil {
		t.Fatalf("expected error for missing user and content")
	}
	if _, err := svc.Save(context.Background(), Entry{UserID: "u"}); err == nil {
		t.Fatalf("expected error for missing content")
	}
	if _, err := svc.Save(context.Background(), Entry{Content: "c"}); err == nil {
		t.Fatalf("expected error for missing user")
	}
}

func TestRecallWithoutTermsReturnsAll(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store)
	saved, err := svc.Save(context.Background(), Entry{
		UserID:    "u",
		Content:   "hello world",
		Keywords:  []string{"greeting"},
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	results, err := svc.Recall(context.Background(), Query{UserID: "u"})
	if err != nil {
		t.Fatalf("recall error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result without terms (list all), got %d", len(results))
	}
	if results[0].Key != saved.Key {
		t.Fatalf("unexpected key: got %s, want %s", results[0].Key, saved.Key)
	}
}

func TestServiceRecallMatchesCJKSubstringsViaInvertedTerms(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store)

	created, err := svc.Save(context.Background(), Entry{
		UserID:   "u",
		Content:  `撰写了面向产品经理的短文《如何用“问题-假设-验证”闭环提升功能迭代质量》`,
		Keywords: []string{"写作"},
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	results, err := svc.Recall(context.Background(), Query{UserID: "u", Keywords: []string{"撰写"}})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(results) != 1 || results[0].Key != created.Key {
		t.Fatalf("expected recall by substring keyword to return saved entry")
	}

	results, err = svc.Recall(context.Background(), Query{UserID: "u", Keywords: []string{"产品经理"}})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(results) != 1 || results[0].Key != created.Key {
		t.Fatalf("expected recall by internal CJK term to return saved entry")
	}
}

func TestServiceRecallPrunesExpiredEntries(t *testing.T) {
	store := NewInMemoryStore()
	policy := RetentionPolicy{
		DefaultTTL:    10 * time.Millisecond,
		PruneOnRecall: true,
	}
	svc := NewServiceWithRetention(store, policy)

	now := time.Now()
	_, err := svc.Save(context.Background(), Entry{
		UserID:    "user-1",
		Content:   "expired note",
		Keywords:  []string{"note"},
		CreatedAt: now.Add(-time.Hour),
		Slots:     map[string]string{"type": "auto_capture"},
	})
	if err != nil {
		t.Fatalf("save expired entry: %v", err)
	}
	_, err = svc.Save(context.Background(), Entry{
		UserID:    "user-1",
		Content:   "fresh note",
		Keywords:  []string{"note"},
		CreatedAt: now,
		Slots:     map[string]string{"type": "auto_capture"},
	})
	if err != nil {
		t.Fatalf("save fresh entry: %v", err)
	}

	results, err := svc.Recall(context.Background(), Query{
		UserID:   "user-1",
		Keywords: []string{"note"},
	})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after pruning, got %d", len(results))
	}
	if results[0].Content != "fresh note" {
		t.Fatalf("expected fresh entry, got %q", results[0].Content)
	}

	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.records["user-1"]) != 1 {
		t.Fatalf("expected expired entry to be deleted from store")
	}
}

func TestServiceRecallRespectsTypeTTL(t *testing.T) {
	store := NewInMemoryStore()
	policy := RetentionPolicy{
		DefaultTTL:    24 * time.Hour,
		PruneOnRecall: true,
		TypeTTL: map[string]time.Duration{
			"chat_turn": 30 * time.Minute,
		},
	}
	svc := NewServiceWithRetention(store, policy)

	now := time.Now()
	_, err := svc.Save(context.Background(), Entry{
		UserID:    "user-1",
		Content:   "old chat",
		Keywords:  []string{"chat"},
		CreatedAt: now.Add(-2 * time.Hour),
		Slots:     map[string]string{"type": "chat_turn"},
	})
	if err != nil {
		t.Fatalf("save chat entry: %v", err)
	}
	_, err = svc.Save(context.Background(), Entry{
		UserID:    "user-1",
		Content:   "old capture",
		Keywords:  []string{"chat"},
		CreatedAt: now.Add(-2 * time.Hour),
		Slots:     map[string]string{"type": "auto_capture"},
	})
	if err != nil {
		t.Fatalf("save capture entry: %v", err)
	}

	results, err := svc.Recall(context.Background(), Query{
		UserID:   "user-1",
		Keywords: []string{"chat"},
	})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "old capture" {
		t.Fatalf("expected non-expired entry, got %q", results[0].Content)
	}
}
