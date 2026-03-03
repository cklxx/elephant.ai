package lark

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	ports "alex/internal/domain/agent/ports"
	"alex/internal/infra/filestore"
	jsonx "alex/internal/shared/json"
	id "alex/internal/shared/utils/id"
)

type deliveryOutboxDoc struct {
	Intents []DeliveryIntent `json:"intents"`
}

// DeliveryOutboxLocalStore persists delivery intents in memory or a local JSON file.
type DeliveryOutboxLocalStore struct {
	coll *filestore.Collection[string, DeliveryIntent]
}

func (s *DeliveryOutboxLocalStore) ensureReady(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if s == nil {
		return fmt.Errorf("delivery outbox store not initialized")
	}
	return nil
}

func NewDeliveryOutboxMemoryStore() *DeliveryOutboxLocalStore {
	return &DeliveryOutboxLocalStore{coll: newDeliveryOutboxCollection("")}
}

func NewDeliveryOutboxFileStore(dir string) (*DeliveryOutboxLocalStore, error) {
	trimmedDir := strings.TrimSpace(dir)
	if trimmedDir == "" {
		return nil, fmt.Errorf("delivery outbox file store dir is required")
	}
	if err := filestore.EnsureDir(trimmedDir); err != nil {
		return nil, fmt.Errorf("create delivery outbox dir: %w", err)
	}
	coll := newDeliveryOutboxCollection(trimmedDir + "/delivery_outbox.json")
	if err := coll.Load(); err != nil {
		return nil, err
	}
	return &DeliveryOutboxLocalStore{coll: coll}, nil
}

func newDeliveryOutboxCollection(filePath string) *filestore.Collection[string, DeliveryIntent] {
	c := filestore.NewCollection[string, DeliveryIntent](filestore.CollectionConfig{
		FilePath: filePath,
		Perm:     0o600,
		Name:     "lark_delivery_outbox",
	})
	c.SetMarshalDoc(func(m map[string]DeliveryIntent) ([]byte, error) {
		doc := deliveryOutboxDoc{Intents: make([]DeliveryIntent, 0, len(m))}
		for _, intent := range m {
			doc.Intents = append(doc.Intents, cloneDeliveryIntent(intent))
		}
		return filestore.MarshalJSONIndent(doc)
	})
	c.SetUnmarshalDoc(func(data []byte) (map[string]DeliveryIntent, error) {
		var doc deliveryOutboxDoc
		if err := jsonx.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("decode delivery outbox: %w", err)
		}
		m := make(map[string]DeliveryIntent, len(doc.Intents))
		for _, intent := range doc.Intents {
			normalized, ok := normalizeDeliveryIntent(intent)
			if !ok {
				continue
			}
			m[normalized.IntentID] = normalized
		}
		return m, nil
	})
	return c
}

func (s *DeliveryOutboxLocalStore) EnsureSchema(ctx context.Context) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	return s.coll.EnsureDir()
}

