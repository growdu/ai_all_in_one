package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/mockprovider"
)

func TestModelsHandler(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterChat(mockprovider.New())
	h := &ModelsHandler{Registry: reg}

	req := httptest.NewRequest("GET", "/api/v1/models", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}
	var body struct {
		Models []core.ModelInfo `json:"models"`
	}
	if err := json.NewDecoder(rw.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Models) < 1 {
		t.Errorf("models count = %d, want >= 1", len(body.Models))
	}
	for _, m := range body.Models {
		if m.Provider != "mock" {
			t.Errorf("model %s provider = %q", m.ID, m.Provider)
		}
	}
}

func TestModelsHandler_MethodNotAllowed(t *testing.T) {
	h := &ModelsHandler{Registry: core.NewRegistry()}
	req := httptest.NewRequest("POST", "/api/v1/models", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rw.Code)
	}
}
