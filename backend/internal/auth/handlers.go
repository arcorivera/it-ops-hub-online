package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/response"
	"itopshub/backend/internal/users"
)

type Handler struct {
	userRepo *users.Repository
	sessions *Store
	audit    *audit.Logger
	logger   *slog.Logger
	secureCk bool
}

func NewHandler(userRepo *users.Repository, sessions *Store, auditLogger *audit.Logger, logger *slog.Logger, secureCookies bool) *Handler {
	return &Handler{userRepo: userRepo, sessions: sessions, audit: auditLogger, logger: logger, secureCk: secureCookies}
}

var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// GET /api/v1/auth/status - tells the frontend whether first-run setup is needed.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	count, err := h.userRepo.Count()
	if err != nil {
		response.Internal(w, h.logger, err, "auth.status")
		return
	}
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"needsSetup": count == 0,
	})
}

type setupRequest struct {
	Username        string `json:"username"`
	FullName        string `json:"fullName"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// POST /api/v1/auth/setup - creates the first administrator account.
// Refuses to run if any user already exists (spec section 9/59).
func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	count, err := h.userRepo.Count()
	if err != nil {
		response.Internal(w, h.logger, err, "auth.setup.count")
		return
	}
	if count > 0 {
		response.Conflict(w, "Setup has already been completed")
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}

	if errs := validateSetup(req); len(errs) > 0 {
		response.BadRequest(w, "Invalid setup data", errs)
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		response.Internal(w, h.logger, err, "auth.setup.hash")
		return
	}

	u, err := h.userRepo.Create(users.CreateInput{
		Username: req.Username,
		FullName: req.FullName,
		Email:    req.Email,
		Roles:    []string{users.RoleAdmin},
	}, hash)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	h.audit.Log(u.ID, "CREATE", "user", u.ID, map[string]interface{}{"note": "initial administrator created via setup"}, r.RemoteAddr)

	sess, err := h.sessions.CreateSession(u.ID, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		response.Internal(w, h.logger, err, "auth.setup.session")
		return
	}
	h.setSessionCookie(w, sess)

	response.Created(w, u)
}

func validateSetup(req setupRequest) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Username) == "" || len(req.Username) < 3 {
		errs["username"] = "Username must be at least 3 characters"
	}
	if strings.TrimSpace(req.FullName) == "" {
		errs["fullName"] = "Full name is required"
	}
	if !emailRe.MatchString(req.Email) {
		errs["email"] = "A valid email is required"
	}
	if len(req.Password) < 8 {
		errs["password"] = "Password must be at least 8 characters"
	}
	if req.Password != req.ConfirmPassword {
		errs["confirmPassword"] = "Passwords do not match"
	}
	return errs
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /api/v1/auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.Username == "" || req.Password == "" {
		response.BadRequest(w, "Username and password are required", nil)
		return
	}

	u, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		if err == users.ErrNotFound {
			// try email login as a convenience, still generic error on failure
			u, err = h.userRepo.GetByEmail(req.Username)
		}
		if err != nil {
			response.Unauthorized(w, "Invalid username or password")
			return
		}
	}

	if !u.IsActive {
		response.Forbidden(w, "This account has been deactivated")
		return
	}

	if !CheckPassword(u.PasswordHash, req.Password) {
		h.audit.Log("", "LOGIN_FAILED", "user", u.ID, map[string]interface{}{"username": req.Username}, r.RemoteAddr)
		response.Unauthorized(w, "Invalid username or password")
		return
	}

	sess, err := h.sessions.CreateSession(u.ID, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		response.Internal(w, h.logger, err, "auth.login.session")
		return
	}
	h.setSessionCookie(w, sess)
	h.audit.Log(u.ID, "LOGIN", "user", u.ID, nil, r.RemoteAddr)

	response.JSON(w, http.StatusOK, u)
}

// POST /api/v1/auth/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("itops_session")
	if err == nil {
		if sess, verr := h.sessions.ValidateSession(cookie.Value); verr == nil {
			h.audit.Log(sess.UserID, "LOGOUT", "user", sess.UserID, nil, r.RemoteAddr)
		}
		_ = h.sessions.DeleteSession(cookie.Value)
	}
	h.clearSessionCookie(w)
	response.JSON(w, http.StatusOK, map[string]bool{"loggedOut": true})
}

// GET /api/v1/auth/me
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("itops_session")
	if err != nil {
		response.Unauthorized(w, "")
		return
	}
	sess, err := h.sessions.ValidateSession(cookie.Value)
	if err != nil {
		response.Unauthorized(w, "")
		return
	}
	u, err := h.userRepo.GetByID(sess.UserID)
	if err != nil {
		response.Unauthorized(w, "")
		return
	}
	response.JSON(w, http.StatusOK, u)
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, sess *Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     "itops_session",
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCk,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "itops_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCk,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
