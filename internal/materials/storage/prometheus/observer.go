package prometheus

import (
	"fmt"
	"time"

	"alex/internal/materials/storage"
	promclient "github.com/prometheus/client_golang/prometheus"
)

// PrometheusObserver exports mapper metrics to Prometheus.
type PrometheusObserver struct {
	uploadDuration  *promclient.HistogramVec
	operationErrors *promclient.CounterVec
	uploadBytes     promclient.Counter
}

// NewPrometheusObserver registers upload/delete/prewarm/refresh metrics.
func NewPrometheusObserver(namespace string, reg promclient.Registerer) (*PrometheusObserver, error) {
	if namespace == "" {
		namespace = "materials_storage"
	}
	if reg == nil {
		reg = promclient.DefaultRegisterer
	}
	observer := &PrometheusObserver{
		uploadDuration: promclient.NewHistogramVec(promclient.HistogramOpts{
			Namespace: namespace,
			Name:      "operation_duration_seconds",
			Help:      "Latency for storage mapper operations.",
			Buckets:   promclient.DefBuckets,
		}, []string{"operation"}),
		operationErrors: promclient.NewCounterVec(promclient.CounterOpts{
			Namespace: namespace,
			Name:      "operation_errors_total",
			Help:      "Count of storage mapper failures.",
		}, []string{"operation"}),
		uploadBytes: promclient.NewCounter(promclient.CounterOpts{
			Namespace: namespace,
			Name:      "uploaded_bytes_total",
			Help:      "Cumulative payload size successfully uploaded to object storage.",
		}),
	}
	if err := reg.Register(observer.uploadDuration); err != nil {
		if are, ok := err.(promclient.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*promclient.HistogramVec); ok {
				observer.uploadDuration = existing
			} else {
				return nil, fmt.Errorf("register storage histogram: %w", err)
			}
		} else {
			return nil, fmt.Errorf("register storage histogram: %w", err)
		}
	}
	if err := reg.Register(observer.operationErrors); err != nil {
		if are, ok := err.(promclient.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(*promclient.CounterVec); ok {
				observer.operationErrors = existing
			} else {
				return nil, fmt.Errorf("register storage counter: %w", err)
			}
		} else {
			return nil, fmt.Errorf("register storage counter: %w", err)
		}
	}
	if err := reg.Register(observer.uploadBytes); err != nil {
		if are, ok := err.(promclient.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(promclient.Counter); ok {
				observer.uploadBytes = existing
			} else {
				return nil, fmt.Errorf("register uploaded bytes counter: %w", err)
			}
		} else {
			return nil, fmt.Errorf("register uploaded bytes counter: %w", err)
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

var _ storage.Observer = (*PrometheusObserver)(nil)
