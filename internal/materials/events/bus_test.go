package events

import (
	"context"
	"testing"
	"time"

	materialapi "alex/internal/materials/api"
)

func TestBusDeliversMaterialEvents(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Watch(ctx, "req-1")
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}

	material := &materialapi.Material{Context: &materialapi.RequestContext{RequestID: "req-1"}}
	if err := bus.PublishMaterial(context.Background(), material); err != nil {
		t.Fatalf("publish material: %v", err)
	}

	select {
	case evt := <-ch:
		if evt == nil || evt.Material != material {
			t.Fatalf("unexpected event payload: %#v", evt)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event")
	}
}

func TestBusCleansUpWatchers(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := bus.Watch(ctx, "req-2")
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel to be closed after cancel")
		}
	case <-time.After(time.Second):
		t.Fatalf("channel did not close")
	}
}

func TestBusPublishesTombstones(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := bus.Watch(ctx, "req-3")
	if err != nil {
		t.Fatalf("watch returned error: %v", err)
	}

	if err := bus.PublishTombstone(context.Background(), "req-3", "mat-1"); err != nil {
		t.Fatalf("publish tombstone: %v", err)
	}

	select {
	case evt := <-ch:
		if evt.TombstoneID != "mat-1" {
			t.Fatalf("expected tombstone id, got %#v", evt)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for tombstone")
	}
}
