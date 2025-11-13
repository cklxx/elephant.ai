package orchestrator

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics exposes Prometheus collectors that report orchestrator activity.
type Metrics struct {
	stageDuration *prometheus.HistogramVec
	stageFailures *prometheus.CounterVec
	stageRetries  *prometheus.CounterVec
	jobsActive    prometheus.Gauge
}

var (
	defaultMetricsOnce sync.Once
	sharedMetrics      *Metrics
)

// defaultMetrics returns the package-level metrics instance registered with the
// global Prometheus registry. The collectors are created only once to avoid
// duplicate registration panics when the orchestrator is instantiated multiple
// times (e.g. in unit tests or multi-tenant runners).
func defaultMetrics() *Metrics {
	defaultMetricsOnce.Do(func() {
		sharedMetrics = MustNewMetrics(prometheus.DefaultRegisterer)
	})
	return sharedMetrics
}

// MustNewMetrics constructs a Metrics instance using the provided registerer.
// The caller is responsible for supplying a fresh registry when unique metric
// names are required (for example in tests). Any registration error will panic
// which mirrors the semantics of promauto helpers and surfaces configuration
// bugs early.
func MustNewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	stageDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "local_av",
			Subsystem: "orchestrator",
			Name:      "job_stage_duration_seconds",
			Help:      "Duration spent in each orchestrator stage.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"stage", "status"},
	)
	stageFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "local_av",
			Subsystem: "orchestrator",
			Name:      "job_stage_failures_total",
			Help:      "Total number of stage executions that failed irrecoverably.",
		},
		[]string{"stage", "reason"},
	)
	stageRetries := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "local_av",
			Subsystem: "orchestrator",
			Name:      "job_stage_retries_total",
			Help:      "Number of times a stage execution required a retry.",
		},
		[]string{"stage"},
	)
	jobsActive := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "local_av",
			Subsystem: "orchestrator",
			Name:      "jobs_active",
			Help:      "Number of jobs currently being executed by the orchestrator.",
		},
	)

	collectors := []prometheus.Collector{stageDuration, stageFailures, stageRetries, jobsActive}
	for _, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
				// Reuse the existing collector when it matches the expected type.
				switch target := collector.(type) {
				case *prometheus.HistogramVec:
					stageDuration = already.ExistingCollector.(*prometheus.HistogramVec)
				case *prometheus.CounterVec:
					switch target { //nolint:exhaustive
					case stageFailures:
						stageFailures = already.ExistingCollector.(*prometheus.CounterVec)
					case stageRetries:
						stageRetries = already.ExistingCollector.(*prometheus.CounterVec)
					}
				case prometheus.Gauge:
					jobsActive = already.ExistingCollector.(prometheus.Gauge)
				}
				continue
			}
			panic(err)
		}
	}

	return &Metrics{
		stageDuration: stageDuration,
		stageFailures: stageFailures,
		stageRetries:  stageRetries,
		jobsActive:    jobsActive,
	}
}

// ObserveStageDuration records the time spent in a stage with the provided status label.
func (m *Metrics) ObserveStageDuration(stage string, status string, duration time.Duration) {
	if m == nil || m.stageDuration == nil {
		return
	}
	m.stageDuration.WithLabelValues(stage, status).Observe(duration.Seconds())
}

// IncStageFailure increments the failure counter for the given stage and reason.
func (m *Metrics) IncStageFailure(stage string, reason string) {
	if m == nil || m.stageFailures == nil {
		return
	}
	m.stageFailures.WithLabelValues(stage, reason).Inc()
}

// IncStageRetry increments the retry counter for the given stage.
func (m *Metrics) IncStageRetry(stage string) {
	if m == nil || m.stageRetries == nil {
		return
	}
	m.stageRetries.WithLabelValues(stage).Inc()
}

// IncActiveJobs marks a job as active.
func (m *Metrics) IncActiveJobs() {
	if m == nil || m.jobsActive == nil {
		return
	}
	m.jobsActive.Inc()
}

// DecActiveJobs marks a job as completed or cancelled.
func (m *Metrics) DecActiveJobs() {
	if m == nil || m.jobsActive == nil {
		return
	}
	m.jobsActive.Dec()
}
