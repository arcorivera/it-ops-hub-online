package incidents

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
	"itopshub/backend/internal/users"
)

type Handler struct {
	repo       *Repository
	audit      *audit.Logger
	logger     *slog.Logger
	userRepo   *users.Repository
	rcaChecker RCAGateChecker
}

func NewHandler(repo *Repository, auditLogger *audit.Logger, logger *slog.Logger, userRepo *users.Repository) *Handler {
	return &Handler{repo: repo, audit: auditLogger, logger: logger, userRepo: userRepo}
}

func (h *Handler) SetRCAChecker(checker RCAGateChecker) {
	h.rcaChecker = checker
}

func (h *Handler) isAdmin(r *http.Request) bool {
	u, err := h.userRepo.GetByID(reqctx.UserID(r))
	if err != nil {
		return false
	}
	for _, role := range u.Roles {
		if role == users.RoleAdmin {
			return true
		}
	}
	return false
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := h.repo.List(q.Get("status"), q.Get("severity"))
	if err != nil {
		response.Internal(w, h.logger, err, "incidents.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	inc, err := h.repo.GetByID(chi.URLParam(r, "id"))
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Incident not found")
			return
		}
		response.Internal(w, h.logger, err, "incidents.get")
		return
	}
	response.JSON(w, http.StatusOK, inc)
}

type createRequest struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Severity       string  `json:"severity"`
	Impact         string  `json:"impact"`
	AffectedSystem string  `json:"affectedSystem"`
	Environment    string  `json:"environment"`
	OwnerID        *string `json:"ownerId"`
	TicketID       *string `json:"ticketId"`
}

var validSeverities = map[string]bool{"S1": true, "S2": true, "S3": true, "S4": true}
var validEnvironments = map[string]bool{"DEV": true, "UAT": true, "PRE-PROD": true, "PRODUCTION": true}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	errs := map[string]string{}
	if req.Title == "" {
		errs["title"] = "Title is required"
	}
	if req.Severity == "" {
		req.Severity = "S3"
	} else if !validSeverities[req.Severity] {
		errs["severity"] = "Invalid severity"
	}
	if req.Environment == "" {
		req.Environment = "PRODUCTION"
	} else if !validEnvironments[req.Environment] {
		errs["environment"] = "Invalid environment"
	}
	if len(errs) > 0 {
		response.BadRequest(w, "Invalid incident data", errs)
		return
	}

	inc, err := h.repo.Create(CreateInput{
		Title: req.Title, Description: req.Description, Severity: req.Severity, Impact: req.Impact,
		AffectedSystem: req.AffectedSystem, Environment: req.Environment, OwnerID: req.OwnerID, TicketID: req.TicketID,
	})
	if err != nil {
		response.Internal(w, h.logger, err, "incidents.create")
		return
	}
	h.audit.Log(reqctx.UserID(r), "CREATE", "incident", inc.ID, map[string]interface{}{"number": inc.IncidentNumber}, r.RemoteAddr)
	response.Created(w, inc)
}

type statusRequest struct {
	Status string `json:"status"`
	Force  bool   `json:"force"`
}

func (h *Handler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req statusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if !ValidStatuses[req.Status] {
		response.BadRequest(w, "Invalid status", nil)
		return
	}

	force := req.Force && h.isAdmin(r)
	inc, err := h.repo.UpdateStatus(id, req.Status, force, h.rcaChecker)
	if err != nil {
		switch err {
		case ErrNotFound:
			response.NotFound(w, "Incident not found")
		case ErrInvalidTransition:
			response.BadRequest(w, "That status transition is not allowed from the incident's current status", nil)
		case ErrRCARequired:
			response.BadRequest(w, "This is an S1/S2 incident and requires a completed RCA before it can be closed. An admin can override this.", nil)
		default:
			response.Internal(w, h.logger, err, "incidents.status")
		}
		return
	}

	action := "STATUS_CHANGE"
	if force {
		action = "STATUS_CHANGE_OVERRIDE"
	}
	h.audit.Log(reqctx.UserID(r), action, "incident", id, map[string]interface{}{"newStatus": req.Status}, r.RemoteAddr)
	response.JSON(w, http.StatusOK, inc)
}
