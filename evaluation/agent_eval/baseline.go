package agent_eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
)

// BaselineSnapshot represents a stored baseline of aggregated evaluation metrics
// at a specific point in time, used as a reference for regression detection.
type BaselineSnapshot struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Metrics   BaselineMetrics `json:"metrics"`
	CreatedAt time.Time       `json:"created_at"`
	Tags      []string        `json:"tags"`
}

// BaselineMetrics holds aggregated evaluation metrics for a baseline snapshot.
type BaselineMetrics struct {
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
	AvgTokens    int     `json:"avg_tokens"`
	AvgCost      float64 `json:"avg_cost"`
	QualityScore float64 `json:"quality_score"`
	TaskCount    int     `json:"task_count"`
}

// RegressionAlert describes a detected metric regression compared to a baseline.
type RegressionAlert struct {
	Metric       string  `json:"metric"`
	Baseline     float64 `json:"baseline"`
	Current      float64 `json:"current"`
	DeltaPercent float64 `json:"delta_percent"`
	Severity     string  `json:"severity"`
	Message      string  `json:"message"`
}

// RegressionConfig controls how regressions are classified and which metrics
// are inspected.
type RegressionConfig struct {
	WarningThresholdPercent  float64  `json:"warning_threshold_percent"`
	CriticalThresholdPercent float64  `json:"critical_threshold_percent"`
	IgnoreMetrics            []string `json:"ignore_metrics"`
}

// DefaultRegressionConfig returns a sensible default configuration for
// regression detection. Warning fires at 10% degradation, critical at 25%.
func DefaultRegressionConfig() RegressionConfig {
	return RegressionConfig{
		WarningThresholdPercent:  10,
		CriticalThresholdPercent: 25,
	}
}

// BaselineStore abstracts persistence of baseline snapshots.
type BaselineStore interface {
	SaveBaseline(ctx context.Context, snapshot BaselineSnapshot) error
	GetBaseline(ctx context.Context, id string) (BaselineSnapshot, error)
	LatestBaseline(ctx context.Context) (BaselineSnapshot, error)
	ListBaselines(ctx context.Context, limit int) ([]BaselineSnapshot, error)
	DeleteBaseline(ctx context.Context, id string) error
}

// FileBaselineStore implements BaselineStore using the local filesystem.
// Each baseline is stored as a JSON file under {basePath}/baselines/{id}.json.
// The most recent baseline is also written to latest.json as a copy.
type FileBaselineStore struct {
	basePath string
	mu       sync.Mutex
}

// NewFileBaselineStore creates a FileBaselineStore rooted at basePath.
func NewFileBaselineStore(basePath string) *FileBaselineStore {
	return &FileBaselineStore{basePath: basePath}
}

func (s *FileBaselineStore) dir() string {
	return filepath.Join(s.basePath, "baselines")
}

func (s *FileBaselineStore) snapshotPath(id string) string {
	return filepath.Join(s.dir(), id+".json")
}

func (s *FileBaselineStore) latestPath() string {
	return filepath.Join(s.dir(), "latest.json")
}

// SaveBaseline persists a snapshot to disk and updates the latest.json copy.
func (s *FileBaselineStore) SaveBaseline(_ context.Context, snapshot BaselineSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snapshot.ID == "" {
		snapshot.ID = ksuid.New().String()
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now()
	}

	if err := os.MkdirAll(s.dir(), 0o755); err != nil {
		return fmt.Errorf("create baselines dir: %w", err)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}

	if err := os.WriteFile(s.snapshotPath(snapshot.ID), data, 0o644); err != nil {
		return fmt.Errorf("write baseline %s: %w", snapshot.ID, err)
	}

	// Write a copy as latest.json (no symlink for portability).
	if err := os.WriteFile(s.latestPath(), data, 0o644); err != nil {
		return fmt.Errorf("write latest baseline: %w", err)
	}

	return nil
}

// GetBaseline retrieves a baseline by ID. Returns an error if the baseline
// does not exist.
func (s *FileBaselineStore) GetBaseline(_ context.Context, id string) (BaselineSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadSnapshot(s.snapshotPath(id))
}

// LatestBaseline returns the most recently saved baseline snapshot.
func (s *FileBaselineStore) LatestBaseline(_ context.Context) (BaselineSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadSnapshot(s.latestPath())
}

