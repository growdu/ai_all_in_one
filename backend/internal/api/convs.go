package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/growdu/ai_all_in_one/backend/internal/storage"
)

// ConvsHandler /api/v1/conversations
type ConvsHandler struct {
	Store       *storage.FileStore
	DefaultUser string
}

func (h *ConvsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "store not initialized", "", 0)
		return
	}
	owner := h.DefaultUser
	if owner == "" {
		owner = "default"
	}

	repo := storage.NewConvRepo(h.Store)

	switch r.Method {
	case http.MethodGet:
		h.list(w, r, owner, repo)
	case http.MethodPost:
		h.create(w, r, owner, repo)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET/POST required", "", 0)
	}
}

func (h *ConvsHandler) list(w http.ResponseWriter, r *http.Request, owner string, repo *storage.ConvRepo) {
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	list, err := repo.List(owner, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = jsonEncode(w, map[string]any{"conversations": list})
}

type createConvReq struct {
	Model string `json:"model"`
	Mode  string `json:"mode"`
}

func (h *ConvsHandler) create(w http.ResponseWriter, r *http.Request, owner string, repo *storage.ConvRepo) {
	var req createConvReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "internal_error", "invalid JSON: "+err.Error(), "", 0)
		return
	}
	if req.Model == "" {
		req.Model = "mock-echo"
	}
	if req.Mode == "" {
		req.Mode = "single"
	}
	c, err := repo.Create(owner, req.Model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
		return
	}
	c.Mode = req.Mode
	// 1.0 简化：mode 不存回文件（只读用）
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = jsonEncode(w, c)
}

// ConvItemHandler /api/v1/conversations/{id}
type ConvItemHandler struct {
	Store       *storage.FileStore
	DefaultUser string
}

func (h *ConvItemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "store not initialized", "", 0)
		return
	}
	owner := h.DefaultUser
	if owner == "" {
		owner = "default"
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
	if id == "" || id == r.URL.Path {
		writeError(w, http.StatusBadRequest, "internal_error", "missing conv id", "", 0)
		return
	}

	repo := storage.NewConvRepo(h.Store)
	msgRepo := storage.NewMsgRepo(h.Store)

	switch r.Method {
	case http.MethodGet:
		msgs, conv, err := msgRepo.ListByConv(id, owner)
		if err != nil {
			h.handleErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, map[string]any{
			"conversation": conv,
			"messages":     msgs,
		})
	case http.MethodPatch:
		var body struct {
			Title  *string `json:"title,omitempty"`
			Pinned *bool   `json:"pinned,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "internal_error", "invalid JSON: "+err.Error(), "", 0)
			return
		}
		if body.Title != nil {
			if err := repo.UpdateTitle(id, owner, *body.Title); err != nil {
				h.handleErr(w, err)
				return
			}
		}
		if body.Pinned != nil {
			if err := repo.Pin(id, owner, *body.Pinned); err != nil {
				h.handleErr(w, err)
				return
			}
		}
		// 返回更新后的 conv
		c, err := repo.Get(id, owner)
		if err != nil {
			h.handleErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, c)
	case http.MethodDelete:
		if err := repo.Delete(id, owner); err != nil {
			h.handleErr(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET/PATCH/DELETE required", "", 0)
	}
}

func (h *ConvItemHandler) handleErr(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrConvNotFound) || errors.Is(err, storage.ErrFileNotFound) {
		writeError(w, http.StatusNotFound, "conv_not_found", "conversation not found", "", 0)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
}
