package api

import (
	"encoding/json"
	"net/http"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

// ModelsHandler 处理 GET /api/v1/models
type ModelsHandler struct {
	Registry *core.Registry
}

func (h *ModelsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required", "", 0)
		return
	}
	models := h.Registry.AllModels()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"models": models})
}
