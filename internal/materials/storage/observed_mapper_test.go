package storage

import (
	"context"
	"testing"
	"time"
)

type fakeObserver struct {
	uploads   int
	deletes   int
	prewarms  int
	refreshes int
}

func (f *fakeObserver) RecordUpload(duration time.Duration, size uint64, err error) {
	f.uploads++
}

func (f *fakeObserver) RecordDelete(duration time.Duration, err error) {
	f.deletes++
}

func (f *fakeObserver) RecordPrewarm(duration time.Duration, err error) {
	f.prewarms++
}

func (f *fakeObserver) RecordRefresh(duration time.Duration, err error) {
	f.refreshes++
}

func TestObservedMapperRecordsOperations(t *testing.T) {
	mapper := NewInMemoryMapper("https://cdn.example.com")
	obs := &fakeObserver{}
	observed := NewObservedMapper(mapper, obs)
	ctx := context.Background()
	_, err := observed.Upload(ctx, UploadRequest{Name: "a.png", MimeType: "image/png", Data: []byte("1234")})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if err := observed.Prewarm(ctx, "materials/foo"); err != nil {
		t.Fatalf("prewarm failed: %v", err)
	}
	if err := observed.Refresh(ctx, "materials/foo"); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if err := observed.Delete(ctx, "materials/foo"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if obs.uploads != 1 || obs.deletes != 1 || obs.prewarms != 1 || obs.refreshes != 1 {
		t.Fatalf("observer counts mismatch: %+v", obs)
	}
}
