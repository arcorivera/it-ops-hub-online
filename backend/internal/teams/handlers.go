package teams

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

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
		response.Internal(w, h.logger, err, "teams.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	t, err := h.repo.GetByID(chi.URLParam(r, "id"))
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Team not found")
			return
		}
		response.Internal(w, h.logger, err, "teams.get")
		return
	}
	response.JSON(w, http.StatusOK, t)
}

type createRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.Name == "" {
		response.BadRequest(w, "Team name is required", map[string]string{"name": "Required"})
		return
	}
	t, err := h.repo.Create(CreateInput{Name: req.Name, Description: req.Description})
	if err != nil {
		if err == ErrDuplicateName {
			response.Conflict(w, "A team with this name already exists")
			return
		}
		response.Internal(w, h.logger, err, "teams.create")
		return
	}
	h.audit.Log(reqctx.UserID(r), "CREATE", "team", t.ID, map[string]interface{}{"name": t.Name}, r.RemoteAddr)
	response.Created(w, t)
}

type updateRequest struct {
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	MemberIDs   []string `json:"memberIds"`
	LeadIDs     []string `json:"leadIds"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	t, err := h.repo.Update(id, UpdateInput{Name: req.Name, Description: req.Description})
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Team not found")
			return
		}
		response.Internal(w, h.logger, err, "teams.update")
		return
	}
	if req.MemberIDs != nil {
		if err := h.repo.SetMembers(id, req.MemberIDs, req.LeadIDs); err != nil {
			response.Internal(w, h.logger, err, "teams.setmembers")
			return
		}
		t, _ = h.repo.GetByID(id)
	}
	h.audit.Log(reqctx.UserID(r), "UPDATE", "team", id, nil, r.RemoteAddr)
	response.JSON(w, http.StatusOK, t)
}
