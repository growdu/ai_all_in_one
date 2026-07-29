// Package openaicompat 提供 OpenAI Chat Completions 兼容的 Provider 基类。
//
// 详见 docs/backend/02-provider.md §七。
//
// 适用：豆包 / DeepSeek / Kimi / 本地 llama.cpp / 任何 OpenAI 兼容服务。
// 1.0 阶段：每个具体厂商只配 base_url + 模型列表。
package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

// OpenAICompat 是所有 OpenAI 兼容厂商的基类
type OpenAICompat struct {
	name     string
	baseURL  string
	models   []core.ModelInfo
	hc       *http.Client
	provider string // 透传用
}

// New 构造一个 OpenAI 兼容 Provider
// baseURL 形如 "https://api.openai.com/v1/chat/completions"（含完整路径）
// 1.0 简化：不读 provider.config 里的 BaseURL（统一从 yaml 注入）
func New(name, baseURL string, models []core.ModelInfo) *OpenAICompat {
	return &OpenAICompat{
		name:     name,
		baseURL:  baseURL,
		models:   models,
		provider: name,
		hc: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *OpenAICompat) Name() string                  { return p.name }
func (p *OpenAICompat) Modality() core.Modality       { return core.ModalityChat }
func (p *OpenAICompat) SupportsStream() bool          { return true }
func (p *OpenAICompat) ListModels() []core.ModelInfo  { return p.models }

// ChatComplete 非流式：POST /chat/completions body 含 stream:false
func (p *OpenAICompat) ChatComplete(ctx context.Context, req core.ChatRequest, userKey string) (core.ChatResponse, error) {
	req.Stream = false
	body, err := json.Marshal(req)
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+userKey)

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return core.ChatResponse{}, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}

	var raw struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage core.ChatUsage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return core.ChatResponse{}, fmt.Errorf("decode: %w", err)
	}
	if len(raw.Choices) == 0 {
		return core.ChatResponse{}, fmt.Errorf("upstream returned 0 choices")
	}
	choice := raw.Choices[0]
	return core.ChatResponse{
		ID:      raw.ID,
		Model:   raw.Model,
		Content: choice.Message.Content,
		Usage:   raw.Usage,
		Created: time.Now(),
		Provider: p.name,
	}, nil
}

// ChatStream 流式：SSE 解析，每个 chunk 转为 core.ChatChunk
func (p *OpenAICompat) ChatStream(ctx context.Context, req core.ChatRequest, userKey string) (<-chan core.ChatChunk, <-chan error, io.Closer, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+userKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("upstream: %w", err)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, nil, nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}

	out := make(chan core.ChatChunk, 16)
	errs := make(chan error, 1)

	go p.parseSSE(ctx, resp.Body, out, errs)

	// closer: 关闭 resp.Body，会让 parseSSE 循环退出
	closer := &bodyCloser{body: resp.Body}
	return out, errs, closer, nil
}

type bodyCloser struct{ body io.ReadCloser }

func (c *bodyCloser) Close() error { return c.body.Close() }

// parseSSE 后台协程：读 SSE 流，转换为 ChatChunk
func (p *OpenAICompat) parseSSE(ctx context.Context, body io.Reader, out chan<- core.ChatChunk, errs chan<- error) {
	defer close(out)
	defer close(errs)

	chunkIdx := 0
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB 行缓冲
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			if payload == "[DONE]" {
				return
			}
			continue
		}
		var raw struct {
			ID      string `json:"id"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *core.ChatUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			// 跳过解析失败的 chunk（不致命）
			continue
		}
		if len(raw.Choices) == 0 {
			continue
		}
		choice := raw.Choices[0]
		chunk := core.ChatChunk{
			ID:         raw.ID,
			Delta:      choice.Delta.Content,
			ChunkIndex: chunkIdx,
		}
		if choice.FinishReason != "" {
			chunk.FinishReason = choice.FinishReason
		}
		if raw.Usage != nil {
			chunk.Usage = raw.Usage
		}
		chunkIdx++

		select {
		case out <- chunk:
		case <-ctx.Done():
			return
		}
	}
	if err := scanner.Err(); err != nil {
		errs <- fmt.Errorf("sse scan: %w", err)
	}
}
