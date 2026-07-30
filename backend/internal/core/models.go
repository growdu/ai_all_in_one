package core

import "time"

// ModelInfo 描述一个可被前端选择的模型。
// JSON 字段严格遵循 docs/api/01-protocol.md §1.1 / §1.1.1。
type ModelInfo struct {
	ID               string    `json:"id"`
	DisplayName      string    `json:"display_name"`
	Provider         string    `json:"provider"`
	Modality         Modality  `json:"modality"`
	Capabilities     []string  `json:"capabilities"`
	ContextWindow    int       `json:"context_window"`
	InputPricePer1k  *float64  `json:"input_price_per_1k,omitempty"`
	OutputPricePer1k *float64  `json:"output_price_per_1k,omitempty"`
	// 复杂能力（见 docs/api/01-protocol.md §1.1.1）
	ToolsSpec     *ToolsSpec     `json:"tools,omitempty"`
	ReasoningSpec *ReasoningSpec `json:"reasoning,omitempty"`
	ImageSpec     *ImageSpec     `json:"image,omitempty"`
}

// ToolsSpec function calling 详细配置
type ToolsSpec struct {
	ParallelCalls bool `json:"parallel_calls"`
	MaxSteps      int  `json:"max_steps"`
}

// ReasoningSpec 推理增强配置
type ReasoningSpec struct {
	Effort string `json:"effort"` // low | medium | high
}

// ImageSpec 图像生成配置（2.0+，预留）
type ImageSpec struct {
	Sizes   []string `json:"sizes"`
	Quality []string `json:"quality"`
	Edit    bool     `json:"edit"`
}

// ChatMessage 单条消息
// 与 OpenAI Chat Completions 的 message 字段兼容。
type ChatMessage struct {
	Role        string   `json:"role"`         // system / user / assistant / tool
	Content     string   `json:"content"`
	Attachments []string `json:"attachments,omitempty"` // 本系统 file_id 列表
	Name        string   `json:"name,omitempty"`        // tool 消息用
	ToolCallID  string   `json:"tool_call_id,omitempty"`
}

// ChatRequest 一次 chat 请求
// 与 OpenAI Chat Completions 请求体兼容 + 扩展。
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
	// OpenAI 标准透传字段
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	ResponseFormat   any      `json:"response_format,omitempty"`
	Tools            any      `json:"tools,omitempty"`
	ToolChoice       any      `json:"tool_choice,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	User             string   `json:"user,omitempty"`
	// 本系统扩展
	InputProcessing *InputProcessing `json:"input_processing,omitempty"`
	Compare         *CompareSpec     `json:"compare,omitempty"`
	// ConvID 可选：非空时把 user/assistant 消息自动写入该会话。
	// 2.0 改为必填。详见 docs/frontend/03-web.md §3.4。
	ConvID string `json:"conv_id,omitempty"`
	// Attachments 顶层附件列表（前端便利字段）；
	// 后端预处理时会合并到最后一条 user 消息的 Attachments 字段。
	Attachments []string `json:"attachments,omitempty"`
}

// InputProcessing 输入处理选项（review C）
type InputProcessing struct {
	PreprocessAttachments bool     `json:"preprocess_attachments"` // 1.0 永远 true
	EnhancePrompt         bool     `json:"enhance_prompt"`
	Rules                 []string `json:"rules,omitempty"` // 子规则；空=默认
}

// CompareSpec 对比模式（详见 docs/architecture/01-routing-strategy.md）
type CompareSpec struct {
	Providers []string `json:"providers"` // "all_configured" 或 provider id 列表
	Format    string   `json:"format"`   // side_by_side | stacked
}

// ChatChunk SSE 流式响应的一个 chunk
// chunk_index 是为断线重连（docs/api/01-protocol.md §1.2.1）准备的。
type ChatChunk struct {
	ID           string    `json:"id"`
	Delta        string    `json:"delta"`
	ChunkIndex   int       `json:"chunk_index"`
	FinishReason string    `json:"finish_reason,omitempty"`
	Provider     string    `json:"provider,omitempty"` // compare 模式用
	Usage        *ChatUsage `json:"usage,omitempty"`
}

// ChatUsage token 用量
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse 非流式响应
type ChatResponse struct {
	ID      string     `json:"id"`
	Model   string     `json:"model"`
	Content string     `json:"content"`
	Usage   ChatUsage  `json:"usage"`
	Created time.Time  `json:"created"`
	Provider string    `json:"provider"`
}

// ProcessingInfo 响应里的处理信息（review C）
type ProcessingInfo struct {
	Attachments     []AttachmentInfo `json:"attachments,omitempty"`
	PromptEnhanced  bool             `json:"prompt_enhanced"`
	RulesApplied    []string         `json:"rules_applied,omitempty"`
}

// AttachmentInfo 单个附件处理结果
type AttachmentInfo struct {
	FileID        string   `json:"file_id"`
	Type          string   `json:"type"`
	OriginalSize  int      `json:"original_size"`
	ProcessedSize int      `json:"processed_size"`
	Truncated     bool     `json:"truncated"`
	Warnings      []string `json:"warnings,omitempty"`
}

// ErrorResponse 统一错误结构
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody 错误详情
type ErrorBody struct {
	Code           string         `json:"code"`
	Message        string         `json:"message,omitempty"`
	UserMessageKey string         `json:"user_message_key,omitempty"`
	Provider       string         `json:"provider,omitempty"`
	RetryAfter     int            `json:"retry_after,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
}
