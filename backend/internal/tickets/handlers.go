package tickets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/notifications"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
	"itopshub/backend/internal/users"
)

type Handler struct {
	repo       *Repository
	audit      *audit.Logger
	logger     *slog.Logger
	uploadsDir string
	userRepo   *users.Repository
	notifRepo  *notifications.Repository
	uatChecker UATGateChecker
}

// UATGateChecker is a narrow interface satisfied by the testing package's
// Repository, letting tickets enforce the UAT gate (spec section 40)
// without importing the testing package directly (would create an import
// cycle, since testing needs the notifications package which tickets also
// uses — keeping this as an interface avoids that entirely).
type UATGateChecker interface {
	HasFailingOrIncompleteTests(ticketID string) (bool, error)
}

func NewHandler(repo *Repository, auditLogger *audit.Logger, logger *slog.Logger, uploadsDir string, userRepo *users.Repository, notifRepo *notifications.Repository) *Handler {
	return &Handler{repo: repo, audit: auditLogger, logger: logger, uploadsDir: uploadsDir, userRepo: userRepo, notifRepo: notifRepo}
}

// SetUATChecker wires in the UAT gate checker after construction, since the
// testing package's repository is built after the tickets handler in
// main.go's dependency graph.
func (h *Handler) SetUATChecker(checker UATGateChecker) {
	h.uatChecker = checker
}

func (h *Handler) actor(r *http.Request) string {
	return reqctx.UserID(r)
}

func (h *Handler) isAdmin(r *http.Request) bool {
	u, err := h.userRepo.GetByID(h.actor(r))
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

// --- List / Get ---

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := ListFilter{
		AssigneeID:  q.Get("assigneeId"),
		TeamID:      q.Get("teamId"),
		ProjectID:   q.Get("projectId"),
		Environment: q.Get("environment"),
		CategoryID:  q.Get("categoryId"),
		Search:      q.Get("search"),
		SortBy:      q.Get("sortBy"),
		SortDir:     q.Get("sortDir"),
	}
	if v := q.Get("status"); v != "" {
		f.Status = strings.Split(v, ",")
	}
	if v := q.Get("severity"); v != "" {
		f.Severity = strings.Split(v, ",")
	}
	if v := q.Get("priority"); v != "" {
		f.Priority = strings.Split(v, ",")
	}
	if v := q.Get("assignee"); v == "me" {
		f.AssigneeID = h.actor(r)
	}
	if v := q.Get("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := q.Get("pageSize"); v != "" {
		f.PageSize, _ = strconv.Atoi(v)
	}

	result, err := h.repo.List(f)
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.list")
		return
	}

	response.JSONWithMeta(w, http.StatusOK, result.Tickets, map[string]interface{}{
		"total": result.Total, "page": result.Page, "pageSize": result.PageSize,
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	t, err := h.repo.GetByID(chi.URLParam(r, "id"))
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Ticket not found")
			return
		}
		response.Internal(w, h.logger, err, "tickets.get")
		return
	}
	response.JSON(w, http.StatusOK, t)
}

// --- Create ---

type createRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	RequesterID *string `json:"requesterId"`
	AssigneeID  *string `json:"assigneeId"`
	TeamID      *string `json:"teamId"`
	ProjectID   *string `json:"projectId"`
	CategoryID  *string `json:"categoryId"`
	Severity    string  `json:"severity"`
	Priority    string  `json:"priority"`
	Environment string  `json:"environment"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}

	errs := map[string]string{}
	if strings.TrimSpace(req.Title) == "" {
		errs["title"] = "Title is required"
	}
	if req.Severity == "" {
		req.Severity = SeverityS3
	} else if !ValidSeverities[req.Severity] {
		errs["severity"] = "Invalid severity"
	}
	if req.Priority == "" {
		req.Priority = PriorityMedium
	} else if !ValidPriorities[req.Priority] {
		errs["priority"] = "Invalid priority"
	}
	if req.Environment == "" {
		req.Environment = EnvProduction
	} else if !ValidEnvironments[req.Environment] {
		errs["environment"] = "Invalid environment"
	}
	if len(errs) > 0 {
		response.BadRequest(w, "Invalid ticket data", errs)
		return
	}

	actorID := h.actor(r)
	requesterID := req.RequesterID
	if requesterID == nil {
		requesterID = &actorID
	}

	t, err := h.repo.Create(CreateInput{
		Title:       req.Title,
		Description: req.Description,
		RequesterID: requesterID,
		AssigneeID:  req.AssigneeID,
		TeamID:      req.TeamID,
		ProjectID:   req.ProjectID,
		CategoryID:  req.CategoryID,
		Severity:    req.Severity,
		Priority:    req.Priority,
		Environment: req.Environment,
	}, actorID)
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.create")
		return
	}

	h.audit.Log(actorID, "CREATE", "ticket", t.ID, map[string]interface{}{"ticketNumber": t.TicketNumber}, r.RemoteAddr)
	response.Created(w, t)
}

// --- Status / Severity / Priority / Assign ---

type statusRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
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
		response.BadRequest(w, "Invalid status", map[string]string{"status": "Unknown status value"})
		return
	}

	force := req.Force && h.isAdmin(r)

	// UAT gate (spec section 40): block NEW_STATUS = UAT_PASSED if any test
	// case is failed, blocked, or not yet run — unless an admin explicitly
	// overrides, which is recorded distinctly in the audit log below.
	if req.Status == StatusUATPassed && !force && h.uatChecker != nil {
		blocked, err := h.uatChecker.HasFailingOrIncompleteTests(id)
		if err != nil {
			response.Internal(w, h.logger, err, "tickets.uatgate")
			return
		}
		if blocked {
			response.BadRequest(w, "Cannot mark UAT_PASSED: one or more test cases are failed, blocked, or not yet executed. An admin can override this.", nil)
			return
		}
	}

	t, err := h.repo.UpdateStatus(id, req.Status, h.actor(r), req.Note, force)
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Ticket not found")
			return
		}
		if err == ErrInvalidTransition {
			response.BadRequest(w, "That status transition is not allowed from the ticket's current status", nil)
			return
		}
		response.Internal(w, h.logger, err, "tickets.status")
		return
	}

	action := "STATUS_CHANGE"
	if force {
		action = "STATUS_CHANGE_OVERRIDE"
	}
	h.audit.Log(h.actor(r), action, "ticket", id, map[string]interface{}{"newStatus": req.Status}, r.RemoteAddr)

	if req.Status == StatusResolved && t.RequesterID != nil {
		h.notifRepo.Create(*t.RequesterID, notifications.TypeResolved, "Ticket resolved: "+t.TicketNumber, t.Title, "ticket", t.ID)
	}
	if req.Status == StatusUATFailed && t.AssigneeID != nil {
		h.notifRepo.Create(*t.AssigneeID, notifications.TypeUATFailed, "UAT failed: "+t.TicketNumber, t.Title, "ticket", t.ID)
	}
	if req.Status == StatusUATPassed && t.AssigneeID != nil {
		h.notifRepo.Create(*t.AssigneeID, notifications.TypeUATPassed, "UAT passed: "+t.TicketNumber, t.Title, "ticket", t.ID)
	}

	response.JSON(w, http.StatusOK, t)
}

type severityRequest struct {
	Severity string `json:"severity"`
}

func (h *Handler) ChangeSeverity(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req severityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !ValidSeverities[req.Severity] {
		response.BadRequest(w, "Invalid severity", nil)
		return
	}
	t, err := h.repo.UpdateSeverity(id, req.Severity, h.actor(r))
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Ticket not found")
			return
		}
		response.Internal(w, h.logger, err, "tickets.severity")
		return
	}
	h.audit.Log(h.actor(r), "SEVERITY_CHANGE", "ticket", id, map[string]interface{}{"newSeverity": req.Severity}, r.RemoteAddr)
	response.JSON(w, http.StatusOK, t)
}

type priorityRequest struct {
	Priority string `json:"priority"`
}

func (h *Handler) ChangePriority(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req priorityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !ValidPriorities[req.Priority] {
		response.BadRequest(w, "Invalid priority", nil)
		return
	}
	t, err := h.repo.UpdatePriority(id, req.Priority, h.actor(r))
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Ticket not found")
			return
		}
		response.Internal(w, h.logger, err, "tickets.priority")
		return
	}
	h.audit.Log(h.actor(r), "PRIORITY_CHANGE", "ticket", id, map[string]interface{}{"newPriority": req.Priority}, r.RemoteAddr)
	response.JSON(w, http.StatusOK, t)
}

type assignRequest struct {
	AssigneeID *string `json:"assigneeId"`
}

func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req assignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	t, err := h.repo.Assign(id, req.AssigneeID, h.actor(r))
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Ticket not found")
			return
		}
		response.Internal(w, h.logger, err, "tickets.assign")
		return
	}
	h.audit.Log(h.actor(r), "ASSIGN", "ticket", id, map[string]interface{}{"assigneeId": req.AssigneeID}, r.RemoteAddr)
	if req.AssigneeID != nil && *req.AssigneeID != h.actor(r) {
		h.notifRepo.Create(*req.AssigneeID, notifications.TypeAssigned, "Ticket assigned to you",
			t.TicketNumber+": "+t.Title, "ticket", t.ID)
	}
	response.JSON(w, http.StatusOK, t)
}

// --- Comments ---

type commentRequest struct {
	Content string `json:"content"`
}

func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")
	var req commentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		response.BadRequest(w, "Comment content is required", nil)
		return
	}
	c, err := h.repo.AddComment(ticketID, h.actor(r), req.Content)
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.comment.add")
		return
	}
	h.audit.Log(h.actor(r), "CREATE", "ticket_comment", c.ID, nil, r.RemoteAddr)

	// Notify the assignee and requester (whichever wasn't the commenter) that
	// a new comment landed on the ticket.
	if t, terr := h.repo.GetByID(ticketID); terr == nil {
		actor := h.actor(r)
		notify := map[string]bool{}
		if t.AssigneeID != nil && *t.AssigneeID != actor {
			notify[*t.AssigneeID] = true
		}
		if t.RequesterID != nil && *t.RequesterID != actor {
			notify[*t.RequesterID] = true
		}
		for uid := range notify {
			h.notifRepo.Create(uid, notifications.TypeComment, "New comment on "+t.TicketNumber, t.Title, "ticket", t.ID)
		}
	}

	response.Created(w, c)
}

func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListComments(chi.URLParam(r, "id"))
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.comment.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentId")
	err := h.repo.DeleteComment(commentID, h.actor(r), h.isAdmin(r))
	if err != nil {
		if err == ErrCommentNotFound {
			response.NotFound(w, "Comment not found")
			return
		}
		response.Forbidden(w, err.Error())
		return
	}
	h.audit.Log(h.actor(r), "DELETE", "ticket_comment", commentID, nil, r.RemoteAddr)
	response.NoContent(w)
}

// --- History ---

func (h *Handler) ListHistory(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListHistory(chi.URLParam(r, "id"))
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.history")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

// --- Attachments ---

var allowedExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".pdf": true,
	".docx": true, ".xlsx": true, ".txt": true, ".zip": true,
}

const maxUploadSize = 25 << 20 // 25MB

func (h *Handler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	ticketID := chi.URLParam(r, "id")

	if _, err := h.repo.GetByID(ticketID); err != nil {
		response.NotFound(w, "Ticket not found")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		response.BadRequest(w, "File too large or invalid upload (max 25MB)", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.BadRequest(w, "No file provided", nil)
		return
	}
	defer file.Close()

	// Validate extension against an allow-list (spec section 27) — never
	// trust the client-provided extension for anything beyond display.
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedExtensions[ext] {
		response.BadRequest(w, "File type not allowed", map[string]string{"file": "Allowed types: png, jpg, jpeg, pdf, docx, xlsx, txt, zip"})
		return
	}

	// Sanitize the display filename (strip any path components) and generate
	// a random on-disk name to prevent path traversal and collisions.
	safeName := filepath.Base(header.Filename)
	storedName, err := randomFileName(ext)
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.attachment.random")
		return
	}

	destPath := filepath.Join(h.uploadsDir, storedName)
	// Defense in depth: confirm the resolved path is still inside uploadsDir.
	if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(h.uploadsDir)) {
		response.Internal(w, h.logger, errors.New("resolved upload path escaped uploads dir"), "tickets.attachment.path")
		return
	}

	dest, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.attachment.create")
		return
	}
	defer dest.Close()

	written, err := io.Copy(dest, file)
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.attachment.write")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	att, err := h.repo.AddAttachment(ticketID, h.actor(r), safeName, storedName, contentType, written)
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.attachment.save")
		return
	}

	h.audit.Log(h.actor(r), "CREATE", "ticket_attachment", att.ID, map[string]interface{}{"fileName": safeName}, r.RemoteAddr)
	response.Created(w, att)
}

func randomFileName(ext string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ext, nil
}

func (h *Handler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListAttachments(chi.URLParam(r, "id"))
	if err != nil {
		response.Internal(w, h.logger, err, "tickets.attachment.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	attachmentID := chi.URLParam(r, "attachmentId")
	att, err := h.repo.GetAttachment(attachmentID)
	if err != nil {
		response.NotFound(w, "Attachment not found")
		return
	}

	path := filepath.Join(h.uploadsDir, att.StoredName)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(h.uploadsDir)) {
		response.Internal(w, h.logger, errors.New("resolved path escaped uploads dir"), "tickets.attachment.download")
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+att.FileName+`"`)
	w.Header().Set("Content-Type", att.ContentType)
	http.ServeFile(w, r, path)
}
