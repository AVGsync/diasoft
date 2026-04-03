package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pubver/internal/domain"
)

type mockVerificationService struct {
	verifyPayloadFunc func(ctx context.Context, token string) (domain.VerifyResponse, error)
	searchFunc        func(ctx context.Context, vuzCode, diplomaNumber string) (domain.SearchResponse, error)
}

func (m *mockVerificationService) VerifyPayload(ctx context.Context, token string) (domain.VerifyResponse, error) {
	if m.verifyPayloadFunc == nil {
		return domain.VerifyResponse{}, nil
	}

	return m.verifyPayloadFunc(ctx, token)
}

func (m *mockVerificationService) Search(ctx context.Context, vuzCode, diplomaNumber string) (domain.SearchResponse, error) {
	if m.searchFunc == nil {
		return domain.SearchResponse{}, nil
	}

	return m.searchFunc(ctx, vuzCode, diplomaNumber)
}

func TestHealthzEndpointSuccess(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testLogger(), time.Second, &mockVerificationService{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusOK)
	}

	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Fatalf("unexpected health status: got %q want %q", response["status"], "ok")
	}
}

func TestVerifyEndpointSuccess(t *testing.T) {
	t.Parallel()

	var receivedPayload string
	var receivedRequestID string
	var hasDeadline bool

	handler := NewRouter(testLogger(), 2*time.Second, &mockVerificationService{
		verifyPayloadFunc: func(ctx context.Context, token string) (domain.VerifyResponse, error) {
			receivedPayload = token
			receivedRequestID = requestIDFromContext(ctx)
			_, hasDeadline = ctx.Deadline()

			return domain.VerifyResponse{
				Valid:         true,
				Status:        domain.DiplomaStatusActive,
				Hash:          "abc123",
				DiplomaNumber: "DVS-2024-001234",
				University:    "Bauman Moscow State Technical University",
				VUZCode:       "bmstu",
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verify?payload=%20jwt-token%20", nil)
	req.Header.Set("X-Request-ID", "req-123")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusOK)
	}

	if receivedPayload != "jwt-token" {
		t.Fatalf("unexpected payload: got %q want %q", receivedPayload, "jwt-token")
	}

	if receivedRequestID != "req-123" {
		t.Fatalf("unexpected request id in context: got %q want %q", receivedRequestID, "req-123")
	}

	if !hasDeadline {
		t.Fatalf("expected request timeout deadline in context")
	}

	if recorder.Header().Get("X-Request-ID") != "req-123" {
		t.Fatalf("unexpected x-request-id header: got %q want %q", recorder.Header().Get("X-Request-ID"), "req-123")
	}

	var response domain.VerifyResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.Valid || response.Status != domain.DiplomaStatusActive {
		t.Fatalf("unexpected verify response: %+v", response)
	}
}

func TestVerifyEndpointInvalidPayload(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testLogger(), time.Second, &mockVerificationService{
		verifyPayloadFunc: func(context.Context, string) (domain.VerifyResponse, error) {
			return domain.VerifyResponse{}, errors.Join(domain.ErrInvalidPayload, errors.New("broken signature"))
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verify?payload=bad", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusBadRequest)
	}

	assertJSONError(t, recorder, "invalid verification payload")
}

func TestSearchEndpointSuccess(t *testing.T) {
	t.Parallel()

	var receivedVUZCode string
	var receivedDiplomaNumber string

	handler := NewRouter(testLogger(), time.Second, &mockVerificationService{
		searchFunc: func(_ context.Context, vuzCode, diplomaNumber string) (domain.SearchResponse, error) {
			receivedVUZCode = vuzCode
			receivedDiplomaNumber = diplomaNumber

			year := 2024
			specialty := "Software Engineering"

			return domain.SearchResponse{
				Valid:      true,
				Status:     domain.DiplomaStatusActive,
				University: "Bauman Moscow State Technical University",
				VUZCode:    "bmstu",
				Year:       &year,
				Specialty:  &specialty,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verify/search?vuz_code=%20bmstu%20&diploma_number=%20DVS-2024-001234%20", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusOK)
	}

	if receivedVUZCode != "bmstu" {
		t.Fatalf("unexpected vuz_code: got %q want %q", receivedVUZCode, "bmstu")
	}

	if receivedDiplomaNumber != "DVS-2024-001234" {
		t.Fatalf("unexpected diploma_number: got %q want %q", receivedDiplomaNumber, "DVS-2024-001234")
	}

	var response domain.SearchResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.Valid || response.Status != domain.DiplomaStatusActive {
		t.Fatalf("unexpected search response: %+v", response)
	}
}

func TestSearchEndpointInvalidInput(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testLogger(), time.Second, &mockVerificationService{
		searchFunc: func(context.Context, string, string) (domain.SearchResponse, error) {
			return domain.SearchResponse{}, domain.ErrInvalidInput
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verify/search?vuz_code=&diploma_number=", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusBadRequest)
	}

	assertJSONError(t, recorder, domain.ErrInvalidInput.Error())
}

func TestSearchEndpointReturnsNullPlaceholdersWhenMetadataMissing(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testLogger(), time.Second, &mockVerificationService{
		searchFunc: func(context.Context, string, string) (domain.SearchResponse, error) {
			return domain.SearchResponse{
				Valid:      true,
				Status:     domain.DiplomaStatusActive,
				University: "Bauman Moscow State Technical University",
				VUZCode:    "bmstu",
				Year:       nil,
				Specialty:  nil,
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verify/search?vuz_code=bmstu&diploma_number=DVS-2024-001234", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := response["year"]; !ok {
		t.Fatalf("expected year placeholder in response")
	}
	if response["year"] != nil {
		t.Fatalf("expected year placeholder to be null, got %v", response["year"])
	}

	if _, ok := response["specialty"]; !ok {
		t.Fatalf("expected specialty placeholder in response")
	}
	if response["specialty"] != nil {
		t.Fatalf("expected specialty placeholder to be null, got %v", response["specialty"])
	}
}

func TestVerifyEndpointPanicRecovery(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testLogger(), time.Second, &mockVerificationService{
		verifyPayloadFunc: func(context.Context, string) (domain.VerifyResponse, error) {
			panic("boom")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verify?payload=panic", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusInternalServerError)
	}

	assertJSONError(t, recorder, "internal server error")
}

func TestVerifyEndpointGeneratesRequestIDWhenMissing(t *testing.T) {
	t.Parallel()

	var receivedRequestID string

	handler := NewRouter(testLogger(), time.Second, &mockVerificationService{
		verifyPayloadFunc: func(ctx context.Context, token string) (domain.VerifyResponse, error) {
			receivedRequestID = requestIDFromContext(ctx)
			return domain.VerifyResponse{Valid: false, Status: domain.DiplomaStatusNotFound}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verify?payload=jwt-token", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusOK)
	}

	if receivedRequestID == "" {
		t.Fatalf("expected generated request id in context")
	}

	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected generated x-request-id header")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assertJSONError(t *testing.T, recorder *httptest.ResponseRecorder, expected string) {
	t.Helper()

	var response map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if response["error"] != expected {
		t.Fatalf("unexpected error message: got %q want %q", response["error"], expected)
	}
}
