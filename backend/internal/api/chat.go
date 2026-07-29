package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/observability"
	"github.com/growdu/ai_all_in_one/backend/internal/routing"
)

// ChatHandler 处理 /api/v1/chat/completions
//
// 1.0 阶段：
//   - 鉴权：Authorization: Bearer <token>，1.0 简化版 token == cfg.JWTSecret
//   - 路由：
//     single:  req.Model 前缀推断 provider
//     auto:    req.Model == "auto" → 4 因子打分选 1，失败 fallback 1 次
//     compare: req.Compare != nil → 并行发 N 个 provider
//   - 流式透传：SSE chunk 直接转给上游，OpenAI 兼容格式
//   - 错误：统一 ErrorResponse 结构（docs/api/01-protocol.md §四）
type ChatHandler struct {
	Logger    *slog.Logger
	Registry  *core.Registry
	AuthToken string     // 1.0 简化：非空即要求；2.0 接 JWT
	Router    *routing.Router // single 模式可空，auto/compare 必须
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required", "", 0)
		return
	}
	if h.Registry == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "registry not initialized", "", 0)
		return
	}

	// 鉴权（1.0 简化）
	if h.AuthToken != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || auth[7:] == "" {
			writeError(w, http.StatusUnauthorized, "auth_missing", "missing bearer token", "", 0)
			return
		}
	}

	var req core.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "internal_error", "invalid JSON: "+err.Error(), "", 0)
		return
	}

	const userKey = "PLACEHOLDER_USER_KEY" // 1.0 简化；Phase 4 接入

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// ---- 路由模式分发 ----
	switch {
	case req.Compare != nil:
		// compare 模式
		h.serveCompare(ctx, w, req, userKey)
		return
	case req.Model == "auto":
		// auto 模式
		h.serveAuto(ctx, w, req, userKey)
		return
	}

	// single 模式（默认）
	provider, err := h.Registry.GetChat(inferProviderFromModel(req.Model))
	if err != nil {
		writeError(w, http.StatusNotFound, "model_not_found", "model not found: "+req.Model, "", 0)
		return
	}

	if req.Stream {
		h.serveStream(ctx, w, provider, req, userKey)
		return
	}
	h.serveComplete(ctx, w, provider, req, userKey)
}

// serveAuto auto 模式：打分选 1 + 失败 fallback
func (h *ChatHandler) serveAuto(ctx context.Context, w http.ResponseWriter, req core.ChatRequest, userKey string) {
	if h.Router == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "router not configured", "", 0)
		return
	}
	candidates := h.Registry.ChatProviders()
	if len(candidates) == 0 {
		writeError(w, http.StatusServiceUnavailable, "no_provider_configured", "no provider configured", "", 0)
		return
	}

	if req.Stream {
		// auto + stream: 先选 1，再走 serveStream
		chosen, err := h.Router.PickProvider(ctx, req, candidates)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
			return
		}
		// 用 chosen 第一个 model 调
		req.Model = firstModelOfProvider(h.Registry, chosen)
		provider, _ := h.Registry.GetChat(chosen)
		h.serveStream(ctx, w, provider, req, userKey)
		return
	}

	resp, _, err := h.Router.AutoChat(ctx, req, candidates, userKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, "all_providers_failed", err.Error(), "", 0)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
	observability.RecordChat(resp.Provider, req.Model, true, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
}

// serveCompare compare 模式：并行发 N 个 provider
func (h *ChatHandler) serveCompare(ctx context.Context, w http.ResponseWriter, req core.ChatRequest, userKey string) {
	if h.Router == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "router not configured", "", 0)
		return
	}
	candidates := req.Compare.Providers
	if len(candidates) == 0 {
		candidates = h.Registry.ChatProviders()
	}
	if len(candidates) < 2 {
		writeError(w, http.StatusBadRequest, "only_one_provider", "compare requires >= 2 providers", "", 0)
		return
	}

	if req.Stream {
		// compare + stream: 暂不实现 1.0，返回 501
		writeError(w, http.StatusNotImplemented, "not_implemented", "compare+stream Phase 2.5 stream variant", "", 0)
		return
	}

	results, err := h.Router.Compare(ctx, req, candidates, userKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), "", 0)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      "compare-" + req.Model,
		"compare": map[string]any{"results": results},
	})
}

