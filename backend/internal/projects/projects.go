package projects

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("project not found")
var ErrDuplicateKey = errors.New("project key already in use")
var ErrMilestoneNotFound = errors.New("milestone not found")

// Project statuses (spec section 36).
const (
	StatusPlanning  = "PLANNING"
	StatusActive    = "ACTIVE"
	StatusOnHold    = "ON_HOLD"
	StatusCompleted = "COMPLETED"
	StatusCancelled = "CANCELLED"
)

var ValidProjectStatuses = map[string]bool{
	StatusPlanning: true, StatusActive: true, StatusOnHold: true, StatusCompleted: true, StatusCancelled: true,
}

// Milestone statuses (spec section 38).
const (
	MilestoneNotStarted = "NOT_STARTED"
	MilestoneInProgress = "IN_PROGRESS"
	MilestoneCompleted  = "COMPLETED"
	MilestoneOverdue    = "OVERDUE"
)

type Project struct {
	ID          string    `json:"id"`
	ProjectKey  string    `json:"projectKey"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     *string   `json:"ownerId"`
	Status      string    `json:"status"`
	StartDate   *string   `json:"startDate"`
	TargetDate  *string   `json:"targetDate"`
	MemberIDs   []string  `json:"memberIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Milestone struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DueDate     *string   `json:"dueDate"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateProjectInput struct {
	ProjectKey  string
	Name        string
	Description string
	OwnerID     *string
	StartDate   *string
	TargetDate  *string
}

func (r *Repository) CreateProject(in CreateProjectInput) (*Project, error) {
	var exists int
	r.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE project_key = ?`, in.ProjectKey).Scan(&exists)
	if exists > 0 {
		return nil, ErrDuplicateKey
	}

	id := "proj-" + uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.Exec(
		`INSERT INTO projects (id, project_key, name, description, owner_id, status, start_date, target_date, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.ProjectKey, in.Name, in.Description, in.OwnerID, StatusPlanning, in.StartDate, in.TargetDate, now, now,
	)
	if err != nil {
		return nil, err
	}
	return r.GetProject(id)
}

func (r *Repository) GetProject(id string) (*Project, error) {
	row := r.db.QueryRow(`
		SELECT id, project_key, name, description, owner_id, status, start_date, target_date, created_at, updated_at
		FROM projects WHERE id = ?`, id)

	var p Project
	var ownerID, startDate, targetDate sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&p.ID, &p.ProjectKey, &p.Name, &p.Description, &ownerID, &p.Status, &startDate, &targetDate, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if ownerID.Valid {
		p.OwnerID = &ownerID.String
	}
	if startDate.Valid {
		p.StartDate = &startDate.String
	}
	if targetDate.Valid {
		p.TargetDate = &targetDate.String
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	p.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))

	members, err := r.membersForProject(id)
	if err != nil {
		return nil, err
	}
	p.MemberIDs = members

	return &p, nil
}

