package testing

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/notifications"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

type Handler struct {
	repo   *Repository
	audit  *audit.Logger
	notif  *notifications.Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, auditLogger *audit.Logger, notif *notifications.Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, audit: auditLogger, notif: notif, logger: logger}
}

// --- Test cases ---

func (h *Handler) ListTestCases(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListTestCases(chi.URLParam(r, "id"))
	if err != nil {
		response.Internal(w, h.logger, err, "testing.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

type createTestCaseRequest struct {
	Description    string `json:"description"`
	Environment    string `json:"environment"`
	Preconditions  string `json:"preconditions"`
	ExpectedResult string `json:"expectedResult"`
}

func (h *Handler) CreateTestCase(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")
	var req createTestCaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		response.BadRequest(w, "Test case description is required", nil)
		return
	}
	if req.Environment == "" {
		req.Environment = "UAT"
	}
	tc, err := h.repo.CreateTestCase(CreateTestCaseInput{
		TicketID: ticketID, Description: req.Description, Environment: req.Environment,
		Preconditions: req.Preconditions, ExpectedResult: req.ExpectedResult,
	})
	if err != nil {
		response.Internal(w, h.logger, err, "testing.create")
		return
	}
	h.audit.Log(reqctx.UserID(r), "CREATE", "test_case", tc.ID, nil, r.RemoteAddr)
	response.Created(w, tc)
}

type executeRequest struct {
	Status       string `json:"status"`
	ActualResult string `json:"actualResult"`
	Comment      string `json:"comment"`
}

// Execute handles Pass/Fail/Block actions (spec section 39).
func (h *Handler) Execute(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	var req executeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if !ValidTestStatuses[req.Status] || req.Status == TestNotStarted {
		response.BadRequest(w, "Status must be PASS, FAIL, or BLOCKED", nil)
		return
	}

	tc, err := h.repo.Execute(caseID, reqctx.UserID(r), req.Status, req.ActualResult, req.Comment)
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Test case not found")
			return
		}
		response.Internal(w, h.logger, err, "testing.execute")
		return
	}

	if req.Status == TestFail {
		assigneeID, number, title := h.repo.TicketInfoForNotify(tc.TicketID)
		if assigneeID != "" {
			h.notif.Create(assigneeID, notifications.TypeUATFailed, "UAT test failed: "+number, title, "ticket", tc.TicketID)
		}
	}

	h.audit.Log(reqctx.UserID(r), "UPDATE", "test_case", caseID, map[string]interface{}{"status": req.Status}, r.RemoteAddr)
	response.JSON(w, http.StatusOK, tc)
}

// --- Deployments ---

func (h *Handler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListDeployments(chi.URLParam(r, "id"))
	if err != nil {
		response.Internal(w, h.logger, err, "deployments.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

type createDeploymentRequest struct {
	Stage               string `json:"stage"`
	Version             string `json:"version"`
	DeploymentReference string `json:"deploymentReference"`
	ValidationResult    string `json:"validationResult"`
}

func (h *Handler) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")
	var req createDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.Stage != StagePreProd && req.Stage != StageProduction {
		response.BadRequest(w, "Stage must be PRE_PROD or PRODUCTION", nil)
		return
	}

	d, err := h.repo.CreateDeployment(CreateDeploymentInput{
		TicketID: ticketID, Stage: req.Stage, Version: req.Version,
		DeploymentReference: req.DeploymentReference, DeployedBy: reqctx.UserID(r), ValidationResult: req.ValidationResult,
	})
	if err != nil {
		response.Internal(w, h.logger, err, "deployments.create")
		return
	}

	notifType := notifications.TypePreProdPassed
	if req.Stage == StageProduction {
		notifType = notifications.TypeProductionDeployment
	}
	assigneeID, number, _ := h.repo.TicketInfoForNotify(ticketID)
	if assigneeID != "" {
		h.notif.Create(assigneeID, notifType, "Deployment recorded: "+number, req.Stage+" v"+req.Version, "ticket", ticketID)
	}

	h.audit.Log(reqctx.UserID(r), "CREATE", "deployment", d.ID, map[string]interface{}{"stage": req.Stage, "version": req.Version}, r.RemoteAddr)
	response.Created(w, d)
}

type updateValidationRequest struct {
	ValidationResult string `json:"validationResult"`
	RollbackReason   string `json:"rollbackReason"`
}

func (h *Handler) UpdateDeploymentValidation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "deploymentId")
	var req updateValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.ValidationResult != ValidationPassed && req.ValidationResult != ValidationFailed && req.ValidationResult != ValidationPending {
		response.BadRequest(w, "Invalid validation result", nil)
		return
	}
	d, err := h.repo.UpdateDeploymentValidation(id, req.ValidationResult, req.RollbackReason)
	if err != nil {
		response.Internal(w, h.logger, err, "deployments.validation")
		return
	}
	h.audit.Log(reqctx.UserID(r), "UPDATE", "deployment", id, map[string]interface{}{"validation": req.ValidationResult}, r.RemoteAddr)
	response.JSON(w, http.StatusOK, d)
}
