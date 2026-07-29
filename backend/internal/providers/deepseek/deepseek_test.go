package deepseek

import (
	"testing"
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
			t.Errorf("model %s provider = %q", m.ID, m.Provider)
		}
	}
}

func TestProvider_ReasoningCapability(t *testing.T) {
	p := New()
	models := p.ListModels()
	var hasReasoner bool
	for _, m := range models {
		if m.ID == "deepseek-reasoner" {
			hasReasoner = true
			if m.ReasoningSpec == nil {
				t.Error("deepseek-reasoner should have reasoning spec")
			}
		}
	}
	if !hasReasoner {
		t.Error("deepseek-reasoner model not found")
	}
}
