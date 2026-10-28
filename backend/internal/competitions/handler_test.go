package competitions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"orenkod/backend/internal/httpserver"
)

func TestCreateRejectsInvalidJSON(t *testing.T) {
	h := NewHandler(NewService(nil), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/competitions", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), ErrInvalidJSON.Error()) {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestCreateRejectsMissingFields(t *testing.T) {
	h := NewHandler(NewService(nil), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/competitions", strings.NewReader(`{"title":""}`))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListUsesJSONEnvelope(t *testing.T) {
	// Keep the response helper contract used by frontend.
	rec := httptest.NewRecorder()
	httpserver.WriteJSON(rec, http.StatusOK, map[string]any{"items": []Competition{}})
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("content-type: %s", rec.Header().Get("Content-Type"))
	}
}
