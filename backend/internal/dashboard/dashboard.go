// Package dashboard computes the platform's core proactive-attention
// features (spec sections 14-16): "What Needs My Attention", "My Work", and
// the priority queue. Everything here is a live database query — nothing is
// hardcoded or precomputed.
package dashboard

import (
	"database/sql"
	"time"

	"itopshub/backend/internal/sla"
)

// TicketSummary is the compact ticket shape used across attention/my-work/
// priority-queue responses — enough for a UI list row without a second
// round trip per ticket.
type TicketSummary struct {
	ID                    string  `json:"id"`
	TicketNumber          string  `json:"ticketNumber"`
	Title                 string  `json:"title"`
	Severity              string  `json:"severity"`
	Priority              string  `json:"priority"`
	Status                string  `json:"status"`
	AssigneeID            *string `json:"assigneeId"`
	SLAStatus             string  `json:"slaStatus"`
	SLAPercentage         float64 `json:"slaPercentage"`
	EscalationLevel       int     `json:"escalationLevel"`
	HasUnresolvedFollowUp bool    `json:"hasUnresolvedFollowUp"`
}

var openStatuses = []string{
	"NEW", "ACKNOWLEDGED", "ASSIGNED", "IN_PROGRESS", "PENDING",
	"FOR_UAT", "UAT_FAILED", "UAT_PASSED", "FOR_PRE_PROD", "PRE_PROD_FAILED",
	"PRE_PROD_PASSED", "FOR_PRODUCTION", "PRODUCTION_FAILED",
}

func placeholders(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += "?"
	}
	return s
}

func toArgs(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

type openTicketRow struct {
	ID, Number, Title, Severity, Priority, Status string
	AssigneeID                                    *string
	EscalationLevel                               int
	RCARequired                                   bool
}

func loadOpenTickets(db *sql.DB) ([]openTicketRow, error) {
	rows, err := db.Query(`
		SELECT id, ticket_number, title, severity, priority, status, assignee_id, escalation_level, rca_required
		FROM tickets WHERE status IN (`+placeholders(len(openStatuses))+`)
	`, toArgs(openStatuses)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]openTicketRow, 0)
	for rows.Next() {
		var r openTicketRow
		var assigneeID sql.NullString
		var rcaRequired int
		if err := rows.Scan(&r.ID, &r.Number, &r.Title, &r.Severity, &r.Priority, &r.Status, &assigneeID, &r.EscalationLevel, &rcaRequired); err != nil {
			return nil, err
		}
		if assigneeID.Valid {
			r.AssigneeID = &assigneeID.String
		}
		r.RCARequired = rcaRequired != 0
		out = append(out, r)
	}
	return out, nil
}

