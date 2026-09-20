// Package savedviews implements user-saved ticket filters (spec section 47).
// System default views are seeded via migration; users can add their own.
package savedviews

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

var ErrNotFound = errors.New("saved view not found")

type SavedView struct {
	ID        string          `json:"id"`
	UserID    *string         `json:"userId"`
	Name      string          `json:"name"`
	Filters   json.RawMessage `json:"filters"`
	IsSystem  bool            `json:"isSystem"`
	CreatedAt time.Time       `json:"createdAt"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// List returns every system default view plus the given user's own views.
func (r *Repository) List(userID string) ([]*SavedView, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, name, filters, is_system, created_at FROM saved_views
		 WHERE is_system = 1 OR user_id = ? ORDER BY is_system DESC, created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*SavedView, 0)
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func scan(rows *sql.Rows) (*SavedView, error) {
	var v SavedView
	var userID sql.NullString
	var filtersRaw string
	var isSystem int
	var createdAt string
	if err := rows.Scan(&v.ID, &userID, &v.Name, &filtersRaw, &isSystem, &createdAt); err != nil {
		return nil, err
	}
	if userID.Valid {
		v.UserID = &userID.String
	}
	v.Filters = json.RawMessage(filtersRaw)
	v.IsSystem = isSystem != 0
	v.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	return &v, nil
}

func (r *Repository) Create(userID, name string, filters json.RawMessage) (*SavedView, error) {
	id := "view-" + uuid.NewString()
	now := time.Now().UTC()
	filterStr := string(filters)
	if filterStr == "" {
		filterStr = "{}"
	}
	_, err := r.db.Exec(
		`INSERT INTO saved_views (id, user_id, name, filters, is_system, created_at) VALUES (?, ?, ?, ?, 0, ?)`,
		id, userID, name, filterStr, now,
	)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRow(`SELECT id, user_id, name, filters, is_system, created_at FROM saved_views WHERE id = ?`, id)
	var v SavedView
	var uid sql.NullString
	var filtersRaw string
	var isSystem int
	var createdAt string
	if err := row.Scan(&v.ID, &uid, &v.Name, &filtersRaw, &isSystem, &createdAt); err != nil {
		return nil, err
	}
	if uid.Valid {
		v.UserID = &uid.String
	}
	v.Filters = json.RawMessage(filtersRaw)
	v.IsSystem = isSystem != 0
	v.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	return &v, nil
}

// Delete removes a view, but only if it belongs to the requesting user and
// isn't a system default (those are never user-deletable).
func (r *Repository) Delete(id, userID string) error {
	res, err := r.db.Exec(`DELETE FROM saved_views WHERE id = ? AND user_id = ? AND is_system = 0`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}

// --- HTTP handlers ---

type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(reqctx.UserID(r))
	if err != nil {
		response.Internal(w, h.logger, err, "savedviews.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

type createRequest struct {
	Name    string          `json:"name"`
	Filters json.RawMessage `json:"filters"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		response.BadRequest(w, "View name is required", nil)
		return
	}
	v, err := h.repo.Create(reqctx.UserID(r), req.Name, req.Filters)
	if err != nil {
		response.Internal(w, h.logger, err, "savedviews.create")
		return
	}
	response.Created(w, v)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(id, reqctx.UserID(r)); err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Saved view not found or not deletable")
			return
		}
		response.Internal(w, h.logger, err, "savedviews.delete")
		return
	}
	response.NoContent(w)
}
