package sla

import (
	"testing"
	"time"
)

func at(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

func TestBusinessMinutesBetween_SameDayMorning(t *testing.T) {
	// Monday 9:00 -> 11:00 = 2 hours = 120 minutes, fully within business hours.
	start := at(2026, 8, 24, 9, 0) // Monday
	end := at(2026, 8, 24, 11, 0)
	got := BusinessMinutesBetween(start, end)
	if got != 120 {
		t.Errorf("expected 120, got %d", got)
	}
}

func TestBusinessMinutesBetween_SpansLunch(t *testing.T) {
	// Monday 11:00 -> 14:00 = 3 hours wall clock, minus 1 hour lunch = 2 hours = 120 min.
	start := at(2026, 8, 24, 11, 0)
	end := at(2026, 8, 24, 14, 0)
	got := BusinessMinutesBetween(start, end)
	if got != 120 {
		t.Errorf("expected 120 (3h minus 1h lunch), got %d", got)
	}
}

func TestBusinessMinutesBetween_FullDay(t *testing.T) {
	// Monday 8:00 -> 17:00 = 9 hours wall clock, minus 1h lunch = 8 hours = 480 min.
	start := at(2026, 8, 24, 8, 0)
	end := at(2026, 8, 24, 17, 0)
	got := BusinessMinutesBetween(start, end)
	if got != 480 {
		t.Errorf("expected 480 (one full business day), got %d", got)
	}
}

func TestBusinessMinutesBetween_OutsideHoursExcluded(t *testing.T) {
	// Monday 6:00 -> 20:00 should only count the 8-17 window minus lunch = 480 min.
	start := at(2026, 8, 24, 6, 0)
	end := at(2026, 8, 24, 20, 0)
	got := BusinessMinutesBetween(start, end)
	if got != 480 {
		t.Errorf("expected 480, got %d", got)
	}
}

func TestBusinessMinutesBetween_WeekendExcluded(t *testing.T) {
	// Friday 16:00 -> Monday 9:00.
	// Friday: 16:00-17:00 = 60 min. Sat/Sun: 0. Monday: 8:00-9:00 = 60 min.
	// Total = 120 min.
	start := at(2026, 8, 21, 16, 0) // Friday
	end := at(2026, 8, 24, 9, 0)    // Monday
	got := BusinessMinutesBetween(start, end)
	if got != 120 {
		t.Errorf("expected 120, got %d", got)
	}
}

func TestBusinessMinutesBetween_MultiDay(t *testing.T) {
	// Monday 8:00 -> Wednesday 8:00 = 2 full business days = 960 min.
	start := at(2026, 8, 24, 8, 0) // Monday
	end := at(2026, 8, 26, 8, 0)   // Wednesday
	got := BusinessMinutesBetween(start, end)
	if got != 960 {
		t.Errorf("expected 960 (2 full business days), got %d", got)
	}
}

func TestBusinessMinutesBetween_EndBeforeStart(t *testing.T) {
	start := at(2026, 8, 24, 12, 0)
	end := at(2026, 8, 24, 10, 0)
	got := BusinessMinutesBetween(start, end)
	if got != 0 {
		t.Errorf("expected 0 for end before start, got %d", got)
	}
}

func TestAddBusinessMinutes_WithinSameDay(t *testing.T) {
	start := at(2026, 8, 24, 9, 0) // Monday
	got := AddBusinessMinutes(start, 60)
	want := at(2026, 8, 24, 10, 0)
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestAddBusinessMinutes_SkipsLunch(t *testing.T) {
	start := at(2026, 8, 24, 11, 30) // Monday 11:30
	got := AddBusinessMinutes(start, 60)
	// 30 min to reach lunch (12:00), then jump to 13:00, use remaining 30 min -> 13:30
	want := at(2026, 8, 24, 13, 30)
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestAddBusinessMinutes_SkipsWeekend(t *testing.T) {
	start := at(2026, 8, 21, 16, 30) // Friday 16:30 (30 min left in the day)
	got := AddBusinessMinutes(start, 60)
	// 30 min uses up Friday (ends at 17:00), remaining 30 min rolls to Monday 8:00 -> 8:30
	want := at(2026, 8, 24, 8, 30)
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestAddBusinessMinutes_FromOutsideHours(t *testing.T) {
	// Starting at 20:00 (after hours) should jump to next business window (Tue 8:00) first.
	start := at(2026, 8, 24, 20, 0) // Monday evening
	got := AddBusinessMinutes(start, 30)
	want := at(2026, 8, 25, 8, 30) // Tuesday 8:30
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestAddBusinessMinutes_RoundTrip(t *testing.T) {
	// Adding N business minutes then measuring business minutes back should recover N,
	// as an internal-consistency check between the two functions.
	start := at(2026, 8, 24, 9, 0)
	for _, n := range []int{15, 60, 240, 480, 960, 2400} {
		end := AddBusinessMinutes(start, n)
		got := BusinessMinutesBetween(start, end)
		if got != n {
			t.Errorf("round trip for %d minutes: got %d business minutes back", n, got)
		}
	}
}
