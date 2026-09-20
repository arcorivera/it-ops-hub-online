// Package incidents implements incident management (spec section 43).
// RCA gating on closure (spec section 44) is enforced via a narrow
// RCAGateChecker interface satisfied by the rca package, avoiding an import
// cycle the same way tickets/testing do for the UAT gate.
package incidents

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("incident not found")
var ErrInvalidTransition = errors.New("invalid status transition")
var ErrRCARequired = errors.New("RCA must be completed before this incident can be closed")

// Statuses (spec section 43).
const (
	StatusOpen          = "OPEN"
	StatusInvestigating = "INVESTIGATING"
	StatusMitigated     = "MITIGATED"
	StatusResolved      = "RESOLVED"
	StatusClosed        = "CLOSED"
)

var ValidStatuses = map[string]bool{
	StatusOpen: true, StatusInvestigating: true, StatusMitigated: true, StatusResolved: true, StatusClosed: true,
}

var validTransitions = map[string][]string{
	StatusOpen:          {StatusInvestigating},
	StatusInvestigating: {StatusMitigated, StatusOpen},
	StatusMitigated:     {StatusResolved, StatusInvestigating},
	StatusResolved:      {StatusClosed, StatusInvestigating},
	StatusClosed:        {},
}

func IsValidTransition(from, to string) bool {
	for _, s := range validTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// RequiresRCA reports whether an incident of this severity must have a
// completed RCA before it can be closed (spec section 44: "S1/S2 incidents
// should require RCA before closure").
func RequiresRCA(severity string) bool {
	return severity == "S1" || severity == "S2"
}

type Incident struct {
	ID             string     `json:"id"`
	IncidentNumber string     `json:"incidentNumber"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Severity       string     `json:"severity"`
	Impact         string     `json:"impact"`
	AffectedSystem string     `json:"affectedSystem"`
	Environment    string     `json:"environment"`
	OwnerID        *string    `json:"ownerId"`
	Status         string     `json:"status"`
	TicketID       *string    `json:"ticketId"`
	StartedAt      time.Time  `json:"startedAt"`
	ResolvedAt     *time.Time `json:"resolvedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	RCARequired    bool       `json:"rcaRequired"`
}

type RCAGateChecker interface {
	IsRCACompleted(incidentID string) (bool, error)
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) nextIncidentNumber() (string, error) {
	year := time.Now().UTC().Year()
	prefix := fmt.Sprintf("INC%d-", year)

	var maxNum sql.NullString
	err := r.db.QueryRow(`SELECT MAX(incident_number) FROM incidents WHERE incident_number LIKE ?`, prefix+"%").Scan(&maxNum)
	if err != nil {
		return "", err
	}
	seq := 1
	if maxNum.Valid {
		var n int
		fmt.Sscanf(strings.TrimPrefix(maxNum.String, prefix), "%d", &n)
		seq = n + 1
	}
	return fmt.Sprintf("%s%05d", prefix, seq), nil
}

type CreateInput struct {
	Title          string
	Description    string
	Severity       string
	Impact         string
	AffectedSystem string
	Environment    string
	OwnerID        *string
	TicketID       *string
}

func (r *Repository) Create(in CreateInput) (*Incident, error) {
	id := "inc-" + uuid.NewString()
	number, err := r.nextIncidentNumber()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	_, err = r.db.Exec(
		`INSERT INTO incidents (id, incident_number, title, description, severity, impact, affected_system, environment, owner_id, status, ticket_id, started_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, number, in.Title, in.Description, in.Severity, in.Impact, in.AffectedSystem, in.Environment,
		in.OwnerID, StatusOpen, in.TicketID, now, now, now,
	)
	if err != nil {
		return nil, err
	}

	// If this incident is linked to a ticket and is S1/S2, flag the ticket
	// as requiring RCA — this is what feeds the dashboard's "RCA Required"
	// bucket (spec section 14) with real data instead of an always-empty column.
	if in.TicketID != nil && RequiresRCA(in.Severity) {
		r.db.Exec(`UPDATE tickets SET rca_required = 1 WHERE id = ?`, *in.TicketID)
	}

	return r.GetByID(id)
}

func (r *Repository) GetByID(id string) (*Incident, error) {
	row := r.db.QueryRow(`
		SELECT id, incident_number, title, description, severity, impact, affected_system, environment,
		       owner_id, status, ticket_id, started_at, resolved_at, created_at, updated_at
		FROM incidents WHERE id = ?`, id)
	return scanIncident(row)
}

func scanIncident(row *sql.Row) (*Incident, error) {
	var inc Incident
	var ownerID, ticketID, resolvedAt sql.NullString
	var startedAt, createdAt, updatedAt string
	err := row.Scan(&inc.ID, &inc.IncidentNumber, &inc.Title, &inc.Description, &inc.Severity, &inc.Impact,
		&inc.AffectedSystem, &inc.Environment, &ownerID, &inc.Status, &ticketID, &startedAt, &resolvedAt, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if ownerID.Valid {
		inc.OwnerID = &ownerID.String
	}
	if ticketID.Valid {
		inc.TicketID = &ticketID.String
	}
	inc.StartedAt, _ = time.Parse(time.RFC3339, normalizeTime(startedAt))
	if resolvedAt.Valid && resolvedAt.String != "" {
		t, _ := time.Parse(time.RFC3339, normalizeTime(resolvedAt.String))
		inc.ResolvedAt = &t
	}
	inc.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	inc.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
	inc.RCARequired = RequiresRCA(inc.Severity)
	return &inc, nil
}

func (r *Repository) List(status, severity string) ([]*Incident, error) {
	query := `SELECT id FROM incidents WHERE 1=1`
	args := []interface{}{}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if severity != "" {
		query += ` AND severity = ?`
		args = append(args, severity)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	out := make([]*Incident, 0)
	for _, id := range ids {
		inc, err := r.GetByID(id)
		if err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, nil
}

// UpdateStatus performs a validated status transition. Closing an S1/S2
// incident requires a completed RCA (spec section 44) unless force is set
// by an authorized (admin) caller — this mirrors the UAT gate pattern in
// the tickets package.
func (r *Repository) UpdateStatus(id, newStatus string, force bool, rcaChecker RCAGateChecker) (*Incident, error) {
	inc, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !force && !IsValidTransition(inc.Status, newStatus) {
		return nil, ErrInvalidTransition
	}

	if newStatus == StatusClosed && !force && RequiresRCA(inc.Severity) && rcaChecker != nil {
		completed, err := rcaChecker.IsRCACompleted(id)
		if err != nil {
			return nil, err
		}
		if !completed {
			return nil, ErrRCARequired
		}
	}

	now := time.Now().UTC()
	setClauses := "status = ?, updated_at = ?"
	args := []interface{}{newStatus, now}
	if newStatus == StatusResolved && inc.ResolvedAt == nil {
		setClauses += ", resolved_at = ?"
		args = append(args, now)
	}
	args = append(args, id)

	_, err = r.db.Exec(`UPDATE incidents SET `+setClauses+` WHERE id = ?`, args...)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
