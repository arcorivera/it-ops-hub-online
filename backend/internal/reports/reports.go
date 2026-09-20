// Package reports implements the reporting suite (spec sections 48-49):
// ticket volume, SLA compliance, response/resolution time, breakdowns by
// team/assignee/category/project, an executive management dashboard, and
// CSV export — all computed live from real ticket data.
package reports

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"itopshub/backend/internal/response"
	"itopshub/backend/internal/sla"
)

type Filter struct {
	StartDate string
	EndDate   string
	TeamID    string
	ProjectID string
	Severity  string
}

func (f Filter) whereClause() (string, []interface{}) {
	where := "WHERE 1=1"
	args := []interface{}{}
	if f.StartDate != "" {
		where += " AND created_at >= ?"
		args = append(args, f.StartDate)
	}
	if f.EndDate != "" {
		where += " AND created_at <= ?"
		args = append(args, f.EndDate+" 23:59:59")
	}
	if f.TeamID != "" {
		where += " AND team_id = ?"
		args = append(args, f.TeamID)
	}
	if f.ProjectID != "" {
		where += " AND project_id = ?"
		args = append(args, f.ProjectID)
	}
	if f.Severity != "" {
		where += " AND severity = ?"
		args = append(args, f.Severity)
	}
	return where, args
}

type VolumePoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type BreakdownItem struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type SLACompliance struct {
	WithinSLA         int     `json:"withinSLA"`
	Breached          int     `json:"breached"`
	TotalMeasured     int     `json:"totalMeasured"`
	CompliancePercent float64 `json:"compliancePercent"`
}

