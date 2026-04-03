package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"
)

func TestWithRequestIDUsesIncomingHeader(t *testing.T) {
	t.Parallel()

	var receivedRequestID string

	handler := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequestID = requestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "req-existing")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusNoContent)
	}

	if receivedRequestID != "req-existing" {
		t.Fatalf("unexpected request id in context: got %q want %q", receivedRequestID, "req-existing")
	}

	if recorder.Header().Get("X-Request-ID") != "req-existing" {
		t.Fatalf("unexpected response request id: got %q want %q", recorder.Header().Get("X-Request-ID"), "req-existing")
	}
}

func TestWithRequestIDGeneratesHeader(t *testing.T) {
	t.Parallel()

	var receivedRequestID string

	handler := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequestID = requestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusNoContent)
	}

	if receivedRequestID == "" {
		t.Fatalf("expected generated request id in context")
	}

	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected generated request id in response header")
	}
}

func TestWithRecoverReturnsInternalServerError(t *testing.T) {
	t.Parallel()

	handler := withRecover(testLogger(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusInternalServerError)
	}

	assertJSONError(t, recorder, "internal server error")
}

func TestWithLoggingWritesRequestMetadata(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, nil))

	handler := withLogging(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestIDFromContext(r.Context()) != "req-log" {
			t.Fatalf("unexpected request id in context")
		}

		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req = req.WithContext(withRequestIDContext(context.Background(), "req-log"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("unexpected status code: got %d want %d", recorder.Code, http.StatusCreated)
	}

	entry := decodeLogEntry(t, buffer.String())

	if entry["msg"] != "http request" {
		t.Fatalf("unexpected log message: %v", entry["msg"])
	}

	if entry["request_id"] != "req-log" {
		t.Fatalf("unexpected logged request id: %v", entry["request_id"])
	}

	if entry["method"] != http.MethodPost {
		t.Fatalf("unexpected logged method: %v", entry["method"])
	}

	if entry["path"] != "/api/v1/verify" {
		t.Fatalf("unexpected logged path: %v", entry["path"])
	}

	if entry["status"] != float64(http.StatusCreated) {
		t.Fatalf("unexpected logged status: %v", entry["status"])
	}

	if entry["remote_addr"] != "127.0.0.1:12345" {
		t.Fatalf("unexpected logged remote_addr: %v", entry["remote_addr"])
	}
}

func decodeLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()

	line := strings.TrimSpace(raw)
	if line == "" {
		t.Fatalf("expected log output")
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}

	return entry
}
