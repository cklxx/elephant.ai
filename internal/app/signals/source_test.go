package signals

import (
	"context"
	"testing"
)

func TestLarkSourceName(t *testing.T) {
	src := NewLarkSource([]string{"chat1"})
	if src.Name() != SourceLark {
		t.Errorf("Name() = %q, want %q", src.Name(), SourceLark)
	}
}

func TestLarkSourceStartStop(t *testing.T) {
	src := NewLarkSource([]string{"chat1"})
	sink := make(chan SignalEvent, 10)
	ctx := context.Background()
	if err := src.Start(ctx, sink); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	src.Stop() // should not panic or block
}

func TestLarkSourceStopBeforeStart(t *testing.T) {
	src := NewLarkSource(nil)
	src.Stop() // should not panic
}
