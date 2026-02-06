package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"alex/internal/domain/auth"
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

func NewPostgresStores(pool *pgxpool.Pool) (*PostgresUserRepo, *PostgresIdentityRepo, *PostgresSessionRepo, *PostgresStateStore) {
	sessions := &PostgresSessionRepo{pool: pool, verifier: func(string, string) (bool, error) {
		return false, fmt.Errorf("refresh token verifier not configured")
	}}
	return &PostgresUserRepo{pool: pool}, &PostgresIdentityRepo{pool: pool}, sessions, &PostgresStateStore{pool: pool}
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
