// Package doubao 是豆包（Doubao / 火山方舟）的 Provider 实现。
//
// 1.0 阶段：豆包走火山方舟 OpenAI 兼容 API。
// 详见 docs/backend/02-provider.md §七。
package doubao

import (
	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/openaicompat"
)

// Name 注册名
const Name = "doubao"

// Models 1.0 阶段暴露的模型
var Models = []core.ModelInfo{
	{
		ID:            "doubao-1-5-pro-32k",
		DisplayName:   "豆包 1.5 Pro 32K",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text", "stream", "vision", "file"},
		ContextWindow: 32000,
	},
	{
		ID:            "doubao-1-5-lite-32k",
		DisplayName:   "豆包 1.5 Lite 32K",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text", "stream"},
		ContextWindow: 32000,
	},
	{
		ID:            "doubao-pro-256k",
		DisplayName:   "豆包 Pro 256K",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text", "stream", "vision", "file"},
		ContextWindow: 256000,
	},
}

// BaseURL 火山方舟 OpenAI 兼容端点
const BaseURL = "https://ark.cn-beijing.volces.com/api/v3/chat/completions"

// New 构造豆包 Provider
func New() *openaicompat.OpenAICompat {
	return openaicompat.New(Name, BaseURL, Models)
}