type Summary struct {
	TicketVolume         []VolumePoint   `json:"ticketVolume"`
	SLACompliance        SLACompliance   `json:"slaCompliance"`
	AvgResponseMinutes   float64         `json:"avgResponseMinutes"`
	AvgResolutionMinutes float64         `json:"avgResolutionMinutes"`
	ByTeam               []BreakdownItem `json:"byTeam"`
	ByAssignee           []BreakdownItem `json:"byAssignee"`
	ByCategory           []BreakdownItem `json:"byCategory"`
	ByProject            []BreakdownItem `json:"byProject"`
	ProductionIncidents  int             `json:"productionIncidents"`
	TotalTickets         int             `json:"totalTickets"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) BuildSummary(f Filter) (*Summary, error) {
	where, args := f.whereClause()
	s := &Summary{}

	if err := r.db.QueryRow(`SELECT COUNT(*) FROM tickets `+where, args...).Scan(&s.TotalTickets); err != nil {
		return nil, err
	}

	volRows, err := r.db.Query(`SELECT date(created_at) as d, COUNT(*) FROM tickets `+where+` GROUP BY d ORDER BY d ASC`, args...)
	if err != nil {
		return nil, err
	}
	for volRows.Next() {
		var v VolumePoint
		if err := volRows.Scan(&v.Date, &v.Count); err != nil {
			volRows.Close()
			return nil, err
		}
		s.TicketVolume = append(s.TicketVolume, v)
	}
	volRows.Close()

	var avgResponse, avgResolution sql.NullFloat64
	err = r.db.QueryRow(
		`SELECT AVG((julianday(acknowledged_at) - julianday(created_at)) * 24 * 60) FROM tickets `+where+` AND acknowledged_at IS NOT NULL`,
		args...,
	).Scan(&avgResponse)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(
		`SELECT AVG((julianday(resolved_at) - julianday(created_at)) * 24 * 60) FROM tickets `+where+` AND resolved_at IS NOT NULL`,
		args...,
	).Scan(&avgResolution)
	if err != nil {
		return nil, err
	}
	s.AvgResponseMinutes = roundTo(avgResponse.Float64, 1)
	s.AvgResolutionMinutes = roundTo(avgResolution.Float64, 1)

	idRows, err := r.db.Query(`SELECT id FROM tickets `+where+` AND sla_policy_id IS NOT NULL`, args...)
	if err != nil {
		return nil, err
	}
	var ticketIDs []string
	for idRows.Next() {
		var id string
		if err := idRows.Scan(&id); err != nil {
			idRows.Close()
			return nil, err
		}
		ticketIDs = append(ticketIDs, id)
	}
	idRows.Close()

	for _, id := range ticketIDs {
		result, err := sla.Compute(r.db, id)
		if err != nil {
			continue
		}
		if result.Status == sla.StatusPaused {
			continue
		}
		s.SLACompliance.TotalMeasured++
		if result.Status == sla.StatusBreached {
			s.SLACompliance.Breached++
		} else {
			s.SLACompliance.WithinSLA++
		}
	}
	if s.SLACompliance.TotalMeasured > 0 {
		s.SLACompliance.CompliancePercent = roundTo(
			float64(s.SLACompliance.WithinSLA)/float64(s.SLACompliance.TotalMeasured)*100, 1)
	}

	s.ByTeam, err = r.breakdown(where, args, "team_id", `SELECT COALESCE(tm.name, 'Unassigned'), COUNT(*) FROM tickets t LEFT JOIN teams tm ON tm.id = t.team_id`)
	if err != nil {
		return nil, err
	}
	s.ByAssignee, err = r.breakdown(where, args, "assignee_id", `SELECT COALESCE(u.full_name, 'Unassigned'), COUNT(*) FROM tickets t LEFT JOIN users u ON u.id = t.assignee_id`)
	if err != nil {
		return nil, err
	}
	s.ByCategory, err = r.breakdown(where, args, "category_id", `SELECT COALESCE(c.name, 'Uncategorized'), COUNT(*) FROM tickets t LEFT JOIN ticket_categories c ON c.id = t.category_id`)
	if err != nil {
		return nil, err
	}
	s.ByProject, err = r.breakdown(where, args, "project_id", `SELECT COALESCE(p.name, 'No Project'), COUNT(*) FROM tickets t LEFT JOIN projects p ON p.id = t.project_id`)
	if err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(`SELECT COUNT(*) FROM incidents WHERE environment = 'PRODUCTION'`).Scan(&s.ProductionIncidents); err != nil {
		return nil, err
	}

	return s, nil
}

// breakdown runs a grouped count query against a join whose tickets table
// is aliased "t", rewriting the plain-column WHERE clause built for the
// unaliased table to match.
func (r *Repository) breakdown(baseWhere string, baseArgs []interface{}, groupCol, selectPrefix string) ([]BreakdownItem, error) {
	where := prefixColumns(baseWhere)
	query := selectPrefix + " " + where + " GROUP BY t." + groupCol + " ORDER BY COUNT(*) DESC LIMIT 10"
	rows, err := r.db.Query(query, baseArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BreakdownItem, 0)
	for rows.Next() {
		var b BreakdownItem
		if err := rows.Scan(&b.Label, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func prefixColumns(where string) string {
	for _, col := range []string{"created_at", "team_id", "project_id", "severity"} {
		where = replaceWord(where, col, "t."+col)
	}
	return where
}

func replaceWord(s, old, new string) string {
	out := ""
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			before := byte(' ')
			if i > 0 {
				before = s[i-1]
			}
			after := byte(' ')
			if i+len(old) < len(s) {
				after = s[i+len(old)]
			}
			if !isIdentChar(before) && !isIdentChar(after) {
				out += new
				i += len(old)
				continue
			}
		}
		out += string(s[i])
		i++
	}
	return out
}

func isIdentChar(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// Management is the executive dashboard response (spec section 49).
type Management struct {
	SLACompliancePercent float64 `json:"slaCompliancePercent"`
	OpenCriticalIssues   int     `json:"openCriticalIssues"`
	SLABreaches          int     `json:"slaBreaches"`
	AvgResponseMinutes   float64 `json:"avgResponseMinutes"`
	AvgResolutionMinutes float64 `json:"avgResolutionMinutes"`
	TicketsThisMonth     int     `json:"ticketsThisMonth"`
	OpenIncidents        int     `json:"openIncidents"`
	ProductionIncidents  int     `json:"productionIncidents"`
}

func (r *Repository) BuildManagement() (*Management, error) {
	firstOfMonth := time.Now().UTC().Format("2006-01") + "-01"
	summary, err := r.BuildSummary(Filter{StartDate: firstOfMonth})
	if err != nil {
		return nil, err
	}

	m := &Management{
		SLACompliancePercent: summary.SLACompliance.CompliancePercent,
		SLABreaches:          summary.SLACompliance.Breached,
		AvgResponseMinutes:   summary.AvgResponseMinutes,
		AvgResolutionMinutes: summary.AvgResolutionMinutes,
		TicketsThisMonth:     summary.TotalTickets,
		ProductionIncidents:  summary.ProductionIncidents,
	}

	openTicketStatuses := []string{
		"NEW", "ACKNOWLEDGED", "ASSIGNED", "IN_PROGRESS", "PENDING",
		"FOR_UAT", "UAT_FAILED", "UAT_PASSED", "FOR_PRE_PROD", "PRE_PROD_FAILED",
		"PRE_PROD_PASSED", "FOR_PRODUCTION", "PRODUCTION_FAILED",
	}
	placeholders := ""
	args := []interface{}{}
	for i, st := range openTicketStatuses {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, st)
	}
	args = append(args, "S1", "S2")
	err = r.db.QueryRow(
		`SELECT COUNT(*) FROM tickets WHERE status IN (`+placeholders+`) AND severity IN (?,?)`, args...,
	).Scan(&m.OpenCriticalIssues)
	if err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(`SELECT COUNT(*) FROM incidents WHERE status != 'CLOSED'`).Scan(&m.OpenIncidents); err != nil {
		return nil, err
	}

	return m, nil
}

func roundTo(v float64, places int) float64 {
	mult := 1.0
	for i := 0; i < places; i++ {
		mult *= 10
	}
	return float64(int(v*mult+0.5)) / mult
}

// --- HTTP handlers ---

type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

func filterFromQuery(r *http.Request) Filter {
	q := r.URL.Query()
	return Filter{
		StartDate: q.Get("startDate"), EndDate: q.Get("endDate"),
		TeamID: q.Get("teamId"), ProjectID: q.Get("projectId"), Severity: q.Get("severity"),
	}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.repo.BuildSummary(filterFromQuery(r))
	if err != nil {
		response.Internal(w, h.logger, err, "reports.summary")
		return
	}
	response.JSON(w, http.StatusOK, summary)
}

func (h *Handler) Management(w http.ResponseWriter, r *http.Request) {
	m, err := h.repo.BuildManagement()
	if err != nil {
		response.Internal(w, h.logger, err, "reports.management")
		return
	}
	response.JSON(w, http.StatusOK, m)
}

// ExportCSV streams matching tickets as a real CSV file (spec section 48).
func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	f := filterFromQuery(r)
	where, args := f.whereClause()

	rows, err := h.repo.db.Query(`
		SELECT ticket_number, title, severity, priority, status, environment, created_at, resolved_at
		FROM tickets `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		response.Internal(w, h.logger, err, "reports.export")
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tickets-export-%s.csv"`, time.Now().Format("2006-01-02")))

	cw := csv.NewWriter(w)
	cw.Write([]string{"Ticket Number", "Title", "Severity", "Priority", "Status", "Environment", "Created At", "Resolved At"})

	for rows.Next() {
		var number, title, severity, priority, status, environment, createdAt string
		var resolvedAt sql.NullString
		if err := rows.Scan(&number, &title, &severity, &priority, &status, &environment, &createdAt, &resolvedAt); err != nil {
			continue
		}
		cw.Write([]string{number, title, severity, priority, status, environment, createdAt, resolvedAt.String})
	}
	cw.Flush()
}
