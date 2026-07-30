package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/mockprovider"
)

func TestChatHandler_NonPOST(t *testing.T) {
	h := &ChatHandler{
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry: core.NewRegistry(),
	}
	req := httptest.NewRequest("GET", "/api/v1/chat/completions", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET status = %d, want 405", rw.Code)
	}
}

func TestChatHandler_InvalidJSON(t *testing.T) {
	reg := core.NewRegistry()
	h := &ChatHandler{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Registry: reg}
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader("not json"))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rw.Code)
	}
}

func TestChatHandler_ModelNotFound(t *testing.T) {
	reg := core.NewRegistry()
	h := &ChatHandler{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Registry: reg}
	body := bytes.NewBufferString(`{"model":"unknown-x","messages":[]}`)
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", body)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rw.Code)
	}
	var resp core.ErrorResponse
	json.NewDecoder(rw.Body).Decode(&resp)
	if resp.Error.Code != "model_not_found" {
		t.Errorf("code = %q", resp.Error.Code)
	}
}

func TestChatHandler_AuthMissing(t *testing.T) {
	reg := core.NewRegistry()
	h := &ChatHandler{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Registry:  reg,
		AuthToken: "secret-token",
	}
	body := bytes.NewBufferString(`{"model":"mock-echo","messages":[]}`)
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", body)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rw.Code)
	}
}

func TestChatHandler_Complete_Success(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterChat(mockprovider.New())
	h := &ChatHandler{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Registry: reg}

	body, _ := json.Marshal(core.ChatRequest{
		Model:    "mock-echo",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	})
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rw.Code, rw.Body.String())
	}
	var resp core.ChatResponse
	if err := json.NewDecoder(rw.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Content, "hi") {
		t.Errorf("content = %q, want echo of hi", resp.Content)
	}
	if resp.Provider != "mock" {
		t.Errorf("provider = %q", resp.Provider)
	}
}

func TestChatHandler_Stream_Success(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterChat(mockprovider.New())
	h := &ChatHandler{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Registry: reg}

	body, _ := json.Marshal(core.ChatRequest{
		Model:    "mock-echo",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
	})
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", bytes.NewReader(body))
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("status = %d", rw.Code)
	}
	if ct := rw.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("content-type = %q", ct)
	}
	// 解析 SSE
	var data []string
	scanner := bufio.NewScanner(rw.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
			data = append(data, line[6:])
		}
	}
	if len(data) < 3 {
		t.Errorf("expected multiple chunks, got %d: %v", len(data), data)
	}
	// 拼起来应当含 "hi"
	var combined string
	for _, d := range data {
		var c core.ChatChunk
		json.Unmarshal([]byte(d), &c)
		combined += c.Delta
	}
	if !strings.Contains(combined, "hi") {
		t.Errorf("combined stream = %q, want echo of hi", combined)
	}
	// 最后一个 chunk 应当有 finish_reason
	var last core.ChatChunk
	json.Unmarshal([]byte(data[len(data)-1]), &last)
	if last.FinishReason != "stop" {
		t.Errorf("last chunk finish_reason = %q, want stop", last.FinishReason)
	}
}

func TestInferProviderFromModel(t *testing.T) {
	cases := map[string]string{
		"doubao-1-5-pro-32k": "doubao",
		"gpt-4o":             "openai",
		"deepseek-chat":      "deepseek",
		"kimi-k2":            "kimi",
		"claude-3-opus":      "claude",
		"mock-echo":          "mock",
		"MiniMax-M3":         "minimax",
		"minimax-m3":         "minimax",
		"foo-bar-baz":        "foo",
	}
	for in, want := range cases {
		if got := inferProviderFromModel(in); got != want {
			t.Errorf("inferProviderFromModel(%q) = %q, want %q", in, got, want)
		}
	}
}
