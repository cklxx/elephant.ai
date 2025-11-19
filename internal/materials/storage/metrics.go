package storage

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Observer captures telemetry for mapper operations.
type Observer interface {
	RecordUpload(duration time.Duration, sizeBytes uint64, err error)
	RecordDelete(duration time.Duration, err error)
	RecordPrewarm(duration time.Duration, err error)
	RecordRefresh(duration time.Duration, err error)
}

// PrometheusObserver exports mapper metrics to Prometheus.
type PrometheusObserver struct {
	uploadDuration  *prometheus.HistogramVec
	operationErrors *prometheus.CounterVec
	uploadBytes     prometheus.Counter
}

// NewPrometheusObserver registers upload/delete/prewarm/refresh metrics.
func NewPrometheusObserver(namespace string, reg prometheus.Registerer) (*PrometheusObserver, error) {
	if namespace == "" {
		namespace = "materials_storage"
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	observer := &PrometheusObserver{
		uploadDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "operation_duration_seconds",
			Help:      "Latency for storage mapper operations.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"operation"}),
		operationErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "operation_errors_total",
			Help:      "Count of storage mapper failures.",
		}, []string{"operation"}),
		uploadBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "uploaded_bytes_total",
			Help:      "Cumulative payload size successfully uploaded to object storage.",
		}),
	}
	collectors := []prometheus.Collector{observer.uploadDuration, observer.operationErrors, observer.uploadBytes}
	for _, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				collector = are.ExistingCollector
				continue
			}
			return nil, fmt.Errorf("register storage metric: %w", err)
		}
	}
	return observer, nil
}

// RecordUpload tracks upload duration, size, and failures.
func (o *PrometheusObserver) RecordUpload(duration time.Duration, sizeBytes uint64, err error) {
	if o == nil {
		return
	}
	o.uploadDuration.WithLabelValues("upload").Observe(duration.Seconds())
	if err != nil {
		o.operationErrors.WithLabelValues("upload").Inc()
		return
	}
	o.uploadBytes.Add(float64(sizeBytes))
}

func (o *PrometheusObserver) RecordDelete(duration time.Duration, err error) {
	recordOperation(o, "delete", duration, err)
}

func (o *PrometheusObserver) RecordPrewarm(duration time.Duration, err error) {
	recordOperation(o, "prewarm", duration, err)
}

func (o *PrometheusObserver) RecordRefresh(duration time.Duration, err error) {
	recordOperation(o, "refresh", duration, err)
}

func recordOperation(o *PrometheusObserver, op string, duration time.Duration, err error) {
	if o == nil {
		return
	}
	o.uploadDuration.WithLabelValues(op).Observe(duration.Seconds())
	if err != nil {
		o.operationErrors.WithLabelValues(op).Inc()
	}
}

type nopObserver struct{}

func (nopObserver) RecordUpload(time.Duration, uint64, error) {}

func (nopObserver) RecordDelete(time.Duration, error) {}

func (nopObserver) RecordPrewarm(time.Duration, error) {}

func (nopObserver) RecordRefresh(time.Duration, error) {}
