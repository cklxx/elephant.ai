package gate

import (
	"sync"
	"time"
)

const defaultEvaluatorWindow = 200

// DirectiveMode captures the coarse action bundle executed for a request.
type DirectiveMode string

const (
	ModeSkip           DirectiveMode = "skip"
	ModeRetrieve       DirectiveMode = "retrieve"
	ModeRetrieveSearch DirectiveMode = "retrieve+search"
	ModeFullLoop       DirectiveMode = "retrieve+search+crawl"
)

// Outcome captures the observed result of executing RAG directives. The values are
// intentionally lightweight so they can be emitted from multiple runtimes and
// aggregated offline for calibration.
type Outcome struct {
	Mode              DirectiveMode
	Satisfied         bool
	FreshnessImproved bool
	RetrievedChunks   int
	ExternalCalls     int
	CostUSD           float64
	Latency           time.Duration
}

// ModeSummary aggregates metrics for a specific DirectiveMode.
type ModeSummary struct {
	Count                    int
	SatisfactionRate         float64
	FreshnessImprovementRate float64
	AverageRetrievedChunks   float64
	AverageExternalCalls     float64
	AverageCostUSD           float64
	AverageLatency           time.Duration
}

// Summary describes the rolling performance of the RAG system.
type Summary struct {
	TotalOutcomes            int
	RollingWindow            int
	OverallSatisfaction      float64
	OverallFreshnessGainRate float64
	AverageRetrievedChunks   float64
	AverageExternalCalls     float64
	AverageCostUSD           float64
	AverageLatency           time.Duration
	Modes                    map[DirectiveMode]ModeSummary
}

// Evaluator keeps a rolling window of Outcomes and exposes aggregated metrics
// that can be used to tune gate thresholds and budget policies.
type Evaluator struct {
	mu       sync.Mutex
	window   int
	outcomes []Outcome
	next     int
	count    int
}

// NewEvaluator creates an Evaluator that stores at most window outcomes. When
// window is not positive a safe default is applied.
func NewEvaluator(window int) *Evaluator {
	if window <= 0 {
		window = defaultEvaluatorWindow
	}
	return &Evaluator{
		window:   window,
		outcomes: make([]Outcome, window),
	}
}

// RecordOutcome registers an observed Outcome. Once the evaluator reaches its
// configured window size older entries are overwritten in FIFO order.
func (e *Evaluator) RecordOutcome(outcome Outcome) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.outcomes) == 0 {
		return
	}
	e.outcomes[e.next] = outcome
	e.next = (e.next + 1) % e.window
	if e.count < e.window {
		e.count++
	}
}

// Reset clears all stored outcomes while preserving the configured window.
func (e *Evaluator) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.next = 0
	e.count = 0
	for i := range e.outcomes {
		e.outcomes[i] = Outcome{}
	}
}

// Snapshot computes a Summary of the currently stored outcomes.
func (e *Evaluator) Snapshot() Summary {
	e.mu.Lock()
	defer e.mu.Unlock()

	summary := Summary{
		RollingWindow: e.window,
		Modes:         make(map[DirectiveMode]ModeSummary),
	}
	if e.count == 0 {
		return summary
	}

	totalSatisfaction := 0.0
	totalFreshness := 0.0
	totalChunks := 0.0
	totalExternal := 0.0
	totalCost := 0.0
	var totalLatency time.Duration

	modeTotals := make(map[DirectiveMode]int)
	modeSatisfaction := make(map[DirectiveMode]float64)
	modeFreshness := make(map[DirectiveMode]float64)
	modeChunks := make(map[DirectiveMode]float64)
	modeExternal := make(map[DirectiveMode]float64)
	modeCost := make(map[DirectiveMode]float64)
	modeLatency := make(map[DirectiveMode]time.Duration)

	start := 0
	if e.count == e.window {
		start = e.next
	}
	for i := 0; i < e.count; i++ {
		idx := (start + i) % e.window
		outcome := e.outcomes[idx]
		mode := outcome.Mode

		modeTotals[mode]++
		summary.TotalOutcomes++

		if outcome.Satisfied {
			modeSatisfaction[mode]++
			totalSatisfaction++
		}
		if outcome.FreshnessImproved {
			modeFreshness[mode]++
			totalFreshness++
		}
		modeChunks[mode] += float64(outcome.RetrievedChunks)
		totalChunks += float64(outcome.RetrievedChunks)

		modeExternal[mode] += float64(outcome.ExternalCalls)
		totalExternal += float64(outcome.ExternalCalls)

		modeCost[mode] += outcome.CostUSD
		totalCost += outcome.CostUSD

		modeLatency[mode] += outcome.Latency
		totalLatency += outcome.Latency
	}

	if summary.TotalOutcomes > 0 {
		summary.OverallSatisfaction = totalSatisfaction / float64(summary.TotalOutcomes)
		summary.OverallFreshnessGainRate = totalFreshness / float64(summary.TotalOutcomes)
		summary.AverageRetrievedChunks = totalChunks / float64(summary.TotalOutcomes)
		summary.AverageExternalCalls = totalExternal / float64(summary.TotalOutcomes)
		summary.AverageCostUSD = totalCost / float64(summary.TotalOutcomes)
		summary.AverageLatency = time.Duration(int64(totalLatency) / int64(summary.TotalOutcomes))
	}

	for mode, total := range modeTotals {
		modeSummary := ModeSummary{Count: total}
		if total > 0 {
			modeSummary.SatisfactionRate = modeSatisfaction[mode] / float64(total)
			modeSummary.FreshnessImprovementRate = modeFreshness[mode] / float64(total)
			modeSummary.AverageRetrievedChunks = modeChunks[mode] / float64(total)
			modeSummary.AverageExternalCalls = modeExternal[mode] / float64(total)
			modeSummary.AverageCostUSD = modeCost[mode] / float64(total)
			modeSummary.AverageLatency = time.Duration(int64(modeLatency[mode]) / int64(total))
		}
		summary.Modes[mode] = modeSummary
	}

	return summary
}

// Window returns the current window size.
func (e *Evaluator) Window() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.window
}

// ModeFromDirectives classifies directives into one of the canonical modes.
func ModeFromDirectives(d Decision) DirectiveMode {
	switch {
	case !d.UseRetrieval && !d.UseSearch && !d.UseCrawl:
		return ModeSkip
	case d.UseRetrieval && d.UseSearch && d.UseCrawl:
		return ModeFullLoop
	case d.UseRetrieval && d.UseSearch:
		return ModeRetrieveSearch
	case d.UseRetrieval:
		return ModeRetrieve
	default:
		return ModeSkip
	}
}
