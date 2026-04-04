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
	GetUniversity(ctx context.Context, id string) (*model.University, error)
	ListUniversities(ctx context.Context) ([]*model.University, error)
	UpdateUniversityStatus(ctx context.Context, id, status string) (*model.University, error)
	DeleteUniversity(ctx context.Context, id string) error
	Stats(ctx context.Context) (*model.AdminStats, error)
}

type AdminHandler struct {
	admin AdminUseCase
}

func NewAdminHandler(admin AdminUseCase) *AdminHandler {
	return &AdminHandler{admin: admin}
}

func (h *AdminHandler) ListUniversities() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		universities, err := h.admin.ListUniversities(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load universities")
			return
		}

		response := make([]*model.UniversityAdminResponse, 0, len(universities))
		for _, university := range universities {
			response = append(response, toUniversityAdminResponse(university))
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func (h *AdminHandler) GetUniversity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing university id")
			return
		}

		university, err := h.admin.GetUniversity(r.Context(), id)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "university not found")
				return
			}

			writeError(w, http.StatusInternalServerError, "failed to load university")
			return
		}

		writeJSON(w, http.StatusOK, toUniversityAdminResponse(university))
	}
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

		writeJSON(w, http.StatusOK, toUniversityAdminResponse(university))
	}
}

func (h *AdminHandler) UpdateUniversityStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing university id")
			return
		}

		request := &model.UpdateUniversityStatusRequest{}
		if err := decodeJSON(r, request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}

		university, err := h.admin.UpdateUniversityStatus(r.Context(), id, request.Status)
		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "university not found")
				return
			}

			writeError(w, http.StatusInternalServerError, "failed to update university status")
			return
		}

		writeJSON(w, http.StatusOK, toUniversityAdminResponse(university))
	}
}

func (h *AdminHandler) DeleteUniversity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "missing university id")
			return
		}

		if err := h.admin.DeleteUniversity(r.Context(), id); err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "university not found")
				return
			}

			writeError(w, http.StatusInternalServerError, "failed to delete university")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"id":     id,
			"status": "deleted",
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

func toUniversityAdminResponse(university *model.University) *model.UniversityAdminResponse {
	return &model.UniversityAdminResponse{
		ID:           university.ID,
		VuzCode:      university.VuzCode,
		Name:         university.Name,
		INN:          university.INN,
		OGRN:         university.OGRN,
		Email:        university.Email,
		Status:       university.Status,
		HasPublicKey: university.PublicKey != nil && *university.PublicKey != "",
		CreatedAt:    university.CreatedAt,
	}
}
