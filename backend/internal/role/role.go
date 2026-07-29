// Package role 包含 Master 与 Worker 两种启动逻辑。
//
// 单二进制双角色：详见 ../../../docs/backend/02-provider.md §四。
package role

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/growdu/ai_all_in_one/backend/internal/api"
	"github.com/growdu/ai_all_in_one/backend/internal/config"
	"github.com/growdu/ai_all_in_one/backend/internal/core"
	"github.com/growdu/ai_all_in_one/backend/internal/observability"
	"github.com/growdu/ai_all_in_one/backend/internal/providers/mockprovider"
	"github.com/growdu/ai_all_in_one/backend/internal/routing"
	"github.com/growdu/ai_all_in_one/backend/internal/security"
)

// RunMaster 启动 Master 进程。
//
// 1.0 阶段：stdlib mux + /health + /metrics + 真实 chat/models 路由。
// 详细见 docs/backend/02-provider.md §四。
func RunMaster(cfg *config.Config, logger *slog.Logger) error {
	if err := os.MkdirAll(filepath.Dir(cfg.Storage.SQLitePath), 0o755); err != nil {
		return err
	}
	dbOK := func() bool {
		_, err := os.Stat(cfg.Storage.SQLitePath)
		return err == nil || os.IsNotExist(err)
	}

	// 初始化 Provider Registry（1.0 阶段注册 mock + slow 用于演示）
	reg := core.NewRegistry()
	reg.RegisterChat(mockprovider.New())
	reg.RegisterChat(mockprovider.NewSlow())

	// Routing：4 因子打分 + 滑动窗口
	signals := routing.NewWindow(200, 0)
	router := routing.NewRouter(reg, signals, routing.DefaultWeights(), 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", observability.HealthHandler(dbOK))
	mux.HandleFunc("/metrics", observability.MetricsHandler())
	mux.Handle("/api/v1/models", &api.ModelsHandler{Registry: reg})
	mux.Handle("/api/v1/chat/completions", &api.ChatHandler{
		Logger:    logger,
		Registry:  reg,
		Router:    router,
		AuthToken: os.Getenv("AIIO_AUTH_TOKEN"),
	})
	// 1.0 简化：Keyring handler 始终挂载（即便 keyring 未初始化，返回 500 时也安全）
	// 实际：见下方的 lazy init
	if kr, err := initKeyring(cfg, logger); err == nil {
		mux.Handle("/api/v1/keys", &api.KeysHandler{Keyring: kr})
		mux.Handle("/api/v1/keys/", &api.KeysHandler{Keyring: kr})
	}

	handler := observability.LogRequest(logger, mux)

	logger.Info("master role started",
		slog.String("addr", cfg.Server.Listen),
		slog.String("storage", cfg.Storage.SQLitePath),
		slog.Int("workers_configured", len(cfg.Workers)),
		slog.Int("providers", len(reg.ChatProviders())),
	)

	return startHTTPServer(context.Background(), cfg.Server.Listen, handler, logger)
}

// RunWorker 启动 Worker 进程。
//
// 1.0 阶段：占位实现，仅暴露 /health。
// 详细见 docs/backend/02-provider.md §四。
func RunWorker(cfg *config.Config, region string, logger *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", observability.HealthHandler(nil))
	mux.HandleFunc("/metrics", observability.MetricsHandler())
	mux.HandleFunc("/internal/chat", notImplemented("Phase 3"))

	handler := observability.LogRequest(logger, mux)

	logger.Info("worker role started",
		slog.String("region", region),
		slog.String("addr", cfg.Server.Listen),
		slog.Int("providers", len(cfg.Providers)),
	)

	addr := cfg.Server.Listen
	if addr == ":8080" {
		// Worker 默认 8443 避免与 master 撞
		addr = ":8443"
	}
	return startHTTPServer(context.Background(), addr, handler, logger)
}

// startHTTPServer 启动 HTTP server，支持优雅退出
func startHTTPServer(ctx context.Context, addr string, h http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case <-ctx.Done():
		return srv.Shutdown(ctx)
	}
}

// notImplemented 返回 501 占位响应
func notImplemented(phase string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(`{"error":{"code":"not_implemented","message":"` + phase + `"}}`))
	}
}

// initKeyring 初始化 keyring（1.0 阶段：仅当 AIIO_MASTER_KEY 是 32 字节时启用）
func initKeyring(cfg *config.Config, logger *slog.Logger) (*security.Keyring, error) {
	masterKey := os.Getenv("AIIO_MASTER_KEY")
	if masterKey == "" || len(masterKey) != 32 {
		return nil, errors.New("AIIO_MASTER_KEY not set or not 32 bytes")
	}
	path := filepath.Join(filepath.Dir(cfg.Storage.SQLitePath), "keyring.json")
	kr, err := security.NewKeyring(path, []byte(masterKey))
	if err != nil {
		return nil, err
	}
	logger.Info("keyring initialized", slog.String("path", path))
	return kr, nil
}