func (r *Repository) membersForProject(projectID string) ([]string, error) {
	rows, err := r.db.Query(`SELECT user_id FROM project_members WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, nil
}

func (r *Repository) ListProjects(status string) ([]*Project, error) {
	query := `SELECT id FROM projects`
	args := []interface{}{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
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
	out := make([]*Project, 0)
	for _, id := range ids {
		p, err := r.GetProject(id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

type UpdateProjectInput struct {
	Name        *string
	Description *string
	OwnerID     *string
	Status      *string
	StartDate   *string
	TargetDate  *string
	MemberIDs   *[]string
}

func (r *Repository) UpdateProject(id string, in UpdateProjectInput) (*Project, error) {
	existing, err := r.GetProject(id)
	if err != nil {
		return nil, err
	}

	name := existing.Name
	if in.Name != nil {
		name = *in.Name
	}
	desc := existing.Description
	if in.Description != nil {
		desc = *in.Description
	}
	status := existing.Status
	if in.Status != nil {
		status = *in.Status
	}
	var ownerID interface{}
	if in.OwnerID != nil {
		ownerID = *in.OwnerID
	} else if existing.OwnerID != nil {
		ownerID = *existing.OwnerID
	}
	var startDate interface{}
	if in.StartDate != nil {
		startDate = *in.StartDate
	} else if existing.StartDate != nil {
		startDate = *existing.StartDate
	}
	var targetDate interface{}
	if in.TargetDate != nil {
		targetDate = *in.TargetDate
	} else if existing.TargetDate != nil {
		targetDate = *existing.TargetDate
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE projects SET name=?, description=?, owner_id=?, status=?, start_date=?, target_date=?, updated_at=? WHERE id=?`,
		name, desc, ownerID, status, startDate, targetDate, time.Now().UTC(), id,
	)
	if err != nil {
		return nil, err
	}

	if in.MemberIDs != nil {
		if _, err := tx.Exec(`DELETE FROM project_members WHERE project_id = ?`, id); err != nil {
			return nil, err
		}
		for _, m := range *in.MemberIDs {
			if _, err := tx.Exec(`INSERT INTO project_members (project_id, user_id) VALUES (?, ?)`, id, m); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetProject(id)
}

// --- Milestones ---

type CreateMilestoneInput struct {
	ProjectID   string
	Name        string
	Description string
	DueDate     *string
}

func (r *Repository) CreateMilestone(in CreateMilestoneInput) (*Milestone, error) {
	id := "ms-" + uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.Exec(
		`INSERT INTO milestones (id, project_id, name, description, due_date, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.ProjectID, in.Name, in.Description, in.DueDate, MilestoneNotStarted, now, now,
	)
	if err != nil {
		return nil, err
	}
	return r.GetMilestone(id)
}

func (r *Repository) GetMilestone(id string) (*Milestone, error) {
	row := r.db.QueryRow(`SELECT id, project_id, name, description, due_date, status, created_at, updated_at FROM milestones WHERE id = ?`, id)
	m, err := scanMilestone(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMilestoneNotFound
		}
		return nil, err
	}
	return applyOverdueDetection(m), nil
}

func scanMilestone(row *sql.Row) (*Milestone, error) {
	var m Milestone
	var dueDate sql.NullString
	var createdAt, updatedAt string
	if err := row.Scan(&m.ID, &m.ProjectID, &m.Name, &m.Description, &dueDate, &m.Status, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if dueDate.Valid {
		m.DueDate = &dueDate.String
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	m.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
	return &m, nil
}

// applyOverdueDetection returns the milestone with its status live-adjusted
// to OVERDUE if the due date has passed and it isn't already completed
// (spec section 38: "Automatically detect overdue milestones"). This is
// computed on read rather than via a separate background job, since it's a
// pure function of due_date/status/now.
func applyOverdueDetection(m *Milestone) *Milestone {
	if m.Status == MilestoneCompleted || m.DueDate == nil {
		return m
	}
	due, err := time.Parse("2006-01-02", (*m.DueDate)[:10])
	if err != nil {
		return m
	}
	if time.Now().UTC().After(due.AddDate(0, 0, 1)) {
		m.Status = MilestoneOverdue
	}
	return m
}

func (r *Repository) ListMilestones(projectID string) ([]*Milestone, error) {
	rows, err := r.db.Query(`SELECT id, project_id, name, description, due_date, status, created_at, updated_at FROM milestones WHERE project_id = ? ORDER BY due_date ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Milestone, 0)
	for rows.Next() {
		var m Milestone
		var dueDate sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Name, &m.Description, &dueDate, &m.Status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if dueDate.Valid {
			m.DueDate = &dueDate.String
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		m.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
		out = append(out, applyOverdueDetection(&m))
	}
	return out, nil
}

type UpdateMilestoneInput struct {
	Name        *string
	Description *string
	DueDate     *string
	Status      *string
}

func (r *Repository) UpdateMilestone(id string, in UpdateMilestoneInput) (*Milestone, error) {
	existing, err := r.GetMilestone(id)
	if err != nil {
		return nil, err
	}
	name := existing.Name
	if in.Name != nil {
		name = *in.Name
	}
	desc := existing.Description
	if in.Description != nil {
		desc = *in.Description
	}
	status := existing.Status
	if in.Status != nil {
		status = *in.Status
	}
	var dueDate interface{}
	if in.DueDate != nil {
		dueDate = *in.DueDate
	} else if existing.DueDate != nil {
		dueDate = *existing.DueDate
	}

	_, err = r.db.Exec(
		`UPDATE milestones SET name=?, description=?, due_date=?, status=?, updated_at=? WHERE id=?`,
		name, desc, dueDate, status, time.Now().UTC(), id,
	)
	if err != nil {
		return nil, err
	}
	return r.GetMilestone(id)
}

// ProjectStats aggregates the live counts shown on a project dashboard
// (spec section 37): progress, open/critical/completed tickets, SLA
// breaches, team members, milestones.
type ProjectStats struct {
	OpenTickets      int `json:"openTickets"`
	CriticalTickets  int `json:"criticalTickets"`
	CompletedTickets int `json:"completedTickets"`
	TotalTickets     int `json:"totalTickets"`
	MilestonesTotal  int `json:"milestonesTotal"`
	MilestonesDone   int `json:"milestonesDone"`
}

var openTicketStatuses = []string{
	"NEW", "ACKNOWLEDGED", "ASSIGNED", "IN_PROGRESS", "PENDING",
	"FOR_UAT", "UAT_FAILED", "UAT_PASSED", "FOR_PRE_PROD", "PRE_PROD_FAILED",
	"PRE_PROD_PASSED", "FOR_PRODUCTION", "PRODUCTION_FAILED",
}

func (r *Repository) GetProjectStats(projectID string) (*ProjectStats, error) {
	stats := &ProjectStats{}

	placeholders := ""
	args := []interface{}{projectID}
	for i, s := range openTicketStatuses {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}

	err := r.db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE project_id = ? AND status IN (`+placeholders+`)`, args...).Scan(&stats.OpenTickets)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE project_id = ? AND severity IN ('S1','S2') AND status IN (`+placeholders+`)`, args...).Scan(&stats.CriticalTickets)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE project_id = ? AND status IN ('RESOLVED','CLOSED')`, projectID).Scan(&stats.CompletedTickets)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE project_id = ?`, projectID).Scan(&stats.TotalTickets)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM milestones WHERE project_id = ?`, projectID).Scan(&stats.MilestonesTotal)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM milestones WHERE project_id = ? AND status = 'COMPLETED'`, projectID).Scan(&stats.MilestonesDone)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
