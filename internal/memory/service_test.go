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

func TestRecallWithoutTermsReturnsEmpty(t *testing.T) {
	store := NewInMemoryStore()
	svc := NewService(store)
	if _, err := svc.Save(context.Background(), Entry{
		UserID:    "u",
		Content:   "hello world",
		Keywords:  []string{"greeting"},
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	results, err := svc.Recall(context.Background(), Query{UserID: "u"})
	if err != nil {
		t.Fatalf("recall error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results without terms, got %d", len(results))
	}
}
