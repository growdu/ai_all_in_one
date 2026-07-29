package routing

import (
	"testing"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

func TestScoreProvider_ColdStart(t *testing.T) {
	// 冷启动：所有 Provider 无信号，应当平手
	signals := NewWindow(100, 0)
	weights := DefaultWeights()

	a := ScoreProvider("doubao", []core.ModelInfo{
		{ID: "doubao-1-5-pro-32k", Provider: "doubao", Modality: core.ModalityChat, Capabilities: []string{"text", "stream"}, ContextWindow: 32000},
	}, signals, weights, time.Hour)
	b := ScoreProvider("openai", []core.ModelInfo{
		{ID: "gpt-4o", Provider: "openai", Modality: core.ModalityChat, Capabilities: []string{"text", "stream"}, ContextWindow: 128000},
	}, signals, weights, time.Hour)

	if a != b {
		t.Errorf("cold start: a=%f b=%f, want equal", a, b)
	}
}

func TestScoreProvider_HighSuccessRate(t *testing.T) {
	signals := NewWindow(100, 0)
	// doubao 10/10 success, openai 5/10
	for i := 0; i < 10; i++ {
		signals.Record(Signal{Provider: "doubao", Success: true, Timestamp: time.Now()})
	}
	for i := 0; i < 5; i++ {
		signals.Record(Signal{Provider: "doubao", Success: true, Timestamp: time.Now()})
	}
	for i := 0; i < 5; i++ {
		signals.Record(Signal{Provider: "openai", Success: false, Timestamp: time.Now()})
	}

	weights := DefaultWeights()
	doubao := ScoreProvider("doubao", modelsFor("doubao"), signals, weights, time.Hour)
	openai := ScoreProvider("openai", modelsFor("openai"), signals, weights, time.Hour)

	if doubao <= openai {
		t.Errorf("doubao score %f should be > openai %f", doubao, openai)
	}
}

func TestScoreProvider_FastLatency(t *testing.T) {
	signals := NewWindow(100, 0)
	// doubao 1000ms, openai 100ms
	for i := 0; i < 5; i++ {
		signals.Record(Signal{Provider: "doubao", Success: true, LatencyMs: 1000, Timestamp: time.Now()})
	}
	for i := 0; i < 5; i++ {
		signals.Record(Signal{Provider: "openai", Success: true, LatencyMs: 100, Timestamp: time.Now()})
	}

	weights := DefaultWeights()
	doubao := ScoreProvider("doubao", modelsFor("doubao"), signals, weights, time.Hour)
	openai := ScoreProvider("openai", modelsFor("openai"), signals, weights, time.Hour)

	if openai <= doubao {
		t.Errorf("openai (faster) score %f should be > doubao %f", openai, doubao)
	}
}

func TestScoreProvider_UserPreference(t *testing.T) {
	signals := NewWindow(100, 0)
	// doubao 偏好 0.9, openai 偏好 0.1
	weights := DefaultWeights()
	doubao := ScoreProviderWithPrefs("doubao", modelsFor("doubao"), signals, weights, time.Hour, 0.9, 0.5)
	openai := ScoreProviderWithPrefs("openai", modelsFor("openai"), signals, weights, time.Hour, 0.1, 0.5)

	if doubao <= openai {
		t.Errorf("doubao (higher pref) score %f should be > openai %f", doubao, openai)
	}
}

func TestDefaultWeights(t *testing.T) {
	w := DefaultWeights()
	if w.SuccessRate != 0.4 {
		t.Errorf("SuccessRate = %f, want 0.4", w.SuccessRate)
	}
	if w.Latency != 0.2 {
		t.Errorf("Latency = %f, want 0.2", w.Latency)
	}
	if w.UserPreference != 0.3 {
		t.Errorf("UserPreference = %f, want 0.3", w.UserPreference)
	}
	if w.Capability != 0.1 {
		t.Errorf("Capability = %f, want 0.1", w.Capability)
	}
	// 权重和应当 ≈ 1（允许微小误差）
	sum := w.SuccessRate + w.Latency + w.UserPreference + w.Capability
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("weights sum = %f, want ~1.0", sum)
	}
}

func modelsFor(provider string) []core.ModelInfo {
	return []core.ModelInfo{
		{ID: provider + "-m1", Provider: provider, Modality: core.ModalityChat, Capabilities: []string{"text", "stream"}, ContextWindow: 8000},
	}
}
