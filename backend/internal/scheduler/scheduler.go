package scheduler

import (
	"context"
	"log/slog"
	"time"
)

type Job func(context.Context) error
type Scheduler struct {
	logger   *slog.Logger
	interval time.Duration
	jobs     []Job
}

func New(logger *slog.Logger, interval time.Duration, jobs ...Job) *Scheduler {
	return &Scheduler{logger: logger, interval: interval, jobs: jobs}
}
func (s *Scheduler) Run(ctx context.Context) {
	s.runJobs(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runJobs(ctx)
		}
	}
}

func (s *Scheduler) runJobs(ctx context.Context) {
	for _, job := range s.jobs {
		if err := job(ctx); err != nil {
			s.logger.Error("scheduled job failed", "error", err)
		}
	}
}
