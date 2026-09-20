package audit

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Logger struct {
	db *sql.DB
}

func NewLogger(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Log records an audit entry. userID may be empty for system-initiated
// actions. details is marshaled to JSON; callers must never pass password
// fields (spec section 51).
func (l *Logger) Log(userID, action, entityType, entityID string, details map[string]interface{}, ip string) {
	if details == nil {
		details = map[string]interface{}{}
	}
	b, _ := json.Marshal(details)

	var uid interface{}
	if userID != "" {
		uid = userID
	}

	_, _ = l.db.Exec(
		`INSERT INTO audit_logs (id, user_id, action, entity_type, entity_id, details, ip_address, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"audit-"+uuid.NewString(), uid, action, entityType, entityID, string(b), ip, time.Now().UTC(),
	)
}

type Entry struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"userId,omitempty"`
	Action     string                 `json:"action"`
	EntityType string                 `json:"entityType"`
	EntityID   string                 `json:"entityId"`
	Details    map[string]interface{} `json:"details"`
	IPAddress  string                 `json:"ipAddress"`
	CreatedAt  time.Time              `json:"createdAt"`
}

type ListFilter struct {
	Action     string
	EntityType string
	UserID     string
	Limit      int
	Offset     int
}

func (l *Logger) List(f ListFilter) ([]Entry, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	if f.Action != "" {
		where += " AND action = ?"
		args = append(args, f.Action)
	}
	if f.EntityType != "" {
		where += " AND entity_type = ?"
		args = append(args, f.EntityType)
	}
	if f.UserID != "" {
		where += " AND user_id = ?"
		args = append(args, f.UserID)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs " + where
	if err := l.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := "SELECT id, user_id, action, entity_type, entity_id, details, ip_address, created_at FROM audit_logs " + where + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, f.Offset)

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		var userID sql.NullString
		var detailsRaw, createdAt string
		if err := rows.Scan(&e.ID, &userID, &e.Action, &e.EntityType, &e.EntityID, &detailsRaw, &e.IPAddress, &createdAt); err != nil {
			return nil, 0, err
		}
		e.UserID = userID.String
		_ = json.Unmarshal([]byte(detailsRaw), &e.Details)
		e.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		out = append(out, e)
	}
	return out, total, nil
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
