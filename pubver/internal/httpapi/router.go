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

	"pubver/internal/analytics"
	"pubver/internal/domain"
	"pubver/pkg/verifyhash"
)

type verificationService interface {
	VerifyPayload(ctx context.Context, token string) (domain.VerifyResponse, error)
	Search(ctx context.Context, vuzCode, diplomaNumber string) (domain.SearchResponse, error)
}

type analyticsTracker interface {
	Track(event analytics.VerificationEvent)
}

type Router struct {
	logger    *slog.Logger
	service   verificationService
	limiter   *RateLimiter
	analytics analyticsTracker
}

func NewRouter(logger *slog.Logger, requestTimeout time.Duration, limiter *RateLimiter, tracker analyticsTracker, verificationService verificationService) http.Handler {
	handler := &Router{
		logger:    logger,
		service:   verificationService,
		limiter:   limiter,
		analytics: tracker,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.healthz)
	mux.HandleFunc("GET /api/v1/verify", handler.verify)
	mux.HandleFunc("GET /api/v1/verify/search", handler.search)

	httpHandler := http.Handler(mux)
	httpHandler = withRateLimit(limiter, httpHandler)

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
	h.trackVerify(r, payload, response, err)
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
	h.trackSearch(r, vuzCode, response, err)
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

func (h *Router) trackVerify(r *http.Request, payload string, response domain.VerifyResponse, err error) {
	if h.analytics == nil {
		return
	}

	event := analytics.VerificationEvent{
		CreatedAt:   time.Now().UTC(),
		Source:      "pubver",
		Endpoint:    "verify",
		RequestID:   requestIDFromContext(r.Context()),
		ClientIP:    h.clientIP(r),
		UserAgent:   r.UserAgent(),
		DiplomaHash: response.Hash,
	}

	if claimsMap, decodeErr := verifyhash.DecodeUnverifiedJWT(payload); decodeErr == nil {
		if vuzID, extractErr := verifyhash.ExtractVUZIDFromMap(claimsMap); extractErr == nil {
			event.VUZID = vuzID
		}
		if outerClaims, outerErr := verifyhash.ExtractOuterQRClaimsFromMap(claimsMap); outerErr == nil && event.DiplomaHash == "" {
			event.DiplomaHash = outerClaims.DiplomaHash
		}
	}

	switch {
	case err == nil:
		event.Valid = response.Valid
		event.Status = string(response.Status)
		if response.VUZCode != "" {
			event.VUZCode = response.VUZCode
		}
	case errors.Is(err, domain.ErrInvalidInput):
		event.Status = "invalid_input"
	case errors.Is(err, domain.ErrInvalidPayload):
		event.Status = "invalid_payload"
	default:
		event.Status = "internal_error"
	}

	h.analytics.Track(event)
}

func (h *Router) trackSearch(r *http.Request, vuzCode string, response domain.SearchResponse, err error) {
	if h.analytics == nil {
		return
	}

	event := analytics.VerificationEvent{
		CreatedAt: time.Now().UTC(),
		Source:    "pubver",
		Endpoint:  "search",
		RequestID: requestIDFromContext(r.Context()),
		ClientIP:  h.clientIP(r),
		UserAgent: r.UserAgent(),
		VUZCode:   vuzCode,
	}

	switch {
	case err == nil:
		event.Valid = response.Valid
		event.Status = string(response.Status)
		if response.VUZCode != "" {
			event.VUZCode = response.VUZCode
		}
	case errors.Is(err, domain.ErrInvalidInput):
		event.Status = "invalid_input"
	case errors.Is(err, domain.ErrInvalidPayload):
		event.Status = "invalid_payload"
	default:
		event.Status = "internal_error"
	}

	h.analytics.Track(event)
}

func (h *Router) clientIP(r *http.Request) string {
	if h.limiter != nil {
		return h.limiter.extractClientIP(r)
	}

	if addr, ok := parseIP(r.RemoteAddr); ok {
		return addr.String()
	}
	return strings.TrimSpace(r.RemoteAddr)
}
