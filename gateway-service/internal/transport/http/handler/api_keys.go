package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/diasoft/gateway-service/internal/authctx"
	"github.com/diasoft/gateway-service/internal/model"
)

type APIKeyUseCase interface {
	Create(ctx context.Context, vuzID string, name *string) (*model.CreateAPIKeyResponse, error)
	List(ctx context.Context, vuzID string) ([]*model.APIKeySummary, error)
}

type APIKeyHandler struct {
	apiKeys   APIKeyUseCase
	validator Validator
}

func NewAPIKeyHandler(apiKeys APIKeyUseCase, validator Validator) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeys:   apiKeys,
		validator: validator,
	}
}

func (h *APIKeyHandler) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		request := &model.CreateAPIKeyRequest{}
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(request); err != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
		}

		if ok, err := h.validator.ValidateStruct(request); !ok {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		response, err := h.apiKeys.Create(r.Context(), vuzID, request.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create api key")
			return
		}

		writeJSON(w, http.StatusCreated, response)
	}
}

func (h *APIKeyHandler) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		items, err := h.apiKeys.List(r.Context(), vuzID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load api keys")
			return
		}

		writeJSON(w, http.StatusOK, items)
	}
}
