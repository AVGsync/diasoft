package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"pubver/internal/domain"
)

type verificationService interface {
	VerifyPayload(ctx context.Context, token string) (domain.VerifyResponse, error)
	Search(ctx context.Context, vuzCode, diplomaNumber string) (domain.SearchResponse, error)
}

type RateLimitConfig struct {
	Enabled         bool
	RequestsPerSec  float64
	Burst           int
	VisitorTTL      time.Duration
	CleanupInterval time.Duration
}

type Router struct {
	logger  *slog.Logger
	service verificationService
}

func NewRouter(logger *slog.Logger, requestTimeout time.Duration, rateLimitConfig RateLimitConfig, verificationService verificationService) http.Handler {
	handler := &Router{
		logger:  logger,
		service: verificationService,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.healthz)
	mux.HandleFunc("GET /api/v1/verify", handler.verify)
	mux.HandleFunc("GET /api/v1/verify/search", handler.search)

	httpHandler := http.Handler(mux)
	if rateLimitConfig.Enabled {
		httpHandler = withRateLimit(logger, rateLimitConfig, httpHandler)
	}

	return withRequestTimeout(
		requestTimeout,
		withRecover(
			logger,
			withRequestID(
				withLogging(logger, httpHandler),
			),
		),
	)
}

func (h *Router) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Router) verify(w http.ResponseWriter, r *http.Request) {
	payload := strings.TrimSpace(r.URL.Query().Get("payload"))

	response, err := h.service.VerifyPayload(r.Context(), payload)
	if err != nil {
		h.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Router) search(w http.ResponseWriter, r *http.Request) {
	diplomaNumber := strings.TrimSpace(r.URL.Query().Get("diploma_number"))
	vuzCode := strings.TrimSpace(r.URL.Query().Get("vuz_code"))

	response, err := h.service.Search(r.Context(), vuzCode, diplomaNumber)
	if err != nil {
		h.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Router) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidPayload):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid verification payload"})
	default:
		h.logger.Error("request failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(payload); err != nil {
		slog.Error("encode json response", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("{\"error\":\"internal server error\"}\n"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buffer.Bytes())
}

func withRequestTimeout(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
