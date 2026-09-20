// Package sla implements the SLA engine: business-hours-aware elapsed/
// remaining/percentage calculation with pause/resume ("stopwatch") support.
package sla

import "time"

// Business hours: 8AM-5PM weekdays, 12PM-1PM lunch excluded, giving 8
// effective business hours per weekday. This matches the standard business
// calendar used across the platform's SLA policies.
const (
	businessStartHour = 8
	businessEndHour   = 17
	lunchStartHour    = 12
	lunchEndHour      = 13
)

func isWeekday(t time.Time) bool {
	d := t.Weekday()
	return d != time.Saturday && d != time.Sunday
}

type timeRange struct {
	Start, End time.Time
}

func (r timeRange) overlap(other timeRange) time.Duration {
	start := maxTime(r.Start, other.Start)
	end := minTime(r.End, other.End)
	if end.Before(start) {
		return 0
	}
	return end.Sub(start)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// dayBusinessWindows returns the two business sub-intervals for the calendar
// day containing t (morning: 8-12, afternoon: 1-5), or nothing on weekends.
func dayBusinessWindows(t time.Time) []timeRange {
	if !isWeekday(t) {
		return nil
	}
	y, m, d := t.Date()
	loc := t.Location()
	morningStart := time.Date(y, m, d, businessStartHour, 0, 0, 0, loc)
	morningEnd := time.Date(y, m, d, lunchStartHour, 0, 0, 0, loc)
	afternoonStart := time.Date(y, m, d, lunchEndHour, 0, 0, 0, loc)
	afternoonEnd := time.Date(y, m, d, businessEndHour, 0, 0, 0, loc)
	return []timeRange{
		{morningStart, morningEnd},
		{afternoonStart, afternoonEnd},
	}
}

// BusinessMinutesBetween returns the number of business minutes (8AM-5PM
// weekdays, minus the 12-1PM lunch hour) that elapsed between start and end.
// If end is before start, returns 0.
func BusinessMinutesBetween(start, end time.Time) int {
	if !end.After(start) {
		return 0
	}
	start = start.UTC()
	end = end.UTC()

	total := time.Duration(0)
	full := timeRange{start, end}

	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	for !day.After(endDay) {
		for _, w := range dayBusinessWindows(day) {
			total += full.overlap(w)
		}
		day = day.AddDate(0, 0, 1)
	}

	return int(total.Minutes())
}

// AddBusinessMinutes projects forward from start by the given number of
// business minutes, skipping nights, weekends, and the lunch hour, and
// returns the resulting timestamp. Used to compute a live-adjusted deadline
// once pauses are accounted for.
func AddBusinessMinutes(start time.Time, minutes int) time.Time {
	if minutes <= 0 {
		return start
	}
	start = start.UTC()
	remaining := time.Duration(minutes) * time.Minute
	cursor := start

	for i := 0; i < 3650; i++ { // hard cap: ~10 years of days, avoids any infinite loop
		windows := dayBusinessWindows(cursor)
		for _, w := range windows {
			if cursor.Before(w.Start) {
				cursor = w.Start
			}
			if cursor.Before(w.End) {
				avail := w.End.Sub(cursor)
				if avail >= remaining {
					return cursor.Add(remaining)
				}
				remaining -= avail
				cursor = w.End
			}
		}
		next := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		cursor = next
	}
	return cursor // fallback, not normally reached
}