// ListBaselines returns stored baselines sorted by CreatedAt descending.
// When limit is positive, at most limit entries are returned.
func (s *FileBaselineStore) ListBaselines(_ context.Context, limit int) ([]BaselineSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read baselines dir: %w", err)
	}

	var snapshots []BaselineSnapshot
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "latest.json" {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		snap, err := s.loadSnapshot(filepath.Join(s.dir(), entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("load baseline %s: %w", entry.Name(), err)
		}
		snapshots = append(snapshots, snap)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	if limit > 0 && len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}

	return snapshots, nil
}

// DeleteBaseline removes a baseline from disk. If the deleted baseline was
// the latest, latest.json is updated to the next most recent baseline.
func (s *FileBaselineStore) DeleteBaseline(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.snapshotPath(id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("baseline %s not found", id)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete baseline %s: %w", id, err)
	}

	// Refresh latest.json: check if the deleted baseline was the latest one.
	latestSnap, err := s.loadSnapshot(s.latestPath())
	if err == nil && latestSnap.ID == id {
		// Find the next most recent baseline.
		entries, err := os.ReadDir(s.dir())
		if err != nil {
			// Directory may be empty or gone; remove latest.
			_ = os.Remove(s.latestPath())
			return nil
		}

		var newest *BaselineSnapshot
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "latest.json" {
				continue
			}
			if filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			snap, err := s.loadSnapshot(filepath.Join(s.dir(), entry.Name()))
			if err != nil {
				continue
			}
			if newest == nil || snap.CreatedAt.After(newest.CreatedAt) {
				cp := snap
				newest = &cp
			}
		}

		if newest != nil {
			data, _ := json.MarshalIndent(newest, "", "  ")
			_ = os.WriteFile(s.latestPath(), data, 0o644)
		} else {
			_ = os.Remove(s.latestPath())
		}
	}

	return nil
}

func (s *FileBaselineStore) loadSnapshot(path string) (BaselineSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BaselineSnapshot{}, fmt.Errorf("read baseline file: %w", err)
	}
	var snap BaselineSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return BaselineSnapshot{}, fmt.Errorf("decode baseline: %w", err)
	}
	return snap, nil
}

// metricComparison defines how a single metric pair is compared.
type metricComparison struct {
	name         string
	baseline     float64
	current      float64
	higherBetter bool
}

// DetectRegressions compares baseline and current metrics, returning alerts
// for any metric that has degraded beyond the configured thresholds.
//
// For "higher is better" metrics (SuccessRate, QualityScore):
//
//	regression = current < baseline
//
// For "lower is better" metrics (AvgLatencyMs, P95LatencyMs, AvgTokens, AvgCost):
//
//	regression = current > baseline
func DetectRegressions(baseline BaselineMetrics, current BaselineMetrics, cfg RegressionConfig) []RegressionAlert {
	ignored := make(map[string]struct{}, len(cfg.IgnoreMetrics))
	for _, m := range cfg.IgnoreMetrics {
		ignored[strings.ToLower(m)] = struct{}{}
	}

	comparisons := []metricComparison{
		{name: "SuccessRate", baseline: baseline.SuccessRate, current: current.SuccessRate, higherBetter: true},
		{name: "AvgLatencyMs", baseline: baseline.AvgLatencyMs, current: current.AvgLatencyMs, higherBetter: false},
		{name: "P95LatencyMs", baseline: baseline.P95LatencyMs, current: current.P95LatencyMs, higherBetter: false},
		{name: "AvgTokens", baseline: float64(baseline.AvgTokens), current: float64(current.AvgTokens), higherBetter: false},
		{name: "AvgCost", baseline: baseline.AvgCost, current: current.AvgCost, higherBetter: false},
		{name: "QualityScore", baseline: baseline.QualityScore, current: current.QualityScore, higherBetter: true},
	}

	var alerts []RegressionAlert
	for _, c := range comparisons {
		if _, skip := ignored[strings.ToLower(c.name)]; skip {
			continue
		}

		deltaPct := calcDeltaPercent(c.baseline, c.current, c.higherBetter)
		if deltaPct <= 0 {
			continue // No regression.
		}

		severity := classifySeverity(deltaPct, cfg)
		if severity == "" {
			continue // Below warning threshold.
		}

		alerts = append(alerts, RegressionAlert{
			Metric:       c.name,
			Baseline:     c.baseline,
			Current:      c.current,
			DeltaPercent: deltaPct,
			Severity:     severity,
			Message:      formatAlertMessage(c.name, c.baseline, c.current, deltaPct, severity),
		})
	}

	return alerts
}