func hasUnresolvedFollowUp(db *sql.DB, ticketID string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM follow_ups WHERE ticket_id = ? AND resolved = 0`, ticketID).Scan(&n)
	return n > 0, err
}

func toSummary(r openTicketRow, slaResult *sla.Result, hasFollowUp bool) TicketSummary {
	s := TicketSummary{
		ID: r.ID, TicketNumber: r.Number, Title: r.Title, Severity: r.Severity,
		Priority: r.Priority, Status: r.Status, AssigneeID: r.AssigneeID,
		EscalationLevel: r.EscalationLevel, HasUnresolvedFollowUp: hasFollowUp,
	}
	if slaResult != nil {
		s.SLAStatus = slaResult.Status
		s.SLAPercentage = slaResult.Percentage
	}
	return s
}

// Attention is the "What Needs My Attention" widget response (spec section 14).
type Attention struct {
	SLABreached           []TicketSummary `json:"slaBreached"`
	SLACritical           []TicketSummary `json:"slaCritical"`
	SLAWarning            []TicketSummary `json:"slaWarning"`
	FollowUpRequired      []TicketSummary `json:"followUpRequired"`
	RCARequired           []TicketSummary `json:"rcaRequired"`
	WaitingForMyAction    int             `json:"waitingForMyActionCount"`
	WaitingForDevelopment int             `json:"waitingForDevelopmentCount"`
	ReadyForUAT           int             `json:"readyForUATCount"`
	ReadyForPreProd       int             `json:"readyForPreProdCount"`
	ReadyForProduction    int             `json:"readyForProductionCount"`
}

// ComputeAttention builds the full attention widget for userID by walking
// every open ticket, computing its live SLA status, and bucketing it.
func ComputeAttention(db *sql.DB, userID string) (*Attention, error) {
	tickets, err := loadOpenTickets(db)
	if err != nil {
		return nil, err
	}

	result := &Attention{
		SLABreached:      make([]TicketSummary, 0),
		SLACritical:      make([]TicketSummary, 0),
		SLAWarning:       make([]TicketSummary, 0),
		FollowUpRequired: make([]TicketSummary, 0),
		RCARequired:      make([]TicketSummary, 0),
	}

	for _, t := range tickets {
		slaResult, err := sla.Compute(db, t.ID)
		if err != nil {
			continue // skip tickets we can't compute SLA for rather than failing the whole widget
		}
		followUp, _ := hasUnresolvedFollowUp(db, t.ID)

		switch slaResult.Status {
		case sla.StatusBreached:
			result.SLABreached = append(result.SLABreached, toSummary(t, slaResult, followUp))
		case sla.StatusCritical:
			result.SLACritical = append(result.SLACritical, toSummary(t, slaResult, followUp))
		case sla.StatusWarning:
			result.SLAWarning = append(result.SLAWarning, toSummary(t, slaResult, followUp))
		}

		if followUp {
			result.FollowUpRequired = append(result.FollowUpRequired, toSummary(t, slaResult, followUp))
		}
		if t.RCARequired {
			result.RCARequired = append(result.RCARequired, toSummary(t, slaResult, followUp))
		}

		if t.AssigneeID != nil && *t.AssigneeID == userID {
			result.WaitingForMyAction++
		}
		switch t.Status {
		case "IN_PROGRESS":
			result.WaitingForDevelopment++
		case "FOR_UAT":
			result.ReadyForUAT++
		case "FOR_PRE_PROD":
			result.ReadyForPreProd++
		case "FOR_PRODUCTION":
			result.ReadyForProduction++
		}
	}

	// Cap list sizes so the widget stays a widget, not a full ticket dump.
	result.SLABreached = capList(result.SLABreached, 10)
	result.SLACritical = capList(result.SLACritical, 10)
	result.SLAWarning = capList(result.SLAWarning, 10)
	result.FollowUpRequired = capList(result.FollowUpRequired, 10)
	result.RCARequired = capList(result.RCARequired, 10)

	return result, nil
}

func capList(list []TicketSummary, max int) []TicketSummary {
	if len(list) > max {
		return list[:max]
	}
	return list
}

// MyWork is the "My Work" page response (spec section 15), scoped entirely
// to tickets relevant to userID.
type MyWork struct {
	Urgent             []TicketSummary `json:"urgent"`
	SLABreached        []TicketSummary `json:"slaBreached"`
	SLACritical        []TicketSummary `json:"slaCritical"`
	SLAWarning         []TicketSummary `json:"slaWarning"`
	FollowUpRequired   []TicketSummary `json:"followUpRequired"`
	AssignedToMe       []TicketSummary `json:"assignedToMe"`
	WaitingForMyAction []TicketSummary `json:"waitingForMyAction"`
	ReadyForUAT        []TicketSummary `json:"readyForUAT"`
	ReadyForPreProd    []TicketSummary `json:"readyForPreProd"`
	ReadyForProduction []TicketSummary `json:"readyForProduction"`
	RecentlyUpdated    []TicketSummary `json:"recentlyUpdated"`
}

func ComputeMyWork(db *sql.DB, userID string) (*MyWork, error) {
	tickets, err := loadOpenTickets(db)
	if err != nil {
		return nil, err
	}

	result := &MyWork{
		Urgent:             make([]TicketSummary, 0),
		SLABreached:        make([]TicketSummary, 0),
		SLACritical:        make([]TicketSummary, 0),
		SLAWarning:         make([]TicketSummary, 0),
		FollowUpRequired:   make([]TicketSummary, 0),
		AssignedToMe:       make([]TicketSummary, 0),
		WaitingForMyAction: make([]TicketSummary, 0),
		ReadyForUAT:        make([]TicketSummary, 0),
		ReadyForPreProd:    make([]TicketSummary, 0),
		ReadyForProduction: make([]TicketSummary, 0),
		RecentlyUpdated:    make([]TicketSummary, 0),
	}

	for _, t := range tickets {
		if t.AssigneeID == nil || *t.AssigneeID != userID {
			continue
		}

		slaResult, err := sla.Compute(db, t.ID)
		if err != nil {
			continue
		}
		followUp, _ := hasUnresolvedFollowUp(db, t.ID)
		summary := toSummary(t, slaResult, followUp)

		result.AssignedToMe = append(result.AssignedToMe, summary)

		// "Urgent" = S1/S2 severity that's also SLA critical or breached —
		// the intersection is what actually deserves top-of-mind status.
		if (t.Severity == "S1" || t.Severity == "S2") &&
			(slaResult.Status == sla.StatusCritical || slaResult.Status == sla.StatusBreached) {
			result.Urgent = append(result.Urgent, summary)
		}

		switch slaResult.Status {
		case sla.StatusBreached:
			result.SLABreached = append(result.SLABreached, summary)
		case sla.StatusCritical:
			result.SLACritical = append(result.SLACritical, summary)
		case sla.StatusWarning:
			result.SLAWarning = append(result.SLAWarning, summary)
		}
		if followUp {
			result.FollowUpRequired = append(result.FollowUpRequired, summary)
		}

		// "Waiting for my action" = anything active that isn't already
		// sitting downstream (with someone else, e.g. QA/deployment stages).
		switch t.Status {
		case "NEW", "ACKNOWLEDGED", "ASSIGNED", "IN_PROGRESS", "UAT_FAILED", "PRE_PROD_FAILED", "PRODUCTION_FAILED":
			result.WaitingForMyAction = append(result.WaitingForMyAction, summary)
		case "FOR_UAT":
			result.ReadyForUAT = append(result.ReadyForUAT, summary)
		case "FOR_PRE_PROD":
			result.ReadyForPreProd = append(result.ReadyForPreProd, summary)
		case "FOR_PRODUCTION":
			result.ReadyForProduction = append(result.ReadyForProduction, summary)
		}
	}

	recentIDs, recentSummaries, err := recentlyUpdated(db, userID, 10)
	if err != nil {
		return nil, err
	}
	_ = recentIDs
	result.RecentlyUpdated = recentSummaries

	return result, nil
}

func recentlyUpdated(db *sql.DB, userID string, limit int) ([]string, []TicketSummary, error) {
	rows, err := db.Query(`
		SELECT id, ticket_number, title, severity, priority, status, assignee_id, escalation_level
		FROM tickets WHERE assignee_id = ? ORDER BY updated_at DESC LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var ids []string
	out := make([]TicketSummary, 0)
	for rows.Next() {
		var s TicketSummary
		var assigneeID sql.NullString
		if err := rows.Scan(&s.ID, &s.TicketNumber, &s.Title, &s.Severity, &s.Priority, &s.Status, &assigneeID, &s.EscalationLevel); err != nil {
			return nil, nil, err
		}
		if assigneeID.Valid {
			s.AssigneeID = &assigneeID.String
		}
		ids = append(ids, s.ID)
		out = append(out, s)
	}
	return ids, out, nil
}

