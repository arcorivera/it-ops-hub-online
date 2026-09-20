package users

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

type PasswordHasher func(plain string) (string, error)

type Handler struct {
	repo   *Repository
	audit  *audit.Logger
	logger *slog.Logger
	hash   PasswordHasher
}

func NewHandler(repo *Repository, auditLogger *audit.Logger, logger *slog.Logger, hasher PasswordHasher) *Handler {
	return &Handler{repo: repo, audit: auditLogger, logger: logger, hash: hasher}
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List()
	if err != nil {
		response.Internal(w, h.logger, err, "users.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	u, err := h.repo.GetByID(id)
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "User not found")
			return
		}
		response.Internal(w, h.logger, err, "users.get")
		return
	}
	response.JSON(w, http.StatusOK, u)
}

type createRequest struct {
	Username string   `json:"username"`
	FullName string   `json:"fullName"`
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}

	errs := map[string]string{}
	if len(req.Username) < 3 {
		errs["username"] = "Username must be at least 3 characters"
	}
	if req.FullName == "" {
		errs["fullName"] = "Full name is required"
	}
	if !emailRe.MatchString(req.Email) {
		errs["email"] = "A valid email is required"
	}
	if len(req.Password) < 8 {
		errs["password"] = "Password must be at least 8 characters"
	}
	if len(req.Roles) == 0 {
		errs["roles"] = "At least one role is required"
	}
	if len(errs) > 0 {
		response.BadRequest(w, "Invalid user data", errs)
		return
	}

	hash, err := h.hash(req.Password)
	if err != nil {
		response.Internal(w, h.logger, err, "users.create.hash")
		return
	}

	u, err := h.repo.Create(CreateInput{
		Username: req.Username,
		FullName: req.FullName,
		Email:    req.Email,
		Roles:    req.Roles,
	}, hash)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	actor := actorID(r)
	h.audit.Log(actor, "CREATE", "user", u.ID, map[string]interface{}{"username": u.Username, "roles": u.Roles}, r.RemoteAddr)

	response.Created(w, u)
}

type updateRequest struct {
	FullName *string   `json:"fullName"`
	Email    *string   `json:"email"`
	IsActive *bool     `json:"isActive"`
	Roles    *[]string `json:"roles"`
	Password *string   `json:"password"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.Email != nil && !emailRe.MatchString(*req.Email) {
		response.BadRequest(w, "Invalid email", nil)
		return
	}

	u, err := h.repo.Update(id, UpdateInput{
		FullName: req.FullName,
		Email:    req.Email,
		IsActive: req.IsActive,
		Roles:    req.Roles,
	})
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "User not found")
			return
		}
		response.Internal(w, h.logger, err, "users.update")
		return
	}

	if req.Password != nil {
		if len(*req.Password) < 8 {
			response.BadRequest(w, "Password must be at least 8 characters", nil)
			return
		}
		hash, err := h.hash(*req.Password)
		if err != nil {
			response.Internal(w, h.logger, err, "users.update.hash")
			return
		}
		if err := h.repo.UpdatePassword(id, hash); err != nil {
			response.Internal(w, h.logger, err, "users.update.password")
			return
		}
	}

	actor := actorID(r)
	h.audit.Log(actor, "UPDATE", "user", u.ID, map[string]interface{}{"fields": changedFields(req)}, r.RemoteAddr)

	response.JSON(w, http.StatusOK, u)
}

func changedFields(req updateRequest) []string {
	var f []string
	if req.FullName != nil {
		f = append(f, "fullName")
	}
	if req.Email != nil {
		f = append(f, "email")
	}
	if req.IsActive != nil {
		f = append(f, "isActive")
	}
	if req.Roles != nil {
		f = append(f, "roles")
	}
	if req.Password != nil {
		f = append(f, "password")
	}
	return f
}

// actorID pulls the acting user's ID from context set by middleware.RequireAuth,
// via the shared reqctx package (avoids an import cycle with middleware).
func actorID(r *http.Request) string {
	return reqctx.UserID(r)
}
