package anthropiccompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

func newTestProvider(t *testing.T, handler http.HandlerFunc) *AnthropicCompat {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	p := New("test-anthropic", srv.URL+"/v1/messages", []core.ModelInfo{
		{ID: "test-model", DisplayName: "Test", Provider: "test-anthropic", Modality: core.ModalityChat},
	})
	return p
}

// TestSystemMessageExtracted 验证 system 消息被提到顶层，不在 messages 数组里
func TestSystemMessageExtracted(t *testing.T) {
	var capturedReq anthropicRequest
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","model":"test-model","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	})

	_, err := p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{
			{Role: "system", Content: "be helpful"},
			{Role: "user", Content: "hi"},
		},
	}, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if capturedReq.System != "be helpful" {
		t.Errorf("system = %q, want 'be helpful'", capturedReq.System)
	}
	if len(capturedReq.Messages) != 1 || capturedReq.Messages[0].Role != "user" {
		t.Errorf("messages = %+v, want only user", capturedReq.Messages)
	}
}

// TestMaxTokensRequired 验证 max_tokens 总是被设置
func TestMaxTokensRequired(t *testing.T) {
	var capturedReq anthropicRequest
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","model":"x","content":[{"type":"text","text":"x"}],"usage":{}}`))
	})

	// 用户没传
	_, _ = p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, "k")
	if capturedReq.MaxTokens == 0 {
		t.Errorf("max_tokens should default, got 0")
	}

	// 用户传了
	customMax := 256
	_, _ = p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
		MaxTokens: &customMax,
	}, "k")
	if capturedReq.MaxTokens != 256 {
		t.Errorf("max_tokens = %d, want 256", capturedReq.MaxTokens)
	}
}

// TestAuthorizationHeader 验证 Bearer token
func TestAuthorizationHeader(t *testing.T) {
	var gotAuth string
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"id":"x","model":"x","content":[{"type":"text","text":"x"}],"usage":{}}`))
	})
	_, _ = p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, "my-key")
	if gotAuth != "Bearer my-key" {
		t.Errorf("auth = %q, want 'Bearer my-key'", gotAuth)
	}
}

// TestResponseDecoding 验证响应解析
func TestResponseDecoding(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"msg_01",
			"model":"test-model",
			"content":[{"type":"text","text":"Hello!"}],
			"stop_reason":"end_turn",
			"usage":{"input_tokens":5,"output_tokens":3}
		}`))
	})
	resp, err := p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, "k")
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "msg_01" {
		t.Errorf("id = %q", resp.ID)
	}
	if resp.Content != "Hello!" {
		t.Errorf("content = %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 5 {
		t.Errorf("prompt = %d, want 5", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 3 {
		t.Errorf("completion = %d, want 3", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Errorf("total = %d, want 8", resp.Usage.TotalTokens)
	}
	if resp.Provider != "test-anthropic" {
		t.Errorf("provider = %q", resp.Provider)
	}
}

// TestErrorResponse 验证 4xx 错误透传
func TestErrorResponse(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid key"}}`))
	})
	_, err := p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, "bad-key")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %v, want contains '401'", err)
	}
}

// TestStreamingChunks 验证 SSE 流式解析
func TestStreamingChunks(t *testing.T) {
	sseBody := `event: message_start
data: {"type":"message_start","message":{"id":"msg_99","model":"test-model"}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"!"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}

`

	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		scanner := bufio.NewScanner(strings.NewReader(sseBody))
		for scanner.Scan() {
			_, _ = fmt.Fprintln(w, scanner.Text())
			if flusher != nil {
				flusher.Flush()
			}
		}
	})

	out, errs, closer, err := p.ChatStream(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
		Stream: true,
	}, "k")
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	var chunks []core.ChatChunk
	for c := range out {
		chunks = append(chunks, c)
	}
	// 检查 errs 没东西
	select {
	case e := <-errs:
		if e != nil {
			t.Errorf("errs = %v", e)
		}
	default:
	}

	if len(chunks) < 3 {
		t.Fatalf("chunks = %d, want >=3", len(chunks))
	}
	var combined string
	for _, c := range chunks {
		combined += c.Delta
	}
	if combined != "Hello!" {
		t.Errorf("combined delta = %q, want 'Hello!'", combined)
	}
	// 最后一个 chunk 应有 finish_reason
	last := chunks[len(chunks)-1]
	if last.FinishReason != "end_turn" {
		t.Errorf("last finish = %q", last.FinishReason)
	}
}

// TestPingAndOtherEvents 验证 ping 等无害事件被跳过
func TestPingAndOtherEvents(t *testing.T) {
	sseBody := `event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"X"}}

event: message_stop
data: {"type":"message_stop"}

`
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sseBody)
	})
	out, errs, closer, _ := p.ChatStream(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
		Stream: true,
	}, "k")
	defer closer.Close()

	var count int
	for range out {
		count++
	}
	select {
	case e := <-errs:
		if e != nil {
			t.Errorf("errs = %v", e)
		}
	default:
	}
	if count != 1 {
		t.Errorf("chunk count = %d, want 1 (only one text_delta)", count)
	}
}

// TestStreamErrorEvent 验证 Anthropic error 事件
func TestStreamErrorEvent(t *testing.T) {
	sseBody := `event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"server overloaded"}}
`
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sseBody)
	})
	out, errs, closer, _ := p.ChatStream(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
		Stream: true,
	}, "k")
	defer closer.Close()

	// chunks 应当空
	for range out {
	}
	// errs 应有错
	select {
	case e := <-errs:
		if e == nil {
			t.Error("want error from error event")
		} else if !strings.Contains(e.Error(), "overloaded") {
			t.Errorf("err = %v, want 'overloaded'", e)
		}
	default:
		t.Error("errs channel closed without error")
	}
}

// TestTemperatureTopP 透传字段
func TestTemperatureTopP(t *testing.T) {
	var capturedReq anthropicRequest
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedReq)
		_, _ = w.Write([]byte(`{"id":"x","model":"x","content":[{"type":"text","text":"x"}],"usage":{}}`))
	})

	temp := 0.7
	topP := 0.9
	_, _ = p.ChatComplete(context.Background(), core.ChatRequest{
		Model: "test-model",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
		Temperature: &temp,
		TopP: &topP,
	}, "k")
	if capturedReq.Temperature == nil || *capturedReq.Temperature != 0.7 {
		t.Errorf("temperature = %v, want 0.7", capturedReq.Temperature)
	}
	if capturedReq.TopP == nil || *capturedReq.TopP != 0.9 {
		t.Errorf("top_p = %v, want 0.9", capturedReq.TopP)
	}
}

// TestInterface 编译期断言
func TestInterface(t *testing.T) {
	var _ core.ChatProvider = (*AnthropicCompat)(nil)
	_ = bytes.NewBuffer // keep import
	time.Sleep(1 * time.Millisecond) // keep import
}