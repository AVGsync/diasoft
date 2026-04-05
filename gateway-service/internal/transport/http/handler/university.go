package handler

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/diasoft/gateway-service/internal/authctx"
	"github.com/diasoft/gateway-service/internal/model"
)

type UniversityCabinetUseCase interface {
	Profile(ctx context.Context, vuzID string) (*model.University, error)
	ListBatches(ctx context.Context, vuzID string, limit int) ([]*model.Batch, error)
}

type UniversityHandler struct {
	cabinet UniversityCabinetUseCase
}

func NewUniversityHandler(cabinet UniversityCabinetUseCase) *UniversityHandler {
	return &UniversityHandler{cabinet: cabinet}
}

func (h *UniversityHandler) Profile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		university, err := h.cabinet.Profile(r.Context(), vuzID)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "university not found")
				return
			}

			writeError(w, http.StatusInternalServerError, "failed to load university profile")
			return
		}

		writeJSON(w, http.StatusOK, toUniversityAdminResponse(university))
	}
}

func (h *UniversityHandler) ListBatches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		limit := 20
		if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
			parsed, err := strconv.Atoi(rawLimit)
			if err != nil {
				writeError(w, http.StatusBadRequest, "limit must be an integer")
				return
			}
			limit = parsed
		}

		batches, err := h.cabinet.ListBatches(r.Context(), vuzID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load university batches")
			return
		}

		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		writeJSON(w, http.StatusOK, batches)
	}
}
