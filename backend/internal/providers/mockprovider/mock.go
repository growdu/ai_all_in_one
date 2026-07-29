// Package mockprovider 是不依赖外部 API 的 ChatProvider 实现。
//
// 1.0 阶段用于：
//   - 端到端测试（httptest server 上跑 chat completion）
//   - 本地开发（无 Key 也能跑通 master → /api/v1/chat/completions）
//   - 演示 / 截图 / CI
//
// 2.0 阶段保留作为 fallback（"未配 Key 时回显"）。
package mockprovider

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

const Name = "mock"

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Name() string                       { return Name }
func (p *Provider) Modality() core.Modality            { return core.ModalityChat }
func (p *Provider) SupportsStream() bool               { return true }
func (p *Provider) ListModels() []core.ModelInfo {
	return []core.ModelInfo{
		{
			ID: "mock-echo", DisplayName: "Mock Echo (test)", Provider: Name,
			Modality: core.ModalityChat, Capabilities: []string{"text", "stream"},
			ContextWindow: 8192,
		},
		{
			ID: "mock-slow", DisplayName: "Mock Slow (test, 1s/chunk)", Provider: Name,
			Modality: core.ModalityChat, Capabilities: []string{"text", "stream"},
			ContextWindow: 8192,
		},
	}
}

func (p *Provider) ChatComplete(ctx context.Context, req core.ChatRequest, userKey string) (core.ChatResponse, error) {
	last := lastUserContent(req)
	return core.ChatResponse{
		ID: fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano()),
		Model: req.Model, Provider: Name,
		Content: "echo: " + last,
		Usage:   core.ChatUsage{PromptTokens: len(last) / 4, CompletionTokens: (len(last) + 5) / 4, TotalTokens: (2*len(last) + 5) / 4},
		Created: time.Now(),
	}, nil
}

func (p *Provider) ChatStream(ctx context.Context, req core.ChatRequest, userKey string) (<-chan core.ChatChunk, <-chan error, io.Closer, error) {
	out := make(chan core.ChatChunk, 16)
	errs := make(chan error, 1)
	closer := io.NopCloser(nil)

	delay := 0 * time.Millisecond
	if strings.HasPrefix(req.Model, "mock-slow") {
		delay = 1 * time.Second
	}

	last := lastUserContent(req)
	reply := "echo: " + last
	id := fmt.Sprintf("chatcmpl-mock-%d", time.Now().UnixNano())

	go func() {
		defer close(out)
		defer close(errs)
		words := strings.Split(reply, "")
		for i, w := range words {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			chunk := core.ChatChunk{ID: id, Delta: w, ChunkIndex: i}
			if i == len(words)-1 {
				chunk.FinishReason = "stop"
				chunk.Usage = &core.ChatUsage{
					PromptTokens:     len(last) / 4,
					CompletionTokens: (len(last) + 5) / 4,
				}
			}
			out <- chunk
		}
	}()
	return out, errs, closer, nil
}

func lastUserContent(req core.ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}
