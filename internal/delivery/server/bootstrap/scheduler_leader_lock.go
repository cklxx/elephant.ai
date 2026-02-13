package bootstrap

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"alex/internal/app/scheduler"
	"alex/internal/shared/logging"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultSchedulerLockName        = "proactive_scheduler"
	defaultSchedulerLockAcquireWait = 15 * time.Second
)

type advisoryConn interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Release()
}

type acquireConnFn func(ctx context.Context) (advisoryConn, error)

type poolConnAdapter struct {
	conn *pgxpool.Conn
}

func (a *poolConnAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.conn.QueryRow(ctx, sql, args...)
}

func (a *poolConnAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.conn.Exec(ctx, sql, args...)
}

func (a *poolConnAdapter) Release() {
	a.conn.Release()
}

type postgresAdvisoryLock struct {
	lockName        string
	lockKey         int64
	ownerID         string
	acquireInterval time.Duration
	logger          logging.Logger
	acquireConn     acquireConnFn

	mu   sync.Mutex
	conn advisoryConn
}

func newPostgresAdvisoryLock(
	pool *pgxpool.Pool,
	lockName string,
	ownerID string,
	acquireInterval time.Duration,
	logger logging.Logger,
) scheduler.LeaderLock {
	acquire := func(ctx context.Context) (advisoryConn, error) {
		if pool == nil {
			return nil, fmt.Errorf("scheduler lock: postgres pool is nil")
		}
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return nil, err
		}
		return &poolConnAdapter{conn: conn}, nil
	}
	return newPostgresAdvisoryLockWithAcquire(acquire, lockName, ownerID, acquireInterval, logger)
}

func newPostgresAdvisoryLockWithAcquire(
	acquire acquireConnFn,
	lockName string,
	ownerID string,
	acquireInterval time.Duration,
	logger logging.Logger,
) *postgresAdvisoryLock {
	name := strings.TrimSpace(lockName)
	if name == "" {
		name = defaultSchedulerLockName
	}
	if acquireInterval <= 0 {
		acquireInterval = defaultSchedulerLockAcquireWait
	}
	return &postgresAdvisoryLock{
		lockName:        name,
		lockKey:         schedulerLockKey(name),
		ownerID:         strings.TrimSpace(ownerID),
		acquireInterval: acquireInterval,
		logger:          logging.OrNop(logger),
		acquireConn:     acquire,
	}
}

func (l *postgresAdvisoryLock) Name() string {
	if l == nil {
		return defaultSchedulerLockName
	}
	return l.lockName
}

func (l *postgresAdvisoryLock) Acquire(ctx context.Context) (bool, error) {
	if l == nil {
		return false, fmt.Errorf("scheduler lock is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	l.mu.Lock()
	if l.conn != nil {
		l.mu.Unlock()
		return true, nil
	}
	l.mu.Unlock()

	for {
		conn, err := l.acquireConn(ctx)
		if err != nil {
			return false, fmt.Errorf("acquire scheduler lock connection: %w", err)
		}
		locked, err := l.tryLock(ctx, conn)
		if err != nil {
			conn.Release()
			return false, err
		}
		if locked {
			l.mu.Lock()
			if l.conn != nil {
				l.mu.Unlock()
				_ = unlockConn(context.Background(), conn, l.lockKey)
				conn.Release()
				return true, nil
			}
			l.conn = conn
			l.mu.Unlock()
			l.logger.Info("Scheduler leader lock acquired: lock=%s owner=%s", l.lockName, l.ownerID)
			return true, nil
		}
		conn.Release()

		timer := time.NewTimer(l.acquireInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false, ctx.Err()
		case <-timer.C:
		}
	}
}

func (l *postgresAdvisoryLock) Release(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	l.mu.Lock()
	conn := l.conn
	l.conn = nil
	l.mu.Unlock()
	if conn == nil {
		return nil
	}
	defer conn.Release()

	if err := unlockConn(ctx, conn, l.lockKey); err != nil {
		return err
	}
	l.logger.Info("Scheduler leader lock released: lock=%s owner=%s", l.lockName, l.ownerID)
	return nil
}

func (l *postgresAdvisoryLock) tryLock(ctx context.Context, conn advisoryConn) (bool, error) {
	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, l.lockKey).Scan(&locked); err != nil {
		return false, fmt.Errorf("query pg_try_advisory_lock: %w", err)
	}
	return locked, nil
}

func unlockConn(ctx context.Context, conn advisoryConn, key int64) error {
	var unlocked bool
	if err := conn.QueryRow(ctx, `SELECT pg_advisory_unlock($1)`, key).Scan(&unlocked); err != nil {
		return fmt.Errorf("query pg_advisory_unlock: %w", err)
	}
	return nil
}

func schedulerLockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}
