// Package scheduler parses termbg's schedule expressions (either a Go
// duration via "@every 30m", or a standard 5-field cron expression
// like "0 9,21 * * *") and runs a background loop that fires a
// callback each time the schedule is due.
package scheduler

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
)

// parser accepts standard 5-field cron expressions plus descriptors
// such as "@every 30m", "@daily", "@hourly".
var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Parse validates and parses a schedule expression. An empty
// expression is invalid here; callers should check for "no schedule
// configured" before calling Parse.
func Parse(expr string) (cron.Schedule, error) {
	return parser.Parse(expr)
}

// Loop runs fn every time sched is due, until ctx is cancelled or
// paused() returns true (in which case the tick is skipped but the
// loop keeps waiting for the next scheduled time). It blocks the
// calling goroutine; call it with `go`.
func Loop(ctx context.Context, sched cron.Schedule, paused func() bool, fn func()) {
	for {
		next := sched.Next(time.Now())
		wait := time.Until(next)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if paused == nil || !paused() {
				fn()
			}
		}
	}
}
