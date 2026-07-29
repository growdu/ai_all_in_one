# 实现进度跟踪

> 每次 commit 更新"已落地"清单 + 验证状态。本文档是事实表，不是设计文档。
> 设计看 docs/architecture / api / backend / frontend / roadmap。

## 状态总览

| Phase | 内容 | 状态 | 验证 |
|-------|------|------|------|
| 0 | 项目脚手架（Gin / health / 配置 / Docker） | ✅ 完成 | build/vet/test 全过，/health 200，/metrics 返回 Prometheus 文本 |
| 1 | 核心抽象（Modality / Provider / Registry） | ✅ 完成 | 12 tests 通过（Modality 2 / ModelInfo 4 / Registry 4 / 其他 2） |
| 2 | Master chat 端到端（single 模式流式） | ✅ 完成 | mock provider + 真实 http server，7 tests + 端到端 stream 验证通过 |
| 2.5 | Routing 进阶（auto / compare） | ✅ 完成 | 9 routing tests + 端到端 auto/compare 验证通过 |
| 3 | Worker role（豆包/DeepSeek/Kimi Provider） | 待办 | — |
| 4 | Key 管理（SQLite + AES-GCM） | ✅ 完成（JSON 文件 + AES-GCM；SQLite 留 2.0） | 14 security tests + 6 keys handler tests + 端到端 PUT/GET/DELETE + 文件 0600 验证 |
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
│   │   ├── observability.go                # 占位 Prometheus 输出 + 状态记录器
│   │   └── observability_test.go           # 5 tests
│   ├── core/                               # Phase 1: 跨角色共享抽象
│   │   ├── capability.go                   # Modality 枚举
│   │   ├── capability_test.go              # 2 tests
│   │   ├── models.go                       # ModelInfo / ChatRequest / ChatChunk / ErrorResponse
│   │   ├── models_test.go                  # 4 tests
│   │   ├── registry.go                     # ChatProvider interface + Registry
│   │   └── registry_test.go                # 4 tests
│   ├── api/                                # Phase 2: HTTP handlers
│   │   ├── chat.go                         # /api/v1/chat/completions
│   │   ├── chat_test.go                    # 7 tests
│   │   ├── models.go                       # /api/v1/models
│   │   └── models_test.go                  # 2 tests
│   ├── providers/                          # Phase 2: mock provider
│   │   └── mockprovider/                   # 不依赖外部 API 的回显 provider
│   │       ├── mock.go
│   │       ├── mock_test.go                # 5 tests
│   │       └── slow.go                     # 慢速回显（演示 auto 模式 fallback）
│   ├── routing/                            # Phase 2.5: 4 因子打分 + 路由
│   │   ├── signals.go                      # 滑动窗口
│   │   ├── scoring.go                      # 4 因子打分公式
│   │   ├── scoring_test.go                 # 5 tests
│   │   ├── router.go                       # single/auto/compare 三模式
│   │   └── router_test.go                  # 4 tests
│   ├── security/                           # Phase 4: AES-GCM + Keyring
│   │   ├── aesgcm.go                       # AES-256-GCM 加密
│   │   ├── aesgcm_test.go                  # 5 tests
│   │   ├── keyring.go                      # Keyring JSON 文件实现
│   │   ├── keyring_test.go                 # 9 tests
│   │   └── codec.go                        # base64 + 时间
│   └── role/                               # Master/Worker 启动
│       └── role.go
```

## 1.0 阶段决策

- **不引入 Gin/Fiber**：用 stdlib `net/http` + 手写 mux。等路由超过 20 个或需要复杂中间件链时再切
- **占位 Prometheus**：1.0 阶段不引入 prometheus client_golang，用简单文本格式暴露指标。Phase 2 升级
- **SQLite 未接**：1.0 阶段 `/health` 的 db_ok 只检查文件路径，2.0 接 modernc.org/sqlite 后改用 `PRAGMA quick_check`
- **Phase 0 改回 master_key_env 宽松校验**：1.0 简单部署场景不该强求 master key env，调用 `MasterKey()` 时再报错

## 端到端验证（Phase 2 验证脚本）

```bash
# 启动 master
AIIO_JWT_SECRET=t \
  AIIO_ROLE=master \
  AIIO_CONFIG=/tmp/master-test.yaml \
  AIIO_AUTH_TOKEN=devtoken \
  /tmp/aiio > /tmp/aiio.log 2>&1 &

# 1. 列模型
curl -s http://localhost:18080/api/v1/models
# → {"models":[{"id":"mock-echo",...}]}

