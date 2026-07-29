// Package deepseek 是 DeepSeek 的 Provider 实现。
//
// 1.0 阶段：DeepSeek 走 OpenAI 兼容 API。
package deepseek

import (
	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/openaicompat"
)

const Name = "deepseek"

var Models = []core.ModelInfo{
	{
		ID:            "deepseek-chat",
		DisplayName:   "DeepSeek-V3 Chat",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text", "stream"},
		ContextWindow: 64000,
	},
	{
		ID:            "deepseek-reasoner",
		DisplayName:   "DeepSeek-R1 Reasoner",
		Provider:      Name,
		Modality:      core.ModalityChat,
		Capabilities:  []string{"text", "stream", "reasoning"},
		ContextWindow: 64000,
		ReasoningSpec: &core.ReasoningSpec{Effort: "high"},
	},
}

const BaseURL = "https://api.deepseek.com/v1/chat/completions"

func New() *openaicompat.OpenAICompat {
	return openaicompat.New(Name, BaseURL, Models)
}
