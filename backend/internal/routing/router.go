// Package routing 实现 Master 端的请求路由策略。
//
// 详见 docs/architecture/01-routing-strategy.md
package routing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

// Router 三模式路由：single / auto / compare
type Router struct {
	registry         *core.Registry
	signals          *Window
	weights          Weights
	maxAutoFallback  int // auto 模式失败最多换几家
	mu               sync.Mutex
}

// NewRouter 创建 router
func NewRouter(reg *core.Registry, signals *Window, w Weights, maxAutoFallback int) *Router {
	if maxAutoFallback < 0 {
		maxAutoFallback = 1
	}
	return &Router{
		registry:        reg,
		signals:         signals,
		weights:         w,
		maxAutoFallback: maxAutoFallback,
	}
}

// PickProvider 从候选 list 里选 1 个（auto 模式内部用）
// 当前实现：按 ScoreProvider 排序取最高
func (r *Router) PickProvider(ctx context.Context, req core.ChatRequest, candidates []string) (string, error) {
	if len(candidates) == 0 {
		return "", errors.New("no candidates")
	}
	scores := make(map[string]float64, len(candidates))
	for _, name := range candidates {
		p, err := r.registry.GetChat(name)
		if err != nil {
			continue
		}
		models := p.ListModels()
		scores[name] = ScoreProvider(name, models, r.signals, r.weights, 5*time.Minute)
	}
	if len(scores) == 0 {
		return "", errors.New("no valid candidates")
	}
	// 取最高分
	best := ""
	bestScore := -1.0
	for name, s := range scores {
		if s > bestScore {
			bestScore = s
			best = name
		}
	}
	return best, nil
}

// AutoChat auto 模式：选 1 + 失败 fallback 最多 maxAutoFallback 次
// keyFor 是回调：从 provider 名取 user key（用于 Keyring 注入）
func (r *Router) AutoChat(ctx context.Context, req core.ChatRequest, candidates []string, userKey string, keyFor func(string) (string, error)) (core.ChatResponse, []core.AttachmentInfo, error) {
	if len(candidates) == 0 {
		return core.ChatResponse{}, nil, errors.New("no_provider_configured")
	}
	// 按分数排序
	scored := make([]string, 0, len(candidates))
	scored = append(scored, candidates...)
	sort.SliceStable(scored, func(i, j int) bool {
		pi, _ := r.registry.GetChat(scored[i])
		pj, _ := r.registry.GetChat(scored[j])
		si := ScoreProvider(scored[i], pi.ListModels(), r.signals, r.weights, 5*time.Minute)
		sj := ScoreProvider(scored[j], pj.ListModels(), r.signals, r.weights, 5*time.Minute)
		return si > sj
	})

	attempts := 0
	maxAttempts := r.maxAutoFallback + 1
	var lastErr error
	for _, name := range scored {
		if attempts >= maxAttempts {
			break
		}
		attempts++
		p, err := r.registry.GetChat(name)
		if err != nil {
			lastErr = err
			continue
		}
		// 取该 provider 的真 Key；缺失时跳过
		provKey, err := keyFor(name)
		if err != nil {
			lastErr = fmt.Errorf("no key for %s: %w", name, err)
			continue
		}
		start := time.Now()
		resp, err := p.ChatComplete(ctx, req, provKey)
		latency := time.Since(start).Milliseconds()
		r.signals.Record(Signal{
			Provider: name, Success: err == nil, LatencyMs: int(latency), Timestamp: time.Now(),
		})
		if err == nil {
			return resp, nil, nil
		}
		lastErr = err
	}
	return core.ChatResponse{}, nil, lastErr
}

// CompareResult compare 模式单 provider 结果
type CompareResult struct {
	Provider   string                `json:"provider"`
	Status     string                `json:"status"` // succeeded / failed
	LatencyMs  int                   `json:"latency_ms"`
	Content    string                `json:"content,omitempty"`
	Usage      *core.ChatUsage       `json:"usage,omitempty"`
	Error      *core.ErrorBody       `json:"error,omitempty"`
	StartedAt  time.Time             `json:"started_at"`
}

// Compare 并行发 N 个 Provider
// keyFor 是回调：从 provider 名取 user key
func (r *Router) Compare(ctx context.Context, req core.ChatRequest, candidates []string, userKey string, keyFor func(string) (string, error)) ([]CompareResult, error) {
	if len(candidates) < 2 {
		return nil, errors.New("only_one_provider")
	}
	results := make([]CompareResult, len(candidates))
	var wg sync.WaitGroup
	for i, name := range candidates {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			p, err := r.registry.GetChat(name)
			if err != nil {
				results[i] = CompareResult{Provider: name, Status: "failed", StartedAt: time.Now(), Error: &core.ErrorBody{Code: "provider_not_found", Message: err.Error()}}
				return
			}
			provKey, kerr := keyFor(name)
			if kerr != nil {
				results[i] = CompareResult{Provider: name, Status: "failed", StartedAt: time.Now(), Error: &core.ErrorBody{Code: "no_provider_configured", Message: kerr.Error(), Provider: name}}
				return
			}
			start := time.Now()
			resp, err := p.ChatComplete(ctx, req, provKey)
			latency := time.Since(start).Milliseconds()
			r.signals.Record(Signal{Provider: name, Success: err == nil, LatencyMs: int(latency), Timestamp: time.Now()})
			if err != nil {
				results[i] = CompareResult{Provider: name, Status: "failed", LatencyMs: int(latency), StartedAt: start, Error: &core.ErrorBody{Code: "provider_error", Message: err.Error(), Provider: name}}
				return
			}
			results[i] = CompareResult{
				Provider: name, Status: "succeeded", LatencyMs: int(latency), StartedAt: start,
				Content: resp.Content, Usage: &resp.Usage,
			}
		}(i, name)
	}
	wg.Wait()
	return results, nil
}
