package projects

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
	status := r.URL.Query().Get("status")
	list, err := h.repo.ListProjects(status)
	if err != nil {
		response.Internal(w, h.logger, err, "projects.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p, err := h.repo.GetProject(chi.URLParam(r, "id"))
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Project not found")
			return
		}
		response.Internal(w, h.logger, err, "projects.get")
		return
	}
	response.JSON(w, http.StatusOK, p)
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.repo.GetProjectStats(chi.URLParam(r, "id"))
	if err != nil {
		response.Internal(w, h.logger, err, "projects.stats")
		return
	}
	response.JSON(w, http.StatusOK, stats)
}

type createRequest struct {
	ProjectKey  string  `json:"projectKey"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	OwnerID     *string `json:"ownerId"`
	StartDate   *string `json:"startDate"`
	TargetDate  *string `json:"targetDate"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	errs := map[string]string{}
	if req.ProjectKey == "" {
		errs["projectKey"] = "Project key is required"
	}
	if req.Name == "" {
		errs["name"] = "Project name is required"
	}
	if len(errs) > 0 {
		response.BadRequest(w, "Invalid project data", errs)
		return
	}

	p, err := h.repo.CreateProject(CreateProjectInput{
		ProjectKey: req.ProjectKey, Name: req.Name, Description: req.Description,
		OwnerID: req.OwnerID, StartDate: req.StartDate, TargetDate: req.TargetDate,
	})
	if err != nil {
		if err == ErrDuplicateKey {
			response.Conflict(w, "A project with this key already exists")
			return
		}
		response.Internal(w, h.logger, err, "projects.create")
		return
	}
	h.audit.Log(reqctx.UserID(r), "CREATE", "project", p.ID, map[string]interface{}{"key": p.ProjectKey}, r.RemoteAddr)
	response.Created(w, p)
}

type updateRequest struct {
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	OwnerID     *string   `json:"ownerId"`
	Status      *string   `json:"status"`
	StartDate   *string   `json:"startDate"`
	TargetDate  *string   `json:"targetDate"`
	MemberIDs   *[]string `json:"memberIds"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.Status != nil && !ValidProjectStatuses[*req.Status] {
		response.BadRequest(w, "Invalid status", map[string]string{"status": "Unknown status value"})
		return
	}
	p, err := h.repo.UpdateProject(id, UpdateProjectInput{
		Name: req.Name, Description: req.Description, OwnerID: req.OwnerID,
		Status: req.Status, StartDate: req.StartDate, TargetDate: req.TargetDate, MemberIDs: req.MemberIDs,
	})
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Project not found")
			return
		}
		response.Internal(w, h.logger, err, "projects.update")
		return
	}
	h.audit.Log(reqctx.UserID(r), "UPDATE", "project", id, nil, r.RemoteAddr)
	response.JSON(w, http.StatusOK, p)
}

// --- Milestones ---

func (h *Handler) ListMilestones(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListMilestones(chi.URLParam(r, "id"))
	if err != nil {
		response.Internal(w, h.logger, err, "projects.milestones.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

type createMilestoneRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	DueDate     *string `json:"dueDate"`
}

func (h *Handler) CreateMilestone(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	var req createMilestoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		response.BadRequest(w, "Milestone name is required", nil)
		return
	}
	m, err := h.repo.CreateMilestone(CreateMilestoneInput{
		ProjectID: projectID, Name: req.Name, Description: req.Description, DueDate: req.DueDate,
	})
	if err != nil {
		response.Internal(w, h.logger, err, "projects.milestones.create")
		return
	}
	h.audit.Log(reqctx.UserID(r), "CREATE", "milestone", m.ID, nil, r.RemoteAddr)
	response.Created(w, m)
}

type updateMilestoneRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	DueDate     *string `json:"dueDate"`
	Status      *string `json:"status"`
}

func (h *Handler) UpdateMilestone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "milestoneId")
	var req updateMilestoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	m, err := h.repo.UpdateMilestone(id, UpdateMilestoneInput{
		Name: req.Name, Description: req.Description, DueDate: req.DueDate, Status: req.Status,
	})
	if err != nil {
		if err == ErrMilestoneNotFound {
			response.NotFound(w, "Milestone not found")
			return
		}
		response.Internal(w, h.logger, err, "projects.milestones.update")
		return
	}
	h.audit.Log(reqctx.UserID(r), "UPDATE", "milestone", id, nil, r.RemoteAddr)
	response.JSON(w, http.StatusOK, m)
}
