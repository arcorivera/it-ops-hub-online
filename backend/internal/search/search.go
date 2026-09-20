// Package search implements global search (spec section 45) across
// tickets, incidents, and comments — real SQL queries, not a stub.
package search

import (
	"database/sql"
	"log/slog"
	"net/http"

	"itopshub/backend/internal/response"
)

type Result struct {
	Type     string `json:"type"` // ticket, incident, comment
	ID       string `json:"id"`
	EntityID string `json:"entityId"` // for comments, the parent ticket ID
	Number   string `json:"number"`
	Title    string `json:"title"`
	Snippet  string `json:"snippet"`
	Status   string `json:"status"`
	Severity string `json:"severity,omitempty"`
}

type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

const maxResultsPerType = 15

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 2 {
		response.JSON(w, http.StatusOK, []Result{})
		return
	}
	like := "%" + q + "%"

	results := make([]Result, 0)

	// Tickets: number, title, description, plus requester/assignee name via join.
	ticketRows, err := h.db.Query(`
		SELECT t.id, t.ticket_number, t.title, t.status, t.severity, t.description
		FROM tickets t
		LEFT JOIN users req ON req.id = t.requester_id
		LEFT JOIN users asg ON asg.id = t.assignee_id
		WHERE t.ticket_number LIKE ? OR t.title LIKE ? OR t.description LIKE ?
		   OR req.full_name LIKE ? OR asg.full_name LIKE ?
		ORDER BY t.created_at DESC LIMIT ?`,
		like, like, like, like, like, maxResultsPerType,
	)
	if err != nil {
		response.Internal(w, h.logger, err, "search.tickets")
		return
	}
	for ticketRows.Next() {
		var res Result
		var desc string
		if err := ticketRows.Scan(&res.ID, &res.Number, &res.Title, &res.Status, &res.Severity, &desc); err != nil {
			ticketRows.Close()
			response.Internal(w, h.logger, err, "search.tickets.scan")
			return
		}
		res.Type = "ticket"
		res.EntityID = res.ID
		res.Snippet = snippet(desc)
		results = append(results, res)
	}
	ticketRows.Close()

	// Incidents: number, title.
	incRows, err := h.db.Query(`
		SELECT id, incident_number, title, status, severity, description
		FROM incidents
		WHERE incident_number LIKE ? OR title LIKE ? OR description LIKE ?
		ORDER BY created_at DESC LIMIT ?`,
		like, like, like, maxResultsPerType,
	)
	if err != nil {
		response.Internal(w, h.logger, err, "search.incidents")
		return
	}
	for incRows.Next() {
		var res Result
		var desc string
		if err := incRows.Scan(&res.ID, &res.Number, &res.Title, &res.Status, &res.Severity, &desc); err != nil {
			incRows.Close()
			response.Internal(w, h.logger, err, "search.incidents.scan")
			return
		}
		res.Type = "incident"
		res.EntityID = res.ID
		res.Snippet = snippet(desc)
		results = append(results, res)
	}
	incRows.Close()

	// Comments: match content, return the parent ticket for navigation.
	commentRows, err := h.db.Query(`
		SELECT c.id, t.id, t.ticket_number, t.title, t.status, t.severity, c.content
		FROM ticket_comments c
		JOIN tickets t ON t.id = c.ticket_id
		WHERE c.content LIKE ? AND c.is_deleted = 0
		ORDER BY c.created_at DESC LIMIT ?`,
		like, maxResultsPerType,
	)
	if err != nil {
		response.Internal(w, h.logger, err, "search.comments")
		return
	}
	for commentRows.Next() {
		var res Result
		var content string
		if err := commentRows.Scan(&res.ID, &res.EntityID, &res.Number, &res.Title, &res.Status, &res.Severity, &content); err != nil {
			commentRows.Close()
			response.Internal(w, h.logger, err, "search.comments.scan")
			return
		}
		res.Type = "comment"
		res.Snippet = snippet(content)
		results = append(results, res)
	}
	commentRows.Close()

	if results == nil {
		results = []Result{}
	}
	response.JSON(w, http.StatusOK, results)
}

func snippet(s string) string {
	if len(s) > 140 {
		return s[:140] + "…"
	}
	return s
}
