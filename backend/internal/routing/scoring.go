// Package routing 实现 Master 端的请求路由策略。
//
// 详见 docs/architecture/01-routing-strategy.md
package routing

import (
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/core"
)

// Weights 4 因子权重
type Weights struct {
	SuccessRate    float64 // 近 5min 成功率
	Latency        float64 // 延迟（1-归一化）
	UserPreference float64 // 用户偏好
	Capability     float64 // 能力匹配
}

// DefaultWeights 1.0 默认权重
func DefaultWeights() Weights {
	return Weights{
		SuccessRate:    0.4,
		Latency:        0.2,
		UserPreference: 0.3,
		Capability:     0.1,
	}
}

// ScoreProvider 给一个 provider 在当前请求下打分
// 0-1 范围，分越高越优
//
// userPref 可选：外部传入（从 user pref store 读），默认 0.5
func ScoreProvider(name string, models []core.ModelInfo, signals *Window, w Weights, window time.Duration) float64 {
	return ScoreProviderWithPrefs(name, models, signals, w, window, 0.5, 0.5)
}

// ScoreProviderWithPrefs 完整版本，可指定 user preference 与 capability 匹配阈值
func ScoreProviderWithPrefs(
	name string,
	models []core.ModelInfo,
	signals *Window,
	w Weights,
	window time.Duration,
	userPref float64,
	capMatch float64,
) float64 {
	if len(models) == 0 {
		return 0
	}
	successRate := signals.SuccessRate(name, window)
	latencyNorm := 1 - signals.NormalizedLatency(name, window)
	score := w.SuccessRate*successRate +
		w.Latency*latencyNorm +
		w.UserPreference*userPref +
		w.Capability*capMatch
	return score
}
