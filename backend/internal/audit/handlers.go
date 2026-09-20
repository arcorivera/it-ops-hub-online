package audit

import (
	"net/http"
	"strconv"

	"itopshub/backend/internal/response"
)

type Handler struct {
	logger *Logger
}

func NewHandler(logger *Logger) *Handler {
	return &Handler{logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := ListFilter{
		Action:     q.Get("action"),
		EntityType: q.Get("entityType"),
		UserID:     q.Get("userId"),
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("offset"); v != "" {
		f.Offset, _ = strconv.Atoi(v)
	}

	entries, total, err := h.logger.List(f)
	if err != nil {
		response.Internal(w, nil, err, "audit.list")
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	response.JSONWithMeta(w, http.StatusOK, entries, map[string]interface{}{"total": total})
}
