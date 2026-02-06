package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	authdomain "alex/internal/domain/auth"
	authadapters "alex/internal/infra/auth/adapters"
	authcrypto "alex/internal/infra/auth/crypto"
	"alex/internal/shared/config"
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
	log.Printf("Test account ready: %s\n", opts.email)
}

func parseFlags() userSeedOptions {
	defaultDB := ""
	if fileCfg, _, err := config.LoadFileConfig(); err == nil && fileCfg.Auth != nil {
		defaultDB = strings.TrimSpace(fileCfg.Auth.DatabaseURL)
	}
	email := flag.String("email", "admin@example.com", "Email for the test account")
	password := flag.String("password", "P@ssw0rd!", "Password for the test account")
	displayName := flag.String("name", "Admin", "Display name for the account")
	points := flag.Int64("points", 0, "Initial points balance")
	status := flag.String("status", string(authdomain.UserStatusActive), "Account status (active|disabled)")
	tier := flag.String("tier", string(authdomain.SubscriptionTierFree), "Subscription tier (free|supporter|professional)")
	expiresAt := flag.String("expires-at", "", "Optional RFC3339 timestamp when the subscription expires")
	dbURL := flag.String("db", defaultDB, "Postgres connection string (defaults to auth.database_url in config.yaml)")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("database connection string is required (set auth.database_url in config.yaml or pass -db)")
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

	userRepo, _, _, _ := authadapters.NewPostgresStores(pool)

	hashed, err := authcrypto.HashPassword(opts.password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	email := strings.TrimSpace(strings.ToLower(opts.email))
	if email == "" {
		return fmt.Errorf("email is required")
	}

	now := time.Now().UTC()

	desired := authdomain.User{
		ID:                    uuid.NewString(),
		Email:                 email,
		DisplayName:           opts.displayName,
		Status:                opts.status,
		PasswordHash:          hashed,
		PointsBalance:         opts.points,
		SubscriptionTier:      opts.tier,
		SubscriptionExpiresAt: opts.expiresAt,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	existing, err := userRepo.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, authdomain.ErrUserNotFound) {
		return fmt.Errorf("lookup user: %w", err)
	}

	if errors.Is(err, authdomain.ErrUserNotFound) {
		if _, err := userRepo.Create(ctx, desired); err != nil {
			if errors.Is(err, authdomain.ErrUserExists) {
				// Race: someone created the user after our lookup - retry as update.
				existing, err := userRepo.FindByEmail(ctx, email)
				if err != nil {
					return fmt.Errorf("reload user after conflict: %w", err)
				}
				desired.ID = existing.ID
				desired.CreatedAt = existing.CreatedAt
				desired.UpdatedAt = now
				if _, err := userRepo.Update(ctx, desired); err != nil {
					return fmt.Errorf("update user after conflict: %w", err)
				}
				return nil
			}
			return fmt.Errorf("create user: %w", err)
		}
		return nil
	}

	desired.ID = existing.ID
	desired.CreatedAt = existing.CreatedAt
	desired.UpdatedAt = now
	if _, err := userRepo.Update(ctx, desired); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}
