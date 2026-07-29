// Package observability 提供日志、metrics、health 三件套。
//
// 1.0 阶段：
//   - 日志：log/slog 标准库，JSON handler 写 stdout
//   - metrics：/metrics 端点，1.0 阶段用最小实现（不引入 prometheus client）
//   - health：/health 端点，返回 db_ok + uptime_sec
//
// 2.0 阶段：引入 prometheus client_golang，OpenTelemetry。
//
// 详见 ../../../docs/backend/02-provider.md §九点四。
package observability

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Version 是当前 binary 版本
const Version = "0.1.0"

// startTime 程序启动时间（用于 uptime_sec）
var startTime = time.Now()

// UptimeSec 返回进程启动至今的秒数
func UptimeSec() int64 {
	return int64(time.Since(startTime).Seconds())
}

// ---- /health ----

// HealthHandler 返回 /health 处理器。
// 1.0 阶段只检查进程存活 + db_ok（DB 健康检查由调用方注入）
func HealthHandler(dbOK func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		ok := true
		if dbOK != nil {
			ok = dbOK()
		}
		status := "ok"
		if !ok {
			status = "degraded"
		}
		fmt.Fprintf(w, `{"status":%q,"version":%q,"uptime_sec":%d,"db_ok":%t}`,
			status, Version, UptimeSec(), ok)
	}
}

// ---- /metrics ----

// 1.0 占位 metrics：只暴露 process uptime + version
// 2.0 切到 prometheus client

var (
	chatTotal      atomic.Int64
	chatFailed     atomic.Int64
	tokensTotal    atomic.Int64
	rateLimitHits  atomic.Int64
	activeStreams  atomic.Int64
)

// RecordChat 记录一次 chat 调用结果
func RecordChat(provider, model string, success bool, promptTokens, completionTokens int) {
	chatTotal.Add(1)
	if !success {
		chatFailed.Add(1)
	}
	tokensTotal.Add(int64(promptTokens + completionTokens))
}

// RecordRateLimit 记录一次限流命中
func RecordRateLimit(layer string) {
	rateLimitHits.Add(1)
}

// IncActiveStreams / DecActiveStreams 跟踪活跃流
func IncActiveStreams() { activeStreams.Add(1) }
func DecActiveStreams() { activeStreams.Add(-1) }

// MetricsHandler 返回 /metrics 处理器（1.0 占位 Prometheus 格式）
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP aiio_version Binary version\n")
		fmt.Fprintf(w, "# TYPE aiio_version gauge\n")
		fmt.Fprintf(w, "aiio_version{version=%q} 1\n", Version)

		fmt.Fprintf(w, "# HELP aiio_uptime_sec Process uptime in seconds\n")
		fmt.Fprintf(w, "# TYPE aiio_uptime_sec counter\n")
		fmt.Fprintf(w, "aiio_uptime_sec %d\n", UptimeSec())

		fmt.Fprintf(w, "# HELP aiio_chat_total Total chat calls\n")
		fmt.Fprintf(w, "# TYPE aiio_chat_total counter\n")
		fmt.Fprintf(w, "aiio_chat_total %d\n", chatTotal.Load())

		fmt.Fprintf(w, "# HELP aiio_chat_failed_total Failed chat calls\n")
		fmt.Fprintf(w, "# TYPE aiio_chat_failed_total counter\n")
		fmt.Fprintf(w, "aiio_chat_failed_total %d\n", chatFailed.Load())

		fmt.Fprintf(w, "# HELP aiio_tokens_total Total tokens consumed\n")
		fmt.Fprintf(w, "# TYPE aiio_tokens_total counter\n")
		fmt.Fprintf(w, "aiio_tokens_total %d\n", tokensTotal.Load())

		fmt.Fprintf(w, "# HELP aiio_rate_limit_hits_total Rate limit hits\n")
		fmt.Fprintf(w, "# TYPE aiio_rate_limit_hits_total counter\n")
		fmt.Fprintf(w, "aiio_rate_limit_hits_total %d\n", rateLimitHits.Load())

		fmt.Fprintf(w, "# HELP aiio_active_streams Active SSE streams\n")
		fmt.Fprintf(w, "# TYPE aiio_active_streams gauge\n")
		fmt.Fprintf(w, "aiio_active_streams %d\n", activeStreams.Load())
	}
}

// ---- Request Logger ----

// LogRequest 简单请求日志中间件（slog JSON）
// 1.0 用 stdlib log/slog，2.0 切到 zap/zerolog 不变接口
type Logger interface {
	Info(msg string, args ...any)
}

func LogRequest(logger Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// statusRecorder 实现 http.Flusher 以保持 SSE 流式能力
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"latency_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.ResponseWriter.WriteHeader(code)
		r.wrote = true
	}
}

// Flush 透传 http.Flusher，保留 SSE 流式能力
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