// PriorityItem is one ranked entry in the priority queue.
type PriorityItem struct {
	TicketSummary
	Rank             int     `json:"rank"`
	Score            float64 `json:"score"`
	AgeHours         float64 `json:"ageHours"`
	SinceUpdateHours float64 `json:"sinceUpdateHours"`
}

// ComputePriorityQueue ranks open tickets by a weighted score combining
// severity, priority, live SLA percentage/status, ticket age, staleness,
// unresolved follow-ups, and escalation level — implemented entirely in Go,
// not a stored/hardcoded order (spec section 16).
func ComputePriorityQueue(db *sql.DB, userID string, scopeAll bool, limit int) ([]PriorityItem, error) {
	tickets, err := loadOpenTickets(db)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	items := make([]PriorityItem, 0)

	for _, t := range tickets {
		if !scopeAll {
			if t.AssigneeID == nil || *t.AssigneeID != userID {
				continue
			}
		}

		slaResult, err := sla.Compute(db, t.ID)
		if err != nil {
			continue
		}
		followUp, _ := hasUnresolvedFollowUp(db, t.ID)

		var createdAtStr, updatedAtStr string
		db.QueryRow(`SELECT created_at, updated_at FROM tickets WHERE id = ?`, t.ID).Scan(&createdAtStr, &updatedAtStr)
		createdAt, err1 := time.Parse(time.RFC3339, normalizeTime(createdAtStr))
		updatedAt, err2 := time.Parse(time.RFC3339, normalizeTime(updatedAtStr))
		if err1 != nil {
			createdAt = now
		}
		if err2 != nil {
			updatedAt = now
		}

		ageHours := now.Sub(createdAt).Hours()
		staleHours := now.Sub(updatedAt).Hours()

		score := severityWeight(t.Severity)*10 +
			priorityWeight(t.Priority)*5 +
			slaResult.Percentage*0.6 +
			breachedBonus(slaResult.Status) +
			float64(t.EscalationLevel)*15 +
			followUpBonus(followUp) +
			clampContribution(ageHours*0.05, 10) +
			clampContribution(staleHours*0.1, 15)

		items = append(items, PriorityItem{
			TicketSummary:    toSummary(t, slaResult, followUp),
			Score:            roundTo(score, 1),
			AgeHours:         roundTo(ageHours, 1),
			SinceUpdateHours: roundTo(staleHours, 1),
		})
	}

	sortByScoreDesc(items)

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	for i := range items {
		items[i].Rank = i + 1
	}

	return items, nil
}

func severityWeight(s string) float64 {
	switch s {
	case "S1":
		return 4
	case "S2":
		return 3
	case "S3":
		return 2
	default:
		return 1
	}
}

func priorityWeight(p string) float64 {
	switch p {
	case "Critical":
		return 4
	case "High":
		return 3
	case "Medium":
		return 2
	default:
		return 1
	}
}

func breachedBonus(status string) float64 {
	switch status {
	case sla.StatusBreached:
		return 50
	case sla.StatusCritical:
		return 20
	case sla.StatusWarning:
		return 5
	default:
		return 0
	}
}

func followUpBonus(has bool) float64 {
	if has {
		return 10
	}
	return 0
}

// clampContribution caps how much a single factor (age, staleness) can
// contribute, so a very old low-severity ticket can't outrank a fresh S1.
func clampContribution(v, max float64) float64 {
	if v > max {
		return max
	}
	return v
}

func sortByScoreDesc(items []PriorityItem) {
	// Simple insertion sort is fine at local-deployment scale (tens to low
	// hundreds of open tickets) and keeps this dependency-free.
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && items[j-1].Score < items[j].Score {
			items[j-1], items[j] = items[j], items[j-1]
			j--
		}
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
