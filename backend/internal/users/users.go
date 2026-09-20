package users

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("user not found")
var ErrDuplicateEmail = errors.New("email already in use")
var ErrDuplicateUsername = errors.New("username already in use")

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	FullName     string    `json:"fullName"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	IsActive     bool      `json:"isActive"`
	Roles        []string  `json:"roles"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Role name constants (spec section 10).
const (
	RoleAdmin     = "ADMIN"
	RoleITManager = "IT_MANAGER"
	RoleTeamLead  = "TEAM_LEAD"
	RoleAgent     = "AGENT"
	RoleRequester = "REQUESTER"
	RoleViewer    = "VIEWER"
)

var roleIDByName = map[string]string{
	RoleAdmin:     "role-admin",
	RoleITManager: "role-it-manager",
	RoleTeamLead:  "role-team-lead",
	RoleAgent:     "role-agent",
	RoleRequester: "role-requester",
	RoleViewer:    "role-viewer",
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Count returns the total number of users. Used to detect first-run.
func (r *Repository) Count() (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

type CreateInput struct {
	Username string
	FullName string
	Email    string
	Password string
	Roles    []string
}

func (r *Repository) Create(in CreateInput, passwordHash string) (*User, error) {
	var exists int
	r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, in.Email).Scan(&exists)
	if exists > 0 {
		return nil, ErrDuplicateEmail
	}
	r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, in.Username).Scan(&exists)
	if exists > 0 {
		return nil, ErrDuplicateUsername
	}

	id := "usr-" + uuid.NewString()
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO users (id, username, full_name, email, password_hash, is_active, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		id, in.Username, in.FullName, in.Email, passwordHash, now, now,
	)
	if err != nil {
		return nil, err
	}

	roles := in.Roles
	if len(roles) == 0 {
		roles = []string{RoleRequester}
	}
	for _, roleName := range roles {
		roleID, ok := roleIDByName[roleName]
		if !ok {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)`, id, roleID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *Repository) scanUser(row *sql.Row) (*User, error) {
	var u User
	var createdAt, updatedAt string
	err := row.Scan(&u.ID, &u.Username, &u.FullName, &u.Email, &u.PasswordHash, &u.IsActive, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	u.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
	roles, err := r.rolesForUser(u.ID)
	if err != nil {
		return nil, err
	}
	u.Roles = roles
	return &u, nil
}

func (r *Repository) GetByID(id string) (*User, error) {
	row := r.db.QueryRow(`SELECT id, username, full_name, email, password_hash, is_active, created_at, updated_at FROM users WHERE id = ?`, id)
	return r.scanUser(row)
}

func (r *Repository) GetByUsername(username string) (*User, error) {
	row := r.db.QueryRow(`SELECT id, username, full_name, email, password_hash, is_active, created_at, updated_at FROM users WHERE username = ?`, username)
	return r.scanUser(row)
}

func (r *Repository) GetByEmail(email string) (*User, error) {
	row := r.db.QueryRow(`SELECT id, username, full_name, email, password_hash, is_active, created_at, updated_at FROM users WHERE email = ?`, email)
	return r.scanUser(row)
}

func (r *Repository) rolesForUser(userID string) ([]string, error) {
	rows, err := r.db.Query(`SELECT r.name FROM roles r JOIN user_roles ur ON ur.role_id = r.id WHERE ur.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, nil
}

func (r *Repository) List() ([]*User, error) {
	rows, err := r.db.Query(`SELECT id FROM users ORDER BY created_at ASC`)
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
	out := make([]*User, 0)
	for _, id := range ids {
		u, err := r.GetByID(id)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

type UpdateInput struct {
	FullName *string
	Email    *string
	IsActive *bool
	Roles    *[]string
}

func (r *Repository) Update(id string, in UpdateInput) (*User, error) {
	existing, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}

	fullName := existing.FullName
	if in.FullName != nil {
		fullName = *in.FullName
	}
	email := existing.Email
	if in.Email != nil {
		email = *in.Email
	}
	isActive := existing.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE users SET full_name = ?, email = ?, is_active = ?, updated_at = ? WHERE id = ?`,
		fullName, email, isActive, time.Now().UTC(), id,
	)
	if err != nil {
		return nil, err
	}

	if in.Roles != nil {
		if _, err := tx.Exec(`DELETE FROM user_roles WHERE user_id = ?`, id); err != nil {
			return nil, err
		}
		for _, roleName := range *in.Roles {
			roleID, ok := roleIDByName[roleName]
			if !ok {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES (?, ?)`, id, roleID); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *Repository) UpdatePassword(id, hash string) error {
	_, err := r.db.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, hash, time.Now().UTC(), id)
	return err
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
