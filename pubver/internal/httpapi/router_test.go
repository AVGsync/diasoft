package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type brokenJSON struct{}

func (brokenJSON) MarshalJSON() ([]byte, error) {
	return nil, errors.New("boom")
}

func TestWriteJSONFallsBackTo500OnEncodeError(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeJSON(recorder, http.StatusOK, brokenJSON{})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(recorder.Body.String(), "internal server error") {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}
