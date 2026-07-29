package doubao

import (
	"testing"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/openaicompat"
)

func TestProvider_Name(t *testing.T) {
	p := New()
	if p.Name() != Name {
		t.Errorf("Name = %q, want %q", p.Name(), Name)
	}
}

func TestProvider_Models(t *testing.T) {
	p := New()
	models := p.ListModels()
	if len(models) < 2 {
		t.Errorf("expected >= 2 models, got %d", len(models))
	}
	for _, m := range models {
		if m.Provider != Name {
			t.Errorf("model %s provider = %q, want %q", m.ID, m.Provider, Name)
		}
	}
}

func TestProvider_ImplementsChatProvider(t *testing.T) {
	var _ core.ChatProvider = (*openaicompat.OpenAICompat)(nil)
	var _ core.ChatProvider = New()
}
