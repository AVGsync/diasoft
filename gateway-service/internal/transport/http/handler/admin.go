package handler

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/diasoft/gateway-service/internal/model"
	"github.com/go-chi/chi/v5"
)

type AdminUseCase interface {
	ApproveUniversity(ctx context.Context, id string) (*model.University, error)
	Stats(ctx context.Context) (*model.AdminStats, error)
}

type AdminHandler struct {
	admin AdminUseCase
}

func NewAdminHandler(admin AdminUseCase) *AdminHandler {
	return &AdminHandler{admin: admin}
}

func (h *AdminHandler) ApproveUniversity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing university id")
			return
		}

		university, err := h.admin.ApproveUniversity(r.Context(), id)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "university not found")
				return
			}

			writeError(w, http.StatusInternalServerError, "failed to approve university")
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":         university.ID,
			"status":     university.Status,
			"name":       university.Name,
			"approvedAt": university.CreatedAt,
		})
	}
}

func (h *AdminHandler) Stats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := h.admin.Stats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load stats")
			return
		}

		writeJSON(w, http.StatusOK, stats)
	}
}
