// Package reqctx defines the shared context keys used to pass the
// authenticated user through the request lifecycle. It exists as its own
// package (rather than living in middleware) so domain packages like
// users/tickets/etc. can read the current actor without importing
// middleware, which itself imports those domain packages (would cause an
// import cycle).
package reqctx

import (
	"context"
	"net/http"
)

type key string

const userIDKey key = "current_user_id"
const sessionIDKey key = "current_session_id"

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserID(r *http.Request) string {
	v, _ := r.Context().Value(userIDKey).(string)
	return v
}

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

func SessionID(r *http.Request) string {
	v, _ := r.Context().Value(sessionIDKey).(string)
	return v
}
