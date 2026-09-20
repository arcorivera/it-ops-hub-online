package sla

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"itopshub/backend/internal/response"
)

type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// GetTicketSLA returns the live, pause-aware SLA status for a ticket.
func (h *Handler) GetTicketSLA(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := Compute(h.db, id)
	if err != nil {
		if err == ErrTicketNotFound {
			response.NotFound(w, "Ticket not found")
			return
		}
		response.Internal(w, h.logger, err, "sla.compute")
		return
	}
	response.JSON(w, http.StatusOK, result)
}

type Policy struct {
	ID                string `json:"id"`
	Severity          string `json:"severity"`
	Name              string `json:"name"`
	ResponseMinutes   int    `json:"responseMinutes"`
	ResolutionMinutes int    `json:"resolutionMinutes"`
	UseBusinessHours  bool   `json:"useBusinessHours"`
	IsActive          bool   `json:"isActive"`
}

// ListPolicies returns the configured SLA policies (spec section 28).
func (h *Handler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, severity, name, response_minutes, resolution_minutes, use_business_hours, is_active FROM sla_policies ORDER BY severity ASC`)
	if err != nil {
		response.Internal(w, h.logger, err, "sla.policies.list")
		return
	}
	defer rows.Close()

	out := make([]Policy, 0)
	for rows.Next() {
		var p Policy
		var useBH, isActive int
		if err := rows.Scan(&p.ID, &p.Severity, &p.Name, &p.ResponseMinutes, &p.ResolutionMinutes, &useBH, &isActive); err != nil {
			response.Internal(w, h.logger, err, "sla.policies.scan")
			return
		}
		p.UseBusinessHours = useBH != 0
		p.IsActive = isActive != 0
		out = append(out, p)
	}
	response.JSON(w, http.StatusOK, out)
}
