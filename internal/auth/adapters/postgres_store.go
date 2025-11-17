package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"alex/internal/auth/domain"
)

type PostgresUserRepo struct {
	pool *pgxpool.Pool
}

type PostgresIdentityRepo struct {
	pool *pgxpool.Pool
}

type PostgresSessionRepo struct {
	pool     *pgxpool.Pool
	verifier func(string, string) (bool, error)
}

type PostgresStateStore struct {
	pool *pgxpool.Pool
}

type PostgresPlanRepo struct {
	pool *pgxpool.Pool
}

type PostgresSubscriptionRepo struct {
	pool *pgxpool.Pool
}

type PostgresPointsLedgerRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresStores(pool *pgxpool.Pool) (*PostgresUserRepo, *PostgresIdentityRepo, *PostgresSessionRepo, *PostgresStateStore) {
	sessions := &PostgresSessionRepo{pool: pool, verifier: func(string, string) (bool, error) {
		return false, fmt.Errorf("refresh token verifier not configured")
	}}
	return &PostgresUserRepo{pool: pool}, &PostgresIdentityRepo{pool: pool}, sessions, &PostgresStateStore{pool: pool}
}

// NewPostgresPlanRepo returns a PlanRepository backed by auth_plans.
func NewPostgresPlanRepo(pool *pgxpool.Pool) *PostgresPlanRepo {
	return &PostgresPlanRepo{pool: pool}
}

// NewPostgresSubscriptionRepo returns a SubscriptionRepository backed by auth_subscriptions.
func NewPostgresSubscriptionRepo(pool *pgxpool.Pool) *PostgresSubscriptionRepo {
	return &PostgresSubscriptionRepo{pool: pool}
}

// NewPostgresPointsLedgerRepo returns a PointsLedgerRepository backed by auth_points_ledger.
func NewPostgresPointsLedgerRepo(pool *pgxpool.Pool) *PostgresPointsLedgerRepo {
	return &PostgresPointsLedgerRepo{pool: pool}
}

func (r *PostgresUserRepo) Create(ctx context.Context, user domain.User) (domain.User, error) {
	query := `
INSERT INTO auth_users (id, email, display_name, status, password_hash, points_balance, subscription_tier, subscription_expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING id, email, display_name, status, password_hash, points_balance, subscription_tier, subscription_expires_at, created_at, updated_at
`
	var created domain.User
	var expiresAt sql.NullTime
	err := r.pool.QueryRow(ctx, query,
		user.ID,
		user.Email,
		user.DisplayName,
		string(user.Status),
		user.PasswordHash,
		user.PointsBalance,
		string(user.SubscriptionTier),
		user.SubscriptionExpiresAt,
		user.CreatedAt,
	).Scan(
		&created.ID,
		&created.Email,
		&created.DisplayName,
		&created.Status,
		&created.PasswordHash,
		&created.PointsBalance,
		&created.SubscriptionTier,
		&expiresAt,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if expiresAt.Valid {
		t := expiresAt.Time
		created.SubscriptionExpiresAt = &t
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, err
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, domain.ErrUserExists
		}
		return domain.User{}, err
	}
	return created, nil
}

