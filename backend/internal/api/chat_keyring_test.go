package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/routing"
	"github.com/growdu/ai_all_in_one/backend/internal/security"
)

// keyCapturingProvider 记录收到的 userKey
type keyCapturingProvider struct {
	name     string
	gotKey   string
	gotKeyMu sync.Mutex
}

func (p *keyCapturingProvider) Name() string      { return p.name }
func (p *keyCapturingProvider) Modality() core.Modality { return core.ModalityChat }
func (p *keyCapturingProvider) SupportsStream() bool { return false }
func (p *keyCapturingProvider) ListModels() []core.ModelInfo {
	return []core.ModelInfo{{ID: p.name + "-m1", Provider: p.name, Modality: core.ModalityChat, Capabilities: []string{"text"}, ContextWindow: 8000}}
}
func (p *keyCapturingProvider) ChatComplete(ctx context.Context, req core.ChatRequest, userKey string) (core.ChatResponse, error) {
	p.gotKeyMu.Lock()
	p.gotKey = userKey
	p.gotKeyMu.Unlock()
	return core.ChatResponse{
		ID: "test", Model: req.Model, Content: "ok", Provider: p.name,
		Usage: core.ChatUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, nil
}
func (p *keyCapturingProvider) ChatStream(ctx context.Context, req core.ChatRequest, userKey string) (<-chan core.ChatChunk, <-chan error, io.Closer, error) {
	out := make(chan core.ChatChunk)
	errs := make(chan error)
	closer := io.NopCloser(nil)
	close(out)
	close(errs)
	return out, errs, closer, nil
}

func newTestKeyringForChat(t *testing.T) *security.Keyring {
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

func TestChatHandler_UsesKeyFromKeyring(t *testing.T) {
	kr := newTestKeyringForChat(t)
	// 预存 doubao 的 key
	if err := kr.Put("doubao", "sk-real-secret-key-abc"); err != nil {
		t.Fatal(err)
	}

	prov := &keyCapturingProvider{name: "doubao"}
	reg := core.NewRegistry()
	reg.RegisterChat(prov)

	h := &ChatHandler{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry: reg,
		Keyring:  kr,
	}

	body, _ := json.Marshal(core.ChatRequest{
		Model:    "doubao-m1",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	})
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rw.Code, rw.Body.String())
	}

	prov.gotKeyMu.Lock()
	got := prov.gotKey
	prov.gotKeyMu.Unlock()

	if got != "sk-real-secret-key-abc" {
		t.Errorf("provider got key = %q, want sk-real-secret-key-abc", got)
	}
}

func TestChatHandler_NoKeyConfigured(t *testing.T) {
	kr := newTestKeyringForChat(t)
	// 不存 doubao 的 key

	prov := &keyCapturingProvider{name: "doubao"}
	reg := core.NewRegistry()
	reg.RegisterChat(prov)

	h := &ChatHandler{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry: reg,
		Keyring:  kr,
	}

	body, _ := json.Marshal(core.ChatRequest{
		Model:    "doubao-m1",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	})
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (no_provider_configured)", rw.Code)
	}
	var resp core.ErrorResponse
	json.NewDecoder(rw.Body).Decode(&resp)
	if resp.Error.Code != "no_provider_configured" {
		t.Errorf("code = %q", resp.Error.Code)
	}
}

func TestChatHandler_NoKeyringFallback(t *testing.T) {
	// 1.0 阶段：Keyring 字段为 nil 时 fallback 到 PLACEHOLDER
	prov := &keyCapturingProvider{name: "doubao"}
	reg := core.NewRegistry()
	reg.RegisterChat(prov)

	h := &ChatHandler{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry: reg,
		Keyring:  nil, // 1.0 简化：未配 keyring 时 fallback
	}

	body, _ := json.Marshal(core.ChatRequest{
		Model:    "doubao-m1",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	})
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}

	prov.gotKeyMu.Lock()
	got := prov.gotKey
	prov.gotKeyMu.Unlock()

	if got != "PLACEHOLDER_USER_KEY" {
		t.Errorf("got = %q, want PLACEHOLDER_USER_KEY (fallback)", got)
	}
}

func TestChatHandler_AutoMode_UsesKeyring(t *testing.T) {
	kr := newTestKeyringForChat(t)
	kr.Put("doubao", "key-for-doubao")
	kr.Put("openai", "key-for-openai")

	prov1 := &keyCapturingProvider{name: "doubao"}
	prov2 := &keyCapturingProvider{name: "openai"}
	reg := core.NewRegistry()
	reg.RegisterChat(prov1)
	reg.RegisterChat(prov2)

	signals := routing.NewWindow(100, 0)
	router := routing.NewRouter(reg, signals, routing.DefaultWeights(), 1)

	h := &ChatHandler{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry: reg,
		Router:   router,
		Keyring:  kr,
	}

	body, _ := json.Marshal(core.ChatRequest{
		Model:    "auto",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	})
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}
	// 至少一个 provider 收到了它的 key
	prov1.gotKeyMu.Lock()
	k1 := prov1.gotKey
	prov1.gotKeyMu.Unlock()
	prov2.gotKeyMu.Lock()
	k2 := prov2.gotKey
	prov2.gotKeyMu.Unlock()

	if k1 != "key-for-doubao" && k2 != "key-for-openai" {
		t.Errorf("auto mode: neither provider got correct key (k1=%q, k2=%q)", k1, k2)
	}
}

// silence unused
var (
	_ = errors.New
	_ = os.Create
	_ = strings.Contains
)
