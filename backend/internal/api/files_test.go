package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/storage"
)

func newTestFileHandler(t *testing.T) *FilesHandler {
	t.Helper()
	dir := t.TempDir()
	fs := storage.NewFileStore(
		filepath.Join(dir, "files"),
		filepath.Join(dir, "index.json"),
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	return &FilesHandler{
		Store:       fs,
		DefaultUser: "user1",
	}
}

func TestFilesHandler_Upload(t *testing.T) {
	h := newTestFileHandler(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	// 显式设 file 字段 Content-Type 为 image/png
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="test.png"`)
	hdr.Set("Content-Type", "image/png")
	fw, _ := w.CreatePart(hdr)
	_, _ = fw.Write([]byte("fake-png-data"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/files", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rw.Code, rw.Body.String())
	}
	var meta storage.FileMeta
	if err := json.Unmarshal(rw.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.ID == "" {
		t.Error("ID should be set")
	}
	if meta.OwnerID != "user1" {
		t.Errorf("owner = %q", meta.OwnerID)
	}
	if meta.Size != int64(len("fake-png-data")) {
		t.Errorf("size = %d", meta.Size)
	}
}

func TestFilesHandler_UploadUnsupportedMime(t *testing.T) {
	h := newTestFileHandler(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "virus.exe")
	_, _ = fw.Write([]byte("MZ\x00\x00"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/files", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestFilesHandler_UploadMissingFile(t *testing.T) {
	h := newTestFileHandler(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.Close() // 没有 file 字段

	req := httptest.NewRequest("POST", "/api/v1/files", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestFilesHandler_List(t *testing.T) {
	h := newTestFileHandler(t)
	h.Store.Put("user1", "a.png", "image/png", []byte("a"))
	h.Store.Put("user1", "b.png", "image/png", []byte("bb"))

	req := httptest.NewRequest("GET", "/api/v1/files", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}
	var resp struct {
		Files []storage.FileMeta `json:"files"`
	}
	json.Unmarshal(rw.Body.Bytes(), &resp)
	if len(resp.Files) != 2 {
		t.Errorf("files count = %d, want 2", len(resp.Files))
	}
}

func TestFilesHandler_NoStore(t *testing.T) {
	h := &FilesHandler{Store: nil}
	req := httptest.NewRequest("GET", "/api/v1/files", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rw.Code)
	}
}

func TestFileItemHandler_Get(t *testing.T) {
	h := newTestFileHandler(t)
	meta, _ := h.Store.Put("user1", "x.png", "image/png", []byte("data"))

	item := &FileItemHandler{Store: h.Store, DefaultUser: "user1"}
	req := httptest.NewRequest("GET", "/api/v1/files/"+meta.ID, nil)
	rw := httptest.NewRecorder()
	item.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}
}

func TestFileItemHandler_Delete(t *testing.T) {
	h := newTestFileHandler(t)
	meta, _ := h.Store.Put("user1", "x.png", "image/png", []byte("data"))

	item := &FileItemHandler{Store: h.Store, DefaultUser: "user1"}
	req := httptest.NewRequest("DELETE", "/api/v1/files/"+meta.ID, nil)
	rw := httptest.NewRecorder()
	item.ServeHTTP(rw, req)

	if rw.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rw.Code)
	}
}

func TestFileItemHandler_NotFound(t *testing.T) {
	h := newTestFileHandler(t)
	item := &FileItemHandler{Store: h.Store, DefaultUser: "user1"}
	req := httptest.NewRequest("GET", "/api/v1/files/never-existed", nil)
	rw := httptest.NewRecorder()
	item.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rw.Code)
	}
}

// silence unused
var _ = io.Discard
