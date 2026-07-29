package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/security"
)

func newTestKeyring(t *testing.T) *security.Keyring {
	t.Helper()
	dir := t.TempDir()
	kr, err := security.NewKeyring(
		filepath.Join(dir, "keyring.json"),
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatal(err)
	}
	return kr
}

func newTestKeysHandler(t *testing.T) *KeysHandler {
	t.Helper()
	return &KeysHandler{Keyring: newTestKeyring(t)}
}

func TestKeysHandler_PutGet(t *testing.T) {
	h := newTestKeysHandler(t)

	// PUT
	body, _ := json.Marshal(map[string]string{"provider": "doubao", "key": "sk-test-12345"})
	req := httptest.NewRequest("POST", "/api/v1/keys", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", rw.Code, rw.Body.String())
	}

	// GET 列表
	req = httptest.NewRequest("GET", "/api/v1/keys", nil)
	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rw.Code)
	}
	var list listKeysResponse
	json.NewDecoder(rw.Body).Decode(&list)
	if len(list.Providers) != 1 || list.Providers[0] != "doubao" {
		t.Errorf("list = %v, want [doubao]", list.Providers)
	}
}

func TestKeysHandler_PutMissingFields(t *testing.T) {
	h := newTestKeysHandler(t)
	req := httptest.NewRequest("POST", "/api/v1/keys", strings.NewReader(`{"provider":""}`))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestKeysHandler_Delete(t *testing.T) {
	h := newTestKeysHandler(t)
	// 先存
	body, _ := json.Marshal(map[string]string{"provider": "openai", "key": "sk-openai"})
	req := httptest.NewRequest("POST", "/api/v1/keys", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	// DELETE
	req = httptest.NewRequest("DELETE", "/api/v1/keys/openai", nil)
	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusNoContent {
		t.Errorf("DELETE status = %d, want 204", rw.Code)
	}

	// 再 GET 列表应空
	req = httptest.NewRequest("GET", "/api/v1/keys", nil)
	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	var list listKeysResponse
	json.NewDecoder(rw.Body).Decode(&list)
	if len(list.Providers) != 0 {
		t.Errorf("after delete, list = %v, want []", list.Providers)
	}
}

func TestKeysHandler_DeleteNotFound(t *testing.T) {
	h := newTestKeysHandler(t)
	req := httptest.NewRequest("DELETE", "/api/v1/keys/never-existed", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rw.Code)
	}
}

func TestKeysHandler_NoKeyring(t *testing.T) {
	h := &KeysHandler{Keyring: nil}
	req := httptest.NewRequest("GET", "/api/v1/keys", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rw.Code)
	}
}

func TestKeysHandler_NoPlaintextLeak(t *testing.T) {
	// 关键安全：GET 不返回明文 Key
	h := newTestKeysHandler(t)
	body, _ := json.Marshal(map[string]string{"provider": "doubao", "key": "sk-SECRET-LEAK-ME"})
	req := httptest.NewRequest("POST", "/api/v1/keys", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	req = httptest.NewRequest("GET", "/api/v1/keys", nil)
	rw = httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if strings.Contains(rw.Body.String(), "SECRET") {
		t.Errorf("GET response leaks key: %s", rw.Body.String())
	}
}

// silence unused imports
var (
	_ = io.Discard
	_ = os.Create
	_ = slog.Default
)
