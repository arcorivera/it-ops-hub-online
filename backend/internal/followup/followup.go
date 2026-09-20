// Package followup implements the follow-up engine (spec sections 32-33):
// a background pass that checks every open ticket against its severity's
// follow-up interval and raises a real follow-up record + notification when
// one is due.
package followup

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/notifications"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

// openStatuses are ticket statuses considered "active" for follow-up and
// escalation purposes — anything not resolved/closed/cancelled.
var openStatuses = []string{
	"NEW", "ACKNOWLEDGED", "ASSIGNED", "IN_PROGRESS", "PENDING",
	"FOR_UAT", "UAT_FAILED", "UAT_PASSED", "FOR_PRE_PROD", "PRE_PROD_FAILED",
	"PRE_PROD_PASSED", "FOR_PRODUCTION", "PRODUCTION_FAILED",
}

type FollowUp struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticketId"`
	CreatedBy *string   `json:"createdBy"`
	Reason    string    `json:"reason"`
	IsSystem  bool      `json:"isSystem"`
	Resolved  bool      `json:"resolved"`
	CreatedAt time.Time `json:"createdAt"`
}

type Engine struct {
	db     *sql.DB
	notif  *notifications.Repository
	logger *slog.Logger
}

func NewEngine(db *sql.DB, notif *notifications.Repository, logger *slog.Logger) *Engine {
	return &Engine{db: db, notif: notif, logger: logger}
}

// RunPass checks every open ticket and, if its follow-up interval has
// elapsed since the last follow-up (or since creation, if none yet), raises
// a system follow-up record and notifies the assignee (or requester if
// unassigned). Returns the number of follow-ups raised.
func (e *Engine) RunPass() (int, error) {
	rows, err := e.db.Query(`
		SELECT t.id, t.ticket_number, t.title, t.severity, t.status, t.created_at,
		       t.assignee_id, t.requester_id, t.last_followup_at,
		       r.interval_minutes
		FROM tickets t
		JOIN follow_up_rules r ON r.severity = t.severity AND r.is_active = 1
		WHERE t.status IN (`+placeholders(len(openStatuses))+`)
	`, toArgs(openStatuses)...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type candidate struct {
		id, number, title, severity, status   string
		createdAt                             time.Time
		assigneeID, requesterID, lastFollowup *string
		intervalMinutes                       int
	}

	var candidates []candidate
	for rows.Next() {
		var c candidate
		var createdAt string
		var assigneeID, requesterID, lastFollowup sql.NullString
		if err := rows.Scan(&c.id, &c.number, &c.title, &c.severity, &c.status, &createdAt,
			&assigneeID, &requesterID, &lastFollowup, &c.intervalMinutes); err != nil {
			return 0, err
		}
		c.createdAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		if assigneeID.Valid {
			c.assigneeID = &assigneeID.String
		}
		if requesterID.Valid {
			c.requesterID = &requesterID.String
		}
		if lastFollowup.Valid && lastFollowup.String != "" {
			c.lastFollowup = &lastFollowup.String
		}
		candidates = append(candidates, c)
	}
	rows.Close()

	now := time.Now().UTC()
	raised := 0

	for _, c := range candidates {
		// PENDING tickets have their SLA (and by extension, follow-up cadence)
		// paused — skip them the same way the SLA engine does.
		if c.status == "PENDING" {
			continue
		}

		var since time.Time
		if c.lastFollowup != nil {
			since, _ = time.Parse(time.RFC3339, normalizeTime(*c.lastFollowup))
		} else {
			since = c.createdAt
		}

		due := since.Add(time.Duration(c.intervalMinutes) * time.Minute)
		if now.Before(due) {
			continue
		}

		reason := "Automatic follow-up: no update within the " + c.severity + " follow-up interval"
		if _, err := e.db.Exec(
			`INSERT INTO follow_ups (id, ticket_id, created_by, reason, is_system, resolved, created_at)
			 VALUES (?, ?, NULL, ?, 1, 0, ?)`,
			"fu-"+uuid.NewString(), c.id, reason, now,
		); err != nil {
			if e.logger != nil {
				e.logger.Error("followup insert failed", "ticket", c.id, "error", err)
			}
			continue
		}

		if _, err := e.db.Exec(
			`UPDATE tickets SET last_followup_at = ?, next_followup_at = ? WHERE id = ?`,
			now, now.Add(time.Duration(c.intervalMinutes)*time.Minute), c.id,
		); err != nil {
			if e.logger != nil {
				e.logger.Error("ticket followup timestamp update failed", "ticket", c.id, "error", err)
			}
		}

		recipient := ""
		if c.assigneeID != nil {
			recipient = *c.assigneeID
		} else if c.requesterID != nil {
			recipient = *c.requesterID
		}
		if recipient != "" {
			e.notif.Create(recipient, notifications.TypeFollowUp, "Follow-up needed: "+c.number, c.title, "ticket", c.id)
		}

		raised++
	}

	return raised, nil
}

func placeholders(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += "?"
	}
	return s
}