// firstModelOfProvider 取 provider 第一个 model id（auto+stream 用）
func firstModelOfProvider(reg *core.Registry, name string) string {
	p, err := reg.GetChat(name)
	if err != nil {
		return name + "-default"
	}
	models := p.ListModels()
	if len(models) == 0 {
		return name + "-default"
	}
	return models[0].ID
}

func (h *ChatHandler) serveComplete(ctx context.Context, w http.ResponseWriter, provider core.ChatProvider, req core.ChatRequest, userKey string) {
	resp, err := provider.ChatComplete(ctx, req, userKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider_error", err.Error(), provider.Name(), 0)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
	observability.RecordChat(provider.Name(), req.Model, true, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
}

func (h *ChatHandler) serveStream(ctx context.Context, w http.ResponseWriter, provider core.ChatProvider, req core.ChatRequest, userKey string) {
	chunks, errs, closer, err := provider.ChatStream(ctx, req, userKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider_error", err.Error(), provider.Name(), 0)
		return
	}
	defer closer.Close()

	// 提前探测 http.Flusher 透传能力（statusRecorder 需实现 Flush）
	// 真正 flush 走 w 直接断言绕开 statusRecorder 包装
	_, _ = w.(http.Flusher)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-AIIO-Chat-Cache-Window", "300")
	w.WriteHeader(http.StatusOK)
	// flush header 一次，确保客户端能识别到 SSE stream
	// 直接对 w 做断言（绕开 statusRecorder 包装）
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	totalPrompt, totalCompletion := 0, 0
	sentAny := false
	for {
		select {
		case chunk, ok := <-chunks:
			if !ok {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			// 收尾 flush
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
				observability.RecordChat(provider.Name(), req.Model, sentAny, totalPrompt, totalCompletion)
				return
			}
			sentAny = true
			if chunk.Usage != nil {
				totalPrompt = chunk.Usage.PromptTokens
				totalCompletion = chunk.Usage.CompletionTokens
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			// flush 每次 chunk（用 w 原始断言绕开 statusRecorder 包装）
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case err := <-errs:
			if err != nil {
			errData, _ := json.Marshal(core.ErrorResponse{Error: core.ErrorBody{
				Code: "provider_error", Message: err.Error(), Provider: provider.Name(),
			}})
			fmt.Fprintf(w, "data: %s\n\n", errData)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
				observability.RecordChat(provider.Name(), req.Model, false, totalPrompt, totalCompletion)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// inferProviderFromModel 按 model id 前缀推断 provider
// "doubao-1-5-pro-32k" -> "doubao"
// "gpt-4o" -> "openai"
// "deepseek-chat" -> "deepseek"
func inferProviderFromModel(modelID string) string {
	// 1. 精确前缀匹配（顺序按长度倒序避免短前缀抢先）
	knownProviders := []string{
		"deepseek", "doubao", "claude", "openai", "kimi", "mock",
		"gpt", "gemini", "qwen", "llama", "mistral",
	}
	for _, p := range knownProviders {
		if strings.HasPrefix(modelID, p) {
			return canonicalProviderName(p)
		}
	}
	// 2. 兜底：按第一个 "-" 切
	if i := strings.Index(modelID, "-"); i > 0 {
		return modelID[:i]
	}
	return modelID
}

// canonicalProviderName 把 model id 前缀归一到 Provider 注册名
func canonicalProviderName(prefix string) string {
	switch prefix {
	case "gpt":
		return "openai"
	case "gemini":
		return "google"
	case "qwen":
		return "dashscope"
	case "llama":
		return "meta"
	case "mistral":
		return "mistral"
	default:
		return prefix
	}
}

func writeError(w http.ResponseWriter, status int, code, msg, provider string, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	}
	w.WriteHeader(status)
	body := core.ErrorResponse{Error: core.ErrorBody{
		Code: code, Message: msg, Provider: provider, RetryAfter: retryAfter,
	}}
	_ = json.NewEncoder(w).Encode(body)
}
