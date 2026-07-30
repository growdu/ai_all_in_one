package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/storage"
)

func newTestConvsHandler(t *testing.T) *ConvsHandler {
	t.Helper()
	dir := t.TempDir()
	fs := storage.NewFileStore(
		filepath.Join(dir, "files"),
		filepath.Join(dir, "index.json"),
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	return &ConvsHandler{Store: fs, DefaultUser: "user1"}
}

func TestConvsHandler_CreateList(t *testing.T) {
	h := newTestConvsHandler(t)

	// Create
	body, _ := json.Marshal(map[string]string{"model": "mock-echo", "mode": "single"})
	req := httptest.NewRequest("POST", "/api/v1/conversations", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rw.Code, rw.Body.String())
	}
	var conv storage.Conversation
	json.Unmarshal(rw.Body.Bytes(), &conv)
	if conv.ID == "" {
		t.Error("ID should be set")
	}
	if conv.OwnerID != "user1" {
		t.Errorf("owner = %q", conv.OwnerID)
	}

	// List
	req = httptest.NewRequest("GET", "/api/v1/conversations", nil)
	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("list status = %d", rw.Code)
	}
	var resp struct {
		Conversations []storage.Conversation `json:"conversations"`
	}
	json.Unmarshal(rw.Body.Bytes(), &resp)
	if len(resp.Conversations) != 1 {
		t.Errorf("list = %d, want 1", len(resp.Conversations))
	}
}

func TestConvsHandler_NoStore(t *testing.T) {
	h := &ConvsHandler{Store: nil}
	req := httptest.NewRequest("GET", "/api/v1/conversations", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rw.Code)
	}
}

func TestConvItemHandler_Get(t *testing.T) {
	h := newTestConvsHandler(t)
	body, _ := json.Marshal(map[string]string{"model": "x"})
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/conversations", bytes.NewReader(body))
	h.ServeHTTP(rw, req)
	var c storage.Conversation
	json.Unmarshal(rw.Body.Bytes(), &c)

	item := &ConvItemHandler{Store: h.Store, DefaultUser: "user1"}
	req = httptest.NewRequest("GET", "/api/v1/conversations/"+c.ID, nil)
	rw = httptest.NewRecorder()
	item.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("status = %d", rw.Code)
	}
}

func TestConvItemHandler_Delete(t *testing.T) {
	h := newTestConvsHandler(t)
	body, _ := json.Marshal(map[string]string{"model": "x"})
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/conversations", bytes.NewReader(body))
	h.ServeHTTP(rw, req)
	var c storage.Conversation
	json.Unmarshal(rw.Body.Bytes(), &c)

	item := &ConvItemHandler{Store: h.Store, DefaultUser: "user1"}
	req = httptest.NewRequest("DELETE", "/api/v1/conversations/"+c.ID, nil)
	rw = httptest.NewRecorder()
	item.ServeHTTP(rw, req)
	if rw.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rw.Code)
	}
}

func TestConvItemHandler_NotFound(t *testing.T) {
	h := newTestConvsHandler(t)
	item := &ConvItemHandler{Store: h.Store, DefaultUser: "user1"}
	req := httptest.NewRequest("GET", "/api/v1/conversations/conv_xxx", nil)
	rw := httptest.NewRecorder()
	item.ServeHTTP(rw, req)
	if rw.Code == http.StatusOK {
		t.Errorf("status = %d, want non-200", rw.Code)
	}
}

func TestConvItemHandler_PatchTitle(t *testing.T) {
	h := newTestConvsHandler(t)
	body, _ := json.Marshal(map[string]string{"model": "x"})
	rw := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/conversations", bytes.NewReader(body))
	h.ServeHTTP(rw, req)
	var c storage.Conversation
	json.Unmarshal(rw.Body.Bytes(), &c)

	item := &ConvItemHandler{Store: h.Store, DefaultUser: "user1"}
	patchBody, _ := json.Marshal(map[string]string{"title": "新标题"})
	req = httptest.NewRequest("PATCH", "/api/v1/conversations/"+c.ID, bytes.NewReader(patchBody))
	rw = httptest.NewRecorder()
	item.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("status = %d", rw.Code)
	}
	var got storage.Conversation
	json.Unmarshal(rw.Body.Bytes(), &got)
	if got.Title != "新标题" {
		t.Errorf("title = %q", got.Title)
	}
}