func toArgs(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}

// --- HTTP: manual follow-up + listing ---

type Handler struct {
	db     *sql.DB
	audit  *audit.Logger
	notif  *notifications.Repository
	logger *slog.Logger
}

func NewHandler(db *sql.DB, auditLogger *audit.Logger, notif *notifications.Repository, logger *slog.Logger) *Handler {
	return &Handler{db: db, audit: auditLogger, notif: notif, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")
	rows, err := h.db.Query(
		`SELECT id, ticket_id, created_by, reason, is_system, resolved, created_at
		 FROM follow_ups WHERE ticket_id = ? ORDER BY created_at DESC`, ticketID)
	if err != nil {
		response.Internal(w, h.logger, err, "followup.list")
		return
	}
	defer rows.Close()

	out := make([]FollowUp, 0)
	for rows.Next() {
		var f FollowUp
		var createdBy sql.NullString
		var isSystem, resolved int
		var createdAt string
		if err := rows.Scan(&f.ID, &f.TicketID, &createdBy, &f.Reason, &isSystem, &resolved, &createdAt); err != nil {
			response.Internal(w, h.logger, err, "followup.scan")
			return
		}
		if createdBy.Valid {
			f.CreatedBy = &createdBy.String
		}
		f.IsSystem = isSystem != 0
		f.Resolved = resolved != 0
		f.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		out = append(out, f)
	}
	response.JSON(w, http.StatusOK, out)
}

type manualRequest struct {
	Reason string `json:"reason"`
}

// Trigger is the manual "Follow Up" action a user can take from a ticket
// (spec section 23/32), as opposed to the automatic system-generated pass.
func (h *Handler) Trigger(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")
	var req manualRequest
	_ = decodeOptionalJSON(r, &req)
	if req.Reason == "" {
		req.Reason = "Manual follow-up requested"
	}

	actorID := reqctx.UserID(r)
	now := time.Now().UTC()

	_, err := h.db.Exec(
		`INSERT INTO follow_ups (id, ticket_id, created_by, reason, is_system, resolved, created_at) VALUES (?, ?, ?, ?, 0, 0, ?)`,
		"fu-"+uuid.NewString(), ticketID, actorID, req.Reason, now,
	)
	if err != nil {
		response.Internal(w, h.logger, err, "followup.trigger")
		return
	}
	h.db.Exec(`UPDATE tickets SET last_followup_at = ? WHERE id = ?`, now, ticketID)

	var assigneeID, requesterID, number, title sql.NullString
	h.db.QueryRow(`SELECT assignee_id, requester_id, ticket_number, title FROM tickets WHERE id = ?`, ticketID).
		Scan(&assigneeID, &requesterID, &number, &title)
	recipient := assigneeID.String
	if recipient == "" {
		recipient = requesterID.String
	}
	if recipient != "" && recipient != actorID {
		h.notif.Create(recipient, notifications.TypeFollowUp, "Follow-up: "+number.String, title.String, "ticket", ticketID)
	}

	h.audit.Log(actorID, "CREATE", "follow_up", ticketID, map[string]interface{}{"reason": req.Reason}, r.RemoteAddr)
	response.Created(w, map[string]bool{"created": true})
}

func decodeOptionalJSON(r *http.Request, v *manualRequest) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	_ = json.NewDecoder(r.Body).Decode(v) // best-effort; empty/absent body is fine
	return nil
}
