package points

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"alex/internal/auth/domain"
)

func TestAdjustCreditsWithPromotions(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	userRepo := &mockUserRepo{user: domain.User{ID: "user-1"}}
	ledgerRepo := &mockLedgerRepo{}
	promos := &mockPromoFeed{promotions: []domain.Promotion{{Code: "WELCOME", PointsBonus: 10}, {Code: "ALMOST", PointsBonus: 5, ExpiresAt: refTime(now.Add(1 * time.Hour))}}}

	svc := NewService(userRepo, ledgerRepo, promos, Config{MaxBalance: 1000})
	svc.WithNow(func() time.Time { return now })
	publisher := &recordingPublisher{}
	svc.AttachPublisher(publisher)

	event, err := svc.Adjust(context.Background(), "user-1", 20, "test-credit", map[string]any{"source": "task"}, "corr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != domain.PointsEventCredited {
		t.Fatalf("expected credited event, got %s", event.Type)
	}
	if got, want := event.Entry.Delta, int64(35); got != want {
		t.Fatalf("expected delta %d, got %d", want, got)
	}
	if event.Entry.Metadata["promotion_bonus"] != int64(15) {
		t.Fatalf("expected promotion bonus metadata")
	}
	if userRepo.user.PointsBalance != 35 {
		t.Fatalf("expected user balance 35, got %d", userRepo.user.PointsBalance)
	}
	if balanceBefore := *event.BalanceBefore; balanceBefore != 0 {
		t.Fatalf("expected balance before 0, got %d", balanceBefore)
	}
	if got := ledgerRepo.entries[0].Reason; got != "test-credit" {
		t.Fatalf("expected ledger reason test-credit, got %s", got)
	}
	if got := len(publisher.pointsEvents); got != 1 {
		t.Fatalf("expected 1 published event, got %d", got)
	}
}

func TestAdjustDebitsAndGuardsAgainstNegative(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	userRepo := &mockUserRepo{user: domain.User{ID: "user-1", PointsBalance: 25}}
	ledgerRepo := &mockLedgerRepo{}
	svc := NewService(userRepo, ledgerRepo, nil, Config{})
	svc.WithNow(func() time.Time { return now })
	publisher := &recordingPublisher{}
	svc.AttachPublisher(publisher)

	event, err := svc.Adjust(context.Background(), "user-1", -20, "redeem", nil, "corr-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != domain.PointsEventDebited {
		t.Fatalf("expected debited event, got %s", event.Type)
	}
	if userRepo.user.PointsBalance != 5 {
		t.Fatalf("expected updated balance 5, got %d", userRepo.user.PointsBalance)
	}

	if _, err := svc.Adjust(context.Background(), "user-1", -10, "overdraft", nil, "corr-3"); !errors.Is(err, domain.ErrInsufficientPoints) {
		t.Fatalf("expected insufficient points error, got %v", err)
	}
	if len(ledgerRepo.entries) != 1 {
		t.Fatalf("expected only one ledger entry after failed debit")
	}
	if got := len(publisher.pointsEvents); got != 1 {
		t.Fatalf("expected 1 published event, got %d", got)
	}
}

func TestAdjustValidatesInputs(t *testing.T) {
	svc := NewService(&mockUserRepo{}, &mockLedgerRepo{}, nil, Config{})
	if _, err := svc.Adjust(context.Background(), "user-1", 0, "noop", nil, "corr"); err == nil {
		t.Fatalf("expected error for zero delta")
	}
	if _, err := svc.Adjust(context.Background(), "user-1", 1, "", nil, "corr"); err == nil {
		t.Fatalf("expected error for missing reason")
	}
}

type mockUserRepo struct {
	user domain.User
}

func (m *mockUserRepo) Create(ctx context.Context, user domain.User) (domain.User, error) {
	return domain.User{}, errors.New("not implemented")
}
func (m *mockUserRepo) Update(ctx context.Context, user domain.User) (domain.User, error) {
	m.user = user
	return user, nil
}
func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return domain.User{}, errors.New("not implemented")
}
func (m *mockUserRepo) FindByID(ctx context.Context, id string) (domain.User, error) {
	return m.user, nil
}

func (m *mockUserRepo) reset() { m.user = domain.User{} }

type mockLedgerRepo struct {
	entries []domain.PointsLedgerEntry
}

func (m *mockLedgerRepo) AppendEntry(ctx context.Context, entry domain.PointsLedgerEntry) (domain.PointsLedgerEntry, error) {
	m.entries = append(m.entries, entry)
	return entry, nil
}
func (m *mockLedgerRepo) ListEntries(ctx context.Context, userID string, limit int) ([]domain.PointsLedgerEntry, error) {
	return nil, nil
}
func (m *mockLedgerRepo) LatestBalance(ctx context.Context, userID string) (int64, error) {
	return 0, domain.ErrPointsLedgerEntryNotFound
}

type mockPromoFeed struct {
	promotions []domain.Promotion
}

func (m *mockPromoFeed) ListActive(ctx context.Context, userID string) ([]domain.Promotion, error) {
	return m.promotions, nil
}

func refTime(t time.Time) *time.Time {
	return &t
}

func TestCloneMetadata(t *testing.T) {
	original := map[string]any{"foo": "bar"}
	cloned := cloneMetadata(original)
	if !reflect.DeepEqual(original, cloned) {
		t.Fatalf("expected clone to match original")
	}
	cloned["foo"] = "baz"
	if original["foo"].(string) != "bar" {
		t.Fatalf("expected original untouched")
	}
	empty := cloneMetadata(nil)
	if len(empty) != 0 {
		t.Fatalf("expected nil metadata to clone to empty map")
	}
}

type recordingPublisher struct {
	pointsEvents []domain.PointsLedgerEvent
}

func (r *recordingPublisher) PublishSubscriptionEvent(ctx context.Context, event domain.SubscriptionEvent) error {
	return nil
}

func (r *recordingPublisher) PublishPointsEvent(ctx context.Context, event domain.PointsLedgerEvent) error {
	r.pointsEvents = append(r.pointsEvents, event)
	return nil
}
