package dashboard

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

type Handler struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewHandler(db *sql.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

func (h *Handler) Attention(w http.ResponseWriter, r *http.Request) {
	result, err := ComputeAttention(h.db, reqctx.UserID(r))
	if err != nil {
		response.Internal(w, h.logger, err, "dashboard.attention")
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) MyWork(w http.ResponseWriter, r *http.Request) {
	result, err := ComputeMyWork(h.db, reqctx.UserID(r))
	if err != nil {
		response.Internal(w, h.logger, err, "dashboard.mywork")
		return
	}
	response.JSON(w, http.StatusOK, result)
}

func (h *Handler) PriorityQueue(w http.ResponseWriter, r *http.Request) {
	scopeAll := r.URL.Query().Get("scope") == "all"
	limit := 25
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	result, err := ComputePriorityQueue(h.db, reqctx.UserID(r), scopeAll, limit)
	if err != nil {
		response.Internal(w, h.logger, err, "dashboard.priorityqueue")
		return
	}
	response.JSON(w, http.StatusOK, result)
}