func (s *DeliveryOutboxLocalStore) Enqueue(ctx context.Context, intents []DeliveryIntent) ([]DeliveryIntent, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if len(intents) == 0 {
		return nil, nil
	}
	now := s.coll.Now()
	out := make([]DeliveryIntent, 0, len(intents))
	err := s.coll.Mutate(func(items map[string]DeliveryIntent) error {
		for _, raw := range intents {
			intent, ok := normalizeDeliveryIntent(raw)
			if !ok {
				return fmt.Errorf("idempotency_key and chat_id are required")
			}
			if existing, found := findDeliveryIntentByIdempotency(items, intent.IdempotencyKey); found {
				out = append(out, cloneDeliveryIntent(existing))
				continue
			}
			if intent.IntentID == "" {
				intent.IntentID = "delivery-" + id.NewKSUID()
			}
			if intent.Channel == "" {
				intent.Channel = chatSessionBindingChannel
			}
			if intent.Status == "" {
				intent.Status = DeliveryIntentPending
			}
			if intent.CreatedAt.IsZero() {
				intent.CreatedAt = now
			}
			if intent.NextAttemptAt.IsZero() {
				intent.NextAttemptAt = now
			}
			intent.UpdatedAt = now
			items[intent.IntentID] = cloneDeliveryIntent(intent)
			out = append(out, cloneDeliveryIntent(intent))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *DeliveryOutboxLocalStore) ClaimPending(ctx context.Context, limit int, now time.Time) ([]DeliveryIntent, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	if now.IsZero() {
		now = s.coll.Now()
	}
	claimed := make([]DeliveryIntent, 0, limit)
	err := s.coll.Mutate(func(items map[string]DeliveryIntent) error {
		candidates := make([]DeliveryIntent, 0, len(items))
		for _, intent := range items {
			if intent.Status != DeliveryIntentPending && intent.Status != DeliveryIntentRetrying {
				continue
			}
			if !intent.NextAttemptAt.IsZero() && now.Before(intent.NextAttemptAt) {
				continue
			}
			candidates = append(candidates, cloneDeliveryIntent(intent))
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
				if candidates[i].Sequence == candidates[j].Sequence {
					return candidates[i].IntentID < candidates[j].IntentID
				}
				return candidates[i].Sequence < candidates[j].Sequence
			}
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		})
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		for _, intent := range candidates {
			stored, ok := items[intent.IntentID]
			if !ok {
				continue
			}
			stored.Status = DeliveryIntentSending
			stored.AttemptCount++
			stored.UpdatedAt = now
			items[intent.IntentID] = stored
			claimed = append(claimed, cloneDeliveryIntent(stored))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func (s *DeliveryOutboxLocalStore) MarkSent(ctx context.Context, intentID string, sentAt time.Time) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	intentID = strings.TrimSpace(intentID)
	if intentID == "" {
		return fmt.Errorf("intent_id is required")
	}
	if sentAt.IsZero() {
		sentAt = s.coll.Now()
	}
	return s.coll.Mutate(func(items map[string]DeliveryIntent) error {
		intent, ok := items[intentID]
		if !ok {
			return fmt.Errorf("delivery intent not found: %s", intentID)
		}
		intent.Status = DeliveryIntentSent
		intent.SentAt = sentAt
		intent.LastError = ""
		intent.NextAttemptAt = time.Time{}
		intent.UpdatedAt = sentAt
		items[intentID] = intent
		return nil
	})
}

func (s *DeliveryOutboxLocalStore) MarkRetry(ctx context.Context, intentID string, nextAttemptAt time.Time, lastErr string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	intentID = strings.TrimSpace(intentID)
	if intentID == "" {
		return fmt.Errorf("intent_id is required")
	}
	now := s.coll.Now()
	if nextAttemptAt.IsZero() {
		nextAttemptAt = now
	}
	return s.coll.Mutate(func(items map[string]DeliveryIntent) error {
		intent, ok := items[intentID]
		if !ok {
			return fmt.Errorf("delivery intent not found: %s", intentID)
		}
		intent.Status = DeliveryIntentRetrying
		intent.LastError = strings.TrimSpace(lastErr)
		intent.NextAttemptAt = nextAttemptAt
		intent.UpdatedAt = now
		items[intentID] = intent
		return nil
	})
}

func (s *DeliveryOutboxLocalStore) MarkDead(ctx context.Context, intentID string, lastErr string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	intentID = strings.TrimSpace(intentID)
	if intentID == "" {
		return fmt.Errorf("intent_id is required")
	}
	now := s.coll.Now()
	return s.coll.Mutate(func(items map[string]DeliveryIntent) error {
		intent, ok := items[intentID]
		if !ok {
			return fmt.Errorf("delivery intent not found: %s", intentID)
		}
		intent.Status = DeliveryIntentDead
		intent.LastError = strings.TrimSpace(lastErr)
		intent.NextAttemptAt = time.Time{}
		intent.UpdatedAt = now
		items[intentID] = intent
		return nil
	})
}

func (s *DeliveryOutboxLocalStore) GetByIdempotencyKey(ctx context.Context, key string) (DeliveryIntent, bool, error) {
	if err := s.ensureReady(ctx); err != nil {
		return DeliveryIntent{}, false, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return DeliveryIntent{}, false, nil
	}
	items := s.coll.Snapshot()
	intent, ok := findDeliveryIntentByIdempotency(items, key)
	if !ok {
		return DeliveryIntent{}, false, nil
	}
	return cloneDeliveryIntent(intent), true, nil
}

func (s *DeliveryOutboxLocalStore) Replay(ctx context.Context, filter ReplayFilter) (int, error) {
	if err := s.ensureReady(ctx); err != nil {
		return 0, err
	}
	idSet := make(map[string]struct{}, len(filter.IntentIDs))
	for _, intentID := range filter.IntentIDs {
		trimmed := strings.TrimSpace(intentID)
		if trimmed == "" {
			continue
		}
		idSet[trimmed] = struct{}{}
	}
	chatID := strings.TrimSpace(filter.ChatID)
	runID := strings.TrimSpace(filter.RunID)
	limit := filter.Limit
	if limit <= 0 {
		limit = len(idSet)
		if limit == 0 {
			limit = 100
		}
	}
	now := s.coll.Now()
	updated := 0
	err := s.coll.Mutate(func(items map[string]DeliveryIntent) error {
		candidates := make([]DeliveryIntent, 0, len(items))
		for _, intent := range items {
			if intent.Status != DeliveryIntentDead {
				continue
			}
			if len(idSet) > 0 {
				if _, ok := idSet[intent.IntentID]; !ok {
					continue
				}
			}
			if chatID != "" && intent.ChatID != chatID {
				continue
			}
			if runID != "" && intent.RunID != runID {
				continue
			}
			candidates = append(candidates, intent)
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
				return candidates[i].IntentID < candidates[j].IntentID
			}
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		})
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		for _, intent := range candidates {
			stored, ok := items[intent.IntentID]
			if !ok {
				continue
			}
			stored.Status = DeliveryIntentPending
			stored.LastError = ""
			stored.NextAttemptAt = now
			stored.UpdatedAt = now
			items[intent.IntentID] = stored
			updated++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return updated, nil
}

func findDeliveryIntentByIdempotency(items map[string]DeliveryIntent, key string) (DeliveryIntent, bool) {
	for _, intent := range items {
		if intent.IdempotencyKey == key {
			return intent, true
		}
	}
	return DeliveryIntent{}, false
}

func normalizeDeliveryIntent(intent DeliveryIntent) (DeliveryIntent, bool) {
	intent.IntentID = strings.TrimSpace(intent.IntentID)
	intent.Channel = strings.TrimSpace(intent.Channel)
	intent.ChatID = strings.TrimSpace(intent.ChatID)
	intent.ReplyToMessageID = strings.TrimSpace(intent.ReplyToMessageID)
	intent.ProgressMessageID = strings.TrimSpace(intent.ProgressMessageID)
	intent.SessionID = strings.TrimSpace(intent.SessionID)
	intent.RunID = strings.TrimSpace(intent.RunID)
	intent.EventType = strings.TrimSpace(intent.EventType)
	intent.IdempotencyKey = strings.TrimSpace(intent.IdempotencyKey)
	intent.MsgType = strings.TrimSpace(intent.MsgType)
	if intent.Channel == "" {
		intent.Channel = chatSessionBindingChannel
	}
	if intent.IdempotencyKey == "" || intent.ChatID == "" {
		return DeliveryIntent{}, false
	}
	if intent.MsgType == "" {
		intent.MsgType = "text"
	}
	switch intent.Status {
	case DeliveryIntentPending, DeliveryIntentSending, DeliveryIntentSent, DeliveryIntentRetrying, DeliveryIntentDead:
	default:
		intent.Status = DeliveryIntentPending
	}
	if len(intent.Attachments) > 0 {
		copied := make(map[string]ports.Attachment, len(intent.Attachments))
		for k, v := range intent.Attachments {
			copied[k] = v
		}
		intent.Attachments = copied
	}
	return intent, true
}

func cloneDeliveryIntent(intent DeliveryIntent) DeliveryIntent {
	copied := intent
	if len(intent.Attachments) > 0 {
		copied.Attachments = make(map[string]ports.Attachment, len(intent.Attachments))
		for k, v := range intent.Attachments {
			copied.Attachments[k] = v
		}
	}
	return copied
}

var _ DeliveryOutboxStore = (*DeliveryOutboxLocalStore)(nil)