func (r *PostgresUserRepo) Update(ctx context.Context, user domain.User) (domain.User, error) {
	query := `
UPDATE auth_users
SET email = $2,
    display_name = $3,
    status = $4,
    password_hash = $5,
    points_balance = $6,
    subscription_tier = $7,
    subscription_expires_at = $8,
    updated_at = $9
WHERE id = $1
RETURNING id, email, display_name, status, password_hash, points_balance, subscription_tier, subscription_expires_at, created_at, updated_at
`
	var updated domain.User
	var expiresAt sql.NullTime
	err := r.pool.QueryRow(ctx, query,
		user.ID,
		user.Email,
		user.DisplayName,
		string(user.Status),
		user.PasswordHash,
		user.PointsBalance,
		string(user.SubscriptionTier),
		user.SubscriptionExpiresAt,
		user.UpdatedAt,
	).Scan(
		&updated.ID,
		&updated.Email,
		&updated.DisplayName,
		&updated.Status,
		&updated.PasswordHash,
		&updated.PointsBalance,
		&updated.SubscriptionTier,
		&expiresAt,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if expiresAt.Valid {
		t := expiresAt.Time
		updated.SubscriptionExpiresAt = &t
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, err
	}
	return updated, nil
}

func (r *PostgresUserRepo) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	query := `
SELECT id, email, display_name, status, password_hash, points_balance, subscription_tier, subscription_expires_at, created_at, updated_at
FROM auth_users
WHERE email = $1
`
	var user domain.User
	var expiresAt sql.NullTime
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.Status,
		&user.PasswordHash,
		&user.PointsBalance,
		&user.SubscriptionTier,
		&expiresAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if expiresAt.Valid {
		t := expiresAt.Time
		user.SubscriptionExpiresAt = &t
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresUserRepo) FindByID(ctx context.Context, id string) (domain.User, error) {
	query := `
SELECT id, email, display_name, status, password_hash, points_balance, subscription_tier, subscription_expires_at, created_at, updated_at
FROM auth_users
WHERE id = $1
`
	var user domain.User
	var expiresAt sql.NullTime
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.Status,
		&user.PasswordHash,
		&user.PointsBalance,
		&user.SubscriptionTier,
		&expiresAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if expiresAt.Valid {
		t := expiresAt.Time
		user.SubscriptionExpiresAt = &t
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrUserNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (r *PostgresIdentityRepo) Create(ctx context.Context, identity domain.Identity) (domain.Identity, error) {
	query := `
INSERT INTO auth_user_identities (id, user_id, provider, provider_uid, access_token, refresh_token, expires_at, scopes, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING id, user_id, provider, provider_uid, access_token, refresh_token, expires_at, scopes, created_at, updated_at
`
	tokens := identity.Tokens
	var scopes []string
	if len(tokens.Scopes) > 0 {
		scopes = tokens.Scopes
	}
	var created domain.Identity
	err := r.pool.QueryRow(ctx, query,
		identity.ID,
		identity.UserID,
		string(identity.Provider),
		identity.ProviderID,
		tokens.AccessToken,
		tokens.RefreshToken,
		tokens.Expiry,
		scopes,
		identity.CreatedAt,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.Provider,
		&created.ProviderID,
		&created.Tokens.AccessToken,
		&created.Tokens.RefreshToken,
		&created.Tokens.Expiry,
		&created.Tokens.Scopes,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Identity{}, fmt.Errorf("identity already exists: %w", err)
		}
		return domain.Identity{}, err
	}
	return created, nil
}

func (r *PostgresIdentityRepo) Update(ctx context.Context, identity domain.Identity) (domain.Identity, error) {
	query := `
UPDATE auth_user_identities
SET access_token = $2,
    refresh_token = $3,
    expires_at = $4,
    scopes = $5,
    updated_at = $6
WHERE id = $1
RETURNING id, user_id, provider, provider_uid, access_token, refresh_token, expires_at, scopes, created_at, updated_at
`
	var updated domain.Identity
	err := r.pool.QueryRow(ctx, query,
		identity.ID,
		identity.Tokens.AccessToken,
		identity.Tokens.RefreshToken,
		identity.Tokens.Expiry,
		identity.Tokens.Scopes,
		identity.UpdatedAt,
	).Scan(
		&updated.ID,
		&updated.UserID,
		&updated.Provider,
		&updated.ProviderID,
		&updated.Tokens.AccessToken,
		&updated.Tokens.RefreshToken,
		&updated.Tokens.Expiry,
		&updated.Tokens.Scopes,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Identity{}, domain.ErrIdentityNotFound
		}
		return domain.Identity{}, err
	}
	return updated, nil
}

func (r *PostgresIdentityRepo) FindByProvider(ctx context.Context, provider domain.ProviderType, providerID string) (domain.Identity, error) {
	query := `
SELECT id, user_id, provider, provider_uid, access_token, refresh_token, expires_at, scopes, created_at, updated_at
FROM auth_user_identities
WHERE provider = $1 AND provider_uid = $2
`
	var identity domain.Identity
	err := r.pool.QueryRow(ctx, query, string(provider), providerID).Scan(
		&identity.ID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderID,
		&identity.Tokens.AccessToken,
		&identity.Tokens.RefreshToken,
		&identity.Tokens.Expiry,
		&identity.Tokens.Scopes,
		&identity.CreatedAt,
		&identity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Identity{}, domain.ErrIdentityNotFound
		}
		return domain.Identity{}, err
	}
	return identity, nil
}

