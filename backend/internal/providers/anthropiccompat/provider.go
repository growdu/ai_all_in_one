// Package anthropiccompat 提供 Anthropic Messages API 兼容的 Provider 基类。
//
// 详见 docs/backend/02-provider.md §七（与 openaicompat 并列）。
//
// 适用：minimax（Anthropic 兼容端点）/ Anthropic Claude / 任何遵循
// https://docs.anthropic.com/en/api/messages 协议的服务。
//
// 与 openaicompat 的关键差异：
//   - system 消息是独立字段，不在 messages 数组里
//   - max_tokens 必填
//   - 响应 content 是 block 数组 [{type:"text", text:"..."}]
//   - usage 是 input_tokens / output_tokens
//   - SSE 事件类型不同（message_start / content_block_delta / message_stop 等）
//   - 1.0 简化：用 Authorization: Bearer 头（minimax 的实测行为），
//     不使用 Anthropic 原生的 x-api-key
package anthropiccompat

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

// AnthropicCompat Anthropic Messages 协议 Provider 基类
type AnthropicCompat struct {
	name     string
	baseURL  string // 形如 https://api.minimaxi.com/anthropic/v1/messages
	models   []core.ModelInfo
	hc       *http.Client
	provider string

	// DefaultMaxTokens Anthropic 必填；用户没传时取这个
	DefaultMaxTokens int
}

// New 构造一个 Anthropic 兼容 Provider
func New(name, baseURL string, models []core.ModelInfo) *AnthropicCompat {
	return &AnthropicCompat{
		name:     name,
		baseURL:  baseURL,
		models:   models,
		provider: name,
		hc: &http.Client{
			Timeout: 60 * time.Second,
		},
		DefaultMaxTokens: 4096,
	}
}

func (p *AnthropicCompat) Name() string                 { return p.name }
func (p *AnthropicCompat) Modality() core.Modality      { return core.ModalityChat }
func (p *AnthropicCompat) SupportsStream() bool         { return true }
func (p *AnthropicCompat) ListModels() []core.ModelInfo { return p.models }

// ---- 协议内部结构 ----

// anthropicRequest 发送的请求体
type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
	// 透传字段
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	Stop        []string `json:"stop_seqs,omitempty"` // Anthropic 字段名
}

// anthropicMessage 单条消息
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse 非流式响应
type anthropicResponse struct {
	ID           string             `json:"id"`
	Model        string             `json:"model"`
	Content      []anthropicContent `json:"content"`
	StopReason   string             `json:"stop_reason"`
	Usage        anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// 构造请求体：把 OpenAI 风格的 ChatRequest 转为 Anthropic 风格
//   - system 提到顶层
//   - max_tokens 必填
//   - 去掉 OpenAI 特有字段（attachments / tool_call_id 等）
func (p *AnthropicCompat) buildRequest(req core.ChatRequest) anthropicRequest {
	var systemPrompt string
	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			// system 提到顶层
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += m.Content
		case "user", "assistant":
			msgs = append(msgs, anthropicMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		default:
			// tool / function 消息：1.0 简化，丢弃或合并到上一条 user
			// Anthropic 协议下工具调用要重新设计；1.0 阶段不支持
		}
	}

	maxTokens := p.DefaultMaxTokens
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	return anthropicRequest{
		Model:       req.Model,
		Messages:    msgs,
		System:      systemPrompt,
		MaxTokens:   maxTokens,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}
}

// ChatComplete 非流式
func (p *AnthropicCompat) ChatComplete(ctx context.Context, req core.ChatRequest, userKey string) (core.ChatResponse, error) {
	ar := p.buildRequest(req)
	ar.Stream = false
	body, err := json.Marshal(ar)
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+userKey)
	// minimax 兼容层可能接受 anthropic-version 也保留无害
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.hc.Do(httpReq)
	if err != nil {
		return core.ChatResponse{}, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return core.ChatResponse{}, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}

	var ares anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ares); err != nil {
		return core.ChatResponse{}, fmt.Errorf("decode: %w", err)
	}
	// content blocks → 平铺文本
	var content string
	for _, b := range ares.Content {
		if b.Type == "text" {
			content += b.Text
		}
	}
	return core.ChatResponse{
		ID:       ares.ID,
		Model:    ares.Model,
		Content:  content,
		Usage: core.ChatUsage{
			PromptTokens:     ares.Usage.InputTokens,
			CompletionTokens: ares.Usage.OutputTokens,
			TotalTokens:      ares.Usage.InputTokens + ares.Usage.OutputTokens,
		},
		Created:  time.Now(),
		Provider: p.name,
	}, nil
}

