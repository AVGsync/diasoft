package handler

import (
	"context"
	"net/http"

	"github.com/diasoft/gateway-service/internal/model"
)

type VerifyUseCase interface {
	VerifyQRCode(ctx context.Context, payload string) (*model.VerifyResponse, error)
}

type VerifyHandler struct {
	verify VerifyUseCase
}

func NewVerifyHandler(verify VerifyUseCase) *VerifyHandler {
	return &VerifyHandler{verify: verify}
}

func (h *VerifyHandler) Verify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload := r.URL.Query().Get("payload")
		if payload == "" {
			writeError(w, http.StatusBadRequest, "missing payload")
			return
		}

		response, err := h.verify.VerifyQRCode(r.Context(), payload)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify diploma")
			return
		}

		writeJSON(w, http.StatusOK, response)
	}
}