# 2. 非流式 chat
curl -s -X POST http://localhost:18080/api/v1/chat/completions \
  -H "Authorization: Bearer devtoken" \
  -H "Content-Type: application/json" \
  -d '{"model":"mock-echo","messages":[{"role":"user","content":"hi"}]}'
# → {"id":"chatcmpl-mock-...","content":"echo: hi","usage":{...}}

# 3. 流式 SSE
curl -s -N -X POST http://localhost:18080/api/v1/chat/completions \
  -H "Authorization: Bearer devtoken" \
  -H "Content-Type: application/json" \
  -d '{"model":"mock-echo","messages":[{"role":"user","content":"hi"}],"stream":true}'
# → data: {"id":"...","delta":"e","chunk_index":0}\n\n  ...
# → data: {"id":"...","delta":"i","chunk_index":7,"finish_reason":"stop"}\n\n
# → data: [DONE]\n\n
```

**关键发现**：`observability.LogRequest` 中间件的 `statusRecorder` 必须实现 `http.Flusher`，否则 SSE 流式失效。已在 Phase 2 修复。

## 端到端验证（Phase 2.5 auto/compare）

```bash
# auto 模式：冷启动打分，2 个候选（mock + slow）
curl -X POST http://localhost:18080/api/v1/chat/completions \
  -H "Authorization: Bearer devtoken" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"hi"}]}'
# → {"provider":"slow","content":"slow: hi",...}
# 冷启动时两 provider 平手，按 Registry 迭代顺序取最后一个

# compare 模式：并行发 2 个 provider
curl -X POST http://localhost:18080/api/v1/chat/completions \
  -H "Authorization: Bearer devtoken" \
  -H "Content-Type: application/json" \
  -d '{"model":"x","messages":[{"role":"user","content":"hi"}],"compare":{"providers":["mock","slow"]}}'
# → {"compare":{"results":[{"provider":"mock","status":"succeeded",...},{"provider":"slow",...}]}}
```

**实现要点**：
- 4 因子打分：`0.4*success_rate + 0.2*(1-latency_norm) + 0.3*user_pref + 0.1*capability`
- 冷启动 0 信号：所有 provider 平手（score=0.5 中立值）
- 滑动窗口 200 条/Provider，1.0 内存存储
- auto 失败 fallback 1 次（max_auto_fallback=1）
- compare 模式 1.0 不支持 stream（流式变体留 2.0）

## 端到端验证（Phase 4 Key 管理）

```bash
# 启动（设 AIIO_MASTER_KEY=32字节）
AIIO_MASTER_KEY=0123456789abcdef0123456789abcdef /tmp/aiio &

# PUT
curl -X POST http://localhost:18080/api/v1/keys \
  -H "Authorization: Bearer devtoken" \
  -d '{"provider":"doubao","key":"sk-test-SECRET-VALUE"}'
# → {"provider":"doubao","updated_at":1785321843}

# GET（不返回明文！）
curl http://localhost:18080/api/v1/keys -H "Authorization: Bearer devtoken"
# → {"providers":["doubao"]}

# DELETE
curl -X DELETE http://localhost:18080/api/v1/keys/doubao -H "Authorization: Bearer devtoken"
# → 204 No Content

# 磁盘上的 keyring.json
# {"version":1,"entries":{"doubao":{"provider":"doubao",
#   "ciphertext":"XUn7fwg...","updated_at":1785321843}}}
# 0600 权限，密文形式存储
```

**安全要点**：
- AES-256-GCM（12B nonce + ciphertext + 16B tag）
- Master key 必须 32 字节（从 env AIIO_MASTER_KEY）
- keyring.json 文件 0600
- GET 不返回明文 Key（只列 provider 名字）
- Decrypt 失败（错 key）返回 error，不 panic

**Phase 4 决策**：
- **JSON 文件而非 SQLite**：1.0 用户量小 + 零 CGO 部署门槛。SQLite 留 2.0（`modernc.org/sqlite` 纯 Go 驱动）
- **Keyring 不接 chat 路由**：1.0 阶段 keyring 暴露为 API（CRUD），chat handler 仍用 PLACEHOLDER_USER_KEY。Phase 5 接入真 Key → Provider 的链路

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
| 2026-07-29 | c567951 | 1 | 核心抽象：Modality / 统一数据模型 / ChatProvider interface / Registry |
| 2026-07-29 | be2a8e1 | 2 | Master chat 端到端：chat handler + models handler + mock provider + SSE 流式透传 |
| 2026-07-29 | 171b7eb | 2.5 | Routing：4 因子打分 + 滑动窗口 + single/auto/compare 三模式 |
| 2026-07-29 | (pending) | 4 | Key 管理：AES-256-GCM 加密 + JSON 文件 Keyring + CRUD API |