func (r *PostgresSessionRepo) SetVerifier(verifier func(string, string) (bool, error)) {
	if verifier == nil {
		return
	}
	r.verifier = verifier
}

func (r *PostgresSessionRepo) Create(ctx context.Context, session domain.Session) (domain.Session, error) {
	now := session.CreatedAt
	if now.IsZero() {
		now = time.Now()
	}
	query := `
INSERT INTO auth_sessions (id, user_id, refresh_token_hash, refresh_token_fingerprint, user_agent, ip_address, created_at, updated_at, expires_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::inet, $7, $7, $8, $7)
RETURNING id, user_id, refresh_token_hash, refresh_token_fingerprint, user_agent, COALESCE(ip_address::text, ''), created_at, expires_at
`
	var created domain.Session
	err := r.pool.QueryRow(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshTokenHash,
		session.RefreshTokenFingerprint,
		session.UserAgent,
		session.IP,
		now,
		session.ExpiresAt,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.RefreshTokenHash,
		&created.RefreshTokenFingerprint,
		&created.UserAgent,
		&created.IP,
		&created.CreatedAt,
		&created.ExpiresAt,
	)
	if err != nil {
		return domain.Session{}, err
	}
	return created, nil
}

func (r *PostgresSessionRepo) DeleteByID(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE id = $1`, id)
	return err
}

func (r *PostgresSessionRepo) DeleteByUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE user_id = $1`, userID)
	return err
}

func (r *PostgresSessionRepo) FindByRefreshToken(ctx context.Context, refreshToken string) (domain.Session, error) {
	fingerprint := domain.FingerprintRefreshToken(refreshToken)
	query := `
SELECT id, user_id, refresh_token_hash, refresh_token_fingerprint, user_agent, COALESCE(ip_address::text, ''), created_at, expires_at
FROM auth_sessions
WHERE refresh_token_fingerprint = $1
ORDER BY created_at DESC
LIMIT 1
`
	var session domain.Session
	err := r.pool.QueryRow(ctx, query, fingerprint).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshTokenHash,
		&session.RefreshTokenFingerprint,
		&session.UserAgent,
		&session.IP,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.ErrSessionNotFound
		}
		return domain.Session{}, err
	}
	match, err := r.verifier(refreshToken, session.RefreshTokenHash)
	if err != nil {
		return domain.Session{}, err
	}
	if !match {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	return session, nil
}

func (s *PostgresStateStore) Save(ctx context.Context, state string, provider domain.ProviderType, expiresAt time.Time) error {
	query := `
INSERT INTO auth_states (state, provider, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (state) DO UPDATE SET provider = EXCLUDED.provider, expires_at = EXCLUDED.expires_at
`
	_, err := s.pool.Exec(ctx, query, state, string(provider), expiresAt)
	return err
}

func (s *PostgresStateStore) Consume(ctx context.Context, state string, provider domain.ProviderType) error {
	query := `
DELETE FROM auth_states
WHERE state = $1 AND provider = $2
RETURNING expires_at
`
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, query, state, string(provider)).Scan(&expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrStateMismatch
		}
		return err
	}
	if !expiresAt.IsZero() && time.Now().After(expiresAt) {
		return domain.ErrStateMismatch
	}
	return nil
}

