package sla

import (
	"database/sql"
	"errors"
	"time"
)

var ErrTicketNotFound = errors.New("ticket not found")

// Status values (spec section 29).
const (
	StatusWithinSLA = "WITHIN_SLA"
	StatusWarning   = "WARNING"
	StatusCritical  = "CRITICAL"
	StatusBreached  = "BREACHED"
	StatusPaused    = "PAUSED"
	StatusResolved  = "RESOLVED"
)

// Thresholds (spec section 29): 50% warning, 75% critical, 100% breached.
const (
	WarningThreshold  = 50
	CriticalThreshold = 75
	BreachThreshold   = 100
)

type ticketRow struct {
	ID                string
	Severity          string
	Status            string
	CreatedAt         time.Time
	ResolvedAt        *time.Time
	ResolutionMinutes int
	ResponseMinutes   int
	UseBusinessHours  bool
	HasSLAPolicy      bool
}

type slaEvent struct {
	EventType string
	CreatedAt time.Time
}

// Result is the computed, live SLA status for a ticket.
type Result struct {
	TicketID          string    `json:"ticketId"`
	Status            string    `json:"status"`
	Percentage        float64   `json:"percentage"`
	ElapsedMinutes    int       `json:"elapsedMinutes"`
	ResolutionMinutes int       `json:"resolutionMinutes"`
	RemainingMinutes  int       `json:"remainingMinutes"`
	ProjectedDeadline time.Time `json:"projectedDeadline"`
	IsPaused          bool      `json:"isPaused"`
	UsesBusinessHours bool      `json:"usesBusinessHours"`
}

// Compute returns the live, pause-aware SLA status for the given ticket ID.
// It reads the ticket's severity-linked policy and its sla_events history
// (STARTED/PAUSED/RESUMED) to compute actual elapsed time against the
// resolution target, projecting a live-adjusted deadline that accounts for
// any time the ticket spent paused.
func Compute(db *sql.DB, ticketID string) (*Result, error) {
	row := db.QueryRow(`
		SELECT t.id, t.severity, t.status, t.created_at, t.resolved_at,
		       p.resolution_minutes, p.response_minutes, p.use_business_hours
		FROM tickets t
		LEFT JOIN sla_policies p ON p.id = t.sla_policy_id
		WHERE t.id = ?
	`, ticketID)

	var tr ticketRow
	var createdAt string
	var resolvedAt sql.NullString
	var resolutionMinutes, responseMinutes sql.NullInt64
	var useBusinessHours sql.NullInt64

	err := row.Scan(&tr.ID, &tr.Severity, &tr.Status, &createdAt, &resolvedAt, &resolutionMinutes, &responseMinutes, &useBusinessHours)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}

	tr.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	if resolvedAt.Valid && resolvedAt.String != "" {
		rt, _ := time.Parse(time.RFC3339, normalizeTime(resolvedAt.String))
		tr.ResolvedAt = &rt
	}
	tr.HasSLAPolicy = resolutionMinutes.Valid
	tr.ResolutionMinutes = int(resolutionMinutes.Int64)
	tr.ResponseMinutes = int(responseMinutes.Int64)
	tr.UseBusinessHours = useBusinessHours.Int64 != 0

	if !tr.HasSLAPolicy {
		return &Result{TicketID: ticketID, Status: StatusWithinSLA, UsesBusinessHours: false}, nil
	}

	events, err := loadEvents(db, ticketID)
	if err != nil {
		return nil, err
	}

	return computeFromEvents(tr, events, time.Now().UTC()), nil
}

func loadEvents(db *sql.DB, ticketID string) ([]slaEvent, error) {
	rows, err := db.Query(
		`SELECT event_type, created_at FROM sla_events WHERE ticket_id = ? AND event_type IN ('STARTED','PAUSED','RESUMED') ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]slaEvent, 0)
	for rows.Next() {
		var e slaEvent
		var createdAt string
		if err := rows.Scan(&e.EventType, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		out = append(out, e)
	}
	return out, nil
}

// computeFromEvents walks the STARTED/PAUSED/RESUMED event log to build the
// list of "running" intervals (the stopwatch was active), sums elapsed
// business or wall-clock minutes across them, and derives status/percentage.
func computeFromEvents(tr ticketRow, events []slaEvent, now time.Time) *Result {
	// Ticket resolved/closed: SLA clock stops at resolution time.
	cutoff := now
	resolved := tr.ResolvedAt != nil
	if resolved {
		cutoff = *tr.ResolvedAt
	}

	// Build running intervals from the event log. If there's no event log at
	// all (shouldn't happen once Create() always inserts STARTED, but this
	// keeps the calculation safe for pre-existing data), treat the whole
	// span from creation as running.
	type interval struct{ start, end time.Time }
	var intervals []interval

	cursor := tr.CreatedAt
	running := true
	for _, e := range events {
		switch e.EventType {
		case "STARTED":
			cursor = e.CreatedAt
			running = true
		case "PAUSED":
			if running {
				intervals = append(intervals, interval{cursor, e.CreatedAt})
			}
			running = false
		case "RESUMED":
			cursor = e.CreatedAt
			running = true
		}
	}
	isPaused := !running && !resolved
	if running {
		intervals = append(intervals, interval{cursor, cutoff})
	}

	elapsed := 0
	for _, iv := range intervals {
		end := iv.end
		if end.After(cutoff) {
			end = cutoff
		}
		if tr.UseBusinessHours {
			elapsed += BusinessMinutesBetween(iv.start, end)
		} else {
			d := end.Sub(iv.start)
			if d > 0 {
				elapsed += int(d.Minutes())
			}
		}
	}

	remaining := tr.ResolutionMinutes - elapsed
	var percentage float64
	if tr.ResolutionMinutes > 0 {
		percentage = float64(elapsed) / float64(tr.ResolutionMinutes) * 100
	}

	status := StatusWithinSLA
	switch {
	case resolved:
		status = StatusResolved
	case isPaused:
		status = StatusPaused
	case percentage >= BreachThreshold:
		status = StatusBreached
	case percentage >= CriticalThreshold:
		status = StatusCritical
	case percentage >= WarningThreshold:
		status = StatusWarning
	}

	var projectedDeadline time.Time
	if remaining > 0 {
		if tr.UseBusinessHours {
			projectedDeadline = AddBusinessMinutes(now, remaining)
		} else {
			projectedDeadline = now.Add(time.Duration(remaining) * time.Minute)
		}
	} else {
		// Already breached (or exactly at 100%): deadline was "now minus overage".
		if tr.UseBusinessHours {
			projectedDeadline = now
		} else {
			projectedDeadline = now.Add(time.Duration(remaining) * time.Minute)
		}
	}

	return &Result{
		TicketID:          tr.ID,
		Status:            status,
		Percentage:        roundTo(percentage, 1),
		ElapsedMinutes:    elapsed,
		ResolutionMinutes: tr.ResolutionMinutes,
		RemainingMinutes:  remaining,
		ProjectedDeadline: projectedDeadline,
		IsPaused:          isPaused,
		UsesBusinessHours: tr.UseBusinessHours,
	}
}

func roundTo(v float64, places int) float64 {
	mult := 1.0
	for i := 0; i < places; i++ {
		mult *= 10
	}
	return float64(int(v*mult+0.5)) / mult
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
