package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/diasoft/gateway-service/internal/authctx"
	"github.com/diasoft/gateway-service/internal/model"
)

type AnalyticsUseCase interface {
	UniversityVerificationStats(ctx context.Context, vuzID string, from, to time.Time) (*model.VerificationStatsResponse, error)
	AdminVerificationStats(ctx context.Context, from, to time.Time) (*model.VerificationStatsResponse, error)
}

type AnalyticsHandler struct {
	analytics AnalyticsUseCase
}

func NewAnalyticsHandler(analytics AnalyticsUseCase) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics}
}

func (h *AnalyticsHandler) UniversityVerificationStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		from, to, err := parseStatsRange(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		stats, err := h.analytics.UniversityVerificationStats(r.Context(), vuzID, from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load verification stats")
			return
		}

		writeJSON(w, http.StatusOK, stats)
	}
}

func (h *AnalyticsHandler) AdminVerificationStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, err := parseStatsRange(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		stats, err := h.analytics.AdminVerificationStats(r.Context(), from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load verification stats")
			return
		}

		writeJSON(w, http.StatusOK, stats)
	}
}

func parseStatsRange(r *http.Request) (time.Time, time.Time, error) {
	from, err := parseOptionalTime(r.URL.Query().Get("from"), false)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parseOptionalTime(r.URL.Query().Get("to"), true)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return from, to, nil
}

func parseOptionalTime(value string, endInclusive bool) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, nil
	}

	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, nil
	}

	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		if endInclusive {
			return parsed.AddDate(0, 0, 1), nil
		}
		return parsed, nil
	}

	return time.Time{}, ErrBadRequest("invalid time format, use RFC3339 or YYYY-MM-DD")
}

type ErrBadRequest string

func (e ErrBadRequest) Error() string {
	return string(e)
}
