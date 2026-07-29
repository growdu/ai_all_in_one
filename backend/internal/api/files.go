package api

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"

	"github.com/growdu/ai_all_in_one/backend/internal/storage"
)

// FilesHandler 处理 /api/v1/files
// 1.0 简化：单用户（owner 固定 "default"）
// 2.0 接 JWT 后用真实 user_id
type FilesHandler struct {
	Store       *storage.FileStore
	DefaultUser string
}

const defaultUser = "default"

func (h *FilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "file store not initialized", "", 0)
		return
	}
	owner := h.DefaultUser
	if owner == "" {
		owner = defaultUser
	}

	switch r.Method {
	case http.MethodPost:
		h.handleUpload(w, r, owner)
	case http.MethodGet:
		h.handleList(w, r, owner)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST/GET required", "", 0)
	}
}

// handleUpload 处理 multipart 上传
func (h *FilesHandler) handleUpload(w http.ResponseWriter, r *http.Request, owner string) {
	// 限制上传大小 10MB（HTTP 层先卡）
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)
	if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "internal_error", "multipart parse: "+err.Error(), "", 0)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "internal_error", "missing 'file' field: "+err.Error(), "", 0)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "read: "+err.Error(), "", 0)
		return
	}

	meta, err := h.Store.Put(owner, filepath.Base(header.Filename), header.Header.Get("Content-Type"), data)
	if err != nil {
		if errors.Is(err, storage.ErrUnsupportedMime) {
			writeError(w, http.StatusBadRequest, "file_unsupported", err.Error(), "", 0)
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = jsonEncode(w, meta)
}

func (h *FilesHandler) handleList(w http.ResponseWriter, r *http.Request, owner string) {
	list := h.Store.ListByOwner(owner)
	w.Header().Set("Content-Type", "application/json")
	_ = jsonEncode(w, map[string]any{"files": list})
}

// FileItemHandler 处理 /api/v1/files/{id} 的 GET 和 DELETE
type FileItemHandler struct {
	Store       *storage.FileStore
	DefaultUser string
}

func (h *FileItemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "file store not initialized", "", 0)
		return
	}
	owner := h.DefaultUser
	if owner == "" {
		owner = defaultUser
	}

	// 路径：/api/v1/files/{id}
	id := r.URL.Path
	if len(id) > len("/api/v1/files/") {
		id = id[len("/api/v1/files/"):]
	} else {
		writeError(w, http.StatusBadRequest, "internal_error", "missing file id", "", 0)
		return
	}

	switch r.Method {
	case http.MethodGet:
		meta, err := h.Store.GetMeta(id, owner)
		if err != nil {
			h.handleGetError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, meta)
	case http.MethodDelete:
		err := h.Store.Delete(id, owner)
		if err != nil {
			h.handleGetError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET/DELETE required", "", 0)
	}
}

func (h *FileItemHandler) handleGetError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrFileNotFound) {
		writeError(w, http.StatusNotFound, "key_not_found", err.Error(), "", 0)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
}
