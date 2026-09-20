// Package settings implements generic application settings (spec section
// 50) as a real key-value store backed by the settings table.
package settings

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List() ([]*Setting, error) {
	rows, err := r.db.Query(`SELECT key, value, updated_at FROM settings ORDER BY key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Setting, 0)
	for rows.Next() {
		var s Setting
		var updatedAt string
		if err := rows.Scan(&s.Key, &s.Value, &updatedAt); err != nil {
			return nil, err
		}
		s.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
		out = append(out, &s)
	}
	return out, nil
}

func (r *Repository) Get(key string) (string, error) {
	var value string
	err := r.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (r *Repository) Set(key, value string) (*Setting, error) {
	now := time.Now().UTC()
	_, err := r.db.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now,
	)
	if err != nil {
		return nil, err
	}
	return &Setting{Key: key, Value: value, UpdatedAt: now}, nil
}

func (r *Repository) Delete(key string) error {
	_, err := r.db.Exec(`DELETE FROM settings WHERE key = ?`, key)
	return err
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
	audit  *audit.Logger
	logger *slog.Logger
}

func NewHandler(repo *Repository, auditLogger *audit.Logger, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, audit: auditLogger, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List()
	if err != nil {
		response.Internal(w, h.logger, err, "settings.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

type setRequest struct {
	Value string `json:"value"`
}

func (h *Handler) Set(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	var req setRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	s, err := h.repo.Set(key, req.Value)
	if err != nil {
		response.Internal(w, h.logger, err, "settings.set")
		return
	}
	h.audit.Log(reqctx.UserID(r), "UPDATE", "setting", key, map[string]interface{}{"key": key}, r.RemoteAddr)
	response.JSON(w, http.StatusOK, s)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if err := h.repo.Delete(key); err != nil {
		response.Internal(w, h.logger, err, "settings.delete")
		return
	}
	h.audit.Log(reqctx.UserID(r), "DELETE", "setting", key, nil, r.RemoteAddr)
	response.NoContent(w)
}