func (s *PostgresStateStore) PurgeExpired(ctx context.Context, before time.Time) (int64, error) {
	cmdTag, err := s.pool.Exec(ctx, `DELETE FROM auth_states WHERE expires_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return cmdTag.RowsAffected(), nil
}

// List returns the subscription catalog from auth_plans.
func (r *PostgresPlanRepo) List(ctx context.Context, includeInactive bool) ([]domain.SubscriptionPlan, error) {
	query := `SELECT tier, display_name, monthly_price_cents, currency, is_active, metadata, created_at, updated_at FROM auth_plans`
	if !includeInactive {
		query += ` WHERE is_active = TRUE`
	}
	query += ` ORDER BY monthly_price_cents ASC, tier ASC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := []domain.SubscriptionPlan{}
	for rows.Next() {
		plan, err := scanSubscriptionPlanRow(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return plans, nil
}

// FindByTier returns the catalog entry for the tier.
func (r *PostgresPlanRepo) FindByTier(ctx context.Context, tier domain.SubscriptionTier) (domain.SubscriptionPlan, error) {
	query := `SELECT tier, display_name, monthly_price_cents, currency, is_active, metadata, created_at, updated_at FROM auth_plans WHERE tier = $1`
	plan, err := scanSubscriptionPlanRow(r.pool.QueryRow(ctx, query, string(tier)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPlan{}, domain.ErrInvalidSubscriptionTier
		}
		return domain.SubscriptionPlan{}, err
	}
	return plan, nil
}

// Upsert inserts or updates a plan definition.
func (r *PostgresPlanRepo) Upsert(ctx context.Context, plan domain.SubscriptionPlan) error {
	metadata, err := encodeMetadata(plan.Metadata)
	if err != nil {
		return err
	}
	query := `
INSERT INTO auth_plans (tier, display_name, monthly_price_cents, currency, is_active, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tier) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    monthly_price_cents = EXCLUDED.monthly_price_cents,
    currency = EXCLUDED.currency,
    is_active = EXCLUDED.is_active,
    metadata = EXCLUDED.metadata,
    updated_at = NOW()
`
	_, err = r.pool.Exec(ctx, query,
		string(plan.Tier),
		plan.DisplayName,
		plan.MonthlyPriceCents,
		plan.Currency,
		plan.IsActive,
		metadata,
	)
	return err
}

// Create inserts a subscription row.
func (r *PostgresSubscriptionRepo) Create(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error) {
	metadata, err := encodeMetadata(subscription.Metadata)
	if err != nil {
		return domain.Subscription{}, err
	}
	query := `
INSERT INTO auth_subscriptions (id, user_id, tier, status, auto_renew, current_period_start, current_period_end, external_customer_id, external_subscription_id, external_plan_id, metadata, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), $11, $12, $12)
RETURNING id, user_id, tier, status, auto_renew, current_period_start, current_period_end, external_customer_id, external_subscription_id, external_plan_id, metadata, created_at, updated_at
`
	var currentPeriodEnd sql.NullTime
	var rawMetadata []byte
	var created domain.Subscription
	err = r.pool.QueryRow(ctx, query,
		subscription.ID,
		subscription.UserID,
		string(subscription.Tier),
		string(subscription.Status),
		subscription.AutoRenew,
		subscription.CurrentPeriodStart,
		subscription.CurrentPeriodEnd,
		subscription.ExternalCustomerID,
		subscription.ExternalSubscriptionID,
		subscription.ExternalPlanID,
		metadata,
		subscription.CreatedAt,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.Tier,
		&created.Status,
		&created.AutoRenew,
		&created.CurrentPeriodStart,
		&currentPeriodEnd,
		&created.ExternalCustomerID,
		&created.ExternalSubscriptionID,
		&created.ExternalPlanID,
		&rawMetadata,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return domain.Subscription{}, err
	}
	if currentPeriodEnd.Valid {
		t := currentPeriodEnd.Time
		created.CurrentPeriodEnd = &t
	}
	if created.Metadata, err = decodeMetadata(rawMetadata); err != nil {
		return domain.Subscription{}, err
	}
	return created, nil
}

// Update persists subscription changes.
func (r *PostgresSubscriptionRepo) Update(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error) {
	metadata, err := encodeMetadata(subscription.Metadata)
	if err != nil {
		return domain.Subscription{}, err
	}
	query := `
UPDATE auth_subscriptions
SET tier = $2,
    status = $3,
    auto_renew = $4,
    current_period_start = $5,
    current_period_end = $6,
    external_customer_id = NULLIF($7, ''),
    external_subscription_id = NULLIF($8, ''),
    external_plan_id = NULLIF($9, ''),
    metadata = $10,
    updated_at = $11
WHERE id = $1
RETURNING id, user_id, tier, status, auto_renew, current_period_start, current_period_end, external_customer_id, external_subscription_id, external_plan_id, metadata, created_at, updated_at
`
	var currentPeriodEnd sql.NullTime
	var rawMetadata []byte
	var updated domain.Subscription
	err = r.pool.QueryRow(ctx, query,
		subscription.ID,
		string(subscription.Tier),
		string(subscription.Status),
		subscription.AutoRenew,
		subscription.CurrentPeriodStart,
		subscription.CurrentPeriodEnd,
		subscription.ExternalCustomerID,
		subscription.ExternalSubscriptionID,
		subscription.ExternalPlanID,
		metadata,
		subscription.UpdatedAt,
	).Scan(
		&updated.ID,
		&updated.UserID,
		&updated.Tier,
		&updated.Status,
		&updated.AutoRenew,
		&updated.CurrentPeriodStart,
		&currentPeriodEnd,
		&updated.ExternalCustomerID,
		&updated.ExternalSubscriptionID,
		&updated.ExternalPlanID,
		&rawMetadata,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, domain.ErrSubscriptionNotFound
		}
		return domain.Subscription{}, err
	}
	if currentPeriodEnd.Valid {
		t := currentPeriodEnd.Time
		updated.CurrentPeriodEnd = &t
	}
	if updated.Metadata, err = decodeMetadata(rawMetadata); err != nil {
		return domain.Subscription{}, err
	}
	return updated, nil
}

// FindActiveByUser returns the most recent non-canceled subscription for a user.
func (r *PostgresSubscriptionRepo) FindActiveByUser(ctx context.Context, userID string) (domain.Subscription, error) {
	query := `
SELECT id, user_id, tier, status, auto_renew, current_period_start, current_period_end, external_customer_id, external_subscription_id, external_plan_id, metadata, created_at, updated_at
FROM auth_subscriptions
WHERE user_id = $1 AND status != 'canceled'
ORDER BY updated_at DESC
LIMIT 1
`
	subscription, err := scanSubscriptionRow(r.pool.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, domain.ErrSubscriptionNotFound
		}
		return domain.Subscription{}, err
	}
	return subscription, nil
}

// ListExpiring returns subscriptions that expire before the provided instant.
func (r *PostgresSubscriptionRepo) ListExpiring(ctx context.Context, before time.Time) ([]domain.Subscription, error) {
	query := `
SELECT id, user_id, tier, status, auto_renew, current_period_start, current_period_end, external_customer_id, external_subscription_id, external_plan_id, metadata, created_at, updated_at
FROM auth_subscriptions
WHERE status = 'active' AND current_period_end IS NOT NULL AND current_period_end <= $1
ORDER BY current_period_end ASC
`
	rows, err := r.pool.Query(ctx, query, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subscriptions []domain.Subscription
	for rows.Next() {
		sub, err := scanSubscriptionRow(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

// AppendEntry writes a new ledger entry.
func (r *PostgresPointsLedgerRepo) AppendEntry(ctx context.Context, entry domain.PointsLedgerEntry) (domain.PointsLedgerEntry, error) {
	metadata, err := encodeMetadata(entry.Metadata)
	if err != nil {
		return domain.PointsLedgerEntry{}, err
	}
	query := `
INSERT INTO auth_points_ledger (id, user_id, delta, balance_after, reason, metadata, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, delta, balance_after, reason, metadata, created_at
`
	var rawMetadata []byte
	var created domain.PointsLedgerEntry
	err = r.pool.QueryRow(ctx, query,
		entry.ID,
		entry.UserID,
		entry.Delta,
		entry.BalanceAfter,
		entry.Reason,
		metadata,
		entry.CreatedAt,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.Delta,
		&created.BalanceAfter,
		&created.Reason,
		&rawMetadata,
		&created.CreatedAt,
	)
	if err != nil {
		return domain.PointsLedgerEntry{}, err
	}
	if created.Metadata, err = decodeMetadata(rawMetadata); err != nil {
		return domain.PointsLedgerEntry{}, err
	}
	return created, nil
}

// ListEntries returns the newest ledger entries up to limit.
func (r *PostgresPointsLedgerRepo) ListEntries(ctx context.Context, userID string, limit int) ([]domain.PointsLedgerEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
SELECT id, user_id, delta, balance_after, reason, metadata, created_at
FROM auth_points_ledger
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2
`
	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []domain.PointsLedgerEntry
	for rows.Next() {
		entry, err := scanLedgerEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// LatestBalance returns the most recent balance recorded in the ledger.
func (r *PostgresPointsLedgerRepo) LatestBalance(ctx context.Context, userID string) (int64, error) {
	query := `
SELECT balance_after
FROM auth_points_ledger
WHERE user_id = $1 AND balance_after IS NOT NULL
ORDER BY created_at DESC
LIMIT 1
`
	var balance sql.NullInt64
	err := r.pool.QueryRow(ctx, query, userID).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrPointsLedgerEntryNotFound
		}
		return 0, err
	}
	if !balance.Valid {
		return 0, domain.ErrPointsLedgerEntryNotFound
	}
	return balance.Int64, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSubscriptionPlanRow(row rowScanner) (domain.SubscriptionPlan, error) {
	var plan domain.SubscriptionPlan
	var rawMetadata []byte
	err := row.Scan(
		&plan.Tier,
		&plan.DisplayName,
		&plan.MonthlyPriceCents,
		&plan.Currency,
		&plan.IsActive,
		&rawMetadata,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	)
	if err != nil {
		return domain.SubscriptionPlan{}, err
	}
	if plan.Metadata, err = decodeMetadata(rawMetadata); err != nil {
		return domain.SubscriptionPlan{}, err
	}
	return plan, nil
}

func scanSubscriptionRow(row rowScanner) (domain.Subscription, error) {
	var subscription domain.Subscription
	var currentPeriodEnd sql.NullTime
	var rawMetadata []byte
	err := row.Scan(
		&subscription.ID,
		&subscription.UserID,
		&subscription.Tier,
		&subscription.Status,
		&subscription.AutoRenew,
		&subscription.CurrentPeriodStart,
		&currentPeriodEnd,
		&subscription.ExternalCustomerID,
		&subscription.ExternalSubscriptionID,
		&subscription.ExternalPlanID,
		&rawMetadata,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	)
	if err != nil {
		return domain.Subscription{}, err
	}
	if currentPeriodEnd.Valid {
		t := currentPeriodEnd.Time
		subscription.CurrentPeriodEnd = &t
	}
	if subscription.Metadata, err = decodeMetadata(rawMetadata); err != nil {
		return domain.Subscription{}, err
	}
	return subscription, nil
}

func scanLedgerEntry(row rowScanner) (domain.PointsLedgerEntry, error) {
	var entry domain.PointsLedgerEntry
	var rawMetadata []byte
	err := row.Scan(
		&entry.ID,
		&entry.UserID,
		&entry.Delta,
		&entry.BalanceAfter,
		&entry.Reason,
		&rawMetadata,
		&entry.CreatedAt,
	)
	if err != nil {
		return domain.PointsLedgerEntry{}, err
	}
	if entry.Metadata, err = decodeMetadata(rawMetadata); err != nil {
		return domain.PointsLedgerEntry{}, err
	}
	return entry, nil
}

func encodeMetadata(metadata map[string]any) ([]byte, error) {
	if metadata == nil {
		return []byte("{}"), nil
	}
	if len(metadata) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(metadata)
}

func decodeMetadata(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, err
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return metadata, nil
}
