package mockprovider

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

// SlowProvider 名字为 "slow"，与 mock 区分
type SlowProvider struct{}

func NewSlow() *SlowProvider { return &SlowProvider{} }

func (p *SlowProvider) Name() string                       { return "slow" }
func (p *SlowProvider) Modality() core.Modality            { return core.ModalityChat }
func (p *SlowProvider) SupportsStream() bool               { return true }
func (p *SlowProvider) ListModels() []core.ModelInfo {
	return []core.ModelInfo{
		{
			ID: "slow-basic", DisplayName: "Slow (test, always succeeds)", Provider: "slow",
			Modality: core.ModalityChat, Capabilities: []string{"text", "stream"},
			ContextWindow: 4096,
		},
	}
}

func (p *SlowProvider) ChatComplete(ctx context.Context, req core.ChatRequest, userKey string) (core.ChatResponse, error) {
	last := lastUser(req)
	time.Sleep(50 * time.Millisecond) // 比 mock 慢
	return core.ChatResponse{
		ID: fmt.Sprintf("chatcmpl-slow-%d", time.Now().UnixNano()),
		Model: req.Model, Provider: "slow",
		Content: "slow: " + last,
		Usage:   core.ChatUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		Created: time.Now(),
	}, nil
}

func (p *SlowProvider) ChatStream(ctx context.Context, req core.ChatRequest, userKey string) (<-chan core.ChatChunk, <-chan error, io.Closer, error) {
	out := make(chan core.ChatChunk, 4)
	errs := make(chan error, 1)
	closer := io.NopCloser(nil)
	last := lastUser(req)
	go func() {
		defer close(out)
		out <- core.ChatChunk{ID: "slow", Delta: "slow: " + last, ChunkIndex: 0, FinishReason: "stop"}
	}()
	return out, errs, closer, nil
}

func lastUser(req core.ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			s := req.Messages[i].Content
			if strings.TrimSpace(s) == "" {
				return "hi"
			}
			return s
		}
	}
	return "hi"
}
