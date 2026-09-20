package categories

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

var ErrNotFound = errors.New("category not found")
var ErrDuplicateName = errors.New("category name already in use")

type Category struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(name string, parentID *string) (*Category, error) {
	var exists int
	r.db.QueryRow(`SELECT COUNT(*) FROM ticket_categories WHERE name = ?`, name).Scan(&exists)
	if exists > 0 {
		return nil, ErrDuplicateName
	}
	id := "cat-" + uuid.NewString()
	_, err := r.db.Exec(`INSERT INTO ticket_categories (id, name, parent_id, created_at) VALUES (?, ?, ?, ?)`, id, name, parentID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return r.GetByID(id)
}

func (r *Repository) GetByID(id string) (*Category, error) {
	row := r.db.QueryRow(`SELECT id, name, parent_id FROM ticket_categories WHERE id = ?`, id)
	var c Category
	var parentID sql.NullString
	if err := row.Scan(&c.ID, &c.Name, &parentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if parentID.Valid {
		c.ParentID = &parentID.String
	}
	return &c, nil
}

func (r *Repository) List() ([]*Category, error) {
	rows, err := r.db.Query(`SELECT id, name, parent_id FROM ticket_categories ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*Category, 0)
	for rows.Next() {
		var c Category
		var parentID sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &parentID); err != nil {
			return nil, err
		}
		if parentID.Valid {
			c.ParentID = &parentID.String
		}
		out = append(out, &c)
	}
	return out, nil
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
		response.Internal(w, h.logger, err, "categories.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

type createRequest struct {
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.Name == "" {
		response.BadRequest(w, "Category name is required", map[string]string{"name": "Required"})
		return
	}
	c, err := h.repo.Create(req.Name, req.ParentID)
	if err != nil {
		if err == ErrDuplicateName {
			response.Conflict(w, "A category with this name already exists")
			return
		}
		response.Internal(w, h.logger, err, "categories.create")
		return
	}
	h.audit.Log(reqctx.UserID(r), "CREATE", "category", c.ID, map[string]interface{}{"name": c.Name}, r.RemoteAddr)
	response.Created(w, c)
}
