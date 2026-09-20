package tickets

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("ticket not found")
var ErrCommentNotFound = errors.New("comment not found")
var ErrInvalidTransition = errors.New("invalid status transition")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// nextTicketNumber generates T{year}-{00001} sequentially per calendar year.
// The single-connection SQLite pool (see database.Open) serializes this
// naturally, so no extra locking is required.
func (r *Repository) nextTicketNumber() (string, error) {
	year := time.Now().UTC().Year()
	prefix := fmt.Sprintf("T%d-", year)

	var maxNum sql.NullString
	err := r.db.QueryRow(
		`SELECT MAX(ticket_number) FROM tickets WHERE ticket_number LIKE ?`,
		prefix+"%",
	).Scan(&maxNum)
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
	Title       string
	Description string
	RequesterID *string
	AssigneeID  *string
	TeamID      *string
	ProjectID   *string
	CategoryID  *string
	Severity    string
	Priority    string
	Environment string
}

// slaPolicy is the minimal shape read from sla_policies for deadline math.
type slaPolicy struct {
	ID                string
	ResponseMinutes   int
	ResolutionMinutes int
}

func (r *Repository) slaPolicyForSeverity(severity string) (*slaPolicy, error) {
	row := r.db.QueryRow(`SELECT id, response_minutes, resolution_minutes FROM sla_policies WHERE severity = ? AND is_active = 1`, severity)
	var p slaPolicy
	if err := row.Scan(&p.ID, &p.ResponseMinutes, &p.ResolutionMinutes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// Create inserts a new ticket, generates its ticket number, assigns an SLA
// policy by severity, and records the CREATED history event.
//
// NOTE on deadlines: this computes response/resolution deadlines as simple
// wall-clock offsets from creation time. This is a correct baseline for S1/S2
// (which are wall-clock per spec section 28) but is a placeholder for S3/S4,
// which are meant to be business-hours-aware — that precision (and SLA pause
// support) is implemented by the SLA engine in the next build phase, which
// will recompute deadlines using business-hours math instead of this
// straight-line estimate.
func (r *Repository) Create(in CreateInput, actorID string) (*Ticket, error) {
	id := "tkt-" + uuid.NewString()
	number, err := r.nextTicketNumber()
	if err != nil {
		return nil, err
	}

	policy, err := r.slaPolicyForSeverity(in.Severity)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var slaPolicyID *string
	var responseDeadline, resolutionDeadline *time.Time
	if policy != nil {
		slaPolicyID = &policy.ID
		rd := now.Add(time.Duration(policy.ResponseMinutes) * time.Minute)
		resd := now.Add(time.Duration(policy.ResolutionMinutes) * time.Minute)
		responseDeadline = &rd
		resolutionDeadline = &resd
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO tickets (
			id, ticket_number, title, description, requester_id, assignee_id, team_id, project_id,
			category_id, severity, priority, status, environment, sla_policy_id,
			response_deadline, resolution_deadline, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, number, in.Title, in.Description, in.RequesterID, in.AssigneeID, in.TeamID, in.ProjectID,
		in.CategoryID, in.Severity, in.Priority, StatusNew, in.Environment, slaPolicyID,
		responseDeadline, resolutionDeadline, now, now,
	)
	if err != nil {
		return nil, err
	}

	if err := insertHistory(tx, id, actorID, "CREATED", "", "", "", "Ticket created"); err != nil {
		return nil, err
	}
	if err := insertSLAEvent(tx, id, "STARTED", ""); err != nil {
		return nil, err
	}
	if in.AssigneeID != nil {
		if err := insertHistory(tx, id, actorID, "ASSIGNED", "assignee", "", *in.AssigneeID, ""); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func insertHistory(tx *sql.Tx, ticketID, userID, eventType, field, oldVal, newVal, note string) error {
	var uid interface{}
	if userID != "" {
		uid = userID
	}
	_, err := tx.Exec(
		`INSERT INTO ticket_history (id, ticket_id, user_id, event_type, field, old_value, new_value, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"hist-"+uuid.NewString(), ticketID, uid, eventType, field, oldVal, newVal, note, time.Now().UTC(),
	)
	return err
}

// insertSLAEvent records an SLA stopwatch event (STARTED/PAUSED/RESUMED/etc)
// used by the SLA engine to compute pause-aware elapsed time.
func insertSLAEvent(tx *sql.Tx, ticketID, eventType, note string) error {
	_, err := tx.Exec(
		`INSERT INTO sla_events (id, ticket_id, event_type, note, created_at) VALUES (?, ?, ?, ?, ?)`,
		"slaevt-"+uuid.NewString(), ticketID, eventType, note, time.Now().UTC(),
	)
	return err
}

func scanTicket(row interface{ Scan(...interface{}) error }) (*Ticket, error) {
	var t Ticket
	var requesterID, assigneeID, teamID, projectID, categoryID, slaPolicyID sql.NullString
	var responseDeadline, resolutionDeadline, acknowledgedAt, resolvedAt, closedAt, lastFollowupAt, nextFollowupAt sql.NullString
	var createdAt, updatedAt string
	var rcaRequired int

	err := row.Scan(
		&t.ID, &t.TicketNumber, &t.Title, &t.Description,
		&requesterID, &assigneeID, &teamID, &projectID, &categoryID,
		&t.Severity, &t.Priority, &t.Status, &t.Environment, &slaPolicyID,
		&responseDeadline, &resolutionDeadline,
		&createdAt, &acknowledgedAt, &resolvedAt, &closedAt, &updatedAt,
		&lastFollowupAt, &nextFollowupAt, &rcaRequired, &t.EscalationLevel,
	)
	if err != nil {
		return nil, err
	}

	t.RequesterID = nullableString(requesterID)
	t.AssigneeID = nullableString(assigneeID)
	t.TeamID = nullableString(teamID)
	t.ProjectID = nullableString(projectID)
	t.CategoryID = nullableString(categoryID)
	t.SLAPolicyID = nullableString(slaPolicyID)
	t.ResponseDeadline = nullableTime(responseDeadline)
	t.ResolutionDeadline = nullableTime(resolutionDeadline)
	t.AcknowledgedAt = nullableTime(acknowledgedAt)
	t.ResolvedAt = nullableTime(resolvedAt)
	t.ClosedAt = nullableTime(closedAt)
	t.LastFollowupAt = nullableTime(lastFollowupAt)
	t.NextFollowupAt = nullableTime(nextFollowupAt)
	t.RCARequired = rcaRequired != 0

	ca, _ := time.Parse(time.RFC3339, normalizeTime(createdAt))
	ua, _ := time.Parse(time.RFC3339, normalizeTime(updatedAt))
	t.CreatedAt = ca
	t.UpdatedAt = ua

	return &t, nil
}

const ticketColumns = `id, ticket_number, title, description, requester_id, assignee_id, team_id, project_id,
	category_id, severity, priority, status, environment, sla_policy_id,
	response_deadline, resolution_deadline, created_at, acknowledged_at, resolved_at, closed_at, updated_at,
	last_followup_at, next_followup_at, rca_required, escalation_level`

func (r *Repository) GetByID(id string) (*Ticket, error) {
	row := r.db.QueryRow(`SELECT `+ticketColumns+` FROM tickets WHERE id = ?`, id)
	t, err := scanTicket(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

type ListFilter struct {
	Status      []string
	Severity    []string
	Priority    []string
	AssigneeID  string
	TeamID      string
	ProjectID   string
	Environment string
	CategoryID  string
	Search      string
	Page        int
	PageSize    int
	SortBy      string // created_at, updated_at, resolution_deadline
	SortDir     string // asc, desc
}

type ListResult struct {
	Tickets  []*Ticket
	Total    int
	Page     int
	PageSize int
}

func (r *Repository) List(f ListFilter) (*ListResult, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if len(f.Status) > 0 {
		where = append(where, inClause("status", len(f.Status)))
		for _, s := range f.Status {
			args = append(args, s)
		}
	}
	if len(f.Severity) > 0 {
		where = append(where, inClause("severity", len(f.Severity)))
		for _, s := range f.Severity {
			args = append(args, s)
		}
	}
	if len(f.Priority) > 0 {
		where = append(where, inClause("priority", len(f.Priority)))
		for _, s := range f.Priority {
			args = append(args, s)
		}
	}
	if f.AssigneeID != "" {
		where = append(where, "assignee_id = ?")
		args = append(args, f.AssigneeID)
	}
	if f.TeamID != "" {
		where = append(where, "team_id = ?")
		args = append(args, f.TeamID)
	}
	if f.ProjectID != "" {
		where = append(where, "project_id = ?")
		args = append(args, f.ProjectID)
	}
	if f.Environment != "" {
		where = append(where, "environment = ?")
		args = append(args, f.Environment)
	}
	if f.CategoryID != "" {
		where = append(where, "category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.Search != "" {
		where = append(where, "(title LIKE ? OR ticket_number LIKE ? OR description LIKE ?)")
		like := "%" + f.Search + "%"
		args = append(args, like, like, like)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM tickets WHERE " + whereClause
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	sortBy := "created_at"
	switch f.SortBy {
	case "updated_at", "resolution_deadline", "created_at", "severity", "priority":
		sortBy = f.SortBy
	}
	sortDir := "DESC"
	if strings.ToUpper(f.SortDir) == "ASC" {
		sortDir = "ASC"
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 25
	}
	offset := (page - 1) * pageSize

	query := "SELECT " + ticketColumns + " FROM tickets WHERE " + whereClause +
		fmt.Sprintf(" ORDER BY %s %s LIMIT ? OFFSET ?", sortBy, sortDir)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Ticket, 0)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}

	return &ListResult{Tickets: out, Total: total, Page: page, PageSize: pageSize}, nil
}

func inClause(col string, n int) string {
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return col + " IN (" + strings.Join(placeholders, ",") + ")"
}

// UpdateStatus performs a validated status transition, stamping
// acknowledged/resolved/closed timestamps as appropriate, and records history.
func (r *Repository) UpdateStatus(id, newStatus, actorID, note string, force bool) (*Ticket, error) {
	t, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	if !force && !IsValidTransition(t.Status, newStatus) {
		return nil, ErrInvalidTransition
	}

	now := time.Now().UTC()
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	setClauses := []string{"status = ?", "updated_at = ?"}
	args := []interface{}{newStatus, now}

	if newStatus == StatusAcknowledged && t.AcknowledgedAt == nil {
		setClauses = append(setClauses, "acknowledged_at = ?")
		args = append(args, now)
	}
	if newStatus == StatusResolved {
		setClauses = append(setClauses, "resolved_at = ?")
		args = append(args, now)
	}
	if newStatus == StatusClosed {
		setClauses = append(setClauses, "closed_at = ?")
		args = append(args, now)
	}

	args = append(args, id)
	_, err = tx.Exec("UPDATE tickets SET "+strings.Join(setClauses, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return nil, err
	}

	eventNote := note
	if force {
		eventNote = "[OVERRIDE] " + note
	}
	if err := insertHistory(tx, id, actorID, "STATUS_CHANGED", "status", t.Status, newStatus, eventNote); err != nil {
		return nil, err
	}

	// The SLA clock automatically pauses while a ticket is PENDING (spec
	// section 31) and resumes the moment it leaves PENDING for any other
	// active status. This keeps the pause/resume "stopwatch" in sync with
	// the ticket's actual status without requiring a separate manual action.
	if newStatus == StatusPending && t.Status != StatusPending {
		if err := insertSLAEvent(tx, id, "PAUSED", "Auto-paused: ticket moved to PENDING"); err != nil {
			return nil, err
		}
	}
	if t.Status == StatusPending && newStatus != StatusPending {
		if err := insertSLAEvent(tx, id, "RESUMED", "Auto-resumed: ticket left PENDING"); err != nil {
			return nil, err
		}
	}
	if newStatus == StatusResolved {
		if err := insertSLAEvent(tx, id, "RESOLVED", ""); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *Repository) UpdateSeverity(id, newSeverity, actorID string) (*Ticket, error) {
	t, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE tickets SET severity = ?, updated_at = ? WHERE id = ?`, newSeverity, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}
	if err := insertHistory(tx, id, actorID, "SEVERITY_CHANGED", "severity", t.Severity, newSeverity, ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *Repository) UpdatePriority(id, newPriority, actorID string) (*Ticket, error) {
	t, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE tickets SET priority = ?, updated_at = ? WHERE id = ?`, newPriority, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}
	if err := insertHistory(tx, id, actorID, "PRIORITY_CHANGED", "priority", t.Priority, newPriority, ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *Repository) Assign(id string, assigneeID *string, actorID string) (*Ticket, error) {
	t, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE tickets SET assignee_id = ?, updated_at = ? WHERE id = ?`, assigneeID, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}

	oldVal := ""
	if t.AssigneeID != nil {
		oldVal = *t.AssigneeID
	}
	newVal := ""
	if assigneeID != nil {
		newVal = *assigneeID
	}
	if err := insertHistory(tx, id, actorID, "ASSIGNED", "assignee", oldVal, newVal, ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

// --- Comments ---

func (r *Repository) AddComment(ticketID, userID, content string) (*Comment, error) {
	id := "cmt-" + uuid.NewString()
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var uid interface{}
	if userID != "" {
		uid = userID
	}
	_, err = tx.Exec(
		`INSERT INTO ticket_comments (id, ticket_id, user_id, content, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
		id, ticketID, uid, content, now, now,
	)
	if err != nil {
		return nil, err
	}
	if err := insertHistory(tx, ticketID, userID, "COMMENT_ADDED", "", "", "", ""); err != nil {
		return nil, err
	}
	_, err = tx.Exec(`UPDATE tickets SET updated_at = ? WHERE id = ?`, now, ticketID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.getComment(id)
}

func (r *Repository) getComment(id string) (*Comment, error) {
	row := r.db.QueryRow(`SELECT id, ticket_id, user_id, content, created_at, updated_at, is_deleted FROM ticket_comments WHERE id = ?`, id)
	var c Comment
	var userID sql.NullString
	var createdAt, updatedAt string
	var isDeleted int
	if err := row.Scan(&c.ID, &c.TicketID, &userID, &c.Content, &createdAt, &updatedAt, &isDeleted); err != nil {
		return nil, err
	}
	c.UserID = nullableString(userID)
	c.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	c.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
	c.IsDeleted = isDeleted != 0
	return &c, nil
}

func (r *Repository) ListComments(ticketID string) ([]*Comment, error) {
	rows, err := r.db.Query(
		`SELECT id, ticket_id, user_id, content, created_at, updated_at, is_deleted
		 FROM ticket_comments WHERE ticket_id = ? AND is_deleted = 0 ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Comment, 0)
	for rows.Next() {
		var c Comment
		var userID sql.NullString
		var createdAt, updatedAt string
		var isDeleted int
		if err := rows.Scan(&c.ID, &c.TicketID, &userID, &c.Content, &createdAt, &updatedAt, &isDeleted); err != nil {
			return nil, err
		}
		c.UserID = nullableString(userID)
		c.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		c.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
		c.IsDeleted = isDeleted != 0
		out = append(out, &c)
	}
	return out, nil
}

func (r *Repository) DeleteComment(commentID, actorID string, isAdmin bool) error {
	row := r.db.QueryRow(`SELECT user_id, ticket_id FROM ticket_comments WHERE id = ?`, commentID)
	var ownerID sql.NullString
	var ticketID string
	if err := row.Scan(&ownerID, &ticketID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCommentNotFound
		}
		return err
	}
	if !isAdmin && (!ownerID.Valid || ownerID.String != actorID) {
		return errors.New("not authorized to delete this comment")
	}
	_, err := r.db.Exec(`UPDATE ticket_comments SET is_deleted = 1, updated_at = ? WHERE id = ?`, time.Now().UTC(), commentID)
	return err
}

// --- History ---

func (r *Repository) ListHistory(ticketID string) ([]*HistoryEvent, error) {
	rows, err := r.db.Query(
		`SELECT id, ticket_id, user_id, event_type, field, old_value, new_value, note, created_at
		 FROM ticket_history WHERE ticket_id = ? ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*HistoryEvent, 0)
	for rows.Next() {
		var h HistoryEvent
		var userID sql.NullString
		var createdAt string
		if err := rows.Scan(&h.ID, &h.TicketID, &userID, &h.EventType, &h.Field, &h.OldValue, &h.NewValue, &h.Note, &createdAt); err != nil {
			return nil, err
		}
		h.UserID = nullableString(userID)
		h.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		out = append(out, &h)
	}
	return out, nil
}

// --- Attachments ---

func (r *Repository) AddAttachment(ticketID, userID, fileName, storedName, contentType string, size int64) (*Attachment, error) {
	id := "att-" + uuid.NewString()
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var uid interface{}
	if userID != "" {
		uid = userID
	}
	_, err = tx.Exec(
		`INSERT INTO ticket_attachments (id, ticket_id, user_id, file_name, stored_name, content_type, size_bytes, created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		id, ticketID, uid, fileName, storedName, contentType, size, now,
	)
	if err != nil {
		return nil, err
	}
	if err := insertHistory(tx, ticketID, userID, "ATTACHMENT_ADDED", "", "", fileName, ""); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	row := r.db.QueryRow(`SELECT id, ticket_id, user_id, file_name, stored_name, content_type, size_bytes, created_at FROM ticket_attachments WHERE id = ?`, id)
	return scanAttachment(row)
}

func scanAttachment(row *sql.Row) (*Attachment, error) {
	var a Attachment
	var userID sql.NullString
	var createdAt string
	if err := row.Scan(&a.ID, &a.TicketID, &userID, &a.FileName, &a.StoredName, &a.ContentType, &a.SizeBytes, &createdAt); err != nil {
		return nil, err
	}
	a.UserID = nullableString(userID)
	a.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	return &a, nil
}

func (r *Repository) ListAttachments(ticketID string) ([]*Attachment, error) {
	rows, err := r.db.Query(`SELECT id, ticket_id, user_id, file_name, stored_name, content_type, size_bytes, created_at FROM ticket_attachments WHERE ticket_id = ? ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Attachment, 0)
	for rows.Next() {
		var a Attachment
		var userID sql.NullString
		var createdAt string
		if err := rows.Scan(&a.ID, &a.TicketID, &userID, &a.FileName, &a.StoredName, &a.ContentType, &a.SizeBytes, &createdAt); err != nil {
			return nil, err
		}
		a.UserID = nullableString(userID)
		a.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		out = append(out, &a)
	}
	return out, nil
}

func (r *Repository) GetAttachment(id string) (*Attachment, error) {
	row := r.db.QueryRow(`SELECT id, ticket_id, user_id, file_name, stored_name, content_type, size_bytes, created_at FROM ticket_attachments WHERE id = ?`, id)
	a, err := scanAttachment(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a, nil
}

// --- helpers ---

func nullableString(ns sql.NullString) *string {
	if ns.Valid {
		v := ns.String
		return &v
	}
	return nil
}

func nullableTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, normalizeTime(ns.String))
	if err != nil {
		return nil
	}
	return &t
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
