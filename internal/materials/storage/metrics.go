package storage

import (
	"time"
)

// Observer captures telemetry for mapper operations.
type Observer interface {
	RecordUpload(duration time.Duration, sizeBytes uint64, err error)
	RecordDelete(duration time.Duration, err error)
	RecordPrewarm(duration time.Duration, err error)
	RecordRefresh(duration time.Duration, err error)
}

type nopObserver struct{}

func (nopObserver) RecordUpload(time.Duration, uint64, error) {}

func (nopObserver) RecordDelete(time.Duration, error) {}

func (nopObserver) RecordPrewarm(time.Duration, error) {}

func (nopObserver) RecordRefresh(time.Duration, error) {}
