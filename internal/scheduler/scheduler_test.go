package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseEveryAndCron(t *testing.T) {
	for _, expr := range []string{"@every 30m", "0 9,21 * * *", "@daily"} {
		if _, err := Parse(expr); err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", expr, err)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse("not a schedule"); err == nil {
		t.Fatal("expected error for invalid schedule expression")
	}
}

func TestLoopFiresAndStopsOnCancel(t *testing.T) {
	sched, err := Parse("@every 1s")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var fires int64
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Loop(ctx, sched, nil, func() { atomic.AddInt64(&fires, 1) })
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Loop did not return after ctx cancellation")
	}

	if atomic.LoadInt64(&fires) == 0 {
		t.Fatal("expected at least one fire before cancellation")
	}
}

func TestLoopSkipsWhilePaused(t *testing.T) {
	sched, err := Parse("@every 1s")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var fires int64
	var paused atomic.Bool
	paused.Store(true)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		Loop(ctx, sched, paused.Load, func() { atomic.AddInt64(&fires, 1) })
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Loop did not return after ctx cancellation")
	}

	if atomic.LoadInt64(&fires) != 0 {
		t.Fatalf("expected no fires while paused, got %d", fires)
	}
}
