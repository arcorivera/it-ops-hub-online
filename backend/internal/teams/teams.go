package teams

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("team not found")
var ErrDuplicateName = errors.New("team name already in use")

type Team struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MemberIDs   []string  `json:"memberIds"`
	LeadIDs     []string  `json:"leadIds"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateInput struct {
	Name        string
	Description string
}

func (r *Repository) Create(in CreateInput) (*Team, error) {
	var exists int
	r.db.QueryRow(`SELECT COUNT(*) FROM teams WHERE name = ?`, in.Name).Scan(&exists)
	if exists > 0 {
		return nil, ErrDuplicateName
	}

	id := "team-" + uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.Exec(
		`INSERT INTO teams (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, in.Name, in.Description, now, now,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *Repository) GetByID(id string) (*Team, error) {
	row := r.db.QueryRow(`SELECT id, name, description, created_at, updated_at FROM teams WHERE id = ?`, id)
	var t Team
	var createdAt, updatedAt string
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	t.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))

	members, leads, err := r.membersForTeam(id)
	if err != nil {
		return nil, err
	}
	t.MemberIDs = members
	t.LeadIDs = leads
	return &t, nil
}

func (r *Repository) membersForTeam(teamID string) ([]string, []string, error) {
	rows, err := r.db.Query(`SELECT user_id, is_lead FROM team_members WHERE team_id = ?`, teamID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var members, leads []string
	for rows.Next() {
		var uid string
		var isLead bool
		if err := rows.Scan(&uid, &isLead); err != nil {
			return nil, nil, err
		}
		members = append(members, uid)
		if isLead {
			leads = append(leads, uid)
		}
	}
	return members, leads, nil
}

func (r *Repository) List() ([]*Team, error) {
	rows, err := r.db.Query(`SELECT id FROM teams ORDER BY name ASC`)
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
	out := make([]*Team, 0)
	for _, id := range ids {
		t, err := r.GetByID(id)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

type UpdateInput struct {
	Name        *string
	Description *string
}

func (r *Repository) Update(id string, in UpdateInput) (*Team, error) {
	existing, err := r.GetByID(id)
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
	_, err = r.db.Exec(`UPDATE teams SET name = ?, description = ?, updated_at = ? WHERE id = ?`, name, desc, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *Repository) SetMembers(teamID string, memberIDs []string, leadIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM team_members WHERE team_id = ?`, teamID); err != nil {
		return err
	}
	leadSet := map[string]bool{}
	for _, l := range leadIDs {
		leadSet[l] = true
	}
	for _, m := range memberIDs {
		isLead := leadSet[m]
		if _, err := tx.Exec(`INSERT INTO team_members (team_id, user_id, is_lead) VALUES (?, ?, ?)`, teamID, m, isLead); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
