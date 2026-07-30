// Package minimax 提供 minimax Provider（Anthropic Messages 兼容协议）。
//
// minimax 的 API 兼容 Anthropic Messages 协议，端点：
//   POST https://api.minimaxi.com/anthropic/v1/messages
//
// 1.0 阶段：单模型 MiniMax-M3，后续增加模型时改 Models 切片。
package minimax

import (
	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/anthropiccompat"
)

// Name minimax 在 registry 里的名字
const Name = "minimax"

// BaseURL minimax Anthropic 兼容端点
const BaseURL = "https://api.minimaxi.com/anthropic/v1/messages"

// Models minimax 当前暴露的模型
// 1.0 阶段：1 个 chat 模型
// 2.0+：增加 vision / embedding 等
var Models = []core.ModelInfo{
	{
		ID:            "MiniMax-M3",
		DisplayName:   "minimax M3",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text"},
		ContextWindow: 200000,
	},
}

// New 构造 minimax Provider
func New() *anthropiccompat.AnthropicCompat {
	return anthropiccompat.New(Name, BaseURL, Models)
}