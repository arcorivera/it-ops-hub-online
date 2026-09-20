package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"itopshub/backend/internal/auth"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
	"itopshub/backend/internal/users"
)

type contextKey string

const (
	ctxKeyUser    contextKey = "user"
	ctxKeySession contextKey = "session"
)

const SessionCookieName = "itops_session"

// RequireAuth validates the session cookie and attaches the current user to
// the request context. It's the single gate every protected route passes
// through.
func RequireAuth(sessionStore *auth.Store, userRepo *users.Repository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				response.Unauthorized(w, "No active session")
				return
			}

			sess, err := sessionStore.ValidateSession(cookie.Value)
			if err != nil {
				response.Unauthorized(w, "Session invalid or expired")
				return
			}

			u, err := userRepo.GetByID(sess.UserID)
			if err != nil {
				response.Unauthorized(w, "User not found")
				return
			}
			if !u.IsActive {
				response.Forbidden(w, "Account is deactivated")
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUser, u)
			ctx = context.WithValue(ctx, ctxKeySession, sess)
			ctx = reqctx.WithUserID(ctx, u.ID)
			ctx = reqctx.WithSessionID(ctx, sess.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CurrentUser retrieves the authenticated user placed on the context by RequireAuth.
func CurrentUser(r *http.Request) *users.User {
	u, _ := r.Context().Value(ctxKeyUser).(*users.User)
	return u
}

func hasAnyRole(u *users.User, allowed ...string) bool {
	for _, r := range u.Roles {
		for _, a := range allowed {
			if r == a {
				return true
			}
		}
	}
	return false
}

// RequireRole builds middleware that only allows the given roles through.
// ADMIN implicitly always passes since it has full system administration
// per spec section 10.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := CurrentUser(r)
			if u == nil {
				response.Unauthorized(w, "")
				return
			}
			if hasAnyRole(u, users.RoleAdmin) || hasAnyRole(u, roles...) {
				next.ServeHTTP(w, r)
				return
			}
			response.Forbidden(w, "This action requires a higher role")
		})
	}
}

// SecurityHeaders sets baseline hardening headers (spec section 54).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-XSS-Protection", "0") // superseded by CSP; explicit disable avoids legacy quirks
		next.ServeHTTP(w, r)
	})
}

// RequestLogger logs method, path, status and duration for every request.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(sw, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote", r.RemoteAddr,
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// RateLimiter implements a simple sliding-window limiter per client IP,
// sufficient for a single-machine local-first deployment (spec section 54).
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		now := time.Now()

		rl.mu.Lock()
		var kept []time.Time
		for _, t := range rl.requests[key] {
			if now.Sub(t) < rl.window {
				kept = append(kept, t)
			}
		}
		if len(kept) >= rl.limit {
			rl.mu.Unlock()
			response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests, please slow down", nil)
			return
		}
		kept = append(kept, now)
		rl.requests[key] = kept
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
