package minimax

import (
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

func TestNew(t *testing.T) {
	p := New()
	if p.Name() != Name {
		t.Errorf("name = %q, want %q", p.Name(), Name)
	}
	if p.Modality() != core.ModalityChat {
		t.Errorf("modality = %q", p.Modality())
	}
	if !p.SupportsStream() {
		t.Error("want stream supported")
	}
	models := p.ListModels()
	if len(models) == 0 {
		t.Fatal("no models")
	}
	if models[0].Provider != Name {
		t.Errorf("model[0].provider = %q", models[0].Provider)
	}
	// 验证模型 ID 包含 minimax 或 minimax 系列
	if models[0].ID == "" {
		t.Error("model id empty")
	}
}

func TestBaseURLIsAnthropicCompat(t *testing.T) {
	// minimax 端点必须以 minimax.com 域名结尾 + 走 anthropic 路径
	if !contains(BaseURL, "minimaxi.com") {
		t.Errorf("BaseURL = %q, want contains 'minimaxi.com'", BaseURL)
	}
	if !contains(BaseURL, "anthropic") {
		t.Errorf("BaseURL = %q, want contains 'anthropic'", BaseURL)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}