package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestWithRateLimitRejectsSecondRequestFromSameIP(t *testing.T) {
	redisServer := miniredis.RunT(t)
	limiter, err := NewRateLimiter(
		t.Context(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		RateLimitConfig{
			Enabled:           true,
			VerifyRPS:         1,
			VerifyBurst:       1,
			SearchRPS:         1,
			SearchBurst:       1,
			KeyTTL:            time.Minute,
			TrustedProxyCIDRs: nil,
			Redis: RedisConfig{
				Addr:         redisServer.Addr(),
				KeyPrefix:    "test:pubver",
				DialTimeout:  time.Second,
				ReadTimeout:  time.Second,
				WriteTimeout: time.Second,
			},
		},
	)
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}
	t.Cleanup(func() {
		_ = limiter.Close()
	})

	handler := withRateLimit(limiter, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

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
	redisServer := miniredis.RunT(t)
	limiter, err := NewRateLimiter(
		t.Context(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		RateLimitConfig{
			Enabled:           true,
			VerifyRPS:         1,
			VerifyBurst:       1,
			SearchRPS:         1,
			SearchBurst:       1,
			KeyTTL:            time.Minute,
			TrustedProxyCIDRs: nil,
			Redis: RedisConfig{
				Addr:         redisServer.Addr(),
				KeyPrefix:    "test:pubver",
				DialTimeout:  time.Second,
				ReadTimeout:  time.Second,
				WriteTimeout: time.Second,
			},
		},
	)
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}
	t.Cleanup(func() {
		_ = limiter.Close()
	})

	handler := withRateLimit(limiter, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

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

func TestExtractClientIPIgnoresSpoofedForwardedHeadersWithoutTrustedProxy(t *testing.T) {
	redisServer := miniredis.RunT(t)
	limiter, err := NewRateLimiter(
		t.Context(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		RateLimitConfig{
			Enabled:     true,
			VerifyRPS:   1,
			VerifyBurst: 1,
			SearchRPS:   1,
			SearchBurst: 1,
			KeyTTL:      time.Minute,
			Redis: RedisConfig{
				Addr:         redisServer.Addr(),
				KeyPrefix:    "test:pubver",
				DialTimeout:  time.Second,
				ReadTimeout:  time.Second,
				WriteTimeout: time.Second,
			},
		},
	)
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}
	t.Cleanup(func() { _ = limiter.Close() })

	request := httptest.NewRequest(http.MethodGet, "/api/v1/verify", nil)
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.10")

	clientIP := limiter.extractClientIP(request)
	if clientIP != "10.0.0.1" {
		t.Fatalf("extractClientIP() = %q, want %q", clientIP, "10.0.0.1")
	}
}
