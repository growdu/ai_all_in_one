# 实现进度跟踪

> 每次 commit 更新"已落地"清单 + 验证状态。本文档是事实表，不是设计文档。
> 设计看 docs/architecture / api / backend / frontend / roadmap。

## 状态总览

| Phase | 内容 | 状态 | 验证 |
|-------|------|------|------|
| 0 | 项目脚手架（Gin / health / 配置 / Docker） | ✅ 完成 | build/vet/test 全过，/health 200，/metrics 返回 Prometheus 文本 |
| 1 | 核心抽象（Modality / Provider / Registry） | ✅ 完成 | 12 tests 通过（Modality 2 / ModelInfo 4 / Registry 4 / 其他 2） |
| 2 | Master chat 端到端（single 模式流式） | 待办 | — |
| 2.5 | Routing 进阶（auto / compare） | 待办 | — |
| 3 | Worker role（豆包/DeepSeek/Kimi Provider） | 待办 | — |
| 4 | Key 管理（SQLite + AES-GCM） | 待办 | — |
| 5 | 前端 MVP（Vue3 + SSE + 5 页） | 待办 | — |
| 6 | 文件上传 + 多模态 | 待办 | — |
| 7 | 移动端打磨 | 待办 | — |
| 8 | 部署与文档 | 待办 | — |

## 已落地模块

```
backend/
├── go.mod                                  # + gopkg.in/yaml.v3
├── cmd/aiio/main.go                        # 入口：双角色分支 + slog
├── internal/
│   ├── config/                             # YAML 配置加载 + 校验
│   │   ├── config.go
│   │   └── config_test.go                  # 3 tests
│   ├── observability/                      # 日志/metrics/health
│   │   ├── observability.go                # 占位 Prometheus 输出
│   │   └── observability_test.go           # 5 tests
│   ├── core/                               # Phase 1: 跨角色共享抽象
│   │   ├── capability.go                   # Modality 枚举
│   │   ├── capability_test.go              # 2 tests
│   │   ├── models.go                       # ModelInfo / ChatRequest / ChatChunk / ErrorResponse
│   │   ├── models_test.go                  # 4 tests
│   │   ├── registry.go                     # ChatProvider interface + Registry
│   │   └── registry_test.go                # 4 tests
│   └── role/                               # Master/Worker 启动
│       └── role.go
```

## 1.0 阶段决策

- **不引入 Gin/Fiber**：用 stdlib `net/http` + 手写 mux。等路由超过 20 个或需要复杂中间件链时再切
- **占位 Prometheus**：1.0 阶段不引入 prometheus client_golang，用简单文本格式暴露指标。Phase 2 升级
- **SQLite 未接**：1.0 阶段 `/health` 的 db_ok 只检查文件路径，2.0 接 modernc.org/sqlite 后改用 `PRAGMA quick_check`
- **Phase 0 改回 master_key_env 宽松校验**：1.0 简单部署场景不该强求 master key env，调用 `MasterKey()` 时再报错

## 实施规则

1. **每个 Phase 1 个或多个 commit**（按 TDD 风格可拆细）
2. **每个 Phase 结束跑全量验证**：`go vet ./... && go build ./cmd/aiio && go test ./...`
3. **二进制能 build** + `/health` 返回 200 + `/metrics` 返回 Prometheus 格式
4. **与设计文档同步**：实现过程中发现设计问题，先改设计再改代码
5. **不在此 commit 中混入文档无关改动**

## 验证命令

```bash
cd /root/ai_all_in_one/backend
go vet ./...
go build -o /tmp/aiio ./cmd/aiio
go test ./...
/tmp/aiio  # 验证启动；测试需要 AIIO_ROLE=master / worker
```

## Commit 日志（实现阶段）

> 每次实现 commit 在此记录一行：commit hash + phase + 一句话。

| 日期 | Commit | Phase | 摘要 |
|------|--------|-------|------|
| 2026-07-29 | 9888955 | 0 | 项目脚手架：config / observability / role / health/metrics 端点 |
| 2026-07-29 | (pending) | 1 | 核心抽象：Modality / 统一数据模型 / ChatProvider interface / Registry |

