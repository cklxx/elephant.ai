package testutil

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	runtimeconfig "alex/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDatabaseEnv = "ALEX_TEST_DATABASE_URL"

func NewPostgresTestPool(t *testing.T) (*pgxpool.Pool, string, func()) {
	t.Helper()

	raw, ok := runtimeconfig.DefaultEnvLookup(testDatabaseEnv)
	if !ok {
		t.Skipf("%s not set", testDatabaseEnv)
	}
	dbURL := strings.TrimSpace(raw)
	if dbURL == "" {
		t.Skipf("%s not set", testDatabaseEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	if err := adminPool.Ping(ctx); err != nil {
		adminPool.Close()
		t.Fatalf("ping postgres: %v", err)
	}

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
		adminPool.Close()
		t.Fatalf("create schema: %v", err)
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse postgres config: %v", err)
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = make(map[string]string)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	schemaURL := config.ConnConfig.ConnString()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		adminPool.Close()
		t.Fatalf("create test postgres pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		adminPool.Close()
		t.Fatalf("ping test postgres: %v", err)
	}

	cleanup := func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
		adminPool.Close()
	}

	return pool, schemaURL, cleanup
}
