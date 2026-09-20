package sla

import (
	"testing"
)

func TestComputeFromEvents_NoPause_Warning(t *testing.T) {
	created := at(2026, 8, 24, 8, 0) // Monday 8:00
	tr := ticketRow{
		ID: "t1", CreatedAt: created, ResolutionMinutes: 480, UseBusinessHours: true, HasSLAPolicy: true,
	}
	events := []slaEvent{{EventType: "STARTED", CreatedAt: created}}
	// 4 business hours later = 50% of an 8-business-hour SLA.
	now := at(2026, 8, 24, 12, 0)

	result := computeFromEvents(tr, events, now)
	if result.ElapsedMinutes != 240 {
		t.Errorf("expected 240 elapsed minutes, got %d", result.ElapsedMinutes)
	}
	if result.Percentage != 50 {
		t.Errorf("expected 50%%, got %v", result.Percentage)
	}
	if result.Status != StatusWarning {
		t.Errorf("expected WARNING at exactly 50%%, got %s", result.Status)
	}
}

func TestComputeFromEvents_PauseExcludedFromElapsed(t *testing.T) {
	created := at(2026, 8, 24, 8, 0) // Monday 8:00
	tr := ticketRow{
		ID: "t1", CreatedAt: created, ResolutionMinutes: 480, UseBusinessHours: true, HasSLAPolicy: true,
	}
	events := []slaEvent{
		{EventType: "STARTED", CreatedAt: created},
		{EventType: "PAUSED", CreatedAt: at(2026, 8, 24, 10, 0)},  // 2h elapsed so far
		{EventType: "RESUMED", CreatedAt: at(2026, 8, 26, 10, 0)}, // 2 days later, paused time doesn't count
	}
	// 2 more business hours after resume.
	now := at(2026, 8, 26, 12, 0)

	result := computeFromEvents(tr, events, now)
	if result.ElapsedMinutes != 240 {
		t.Errorf("expected 240 elapsed minutes (2h + 2h, pause excluded), got %d", result.ElapsedMinutes)
	}
	if result.IsPaused {
		t.Error("ticket should not currently be paused (was resumed)")
	}
}

func TestComputeFromEvents_CurrentlyPaused(t *testing.T) {
	created := at(2026, 8, 24, 8, 0)
	tr := ticketRow{
		ID: "t1", CreatedAt: created, ResolutionMinutes: 480, UseBusinessHours: true, HasSLAPolicy: true,
	}
	events := []slaEvent{
		{EventType: "STARTED", CreatedAt: created},
		{EventType: "PAUSED", CreatedAt: at(2026, 8, 24, 10, 0)},
	}
	now := at(2026, 8, 24, 15, 0) // well after the pause, should not accrue more elapsed time

	result := computeFromEvents(tr, events, now)
	if !result.IsPaused {
		t.Error("expected ticket to be currently paused")
	}
	if result.Status != StatusPaused {
		t.Errorf("expected PAUSED status, got %s", result.Status)
	}
	if result.ElapsedMinutes != 120 {
		t.Errorf("expected elapsed to freeze at 120 (time of pause), got %d", result.ElapsedMinutes)
	}
}

func TestComputeFromEvents_Breached(t *testing.T) {
	created := at(2026, 8, 24, 8, 0)
	tr := ticketRow{
		ID: "t1", CreatedAt: created, ResolutionMinutes: 480, UseBusinessHours: true, HasSLAPolicy: true,
	}
	events := []slaEvent{{EventType: "STARTED", CreatedAt: created}}
	// 3 full business days later — way past the 8-hour (1 day) SLA target.
	now := at(2026, 8, 27, 8, 0)

	result := computeFromEvents(tr, events, now)
	if result.Status != StatusBreached {
		t.Errorf("expected BREACHED, got %s", result.Status)
	}
	if result.RemainingMinutes >= 0 {
		t.Errorf("expected negative remaining minutes, got %d", result.RemainingMinutes)
	}
}

func TestComputeFromEvents_Resolved(t *testing.T) {
	created := at(2026, 8, 24, 8, 0)
	resolvedAt := at(2026, 8, 24, 10, 0)
	tr := ticketRow{
		ID: "t1", CreatedAt: created, ResolvedAt: &resolvedAt, ResolutionMinutes: 480, UseBusinessHours: true, HasSLAPolicy: true,
	}
	events := []slaEvent{{EventType: "STARTED", CreatedAt: created}}
	now := at(2026, 8, 28, 8, 0) // long after resolution — should not matter

	result := computeFromEvents(tr, events, now)
	if result.Status != StatusResolved {
		t.Errorf("expected RESOLVED, got %s", result.Status)
	}
	if result.ElapsedMinutes != 120 {
		t.Errorf("expected elapsed frozen at resolution time (120 min), got %d", result.ElapsedMinutes)
	}
}

func TestComputeFromEvents_WallClockForS1(t *testing.T) {
	created := at(2026, 8, 24, 8, 0)
	tr := ticketRow{
		ID: "t1", CreatedAt: created, ResolutionMinutes: 240, UseBusinessHours: false, HasSLAPolicy: true, // S1: 4h wall clock
	}
	events := []slaEvent{{EventType: "STARTED", CreatedAt: created}}
	// 2 hours later, including outside-business-hours doesn't matter for wall clock.
	now := at(2026, 8, 24, 10, 0)

	result := computeFromEvents(tr, events, now)
	if result.ElapsedMinutes != 120 {
		t.Errorf("expected 120 wall-clock minutes, got %d", result.ElapsedMinutes)
	}
	if result.Percentage != 50 {
		t.Errorf("expected 50%%, got %v", result.Percentage)
	}
}

func TestComputeFromEvents_CriticalThreshold(t *testing.T) {
	created := at(2026, 8, 24, 8, 0)
	tr := ticketRow{
		ID: "t1", CreatedAt: created, ResolutionMinutes: 480, UseBusinessHours: true, HasSLAPolicy: true,
	}
	events := []slaEvent{{EventType: "STARTED", CreatedAt: created}}
	// 6 business hours = 75% exactly.
	now := at(2026, 8, 24, 15, 0) // 8-12 (4h) + 13-15 (2h) = 6h
	result := computeFromEvents(tr, events, now)
	if result.Status != StatusCritical {
		t.Errorf("expected CRITICAL at 75%%, got %s (%v%%)", result.Status, result.Percentage)
	}
}
