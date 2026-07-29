// Package kimi 是月之暗面 Kimi 的 Provider 实现。
//
// 1.0 阶段：Kimi 走 Moonshot OpenAI 兼容 API。
package kimi

import (
	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/openaicompat"
)

const Name = "kimi"

var Models = []core.ModelInfo{
	{
		ID:            "kimi-k2-0711-preview",
		DisplayName:   "Kimi K2 (preview)",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text", "stream", "file"},
		ContextWindow: 128000,
	},
	{
		ID:            "moonshot-v1-128k",
		DisplayName:   "Moonshot v1 128K",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text", "stream", "file"},
		ContextWindow: 128000,
	},
}

const BaseURL = "https://api.moonshot.cn/v1/chat/completions"

func New() *openaicompat.OpenAICompat {
	return openaicompat.New(Name, BaseURL, Models)
}
