package routing

import (
	"sort"
	"sync"
	"time"
)

// Signal 一次 chat 调用的结果信号
//
// 用于 Master 端滑动窗口打分（详见 docs/architecture/01-routing-strategy.md §四）
type Signal struct {
	Provider  string
	Timestamp time.Time
	LatencyMs int
	Success   bool
	// 可选：auto 模式用户切换时记录（用于 user_preference 学习）
	// "doubao" 意味着用户切到了 doubao（从其他 provider 切走）
	UserPicked string
}

// Window 线程安全的滑动窗口
type Window struct {
	mu       sync.RWMutex
	capacity int
	items    []Signal
}

// NewWindow 创建 capacity 大小的窗口
// 1.0 阶段默认 200 条/Provider（内存足够）
func NewWindow(capacity int, _ int) *Window {
	if capacity <= 0 {
		capacity = 200
	}
	return &Window{capacity: capacity, items: make([]Signal, 0, capacity)}
}

// Record 写入一条信号
func (w *Window) Record(s Signal) {
	if s.Timestamp.IsZero() {
		s.Timestamp = time.Now()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = append(w.items, s)
	if len(w.items) > w.capacity {
		// 环形：去掉最老的
		w.items = w.items[len(w.items)-w.capacity:]
	}
}

// CountByProvider 在 window 时间窗内的记录数
func (w *Window) CountByProvider(name string, window time.Duration) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cutoff := time.Now().Add(-window)
	count := 0
	for _, s := range w.items {
		if s.Provider != name {
			continue
		}
		if s.Timestamp.Before(cutoff) {
			continue
		}
		count++
	}
	return count
}

// SuccessRate 近 window 时间内的成功率（0-1）
// 0 条样本时返回 0.5（中立，docs/architecture/01-routing-strategy.md §3.2 冷启动）
func (w *Window) SuccessRate(name string, window time.Duration) float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cutoff := time.Now().Add(-window)
	total, ok := 0, 0
	for _, s := range w.items {
		if s.Provider != name || s.Timestamp.Before(cutoff) {
			continue
		}
		total++
		if s.Success {
			ok++
		}
	}
	if total == 0 {
		return 0.5
	}
	return float64(ok) / float64(total)
}

// NormalizedLatency 把 window 内的 P50 延迟归一到 0-1（越快越高）
//
// 归一化基准：把当前所有 provider 的最小 / 最大 P50 作为 [min, max]，
// 该 provider 的 P50 落在 (max - norm_score * (max - min)) 上。
// 没有样本时返回 0.5（中立）
//
// 1.0 简化：只对该 provider 自身历史做归一化（自身最慢 vs 自身最快 100ms 基准）
// 2.0 跨 provider 归一化
func (w *Window) NormalizedLatency(name string, window time.Duration) float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cutoff := time.Now().Add(-window)
	latencies := []int{}
	for _, s := range w.items {
		if s.Provider != name || s.Timestamp.Before(cutoff) || !s.Success {
			continue
		}
		latencies = append(latencies, s.LatencyMs)
	}
	if len(latencies) == 0 {
		return 0.5
	}
	sort.Ints(latencies)
	p50 := latencies[len(latencies)/2]
	// 1.0 简化归一化：100ms = 满分 1.0，每多 100ms 减 0.1，最低 0
	if p50 <= 100 {
		return 0
	}
	score := 1.0 - float64(p50-100)/1000.0
	if score < 0 {
		score = 0
	}
	return score
}

// UserPickedCount 返回 window 内 UserPicked=name 的次数（auto 模式切换学习）
func (w *Window) UserPickedCount(name string, window time.Duration) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cutoff := time.Now().Add(-window)
	count := 0
	for _, s := range w.items {
		if s.UserPicked == "" || s.UserPicked != name {
			continue
		}
		if s.Timestamp.Before(cutoff) {
			continue
		}
		count++
	}
	return count
}
