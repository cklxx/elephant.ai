package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type sessionCleanupOptions struct {
	olderThan  time.Duration
	keepLatest int
	dryRun     bool
}

func defaultSessionCleanupOptions() sessionCleanupOptions {
	return sessionCleanupOptions{
		olderThan:  30 * 24 * time.Hour,
		keepLatest: 0,
		dryRun:     false,
	}
}

func (c *CLI) cleanupSessions(ctx context.Context, args []string) error {
	opts, err := parseSessionCleanupArgs(args)
	if err != nil {
		return err
	}

	sessionIDs, err := c.listAllSessions(ctx)
	if err != nil {
		return err
	}
	if len(sessionIDs) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	metas := make([]sessionMetadata, 0, len(sessionIDs))
	for _, sid := range sessionIDs {
		session, err := c.container.SessionStore.Get(ctx, sid)
		if err != nil {
			fmt.Printf("Skipping %s: %v\n", sid, err)
			continue
		}
		metas = append(metas, sessionMetadata{
			ID:        sid,
			UpdatedAt: session.UpdatedAt,
		})
	}
	if len(metas) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	targets := selectSessionsForCleanup(metas, opts, time.Now())
	if len(targets) == 0 {
		fmt.Println("No sessions matched cleanup criteria.")
		return nil
	}

	fmt.Printf(
		"Matched %d session(s) for cleanup (older than %s, keeping latest %d).%s\n",
		len(targets),
		formatCleanupDuration(opts.olderThan),
		opts.keepLatest,
		boolToSuffix(opts.dryRun, " Dry run enabled."),
	)

	if opts.dryRun {
		for _, target := range targets {
			fmt.Printf("  - %s (updated %s)\n", target.ID, target.UpdatedAt.Format(time.RFC3339))
		}
		return nil
	}

	deleted := 0
	for _, target := range targets {
		if err := c.container.SessionStore.Delete(ctx, target.ID); err != nil {
			fmt.Printf("  ✗ %s (error: %v)\n", target.ID, err)
			continue
		}
		fmt.Printf("  ✓ %s\n", target.ID)
		deleted++
	}
	fmt.Printf("Deleted %d session(s).\n", deleted)
	return nil
}

func parseSessionCleanupArgs(args []string) (sessionCleanupOptions, error) {
	opts := defaultSessionCleanupOptions()
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--older-than":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--older-than requires a value (e.g. 30d, 720h)")
			}
			i++
			d, err := parseRetentionDuration(args[i])
			if err != nil {
				return opts, fmt.Errorf("parse --older-than: %w", err)
			}
			opts.olderThan = d
		case "--keep-latest":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--keep-latest requires a value")
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 0 {
				return opts, fmt.Errorf("--keep-latest expects a non-negative integer")
			}
			opts.keepLatest = value
		case "--dry-run":
			opts.dryRun = true
		default:
			return opts, fmt.Errorf("unknown cleanup option: %s", args[i])
		}
	}
	return opts, nil
}

func parseRetentionDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, fmt.Errorf("retention duration cannot be empty")
	}
	if strings.HasSuffix(value, "d") {
		num, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(num) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(value, "w") {
		num, err := strconv.Atoi(strings.TrimSuffix(value, "w"))
		if err != nil {
			return 0, err
		}
		return time.Duration(num*7) * 24 * time.Hour, nil
	}
	if strings.ContainsAny(value, "hms") {
		return time.ParseDuration(value)
	}
	if num, err := strconv.Atoi(value); err == nil {
		return time.Duration(num) * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

type sessionMetadata struct {
	ID        string
	UpdatedAt time.Time
}

func selectSessionsForCleanup(all []sessionMetadata, opts sessionCleanupOptions, now time.Time) []sessionMetadata {
	if len(all) == 0 {
		return nil
	}

	metas := make([]sessionMetadata, len(all))
	copy(metas, all)
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})

	startIdx := opts.keepLatest
	if startIdx >= len(metas) {
		return nil
	}

	candidates := metas[startIdx:]
	if opts.olderThan <= 0 {
		return candidates
	}

	cutoff := now.Add(-opts.olderThan)
	result := make([]sessionMetadata, 0, len(candidates))
	for _, meta := range candidates {
		if meta.UpdatedAt.Before(cutoff) {
			result = append(result, meta)
		}
	}
	return result
}

func formatCleanupDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return d.String()
}

func boolToSuffix(condition bool, suffix string) string {
	if condition {
		return suffix
	}
	return ""
}
