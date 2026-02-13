package bootstrap

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.scanFn == nil {
		return nil
	}
	return r.scanFn(dest...)
}

type fakeConn struct {
	mu           sync.Mutex
	tryLockOK    bool
	tryLockErr   error
	unlockOK     bool
	unlockErr    error
	releaseCalls int
	tryLockCalls int
	unlockCalls  int
}

func (c *fakeConn) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "pg_try_advisory_lock"):
		return &fakeRow{scanFn: func(dest ...any) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.tryLockCalls++
			if c.tryLockErr != nil {
				return c.tryLockErr
			}
			*(dest[0].(*bool)) = c.tryLockOK
			return nil
		}}
	case strings.Contains(sql, "pg_advisory_unlock"):
		return &fakeRow{scanFn: func(dest ...any) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.unlockCalls++
			if c.unlockErr != nil {
				return c.unlockErr
			}
			*(dest[0].(*bool)) = c.unlockOK
			return nil
		}}
	default:
		return &fakeRow{scanFn: func(_ ...any) error {
			return errors.New("unexpected query")
		}}
	}
}

func (c *fakeConn) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (c *fakeConn) Release() {
	c.mu.Lock()
	c.releaseCalls++
	c.mu.Unlock()
}

func TestSchedulerLeaderLockAcquireAndRelease(t *testing.T) {
	conn := &fakeConn{tryLockOK: true, unlockOK: true}
	lock := newPostgresAdvisoryLockWithAcquire(
		func(context.Context) (advisoryConn, error) { return conn, nil },
		"scheduler-main",
		"owner-a",
		time.Millisecond,
		nil,
	)

	acquired, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquired=true")
	}
	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release: %v", err)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.tryLockCalls != 1 {
		t.Fatalf("expected one pg_try_advisory_lock call, got %d", conn.tryLockCalls)
	}
	if conn.unlockCalls != 1 {
		t.Fatalf("expected one pg_advisory_unlock call, got %d", conn.unlockCalls)
	}
	if conn.releaseCalls != 1 {
		t.Fatalf("expected one connection release, got %d", conn.releaseCalls)
	}
}

func TestSchedulerLeaderLockAcquireRetriesUntilSuccess(t *testing.T) {
	first := &fakeConn{tryLockOK: false, unlockOK: true}
	second := &fakeConn{tryLockOK: true, unlockOK: true}
	call := 0
	lock := newPostgresAdvisoryLockWithAcquire(
		func(context.Context) (advisoryConn, error) {
			call++
			if call == 1 {
				return first, nil
			}
			return second, nil
		},
		"scheduler-main",
		"owner-a",
		2*time.Millisecond,
		nil,
	)

	acquired, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !acquired {
		t.Fatal("expected acquired=true")
	}
	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release: %v", err)
	}

	first.mu.Lock()
	firstReleaseCalls := first.releaseCalls
	first.mu.Unlock()
	if firstReleaseCalls != 1 {
		t.Fatalf("expected first unsuccessful connection released once, got %d", firstReleaseCalls)
	}
	if call < 2 {
		t.Fatalf("expected retry acquire calls >=2, got %d", call)
	}
}

func TestSchedulerLeaderLockAcquireContextDone(t *testing.T) {
	conn := &fakeConn{tryLockOK: false, unlockOK: true}
	lock := newPostgresAdvisoryLockWithAcquire(
		func(context.Context) (advisoryConn, error) { return conn, nil },
		"scheduler-main",
		"owner-a",
		30*time.Millisecond,
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	acquired, err := lock.Acquire(ctx)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
	if acquired {
		t.Fatal("expected acquired=false on context timeout")
	}
}

func TestSchedulerLeaderLockReleaseWithoutAcquireIsNoop(t *testing.T) {
	lock := newPostgresAdvisoryLockWithAcquire(
		func(context.Context) (advisoryConn, error) {
			return nil, errors.New("should not be called")
		},
		"",
		"",
		time.Millisecond,
		nil,
	)
	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release should be noop: %v", err)
	}
}
