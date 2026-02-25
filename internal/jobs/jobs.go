package jobs

import (
	"context"
	"log/slog"
	"time"

	"goclaw/internal/memory"
)

// Runner manages periodic background tasks.
type Runner struct {
	compressor *memory.Compressor
	interval   time.Duration
}

// NewRunner creates a new jobs runner.
func NewRunner(compressor *memory.Compressor, interval time.Duration) *Runner {
	if interval == 0 {
		interval = 6 * time.Hour
	}
	return &Runner{
		compressor: compressor,
		interval:   interval,
	}
}

// Start runs the periodic background tasks until the context is cancelled.
func (r *Runner) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	slog.Info("background jobs runner started", "interval", r.interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("background jobs runner stopping")
			return
		case <-ticker.C:
			r.runTasks(ctx)
		}
	}
}

func (r *Runner) runTasks(ctx context.Context) {
	slog.Info("running background tasks")

	// Example task: auto-compress sessions (placeholder logic)
	// In a real app, you might iterate over active sessions or a DB.
	// For this daemon, we'll let the Gateway trigger compression or
	// scan the history directory.
}
