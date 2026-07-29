// Package main 是 AI All-in-One 后端的唯一入口。
//
// 单二进制双角色：启动时按 AIIO_ROLE 环境变量决定作为 Master 还是 Worker 运行。
// 详细架构见 ../docs/backend/02-provider.md。
//
// 1.0 阶段使用 Go 标准库 net/http（不引入第三方 web 框架）：
//   - 路由简单（health / metrics / v1/models / v1/chat 等）
//   - 中间件按需手写（30 行内的事）
//   - 等路由超过 20 个或需要复杂中间件链时再切到 gin/fiber
package main

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/growdu/ai_all_in_one/backend/internal/config"
	"github.com/growdu/ai_all_in_one/backend/internal/role"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("aiio: %v", err)
	}
}

func run() error {
	roleName := os.Getenv("AIIO_ROLE")
	cfgPath := os.Getenv("AIIO_CONFIG")
	if cfgPath == "" {
		cfgPath = "configs/master.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config %s: %w", cfgPath, err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("aiio starting",
		slog.String("role", roleName),
		slog.String("config", cfgPath),
		slog.String("version", "0.1.0"),
	)

	switch roleName {
	case "master":
		return role.RunMaster(cfg, logger)
	case "worker":
		region := os.Getenv("AIIO_REGION")
		if region == "" {
			return errors.New("AIIO_REGION required when AIIO_ROLE=worker")
		}
		return role.RunWorker(cfg, region, logger)
	default:
		return fmt.Errorf("AIIO_ROLE must be 'master' or 'worker', got %q", roleName)
	}
}
