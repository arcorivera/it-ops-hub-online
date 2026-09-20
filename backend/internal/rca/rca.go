// Package rca implements root cause analysis records (spec section 44),
// tied 1:1 to an incident, with a DRAFT -> IN_REVIEW -> COMPLETED workflow.
package rca

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

var ErrNotFound = errors.New("RCA record not found")

const (
	StatusDraft     = "DRAFT"
	StatusInReview  = "IN_REVIEW"
	StatusCompleted = "COMPLETED"
)

var ValidStatuses = map[string]bool{StatusDraft: true, StatusInReview: true, StatusCompleted: true}

type RCA struct {
	ID                  string    `json:"id"`
	IncidentID          string    `json:"incidentId"`
	IncidentSummary     string    `json:"incidentSummary"`
	BusinessImpact      string    `json:"businessImpact"`
	RootCause           string    `json:"rootCause"`
	ContributingFactors string    `json:"contributingFactors"`
	Timeline            string    `json:"timeline"`
	ImmediateFix        string    `json:"immediateFix"`
	PermanentFix        string    `json:"permanentFix"`
	PreventiveAction    string    `json:"preventiveAction"`
	ResponsibleTeamID   *string   `json:"responsibleTeamId"`
	ResponsibleUserID   *string   `json:"responsibleUserId"`
	DeploymentReference string    `json:"deploymentReference"`
	Status              string    `json:"status"`
	OverrideBy          *string   `json:"overrideBy"`
	OverrideReason      string    `json:"overrideReason"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// IsRCACompleted implements incidents.RCAGateChecker.
func (r *Repository) IsRCACompleted(incidentID string) (bool, error) {
	var status sql.NullString
	err := r.db.QueryRow(`SELECT status FROM rca_records WHERE incident_id = ?`, incidentID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status.String == StatusCompleted, nil
}

type UpsertInput struct {
	IncidentSummary     string
	BusinessImpact      string
	RootCause           string
	ContributingFactors string
	Timeline            string
	ImmediateFix        string
	PermanentFix        string
	PreventiveAction    string
	ResponsibleTeamID   *string
	ResponsibleUserID   *string
	DeploymentReference string
}

// GetOrCreate returns the RCA record for an incident, creating an empty
// DRAFT one if none exists yet — an incident always has at most one RCA.
func (r *Repository) GetOrCreate(incidentID string) (*RCA, error) {
	existing, err := r.GetByIncidentID(incidentID)
	if err == nil {
		return existing, nil
	}
	if err != ErrNotFound {
		return nil, err
	}

	id := "rca-" + uuid.NewString()
	now := time.Now().UTC()
	_, err = r.db.Exec(
		`INSERT INTO rca_records (id, incident_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, incidentID, StatusDraft, now, now,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByIncidentID(incidentID)
}

func (r *Repository) GetByIncidentID(incidentID string) (*RCA, error) {
	row := r.db.QueryRow(`
		SELECT id, incident_id, incident_summary, business_impact, root_cause, contributing_factors, timeline,
		       immediate_fix, permanent_fix, preventive_action, responsible_team_id, responsible_user_id,
		       deployment_reference, status, override_by, override_reason, created_at, updated_at
		FROM rca_records WHERE incident_id = ?`, incidentID)
	return scanRCA(row)
}

func scanRCA(row *sql.Row) (*RCA, error) {
	var rc RCA
	var teamID, userID, overrideBy sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&rc.ID, &rc.IncidentID, &rc.IncidentSummary, &rc.BusinessImpact, &rc.RootCause, &rc.ContributingFactors,
		&rc.Timeline, &rc.ImmediateFix, &rc.PermanentFix, &rc.PreventiveAction, &teamID, &userID,
		&rc.DeploymentReference, &rc.Status, &overrideBy, &rc.OverrideReason, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if teamID.Valid {
		rc.ResponsibleTeamID = &teamID.String
	}
	if userID.Valid {
		rc.ResponsibleUserID = &userID.String
	}
	if overrideBy.Valid {
		rc.OverrideBy = &overrideBy.String
	}
	rc.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	rc.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
	return &rc, nil
}

func (r *Repository) Update(incidentID string, in UpsertInput) (*RCA, error) {
	if _, err := r.GetOrCreate(incidentID); err != nil {
		return nil, err
	}
	_, err := r.db.Exec(
		`UPDATE rca_records SET incident_summary=?, business_impact=?, root_cause=?, contributing_factors=?, timeline=?,
		 immediate_fix=?, permanent_fix=?, preventive_action=?, responsible_team_id=?, responsible_user_id=?,
		 deployment_reference=?, updated_at=? WHERE incident_id=?`,
		in.IncidentSummary, in.BusinessImpact, in.RootCause, in.ContributingFactors, in.Timeline,
		in.ImmediateFix, in.PermanentFix, in.PreventiveAction, in.ResponsibleTeamID, in.ResponsibleUserID,
		in.DeploymentReference, time.Now().UTC(), incidentID,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByIncidentID(incidentID)
}

// UpdateStatus moves the RCA through DRAFT -> IN_REVIEW -> COMPLETED. A
// direct jump to COMPLETED is intentionally allowed (small teams often skip
// a formal review step) but the fields should be substantively filled in —
// that's enforced by requiring root_cause and permanent_fix to be non-empty
// before completion, a lightweight content gate on top of the status gate.
func (r *Repository) UpdateStatus(incidentID, status string) (*RCA, error) {
	rc, err := r.GetOrCreate(incidentID)
	if err != nil {
		return nil, err
	}
	if status == StatusCompleted && (rc.RootCause == "" || rc.PermanentFix == "") {
		return nil, errors.New("root cause and permanent fix must be filled in before marking RCA complete")
	}
	_, err = r.db.Exec(`UPDATE rca_records SET status = ?, updated_at = ? WHERE incident_id = ?`, status, time.Now().UTC(), incidentID)
	if err != nil {
		return nil, err
	}
	return r.GetByIncidentID(incidentID)
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

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "id")
	rc, err := h.repo.GetOrCreate(incidentID)
	if err != nil {
		response.Internal(w, h.logger, err, "rca.get")
		return
	}
	response.JSON(w, http.StatusOK, rc)
}

type updateRequest struct {
	IncidentSummary     string  `json:"incidentSummary"`
	BusinessImpact      string  `json:"businessImpact"`
	RootCause           string  `json:"rootCause"`
	ContributingFactors string  `json:"contributingFactors"`
	Timeline            string  `json:"timeline"`
	ImmediateFix        string  `json:"immediateFix"`
	PermanentFix        string  `json:"permanentFix"`
	PreventiveAction    string  `json:"preventiveAction"`
	ResponsibleTeamID   *string `json:"responsibleTeamId"`
	ResponsibleUserID   *string `json:"responsibleUserId"`
	DeploymentReference string  `json:"deploymentReference"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "id")
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	rc, err := h.repo.Update(incidentID, UpsertInput{
		IncidentSummary: req.IncidentSummary, BusinessImpact: req.BusinessImpact, RootCause: req.RootCause,
		ContributingFactors: req.ContributingFactors, Timeline: req.Timeline, ImmediateFix: req.ImmediateFix,
		PermanentFix: req.PermanentFix, PreventiveAction: req.PreventiveAction,
		ResponsibleTeamID: req.ResponsibleTeamID, ResponsibleUserID: req.ResponsibleUserID,
		DeploymentReference: req.DeploymentReference,
	})
	if err != nil {
		response.Internal(w, h.logger, err, "rca.update")
		return
	}
	h.audit.Log(reqctx.UserID(r), "UPDATE", "rca", rc.ID, nil, r.RemoteAddr)
	response.JSON(w, http.StatusOK, rc)
}

type statusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "id")
	var req statusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !ValidStatuses[req.Status] {
		response.BadRequest(w, "Invalid RCA status", nil)
		return
	}
	rc, err := h.repo.UpdateStatus(incidentID, req.Status)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}
	h.audit.Log(reqctx.UserID(r), "UPDATE", "rca_status", rc.ID, map[string]interface{}{"status": req.Status}, r.RemoteAddr)
	response.JSON(w, http.StatusOK, rc)
}
