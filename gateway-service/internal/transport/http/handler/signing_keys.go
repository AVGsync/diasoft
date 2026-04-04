package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/diasoft/gateway-service/internal/authctx"
	"github.com/diasoft/gateway-service/internal/model"
	"github.com/diasoft/gateway-service/internal/service"
)

type SigningKeyUseCase interface {
	Upsert(ctx context.Context, vuzID string, request *model.UpsertSigningKeyRequest) (*model.SigningKeyStatusResponse, error)
	Status(ctx context.Context, vuzID string) (*model.SigningKeyStatusResponse, error)
}

type SigningKeyHandler struct {
	keys      SigningKeyUseCase
	validator Validator
}

func NewSigningKeyHandler(keys SigningKeyUseCase, validator Validator) *SigningKeyHandler {
	return &SigningKeyHandler{
		keys:      keys,
		validator: validator,
	}
}

func (h *SigningKeyHandler) Upsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		request := &model.UpsertSigningKeyRequest{}
		if err := json.NewDecoder(r.Body).Decode(request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if ok, err := h.validator.ValidateStruct(request); !ok {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		response, err := h.keys.Upsert(r.Context(), vuzID, request)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidSigningKey), errors.Is(err, service.ErrSigningKeyMismatch):
				writeError(w, http.StatusBadRequest, err.Error())
			case errors.Is(err, sql.ErrNoRows):
				writeError(w, http.StatusNotFound, "university not found")
			default:
				writeError(w, http.StatusInternalServerError, "failed to save signing key")
			}
			return
		}

		writeJSON(w, http.StatusCreated, response)
	}
}

func (h *SigningKeyHandler) Status() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		response, err := h.keys.Status(r.Context(), vuzID)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrSigningKeyNotFound):
				writeError(w, http.StatusNotFound, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "failed to load signing key status")
			}
			return
		}

		writeJSON(w, http.StatusOK, response)
	}
}
