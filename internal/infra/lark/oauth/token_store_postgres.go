package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const oauthTokenTable = "lark_oauth_tokens"

type PostgresTokenStore struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewPostgresTokenStore(pool *pgxpool.Pool) *PostgresTokenStore {
	return &PostgresTokenStore{pool: pool, now: time.Now}
}

func (s *PostgresTokenStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres token store not initialized")
	}
	statements := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  open_id TEXT PRIMARY KEY,
  access_token TEXT NOT NULL,
  refresh_token TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  refresh_expires_at TIMESTAMPTZ,
  scope TEXT,
  token_type TEXT,
  updated_at TIMESTAMPTZ NOT NULL
);`, oauthTokenTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_updated_at ON %s (updated_at DESC);`, oauthTokenTable, oauthTokenTable),
	}
	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresTokenStore) Get(ctx context.Context, openID string) (Token, error) {
	if s == nil || s.pool == nil {
		return Token{}, fmt.Errorf("postgres token store not initialized")
	}
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return Token{}, fmt.Errorf("open_id required")
	}
	if err := ctx.Err(); err != nil {
		return Token{}, err
	}
	var token Token
	row := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT open_id, access_token, refresh_token, expires_at, refresh_expires_at, scope, token_type, updated_at FROM %s WHERE open_id=$1`, oauthTokenTable), openID)
	var refreshExpiresAt *time.Time
	var scope *string
	var tokenType *string
	if err := row.Scan(&token.OpenID, &token.AccessToken, &token.RefreshToken, &token.ExpiresAt, &refreshExpiresAt, &scope, &tokenType, &token.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Token{}, ErrTokenNotFound
		}
		return Token{}, err
	}
	if refreshExpiresAt != nil {
		token.RefreshExpiresAt = *refreshExpiresAt
	}
	if scope != nil {
		token.Scope = *scope
	}
	if tokenType != nil {
		token.TokenType = *tokenType
	}
	return token, nil
}

func (s *PostgresTokenStore) Upsert(ctx context.Context, token Token) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres token store not initialized")
	}
	token.OpenID = strings.TrimSpace(token.OpenID)
	if token.OpenID == "" {
		return fmt.Errorf("open_id required")
	}
	if token.AccessToken == "" {
		return fmt.Errorf("access_token required")
	}
	if token.RefreshToken == "" {
		return fmt.Errorf("refresh_token required")
	}
	if token.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at required")
	}
	if token.UpdatedAt.IsZero() {
		token.UpdatedAt = s.now()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var refreshExpiresAt *time.Time
	if !token.RefreshExpiresAt.IsZero() {
		refreshExpiresAt = &token.RefreshExpiresAt
	}
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (open_id, access_token, refresh_token, expires_at, refresh_expires_at, scope, token_type, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (open_id) DO UPDATE SET
  access_token=EXCLUDED.access_token,
  refresh_token=EXCLUDED.refresh_token,
  expires_at=EXCLUDED.expires_at,
  refresh_expires_at=EXCLUDED.refresh_expires_at,
  scope=EXCLUDED.scope,
  token_type=EXCLUDED.token_type,
  updated_at=EXCLUDED.updated_at`, oauthTokenTable),
		token.OpenID,
		token.AccessToken,
		token.RefreshToken,
		token.ExpiresAt,
		refreshExpiresAt,
		emptyToNil(token.Scope),
		emptyToNil(token.TokenType),
		token.UpdatedAt,
	)
	return err
}

func (s *PostgresTokenStore) Delete(ctx context.Context, openID string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres token store not initialized")
	}
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return fmt.Errorf("open_id required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE open_id=$1`, oauthTokenTable), openID)
	return err
}

func emptyToNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

var _ TokenStore = (*PostgresTokenStore)(nil)
