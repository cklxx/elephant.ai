package storage

import (
	"context"
	"time"
)

// ObservedMapper wraps another mapper and forwards telemetry to an observer.
type ObservedMapper struct {
	delegate Mapper
	observer Observer
}

// NewObservedMapper instruments delegate with the provided observer.
func NewObservedMapper(delegate Mapper, observer Observer) *ObservedMapper {
	if observer == nil {
		observer = nopObserver{}
	}
	return &ObservedMapper{delegate: delegate, observer: observer}
}

func (m *ObservedMapper) Upload(ctx context.Context, req UploadRequest) (UploadResult, error) {
	start := time.Now()
	res, err := m.delegate.Upload(ctx, req)
	m.observer.RecordUpload(time.Since(start), res.SizeBytes, err)
	return res, err
}

func (m *ObservedMapper) Delete(ctx context.Context, storageKey string) error {
	start := time.Now()
	err := m.delegate.Delete(ctx, storageKey)
	m.observer.RecordDelete(time.Since(start), err)
	return err
}

func (m *ObservedMapper) Prewarm(ctx context.Context, storageKey string) error {
	start := time.Now()
	err := m.delegate.Prewarm(ctx, storageKey)
	m.observer.RecordPrewarm(time.Since(start), err)
	return err
}

func (m *ObservedMapper) Refresh(ctx context.Context, storageKey string) error {
	start := time.Now()
	err := m.delegate.Refresh(ctx, storageKey)
	m.observer.RecordRefresh(time.Since(start), err)
	return err
}

var _ Mapper = (*ObservedMapper)(nil)
