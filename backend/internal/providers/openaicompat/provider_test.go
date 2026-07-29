package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

// 起一个 mock OpenAI 兼容 server，返回预设响应
func newMockUpstream(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestProvider_ChatComplete_Success(t *testing.T) {
	upstream := newMockUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("auth header missing or wrong: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-xxx",
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "hello from upstream"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
		})
	})

	p := New("test", upstream.URL+"/chat/completions", []core.ModelInfo{
		{ID: "test-m1", Provider: "test", Modality: core.ModalityChat, Capabilities: []string{"text", "stream"}, ContextWindow: 8000},
	})

	resp, err := p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-m1",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, "test-key-12345")

	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello from upstream" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

func TestProvider_ChatComplete_UpstreamError(t *testing.T) {
	upstream := newMockUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream fail"}}`))
	})

	p := New("test", upstream.URL, []core.ModelInfo{{ID: "test-m1", Provider: "test", Modality: core.ModalityChat}})
	_, err := p.ChatComplete(context.Background(), core.ChatRequest{
		Model:    "test-m1",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, "k")
	if err == nil {
		t.Error("expected error on 500 upstream")
	}
}

func TestProvider_ChatStream_Success(t *testing.T) {
	upstream := newMockUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			"data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}",
			"data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"world\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":10,\"total_tokens\":15}}",
			"data: [DONE]",
		}
		for _, c := range chunks {
			_, _ = io.WriteString(w, c+"\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		}
	})

	p := New("test", upstream.URL, []core.ModelInfo{{ID: "test-m1", Provider: "test", Modality: core.ModalityChat, Capabilities: []string{"stream"}}})

	chunks, errs, closer, err := p.ChatStream(context.Background(), core.ChatRequest{
		Model:    "test-m1",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
	}, "k")
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	var combined string
	var lastChunk core.ChatChunk
	for c := range chunks {
		combined += c.Delta
		lastChunk = c
	}
	// errs 应当已 close
	for range errs {
	}

	if combined != "hello world" {
		t.Errorf("combined = %q, want 'hello world'", combined)
	}
	if lastChunk.FinishReason != "stop" {
		t.Errorf("lastChunk.FinishReason = %q, want stop", lastChunk.FinishReason)
	}
	if lastChunk.Usage == nil || lastChunk.Usage.TotalTokens != 15 {
		t.Errorf("usage = %+v", lastChunk.Usage)
	}
}

func TestProvider_ChatStream_ContextCancel(t *testing.T) {
	upstream := newMockUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(50 * time.Millisecond)
		}
	})

	p := New("test", upstream.URL, []core.ModelInfo{{ID: "test-m1", Provider: "test", Modality: core.ModalityChat}})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	chunks, _, closer, err := p.ChatStream(ctx, core.ChatRequest{
		Model: "test-m1", Messages: []core.ChatMessage{{Role: "user", Content: "x"}}, Stream: true,
	}, "k")
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()
	for range chunks {
		// drain
	}
}

func TestProvider_Name(t *testing.T) {
	p := New("myprovider", "http://x", nil)
	if p.Name() != "myprovider" {
		t.Errorf("name = %q", p.Name())
	}
	if p.Modality() != core.ModalityChat {
		t.Errorf("modality = %v, want chat", p.Modality())
	}
	if !p.SupportsStream() {
		t.Error("OpenAI compat supports stream")
	}
}

// 解析 SSE 的边界测试
func TestParseSSE_LineFormats(t *testing.T) {
	input := "" +
		"data: {\"a\":1}\n" +
		"\n" +
		"data:[DONE]\n" +
		"\n" +
		"data:   {\"b\":2}   \n" +
		"\n"
	scanner := bufio.NewScanner(bytes.NewReader([]byte(input)))
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") || strings.HasPrefix(line, "data:") {
			// 提取 data: 后内容
			payload := strings.TrimPrefix(line, "data:")
			payload = strings.TrimPrefix(payload, " ")
			payload = strings.TrimSpace(payload)
			events = append(events, payload)
		}
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3: %v", len(events), events)
	}
	if events[0] != `{"a":1}` {
		t.Errorf("event[0] = %q", events[0])
	}
	if events[1] != "[DONE]" {
		t.Errorf("event[1] = %q", events[1])
	}
	if events[2] != `{"b":2}` {
		t.Errorf("event[2] = %q", events[2])
	}
}
