package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWithRateLimitRejectsSecondRequestFromSameIP(t *testing.T) {
	handler := withRateLimit(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  1,
			Burst:           1,
			VisitorTTL:      time.Minute,
			CleanupInterval: time.Minute,
		},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/v1/verify", nil)
	firstRequest.RemoteAddr = "10.0.0.1:1234"
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", firstResponse.Code, http.StatusOK)
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/verify", nil)
	secondRequest.RemoteAddr = "10.0.0.1:4321"
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)

	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", secondResponse.Code, http.StatusTooManyRequests)
	}
	if secondResponse.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is empty")
	}
}

func TestWithRateLimitAllowsHealthzWithoutLimiting(t *testing.T) {
	handler := withRateLimit(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		RateLimitConfig{
			Enabled:         true,
			RequestsPerSec:  1,
			Burst:           1,
			VisitorTTL:      time.Minute,
			CleanupInterval: time.Minute,
		},
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	for i := 0; i < 3; i++ {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		request.RemoteAddr = "10.0.0.1:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("healthz status = %d, want %d", response.Code, http.StatusOK)
		}
	}
}
