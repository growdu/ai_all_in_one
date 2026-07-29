package core

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
)

// fakeProvider 是测试用的 ChatProvider 实现
type fakeProvider struct {
	name  string
	models []ModelInfo
}

func (p *fakeProvider) Name() string { return p.name }
func (p *fakeProvider) Modality() Modality { return ModalityChat }
func (p *fakeProvider) ListModels() []ModelInfo { return p.models }
func (p *fakeProvider) SupportsStream() bool { return true }

func (p *fakeProvider) ChatComplete(ctx context.Context, req ChatRequest, userKey string) (ChatResponse, error) {
	return ChatResponse{
		ID:      "chatcmpl-fake",
		Model:   req.Model,
		Content: "fake response from " + p.name,
		Provider: p.name,
	}, nil
}

func (p *fakeProvider) ChatStream(ctx context.Context, req ChatRequest, userKey string) (<-chan ChatChunk, <-chan error, io.Closer, error) {
	out := make(chan ChatChunk, 2)
	errs := make(chan error, 1)
	closer := io.NopCloser(nil)
	go func() {
		defer close(out)
		out <- ChatChunk{ID: "chatcmpl-fake", Delta: "hello ", ChunkIndex: 0}
		out <- ChatChunk{ID: "chatcmpl-fake", Delta: "world", ChunkIndex: 1, FinishReason: "stop"}
	}()
	return out, errs, closer, nil
}

func TestRegistry_Register_And_Get(t *testing.T) {
	r := NewRegistry()
	p := &fakeProvider{name: "fake1", models: []ModelInfo{{ID: "m1", Provider: "fake1"}}}
	r.RegisterChat(p)

	got, err := r.GetChat("fake1")
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	if got.Name() != "fake1" {
		t.Errorf("Name = %q, want fake1", got.Name())
	}
}

func TestRegistry_GetChat_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.GetChat("nope")
	if err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestRegistry_AllModels(t *testing.T) {
	r := NewRegistry()
	r.RegisterChat(&fakeProvider{name: "a", models: []ModelInfo{
		{ID: "a-m1", Provider: "a", Modality: ModalityChat, Capabilities: []string{"text"}, ContextWindow: 1000},
	}})
	r.RegisterChat(&fakeProvider{name: "b", models: []ModelInfo{
		{ID: "b-m1", Provider: "b", Modality: ModalityChat, Capabilities: []string{"text"}, ContextWindow: 2000},
		{ID: "b-m2", Provider: "b", Modality: ModalityChat, Capabilities: []string{"text", "stream"}, ContextWindow: 4000},
	}})

	all := r.AllModels()
	if len(all) != 3 {
		t.Errorf("AllModels len = %d, want 3", len(all))
	}
}

func TestRegistry_ConcurrentRegister(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.RegisterChat(&fakeProvider{name: "p" + string(rune('A'+n%26))})
		}(i)
	}
	wg.Wait()
}

func TestProviderNotFound_Error(t *testing.T) {
	err := &ProviderNotFoundError{Name: "x"}
	if err.Error() != `provider "x" not found` {
		t.Errorf("Error() = %q", err.Error())
	}
	// 断言 errors.Is 不行（自己实现），但断言字符串 ok
	if !errors.Is(err, err) { // 占位用
		t.Log("ok")
	}
}
