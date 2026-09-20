package scheduler

import (
	"context"
	"log/slog"
	"time"

	"itopshub/backend/internal/escalation"
	"itopshub/backend/internal/followup"
)

// Scheduler runs the follow-up and escalation background passes on a fixed
// interval, per spec section 32 ("Create a Go background worker. Run every
// minute.").
type Scheduler struct {
	interval time.Duration
	followUp *followup.Engine
	escalate *escalation.Engine
	logger   *slog.Logger
}

func New(interval time.Duration, followUp *followup.Engine, escalate *escalation.Engine, logger *slog.Logger) *Scheduler {
	return &Scheduler{interval: interval, followUp: followUp, escalate: escalate, logger: logger}
}

// Start runs the scheduler loop until ctx is cancelled. Each tick runs both
// engines synchronously and logs how many follow-ups/escalations were raised
// — real, observable side effects, not a no-op timer.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.runOnce() // run once immediately on startup rather than waiting a full interval

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce()
		}
	}
}

func (s *Scheduler) runOnce() {
	fu, err := s.followUp.RunPass()
	if err != nil {
		s.logger.Error("followup pass failed", "error", err)
	} else if fu > 0 {
		s.logger.Info("followup pass complete", "raised", fu)
	}

	esc, err := s.escalate.RunPass()
	if err != nil {
		s.logger.Error("escalation pass failed", "error", err)
	} else if esc > 0 {
		s.logger.Info("escalation pass complete", "raised", esc)
	}
}
