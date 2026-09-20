// Package escalation implements the escalation engine (spec section 34):
// a background pass that checks every open ticket's live SLA percentage
// against its severity's escalation thresholds (50/75/100%) and escalates
// to the configured role the first time each threshold is crossed.
package escalation

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/notifications"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
	"itopshub/backend/internal/sla"
	"itopshub/backend/internal/users"
)

var openStatuses = []string{
	"NEW", "ACKNOWLEDGED", "ASSIGNED", "IN_PROGRESS",
	"FOR_UAT", "UAT_FAILED", "UAT_PASSED", "FOR_PRE_PROD", "PRE_PROD_FAILED",
	"PRE_PROD_PASSED", "FOR_PRODUCTION", "PRODUCTION_FAILED",
}

type Event struct {
	ID               string    `json:"id"`
	TicketID         string    `json:"ticketId"`
	ThresholdPercent int       `json:"thresholdPercent"`
	EscalatedToRole  string    `json:"escalatedToRole"`
	EscalatedToUser  *string   `json:"escalatedToUser"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Engine struct {
	db     *sql.DB
	notif  *notifications.Repository
	logger *slog.Logger
}

func NewEngine(db *sql.DB, notif *notifications.Repository, logger *slog.Logger) *Engine {
	return &Engine{db: db, notif: notif, logger: logger}
}

// RunPass computes live SLA percentage for every open ticket and, for each
// escalation rule threshold that has been crossed and not yet escalated,
// creates a real escalation_events record, resolves the target user for the
// configured role, and sends a notification. Returns the number of new
// escalations raised.
func (e *Engine) RunPass() (int, error) {
	rows, err := e.db.Query(`
		SELECT id, ticket_number, title, severity, status, team_id, assignee_id
		FROM tickets WHERE status IN (`+placeholders(len(openStatuses))+`)
	`, toArgs(openStatuses)...)
	if err != nil {
		return 0, err
	}

	type ticket struct {
		id, number, title, severity, status string
		teamID, assigneeID                  *string
	}
	var tickets []ticket
	for rows.Next() {
		var t ticket
		var teamID, assigneeID sql.NullString
		if err := rows.Scan(&t.id, &t.number, &t.title, &t.severity, &t.status, &teamID, &assigneeID); err != nil {
			rows.Close()
			return 0, err
		}
		if teamID.Valid {
			t.teamID = &teamID.String
		}
		if assigneeID.Valid {
			t.assigneeID = &assigneeID.String
		}
		tickets = append(tickets, t)
	}
	rows.Close()

	raised := 0
	for _, t := range tickets {
		result, err := sla.Compute(e.db, t.id)
		if err != nil || result.IsPaused {
			continue // paused tickets don't accrue escalation either
		}

		rules, err := e.rulesForSeverity(t.severity)
		if err != nil {
			continue
		}

		for _, rule := range rules {
			crossed := (rule.ThresholdPercent >= 100 && result.Percentage >= 100) ||
				(rule.ThresholdPercent < 100 && result.Percentage >= float64(rule.ThresholdPercent))
			if !crossed {
				continue
			}

			already, err := e.alreadyEscalated(t.id, rule.ThresholdPercent)
			if err != nil || already {
				continue
			}

			targetUser := e.resolveTarget(rule.EscalateToRole, t.teamID, t.assigneeID)

			var targetUserArg interface{}
			if targetUser != "" {
				targetUserArg = targetUser
			}

			_, err = e.db.Exec(
				`INSERT INTO escalation_events (id, ticket_id, rule_id, threshold_percent, escalated_to_role, escalated_to_user, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				"esc-"+uuid.NewString(), t.id, rule.ID, rule.ThresholdPercent, rule.EscalateToRole, targetUserArg, time.Now().UTC(),
			)
			if err != nil {
				if e.logger != nil {
					e.logger.Error("escalation insert failed", "ticket", t.id, "error", err)
				}
				continue
			}

			e.db.Exec(`UPDATE tickets SET escalation_level = escalation_level + 1 WHERE id = ?`, t.id)

			if targetUser != "" {
				title := "Escalation: " + t.number + " at " + itoa(rule.ThresholdPercent) + "% SLA"
				e.notif.Create(targetUser, notifications.TypeEscalation, title, t.title, "ticket", t.id)
			}
			raised++
		}
	}

	return raised, nil
}

type rule struct {
	ID               string
	ThresholdPercent int
	EscalateToRole   string
}

