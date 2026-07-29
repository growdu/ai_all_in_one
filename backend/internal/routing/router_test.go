package routing

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/mockprovider"
)

func TestAutoMode_SelectBest(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterChat(&fakeProvider{name: "doubao", delay: 100 * time.Millisecond, shouldFail: false})
	reg.RegisterChat(&fakeProvider{name: "openai", delay: 50 * time.Millisecond, shouldFail: false})

	signals := NewWindow(100, 0)
	// 训练：openai 总是更快 + 100% 成功
	for i := 0; i < 5; i++ {
		signals.Record(Signal{Provider: "openai", Success: true, LatencyMs: 50, Timestamp: time.Now()})
	}
	for i := 0; i < 5; i++ {
		signals.Record(Signal{Provider: "doubao", Success: true, LatencyMs: 200, Timestamp: time.Now()})
	}

	r := NewRouter(reg, signals, DefaultWeights(), 1)
	chosen, err := r.PickProvider(context.Background(), core.ChatRequest{
		Model: "auto",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, []string{"doubao", "openai"})
	if err != nil {
		t.Fatal(err)
	}
	if chosen != "openai" {
		t.Errorf("chosen = %q, want openai", chosen)
	}
}

func TestAutoMode_FallbackOnFailure(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterChat(&fakeProvider{name: "doubao", delay: 10 * time.Millisecond, shouldFail: false})
	reg.RegisterChat(&fakeProvider{name: "openai", delay: 10 * time.Millisecond, shouldFail: true})

	r := NewRouter(reg, NewWindow(100, 0), DefaultWeights(), 1)
	keyFor := func(name string) (string, error) { return "key-" + name, nil }
	resp, _, err := r.AutoChat(context.Background(), core.ChatRequest{
		Model:    "auto",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, []string{"doubao", "openai"}, "ignored", keyFor)
	if err != nil {
		t.Fatal(err)
	}
	// 应当 fallback 到 doubao
	if resp.Provider != "doubao" {
		t.Errorf("provider = %q, want doubao (fallback)", resp.Provider)
	}
}

func TestAutoMode_NoProviders(t *testing.T) {
	reg := core.NewRegistry()
	r := NewRouter(reg, NewWindow(100, 0), DefaultWeights(), 1)
	keyFor := func(name string) (string, error) { return "k", nil }
	_, _, err := r.AutoChat(context.Background(), core.ChatRequest{
		Model:    "auto",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, nil, "ignored", keyFor)
	if err == nil {
		t.Error("expected error for empty provider list")
	}
}

func TestCompareMode_AllSucceed(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterChat(&fakeProvider{name: "doubao", delay: 10 * time.Millisecond, shouldFail: false, content: "doubao-reply"})
	reg.RegisterChat(&fakeProvider{name: "openai", delay: 10 * time.Millisecond, shouldFail: false, content: "openai-reply"})

	r := NewRouter(reg, NewWindow(100, 0), DefaultWeights(), 1)
	keyFor := func(name string) (string, error) { return "key-" + name, nil }
	results, err := r.Compare(context.Background(), core.ChatRequest{
		Model:    "compare",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, []string{"doubao", "openai"}, "ignored", keyFor)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].Provider != "doubao" {
		t.Errorf("results[0].provider = %q", results[0].Provider)
	}
	if results[1].Provider != "openai" {
		t.Errorf("results[1].provider = %q", results[1].Provider)
	}
	// 按 latency 排序：doubao 先完成
	if results[0].Status != "succeeded" {
		t.Errorf("results[0].status = %q", results[0].Status)
	}
}

func TestCompareMode_SomeFailed(t *testing.T) {
	reg := core.NewRegistry()
	reg.RegisterChat(&fakeProvider{name: "doubao", delay: 10 * time.Millisecond, shouldFail: false})
	reg.RegisterChat(&fakeProvider{name: "openai", delay: 10 * time.Millisecond, shouldFail: true})

	r := NewRouter(reg, NewWindow(100, 0), DefaultWeights(), 1)
	keyFor := func(name string) (string, error) { return "key-" + name, nil }
	results, err := r.Compare(context.Background(), core.ChatRequest{
		Model:    "compare",
		Messages: []core.ChatMessage{{Role: "user", Content: "hi"}},
	}, []string{"doubao", "openai"}, "ignored", keyFor)
	if err != nil {
		t.Fatal(err)
	}
	// 没有 fatal err，但个别 provider 失败时返回 results + perProviderErrors
	failedCount := 0
	for _, r := range results {
		if r.Status == "failed" {
			failedCount++
		}
	}
	if failedCount != 1 {
		t.Errorf("failed count = %d, want 1", failedCount)
	}
}

// ----- fake provider -----

type fakeProvider struct {
	name        string
	delay       time.Duration
	shouldFail  bool
	content     string
	chunkDelay  time.Duration
}

func (p *fakeProvider) Name() string      { return p.name }
func (p *fakeProvider) Modality() core.Modality { return core.ModalityChat }
func (p *fakeProvider) SupportsStream() bool { return true }
func (p *fakeProvider) ListModels() []core.ModelInfo {
	return []core.ModelInfo{{ID: p.name + "-m1", Provider: p.name, Modality: core.ModalityChat, Capabilities: []string{"text", "stream"}, ContextWindow: 8000}}
}
func (p *fakeProvider) ChatComplete(ctx context.Context, req core.ChatRequest, key string) (core.ChatResponse, error) {
	select {
	case <-time.After(p.delay):
	case <-ctx.Done():
		return core.ChatResponse{}, ctx.Err()
	}
	if p.shouldFail {
		return core.ChatResponse{}, &fakeErr{msg: "fake failure"}
	}
	content := p.content
	if content == "" {
		content = "echo: " + req.Messages[len(req.Messages)-1].Content
	}
	return core.ChatResponse{
		ID: "chatcmpl-" + p.name, Model: req.Model, Content: content, Provider: p.name,
		Usage: core.ChatUsage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, nil
}
func (p *fakeProvider) ChatStream(ctx context.Context, req core.ChatRequest, key string) (<-chan core.ChatChunk, <-chan error, io.Closer, error) {
	out := make(chan core.ChatChunk, 4)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		out <- core.ChatChunk{ID: p.name, Delta: "x", ChunkIndex: 0}
	}()
	_ = errs
	_ = ctx
	return out, errs, io.NopCloser(nil), nil
}

type fakeErr struct{ msg string }
func (e *fakeErr) Error() string { return e.msg }

// silence unused import for sync
var _ = sync.Mutex{}
var _ = mockprovider.Name
