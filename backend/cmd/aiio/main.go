// Package main 是 AI All-in-One 后端的唯一入口。
//
// 单二进制双角色：启动时按 AIIO_ROLE 环境变量决定作为 Master 还是 Worker 运行。
// 详细架构见 ../docs/backend/02-provider.md。
package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	role := os.Getenv("AIIO_ROLE")
	cfgPath := os.Getenv("AIIO_CONFIG")
	if cfgPath == "" {
		cfgPath = "configs/master.yaml"
	}

	fmt.Printf("aiio starting: role=%s config=%s\n", role, cfgPath)

	switch role {
	case "master":
		log.Println("TODO: 启动 Master — 见 Phase 2 任务")
	case "worker":
		region := os.Getenv("AIIO_REGION")
		if region == "" {
			log.Fatal("AIIO_REGION required when AIIO_ROLE=worker")
		}
		log.Printf("TODO: 启动 Worker (region=%s) — 见 Phase 3 任务\n", region)
	default:
		log.Fatal("AIIO_ROLE must be 'master' or 'worker'")
	}
}