// ChatStream 流式
func (p *AnthropicCompat) ChatStream(ctx context.Context, req core.ChatRequest, userKey string) (<-chan core.ChatChunk, <-chan error, io.Closer, error) {
	ar := p.buildRequest(req)
	ar.Stream = true
	body, err := json.Marshal(ar)
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
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
	closer := &bodyCloser{body: resp.Body}
	go p.parseSSE(ctx, resp.Body, out, errs)
	return out, errs, closer, nil
}

type bodyCloser struct{ body io.ReadCloser }

func (c *bodyCloser) Close() error { return c.body.Close() }

// SSE 事件类型（Anthropic）
//
//	event: message_start
//	data: {"type":"message_start","message":{...}}
//
//	event: content_block_start
//	data: {"type":"content_block_start","index":0,"content_block":{...}}
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}
//
//	event: content_block_stop
//	data: {"type":"content_block_stop","index":0}
//
//	event: message_delta
//	data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}
//
//	event: message_stop
//	data: {"type":"message_stop"}
//
//	event: ping
//	data: {"type":"ping"}
//
//	event: error
//	data: {"type":"error","error":{"type":"api_error","message":"..."}}
func (p *AnthropicCompat) parseSSE(ctx context.Context, body io.Reader, out chan<- core.ChatChunk, errs chan<- error) {
	defer close(out)
	defer close(errs)

	chunkIdx := 0
	var msgID string
	var totalUsage core.ChatUsage

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "event: "):
			// event 行仅用于调试，生产环境不依赖其内容（data 自带 type）
			continue
		case strings.HasPrefix(line, "data: "):
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(payload), &raw); err != nil {
				continue
			}

			// 提取 type 字段（事件类型判断）
			var typ string
			if t, ok := raw["type"]; ok {
				_ = json.Unmarshal(t, &typ)
			}

			switch typ {
			case "message_start":
				// 提取 message.id
				if m, ok := raw["message"]; ok {
					var msg struct {
						ID string `json:"id"`
					}
					_ = json.Unmarshal(m, &msg)
					msgID = msg.ID
				}
			case "content_block_delta":
				// 提取 delta.text
				var delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}
				if d, ok := raw["delta"]; ok {
					_ = json.Unmarshal(d, &delta)
				}
				if delta.Text == "" {
					continue
				}
				chunk := core.ChatChunk{
					ID:         msgID,
					Delta:      delta.Text,
					ChunkIndex: chunkIdx,
				}
				chunkIdx++
				select {
				case out <- chunk:
				case <-ctx.Done():
					return
				}
			case "message_delta":
				// 提取 delta.stop_reason + usage
				var delta struct {
					StopReason string `json:"stop_reason"`
				}
				if d, ok := raw["delta"]; ok {
					_ = json.Unmarshal(d, &delta)
				}
				var usage struct {
					OutputTokens int `json:"output_tokens"`
				}
				if u, ok := raw["usage"]; ok {
					_ = json.Unmarshal(u, &usage)
					totalUsage.CompletionTokens = usage.OutputTokens
				}
				if delta.StopReason != "" {
					finalChunk := core.ChatChunk{
						ID:           msgID,
						ChunkIndex:   chunkIdx,
						FinishReason: delta.StopReason,
						Usage:        &totalUsage,
					}
					chunkIdx++
					select {
					case out <- finalChunk:
					case <-ctx.Done():
						return
					}
				}
			case "message_stop":
				return
			case "error":
				var errBody struct {
					Error struct {
						Message string `json:"message"`
						Type    string `json:"type"`
					} `json:"error"`
				}
				_ = json.Unmarshal([]byte(payload), &errBody)
				errs <- fmt.Errorf("upstream error %s: %s", errBody.Error.Type, errBody.Error.Message)
				return
			case "ping", "content_block_start", "content_block_stop":
				// 忽略
			}
		}
		// 其它行（空行 / 注释）忽略
	}
	if err := scanner.Err(); err != nil {
		errs <- fmt.Errorf("sse scan: %w", err)
	}
}