package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/diasoft/gateway-service/internal/model"
	"github.com/diasoft/gateway-service/internal/service"
)

type AuthUseCase interface {
	Register(ctx context.Context, request *model.RegisterUniversityRequest) (*model.RegisterUniversityResponse, error)
	Login(ctx context.Context, request *model.LoginRequest) (*model.AuthResponse, error)
}

type Validator interface {
	ValidateStruct(value interface{}) (bool, error)
}

type AuthHandler struct {
	auth      AuthUseCase
	validator Validator
}

func NewAuthHandler(auth AuthUseCase, validator Validator) *AuthHandler {
	return &AuthHandler{
		auth:      auth,
		validator: validator,
	}
}

func (h *AuthHandler) Register() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := &model.RegisterUniversityRequest{}
		if err := json.NewDecoder(r.Body).Decode(request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if ok, err := h.validator.ValidateStruct(request); !ok {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		response, err := h.auth.Register(r.Context(), request)
		if err != nil {
			if errors.Is(err, service.ErrDuplicateEntity) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to register university")
			return
		}

		writeJSON(w, http.StatusCreated, response)
	}
}

func (h *AuthHandler) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := &model.LoginRequest{}
		if err := json.NewDecoder(r.Body).Decode(request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if ok, err := h.validator.ValidateStruct(request); !ok {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		response, err := h.auth.Login(r.Context(), request)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidCredentials):
				writeError(w, http.StatusUnauthorized, err.Error())
			case errors.Is(err, service.ErrUniversityPending), errors.Is(err, service.ErrUniversityBlocked), errors.Is(err, service.ErrUniversityInactive):
				writeError(w, http.StatusForbidden, err.Error())
			default:
				writeError(w, http.StatusInternalServerError, "failed to authenticate")
			}
			return
		}

		writeJSON(w, http.StatusOK, response)
	}
}
