package core

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// ChatProvider 是所有 chat 厂商必须实现的接口。
// 详细设计见 ../../../docs/backend/02-provider.md §二。
type ChatProvider interface {
	Name() string
	Modality() Modality
	ListModels() []ModelInfo
	SupportsStream() bool

	// ChatComplete 非流式调用
	ChatComplete(ctx context.Context, req ChatRequest, userKey string) (ChatResponse, error)

	// ChatStream 流式调用。返回 (chunks, errors, closer, error)
	//   - chunks 收完所有 chunk 后由 provider 主动 close
	//   - errors 第一次出错后由 provider 主动 close
	//   - closer 用于中断时清理底层连接（由调用方在停止时调用）
	ChatStream(ctx context.Context, req ChatRequest, userKey string) (<-chan ChatChunk, <-chan error, io.Closer, error)
}

// ProviderNotFoundError 注册表中找不到 Provider
type ProviderNotFoundError struct{ Name string }

func (e *ProviderNotFoundError) Error() string {
	return fmt.Sprintf("provider %q not found", e.Name)
}

// Registry 全局 Provider 注册表（单例）
type Registry struct {
	mu    sync.RWMutex
	chats map[string]ChatProvider
}

// NewRegistry 构造注册表
func NewRegistry() *Registry {
	return &Registry{chats: make(map[string]ChatProvider)}
}

// RegisterChat 注册一个 chat Provider。同名后者覆盖前者。
func (r *Registry) RegisterChat(p ChatProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chats[p.Name()] = p
}

// GetChat 按 name 取 chat Provider
func (r *Registry) GetChat(name string) (ChatProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.chats[name]
	if !ok {
		return nil, &ProviderNotFoundError{Name: name}
	}
	return p, nil
}

// AllModels 聚合所有 chat Provider 的模型清单。
// 供 GET /api/v1/models 使用。
func (r *Registry) AllModels() []ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ModelInfo
	for _, p := range r.chats {
		out = append(out, p.ListModels()...)
	}
	return out
}

// ChatProviders 列出所有 chat provider 名
func (r *Registry) ChatProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.chats))
	for n := range r.chats {
		names = append(names, n)
	}
	return names
}

// 编译期断言：ProviderNotFoundError 实现了 error 接口
var _ error = (*ProviderNotFoundError)(nil)
