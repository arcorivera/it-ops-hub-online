package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
)

// Envelope is the consistent success response shape used by every endpoint.
type Envelope struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// ErrorBody is the consistent error response shape.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// normalizeNilSlices converts nil Go slices to empty slices before JSON
// encoding. JSON consumers should receive [] for empty collections, never null.
func normalizeNilSlices(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	if v.Kind() == reflect.Interface {
		if !v.IsNil() {
			normalizeNilSlices(v.Elem())
		}
		return
	}
	if v.Kind() == reflect.Pointer {
		if !v.IsNil() {
			normalizeNilSlices(v.Elem())
		}
		return
	}
	switch v.Kind() {
	case reflect.Slice:
		if v.CanSet() && v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			return
		}
		for i := 0; i < v.Len(); i++ {
			normalizeNilSlices(v.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if f.CanSet() {
				normalizeNilSlices(f)
			}
		}
	case reflect.Map:
		if v.IsNil() {
			return
		}
		for _, key := range v.MapKeys() {
			value := v.MapIndex(key)
			copyValue := reflect.New(value.Type()).Elem()
			copyValue.Set(value)
			normalizeNilSlices(copyValue)
			v.SetMapIndex(key, copyValue)
		}
	}
}

func normalized(data interface{}) interface{} {
	if data == nil {
		return data
	}
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			normalizeNilSlices(v)
		}
	case reflect.Struct, reflect.Slice, reflect.Map:
		copyValue := reflect.New(v.Type()).Elem()
		copyValue.Set(v)
		normalizeNilSlices(copyValue)
		return copyValue.Interface()
	}
	return data
}

func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Data: normalized(data), Message: "Success"})
}

func JSONWithMeta(w http.ResponseWriter, status int, data interface{}, meta interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Data: normalized(data), Message: "Success", Meta: normalized(meta)})
}

func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, data)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error writes a structured error response. It never leaks internal details
// (stack traces, raw driver errors) to the client — only the safe message
// passed in is exposed. Full detail should already have been logged by the
// caller before this is invoked.
func Error(w http.ResponseWriter, status int, code, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{Error: ErrorDetail{
		Code:    code,
		Message: message,
		Details: details,
	}})
}

func BadRequest(w http.ResponseWriter, message string, details interface{}) {
	Error(w, http.StatusBadRequest, "VALIDATION_ERROR", message, details)
}

func Unauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Authentication required"
	}
	Error(w, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

func Forbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "You do not have permission to perform this action"
	}
	Error(w, http.StatusForbidden, "FORBIDDEN", message, nil)
}

func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "Resource not found"
	}
	Error(w, http.StatusNotFound, "NOT_FOUND", message, nil)
}

func Conflict(w http.ResponseWriter, message string) {
	Error(w, http.StatusConflict, "CONFLICT", message, nil)
}

// Internal logs the real error server-side and returns a generic message to
// the client, per spec section 54 ("never expose internal errors").
func Internal(w http.ResponseWriter, logger *slog.Logger, err error, context string) {
	if logger != nil {
		logger.Error("internal error", "context", context, "error", err)
	}
	Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred", nil)
}