func (e *Engine) rulesForSeverity(severity string) ([]rule, error) {
	rows, err := e.db.Query(`SELECT id, threshold_percent, escalate_to_role FROM escalation_rules WHERE severity = ? AND is_active = 1 ORDER BY threshold_percent ASC`, severity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]rule, 0)
	for rows.Next() {
		var r rule
		if err := rows.Scan(&r.ID, &r.ThresholdPercent, &r.EscalateToRole); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (e *Engine) alreadyEscalated(ticketID string, threshold int) (bool, error) {
	var n int
	err := e.db.QueryRow(`SELECT COUNT(*) FROM escalation_events WHERE ticket_id = ? AND threshold_percent = ?`, ticketID, threshold).Scan(&n)
	return n > 0, err
}

// resolveTarget maps an escalation role to a concrete user ID: the current
// assignee, the team's lead, or any active IT_MANAGER, depending on role.
func (e *Engine) resolveTarget(role string, teamID, assigneeID *string) string {
	switch role {
	case "ASSIGNEE":
		if assigneeID != nil {
			return *assigneeID
		}
		return ""
	case "TEAM_LEAD":
		if teamID == nil {
			return ""
		}
		var leadID string
		e.db.QueryRow(`SELECT user_id FROM team_members WHERE team_id = ? AND is_lead = 1 LIMIT 1`, *teamID).Scan(&leadID)
		return leadID
	case "IT_MANAGER":
		var userID string
		e.db.QueryRow(`
			SELECT u.id FROM users u
			JOIN user_roles ur ON ur.user_id = u.id
			JOIN roles r ON r.id = ur.role_id
			WHERE r.name = ? AND u.is_active = 1 LIMIT 1`, users.RoleITManager).Scan(&userID)
		return userID
	}
	return ""
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

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// --- HTTP: listing + manual escalate ---

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
		`SELECT id, ticket_id, threshold_percent, escalated_to_role, escalated_to_user, created_at
		 FROM escalation_events WHERE ticket_id = ? ORDER BY created_at DESC`, ticketID)
	if err != nil {
		response.Internal(w, h.logger, err, "escalation.list")
		return
	}
	defer rows.Close()

	out := make([]Event, 0)
	for rows.Next() {
		var ev Event
		var toUser sql.NullString
		var createdAt string
		if err := rows.Scan(&ev.ID, &ev.TicketID, &ev.ThresholdPercent, &ev.EscalatedToRole, &toUser, &createdAt); err != nil {
			response.Internal(w, h.logger, err, "escalation.scan")
			return
		}
		if toUser.Valid {
			ev.EscalatedToUser = &toUser.String
		}
		ev.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		out = append(out, ev)
	}
	response.JSON(w, http.StatusOK, out)
}

// Trigger is the manual "Escalate" action (spec section 23), separate from
// the automatic threshold-based escalation the background engine performs.
func (h *Handler) Trigger(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	var number, title string
	var teamID, assigneeID sql.NullString
	err := h.db.QueryRow(`SELECT ticket_number, title, team_id, assignee_id FROM tickets WHERE id = ?`, ticketID).
		Scan(&number, &title, &teamID, &assigneeID)
	if err != nil {
		response.NotFound(w, "Ticket not found")
		return
	}

	engine := &Engine{db: h.db, notif: h.notif, logger: h.logger}
	var team, assignee *string
	if teamID.Valid {
		team = &teamID.String
	}
	if assigneeID.Valid {
		assignee = &assigneeID.String
	}
	target := engine.resolveTarget("TEAM_LEAD", team, assignee)
	if target == "" {
		target = engine.resolveTarget("IT_MANAGER", team, assignee)
	}

	now := time.Now().UTC()
	_, err = h.db.Exec(
		`INSERT INTO escalation_events (id, ticket_id, rule_id, threshold_percent, escalated_to_role, escalated_to_user, created_at)
		 VALUES (?, ?, NULL, 0, 'MANUAL', ?, ?)`,
		"esc-"+uuid.NewString(), ticketID, nullIfEmpty(target), now,
	)
	if err != nil {
		response.Internal(w, h.logger, err, "escalation.trigger")
		return
	}
	h.db.Exec(`UPDATE tickets SET escalation_level = escalation_level + 1 WHERE id = ?`, ticketID)

	if target != "" {
		h.notif.Create(target, notifications.TypeEscalation, "Manual escalation: "+number, title, "ticket", ticketID)
	}

	h.audit.Log(reqctx.UserID(r), "ESCALATE", "ticket", ticketID, map[string]interface{}{"manual": true}, r.RemoteAddr)
	response.Created(w, map[string]bool{"created": true})
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
