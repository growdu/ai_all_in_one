package core

import (
	"encoding/json"
	"testing"
)

func TestModelInfo_JSON(t *testing.T) {
	m := ModelInfo{
		ID:              "doubao-1-5-pro-32k",
		DisplayName:     "豆包 1.5 Pro",
		Provider:        "doubao",
		Modality:        ModalityChat,
		Capabilities:    []string{"text", "stream", "vision", "file"},
		ContextWindow:   32000,
		InputPricePer1k: ptr(0.0008),
		OutputPricePer1k: ptr(0.002),
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	// 验证字段名遵循 OpenAI 兼容协议
	s := string(data)
	for _, want := range []string{
		`"id":"doubao-1-5-pro-32k"`,
		`"display_name":"豆包 1.5 Pro"`,
		`"provider":"doubao"`,
		`"modality":"chat"`,
		`"context_window":32000`,
		`"input_price_per_1k":0.0008`,
		`"output_price_per_1k":0.002`,
	} {
		if !contains(s, want) {
			t.Errorf("JSON missing %q\nfull: %s", want, s)
		}
	}
}

func TestModelInfo_JSON_OmitZero(t *testing.T) {
	m := ModelInfo{
		ID:            "gpt-4o",
		DisplayName:   "GPT-4o",
		Provider:      "openai",
		Modality:      ModalityChat,
		Capabilities:  []string{"text", "stream"},
		ContextWindow: 128000,
		// 缺 InputPricePer1k / OutputPricePer1k
	}
	data, _ := json.Marshal(m)
	s := string(data)
	if contains(s, "input_price_per_1k") {
		t.Errorf("zero price should be omitted, got: %s", s)
	}
}

func TestChatRequest_JSON(t *testing.T) {
	req := ChatRequest{
		Model:    "gpt-4o",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
		Stream:   true,
	}
	data, _ := json.Marshal(req)
	s := string(data)
	for _, want := range []string{
		`"model":"gpt-4o"`,
		`"role":"user"`,
		`"stream":true`,
	} {
		if !contains(s, want) {
			t.Errorf("JSON missing %q\nfull: %s", want, s)
		}
	}
}

func TestChatMessage_Attachment(t *testing.T) {
	msg := ChatMessage{
		Role:        "user",
		Content:     "看看这个",
		Attachments: []string{"file_xxx"},
	}
	data, _ := json.Marshal(msg)
	s := string(data)
	if !contains(s, `"attachments":["file_xxx"]`) {
		t.Errorf("attachments 字段缺失: %s", s)
	}
}

func TestChatChunk_Delta(t *testing.T) {
	chunk := ChatChunk{
		ID:           "chatcmpl-xxx",
		Delta:        "你好",
		ChunkIndex:   0,
		FinishReason: "",
	}
	data, _ := json.Marshal(chunk)
	s := string(data)
	for _, want := range []string{
		`"id":"chatcmpl-xxx"`,
		`"delta":"你好"`,
		`"chunk_index":0`,
	} {
		if !contains(s, want) {
			t.Errorf("JSON missing %q\nfull: %s", want, s)
		}
	}
}

func TestChatUsage_Unmarshal(t *testing.T) {
	// SSE chunk 的 usage 字段（流式最后一个 chunk）
	raw := `{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}`
	var u ChatUsage
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		t.Fatal(err)
	}
	if u.PromptTokens != 10 || u.CompletionTokens != 20 || u.TotalTokens != 30 {
		t.Errorf("usage parse wrong: %+v", u)
	}
}

// helpers

func ptr(f float64) *float64 { return &f }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
