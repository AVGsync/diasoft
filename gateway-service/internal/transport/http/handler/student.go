package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
	"github.com/diasoft/gateway-service/internal/service"
	"github.com/go-chi/chi/v5"
)

type StudentUseCase interface {
	Search(ctx context.Context, diplomaNumber, fullName string) ([]*model.StudentSearchResult, error)
	CreateShareLink(ctx context.Context, diplomaHash string, ttl time.Duration) (*model.ShareLinkResponse, error)
	RenderShareQR(ctx context.Context, diplomaHash, format string, ttl time.Duration) ([]byte, string, error)
	ResolveShareLink(ctx context.Context, tokenValue string) (*model.SharedDiplomaResponse, error)
}

type StudentHandler struct {
	students  StudentUseCase
	validator Validator
}

func NewStudentHandler(students StudentUseCase, validator Validator) *StudentHandler {
	return &StudentHandler{
		students:  students,
		validator: validator,
	}
}

func (h *StudentHandler) Search() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diplomaNumber := r.URL.Query().Get("diploma_number")
		fullName := r.URL.Query().Get("full_name")
		if diplomaNumber == "" && fullName == "" {
			writeError(w, http.StatusBadRequest, "at least one search parameter is required")
			return
		}

		items, err := h.students.Search(r.Context(), diplomaNumber, fullName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to search students")
			return
		}

		writeJSON(w, http.StatusOK, items)
	}
}

func (h *StudentHandler) Share() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := &model.CreateShareLinkRequest{}
		if err := json.NewDecoder(r.Body).Decode(request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if ok, err := h.validator.ValidateStruct(request); !ok {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		response, err := h.students.CreateShareLink(r.Context(), request.DiplomaHash, hoursToDuration(request.TTLHours))
		if err != nil {
			switch {
			case errors.Is(err, service.ErrDiplomaNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "failed to create share link")
			}
			return
		}

		writeJSON(w, http.StatusCreated, response)
	}
}

func (h *StudentHandler) QR() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		diplomaHash := r.URL.Query().Get("diploma_hash")
		if diplomaHash == "" {
			writeError(w, http.StatusBadRequest, "missing diploma_hash")
			return
		}

		format := r.URL.Query().Get("format")
		ttlHours := parseOptionalHours(r.URL.Query().Get("ttl_hours"))

		payload, contentType, err := h.students.RenderShareQR(r.Context(), diplomaHash, format, ttlHours)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrDiplomaNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "failed to generate qr code")
			}
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}
}

func (h *StudentHandler) SharedDiploma() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenValue := chi.URLParam(r, "token")
		if tokenValue == "" {
			writeError(w, http.StatusBadRequest, "missing share token")
			return
		}

		response, err := h.students.ResolveShareLink(r.Context(), tokenValue)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidShareToken):
				writeError(w, http.StatusUnauthorized, err.Error())
			case errors.Is(err, service.ErrShareLinkExpired):
				writeError(w, http.StatusGone, err.Error())
			case errors.Is(err, service.ErrDiplomaNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "failed to resolve share link")
			}
			return
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func hoursToDuration(hours *int) time.Duration {
	if hours == nil || *hours <= 0 {
		return 0
	}

	return time.Duration(*hours) * time.Hour
}

func parseOptionalHours(value string) time.Duration {
	if value == "" {
		return 0
	}

	hours, err := strconv.Atoi(value)
	if err != nil || hours <= 0 {
		return 0
	}

	return time.Duration(hours) * time.Hour
}
