package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	authcrypto "alex/internal/auth/crypto"
	authdomain "alex/internal/auth/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userSeedOptions struct {
	email       string
	password    string
	displayName string
	status      authdomain.UserStatus
	points      int64
	tier        authdomain.SubscriptionTier
	expiresAt   *time.Time
	dbURL       string
}

func main() {
	opts := parseFlags()
	if err := seedUser(opts); err != nil {
		log.Fatalf("seed test account: %v", err)
	}
	log.Printf("Test account ready: %s (password: %s)\n", opts.email, opts.password)
}

func parseFlags() userSeedOptions {
	defaultDB := os.Getenv("AUTH_DATABASE_URL")
	email := flag.String("email", "admin@example.com", "Email for the test account")
	password := flag.String("password", "P@ssw0rd!", "Password for the test account")
	displayName := flag.String("name", "Admin", "Display name for the account")
	points := flag.Int64("points", 0, "Initial points balance")
	status := flag.String("status", string(authdomain.UserStatusActive), "Account status (active|disabled)")
	tier := flag.String("tier", string(authdomain.SubscriptionTierFree), "Subscription tier (free|supporter|professional)")
	expiresAt := flag.String("expires-at", "", "Optional RFC3339 timestamp when the subscription expires")
	dbURL := flag.String("db", defaultDB, "Postgres connection string (defaults to AUTH_DATABASE_URL)")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("database connection string is required (set AUTH_DATABASE_URL or pass -db)")
	}

	statusValue := authdomain.UserStatus(strings.ToLower(*status))
	if statusValue != authdomain.UserStatusActive && statusValue != authdomain.UserStatusDisabled {
		log.Fatalf("invalid status %q (expected active or disabled)", *status)
	}

	tierValue := authdomain.SubscriptionTier(strings.ToLower(*tier))
	if !tierValue.IsValid() {
		log.Fatalf("invalid subscription tier %q (expected free, supporter, or professional)", *tier)
	}

	var expiresPtr *time.Time
	if strings.TrimSpace(*expiresAt) != "" {
		ts, err := time.Parse(time.RFC3339, *expiresAt)
		if err != nil {
			log.Fatalf("parse expires-at: %v", err)
		}
		expiresPtr = &ts
	}

	return userSeedOptions{
		email:       *email,
		password:    *password,
		displayName: *displayName,
		status:      statusValue,
		points:      *points,
		tier:        tierValue,
		expiresAt:   expiresPtr,
		dbURL:       *dbURL,
	}
}

func seedUser(opts userSeedOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, opts.dbURL)
	if err != nil {
		return fmt.Errorf("connect to Postgres: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping Postgres: %w", err)
	}

	hashed, err := authcrypto.HashPassword(opts.password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	email := strings.TrimSpace(strings.ToLower(opts.email))
	if email == "" {
		return fmt.Errorf("email is required")
	}

	now := time.Now().UTC()
	args := []any{
		uuid.NewString(),
		email,
		opts.displayName,
		opts.status,
		hashed,
		opts.points,
		opts.tier,
		opts.expiresAt,
		now,
		now,
	}

	const query = `
INSERT INTO auth_users (id, email, display_name, status, password_hash, points_balance, subscription_tier, subscription_expires_at, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (email) DO UPDATE
SET display_name = EXCLUDED.display_name,
    status = EXCLUDED.status,
    password_hash = EXCLUDED.password_hash,
    points_balance = EXCLUDED.points_balance,
    subscription_tier = EXCLUDED.subscription_tier,
    subscription_expires_at = EXCLUDED.subscription_expires_at,
    updated_at = NOW();`

	if _, err := pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}