// calcDeltaPercent returns the percentage of degradation. A positive return
// value means the metric regressed; zero or negative means it held or improved.
func calcDeltaPercent(baseline, current float64, higherBetter bool) float64 {
	if baseline == 0 {
		if higherBetter && current < baseline {
			return 100
		}
		if !higherBetter && current > baseline {
			return 100
		}
		return 0
	}

	var pct float64
	if higherBetter {
		// Regression when current < baseline.
		pct = (baseline - current) / math.Abs(baseline) * 100
	} else {
		// Regression when current > baseline.
		pct = (current - baseline) / math.Abs(baseline) * 100
	}

	if pct <= 0 {
		return 0
	}
	return math.Round(pct*100) / 100 // Two decimal places.
}

func classifySeverity(deltaPct float64, cfg RegressionConfig) string {
	if deltaPct >= cfg.CriticalThresholdPercent {
		return "critical"
	}
	if deltaPct >= cfg.WarningThresholdPercent {
		return "warning"
	}
	return ""
}

func formatAlertMessage(name string, baseline, current, deltaPct float64, severity string) string {
	return fmt.Sprintf("[%s] %s regressed by %.2f%% (baseline: %.4g, current: %.4g)",
		strings.ToUpper(severity), name, deltaPct, baseline, current)
}

// FormatRegressionReport produces a Markdown-formatted report of regression alerts.
func FormatRegressionReport(alerts []RegressionAlert) string {
	if len(alerts) == 0 {
		return "# Regression Report\n\nNo regressions detected.\n"
	}

	var b strings.Builder
	b.WriteString("# Regression Report\n\n")
	b.WriteString(fmt.Sprintf("**%d regression(s) detected.**\n\n", len(alerts)))

	b.WriteString("| Metric | Baseline | Current | Delta (%) | Severity |\n")
	b.WriteString("|--------|----------|---------|-----------|----------|\n")

	for _, a := range alerts {
		b.WriteString(fmt.Sprintf("| %s | %.4g | %.4g | %.2f%% | %s |\n",
			a.Metric, a.Baseline, a.Current, a.DeltaPercent, a.Severity))
	}

	b.WriteString("\n### Details\n\n")
	for i, a := range alerts {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, a.Message))
	}

	return b.String()
}

// BuildBaselineMetrics converts evaluation metrics into a baseline snapshot summary.
func BuildBaselineMetrics(metrics *EvaluationMetrics) (BaselineMetrics, error) {
	if metrics == nil {
		return BaselineMetrics{}, fmt.Errorf("metrics are required")
	}

	return BaselineMetrics{
		SuccessRate:  metrics.Performance.SuccessRate,
		AvgLatencyMs: float64(metrics.Performance.AvgExecutionTime.Milliseconds()),
		P95LatencyMs: float64(metrics.Performance.P95Time.Milliseconds()),
		AvgTokens:    metrics.Resources.AvgTokensUsed,
		AvgCost:      metrics.Resources.AvgCostPerTask,
		QualityScore: metrics.Quality.SolutionQuality,
		TaskCount:    metrics.TotalTasks,
	}, nil
}

// BuildBaselineSnapshot builds a baseline snapshot from evaluation results.
func BuildBaselineSnapshot(results *EvaluationResults, name, version string, tags []string) (BaselineSnapshot, error) {
	if results == nil {
		return BaselineSnapshot{}, fmt.Errorf("evaluation results are required")
	}
	metrics, err := BuildBaselineMetrics(results.Metrics)
	if err != nil {
		return BaselineSnapshot{}, err
	}

	return BaselineSnapshot{
		Name:      name,
		Version:   version,
		Metrics:   metrics,
		CreatedAt: time.Now(),
		Tags:      append([]string(nil), tags...),
	}, nil
}
