package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/growdu/ai_all_in_one/backend/internal/security"
)

// KeysHandler 处理 /api/v1/keys
// 1.0 阶段：CRUD 用户 API Key（明文提交，TLS 由部署层负责）
//
// POST   /api/v1/keys        body: { provider, key }        存
// GET    /api/v1/keys                                       列已配 provider
// DELETE /api/v1/keys/{provider}                            删
//
// 安全：返回时**不返回明文 Key**，只返回 provider 列表
type KeysHandler struct {
	Keyring *security.Keyring
}

type putKeyRequest struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

type putKeyResponse struct {
	Provider  string `json:"provider"`
	UpdatedAt int64  `json:"updated_at"`
}

type listKeysResponse struct {
	Providers []string `json:"providers"`
}

func (h *KeysHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Keyring == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "keyring not initialized", "", 0)
		return
	}

	// DELETE /api/v1/keys/{provider}
	if r.Method == http.MethodDelete {
		provider := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/")
		if provider == "" || provider == r.URL.Path {
			writeError(w, http.StatusBadRequest, "internal_error", "missing provider in path", "", 0)
			return
		}
		if err := h.Keyring.Delete(provider); err != nil {
			if errors.Is(err, security.ErrKeyNotFound) {
				writeError(w, http.StatusNotFound, "key_not_found", "no key for provider: "+provider, "", 0)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(listKeysResponse{Providers: h.Keyring.List()})
	case http.MethodPost:
		var req putKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "internal_error", "invalid JSON: "+err.Error(), "", 0)
			return
		}
		if req.Provider == "" || req.Key == "" {
			writeError(w, http.StatusBadRequest, "internal_error", "provider and key required", "", 0)
			return
		}
		entry, err := h.Keyring.PutWithMeta(req.Provider, req.Key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(putKeyResponse{Provider: entry.Provider, UpdatedAt: entry.UpdatedAt})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET/POST/DELETE required", "", 0)
	}
}
