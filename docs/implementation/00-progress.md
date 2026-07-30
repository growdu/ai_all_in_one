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
| 3 | Worker role + 豆包/DeepSeek/Kimi 真 Provider | ✅ 完成（单进程内注册） | openaicompat 6 tests + doubao 3 + deepseek 3 + kimi 2；端到端 5 provider 10 model 列出 |
| 4 | Key 管理（SQLite + AES-GCM） | ✅ 完成（JSON 文件 + AES-GCM；SQLite 留 2.0） | 14 security tests + 6 keys handler tests + 端到端 PUT/GET/DELETE + 文件 0600 验证 |
| 4.5 | Keyring ↔ chat 路由集成 | ✅ 完成 | 4 chat keyring tests + 端到端 4 场景（无 key 400 / 有 key 200 / auto 部分无 key fallback） |
| 5 | 前端 MVP（HTML+JS 5 页） | ✅ 完成（1.0 极简版，5 HTML + 5 JS + 1 CSS） | 静态文件服务 + API 共存，端到端 GET 4 页 + 3 资源 + /api/v1/models 全 200 |
| 6 | 文件上传 + 多模态 | ✅ 完成（本地文件系统 + 截断） | 10 storage tests + 8 file handler tests + 端到端 upload/list/reject |
| 6.5 | 附件注入 chat 链路 | ✅ 完成（file_id → messages） | 6 preprocessing tests + 端到端上传→chat→AI 看到附件内容 |
| 1.1.1 | 历史会话 CRUD | ✅ 完成（conv/msg repo + 5 端点 + 前端 history 页） | 7 conv/msg tests + 6 convs handler tests + 端到端 POST/GET/PATCH/DELETE + 404 |
| 7 | 移动端打磨（暗色 / a11y / PWA） | ✅ 完成 | CSS 暗色变量 + prefers-color-scheme + 主题切换；aria-label + focus 环 + 44pt 触摸目标；PWA manifest.webmanifest 合法 |
| 8 | 部署与文档 | ✅ 完成 | Dockerfile 写好、docker-compose.yml 通过 `docker compose config` 校验、deploy.md + user-guide.md + .env.example 完整 |

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
│   ├── api/                                # Phase 2 + 4 + 4.5 + 6: HTTP handlers
│   │   ├── chat.go                         # /api/v1/chat/completions（单/auto/compare + Keyring + 附件注入）
│   │   ├── chat_test.go                    # 7 tests
│   │   ├── chat_keyring_test.go            # 4 tests（Keyring 集成）
│   │   ├── models.go                       # /api/v1/models
│   │   ├── models_test.go                  # 2 tests
│   │   ├── keys.go                         # /api/v1/keys CRUD
│   │   ├── keys_test.go                    # 6 tests
│   │   ├── files.go                        # /api/v1/files CRUD
│   │   ├── files_test.go                   # 8 tests
│   │   └── codec.go                        # jsonEncode helper
│   ├── capabilities/                       # Phase 1.1: 业务能力
│   │   └── chat/
│   │       └── preprocessing/              # 附件预处理（file_id → text）
│   │           ├── preprocessor.go         # 注入逻辑
│   │           └── preprocessor_test.go    # 6 tests
│   │   │   ├── mock_test.go                # 5 tests
│   │   │   └── slow.go                     # 慢速回显
│   │   ├── openaicompat/                   # Phase 3: OpenAI 兼容基类
│   │   │   ├── provider.go                 # SSE 解析 + 非流式
│   │   │   └── provider_test.go            # 6 tests（httptest upstream）
│   │   ├── doubao/                         # 豆包（火山方舟 OpenAI 兼容）
│   │   │   ├── doubao.go
│   │   │   └── doubao_test.go              # 3 tests
│   │   ├── deepseek/                       # DeepSeek
│   │   │   ├── deepseek.go
│   │   │   └── deepseek_test.go            # 3 tests
│   │   └── kimi/                           # Kimi（Moonshot）
│   │       ├── kimi.go
│   │       └── kimi_test.go                # 2 tests
│   ├── storage/                            # Phase 6 + 1.1.1: 文件/会话/消息存储
│   │   ├── file.go                         # 本地文件系统 + 截断
│   │   ├── file_test.go                    # 10 tests
│   │   ├── conv.go                         # 会话仓储（CRUD + pin）
│   │   ├── conv_test.go                    # 7 tests
│   │   ├── msg.go                          # 消息仓储
│   │   └── codec.go                        # jsonEncode helper
│   │   ├── onboarding.html / .js           # 4 步引导
│   │   ├── home.html / .js                 # Chat 主页面 + SSE 流式
│   │   ├── settings.html / .js             # API Key + AI 角色 + 高级参数
│   │   ├── history.html / .js              # 历史对话（占位，1.1 上线）
│   │   ├── i18n.js                         # zh/en 翻译
│   │   ├── app.js                          # 共享 API 客户端
│   │   └── style.css                       # 移动端优先样式
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

## 端到端验证（Phase 4.5 Keyring ↔ Chat 集成）

```bash
# 启动（设 AIIO_MASTER_KEY=32字节）
AIIO_MASTER_KEY=0123456789abcdef0123456789abcdef /tmp/aiio &

# 场景 1：chat 无 key → 400 引导用户去设置
curl -X POST .../chat/completions -d '{"model":"mock-echo",...}'
# → 400 {"error":{"code":"no_provider_configured",
#                "message":"no key configured for provider \"mock\", please add one in Settings"}}

# 场景 2：PUT key + chat → 200 用真 key
curl -X POST .../keys -d '{"provider":"mock","key":"my-real-secret"}'
# → 200 {"provider":"mock","updated_at":1785322564}
curl -X POST .../chat/completions -d '{"model":"mock-echo",...}'
# → 200 {"content":"echo: hi","provider":"mock",...}

# 场景 3：auto 模式部分无 key → 自动跳过无 key 的 provider
# (mock 配了 key，slow 没配)
curl -X POST .../chat/completions -d '{"model":"auto",...}'
# → 200 {"content":"echo: hi","provider":"mock",...}
# router 内部 keyFor 回调跳过 slow（无 key）
```

**核心设计**：
- `userKeyFor(provider)` 统一封装 Keyring 查询
  - Keyring == nil → fallback placeholder（仅 mock 演示用）
  - Keyring != nil 但 provider 无 key → 400 引导
- router 的 `AutoChat` / `Compare` 接收 `keyFor func(string) (string, error)` 回调
  - 每个 provider 调前各自取 key，无 key 时跳过该 provider
  - 不再传单一 userKey 字符串

**代码变更**：
- `internal/api/chat.go`: ChatHandler 加 Keyring 字段 + userKeyFor 方法
- `internal/routing/router.go`: AutoChat/Compare 改 keyFor 回调签名
- `internal/api/chat_keyring_test.go`: 4 新 test
- `internal/role/role.go`: 启动时把 keyring 传给 ChatHandler

## 端到端验证（Phase 3 真 Provider）

```bash
# 启动 master（5 provider 自动注册）
AIIO_MASTER_KEY=0123456789abcdef0123456789abcdef /tmp/aiio &

# 1. 列模型（10 个 model 来自 5 provider）
curl http://localhost:18080/api/v1/models
# mock / slow / doubao 3 / deepseek 2 / kimi 2

# 2. chat doubao 无 key → 400 引导
curl -X POST .../chat/completions -d '{"model":"doubao-1-5-pro-32k",...}'
# → 400 no_provider_configured

# 3. chat doubao 配 key 后（需要真 API key，未演示）
# → 200 + 豆包真实回复
```

**实现要点**：
- **OpenAI 兼容基类** (`internal/providers/openaicompat`)：复用 95% 代码
  - 60s timeout http.Client
  - Bearer token 透传
  - SSE 解析：bufio.Scanner + 1MB 行缓冲
  - 上下文取消：ctx.Done() 协程退出
- **3 个具体 provider** = 配置文件级别（每个 30-50 行）
  - doubao: 火山方舟 OpenAI 兼容端点
  - deepseek: api.deepseek.com/v1
  - kimi: api.moonshot.cn/v1
- **1.0 简化**：单进程内注册（不真做 Worker 远程转发）
  - docs/backend/02-provider.md §四 的多区域架构留 2.0
  - 当前 5 provider 都在 Master 进程内

**测试统计**：
- 累计 78 tests passed
  - config 3 + core 10 + observability 5 + api 18 + routing 9 + security 14
  - openaicompat 6 + doubao 3 + deepseek 3 + kimi 2
  - mockprovider 5

## 端到端验证（Phase 5 前端 MVP）

```bash
# 启动
AIIO_MASTER_KEY=... AIIO_ROLE=master AIIO_AUTH_TOKEN=devtoken /tmp/aiio &

# 静态文件服务
curl -sI http://localhost:18080/onboarding.html  # → 200
curl -sI http://localhost:18080/home.html         # → 200
curl -sI http://localhost:18080/settings.html      # → 200
curl -sI http://localhost:18080/history.html       # → 200
curl -s  http://localhost:18080/style.css         # → CSS
curl -s  http://localhost:18080/app.js            # → JS
curl -s  http://localhost:18080/i18n.js           # → JS

# API 仍然在原路径工作
curl -s  http://localhost:18080/api/v1/models      # → JSON
```

**1.0 极简决策**：
- **不引入 Vue 工具链**：4 HTML + 5 JS + 1 CSS = 10 个文件
- **零构建步骤**：直接 serve static/
- **i18n 自实现**（zh/en）：localStorage 存 lang，data-i18n 属性遍历
- **后端 SSE 兼容**：fetch + ReadableStream 直接读 data: 格式
- **不引入 Vant/Element Plus**：自写 CSS 移动端优先

**5 页内容**：
- `onboarding.html` — 4 步：选 provider → 配 key → 进聊天
- `home.html` — 模型 + 模式 + 消息流 + 输入框
  - 单个 / 自动选 / 对比 三模式
  - Ctrl+Enter 发送
  - 流式打字效果
- `settings.html` — API Key CRUD + AI 角色 + 温度/最大长度
- `history.html` — 占位（Phase 1.1 上线，详见 Phase 6 之后）
- i18n: 顶栏下拉切换 zh/en

**为什么不做 Vue 工程**：
- 1.0 用户量小，HTML+JS 体验足够
- 零 npm install 步骤，部署简单
- 后续可平滑升级：把 app.js 拆成 Vue SFC，HTML 改 SPA router

**文件清单**（`backend/static/`）：
```
onboarding.html  1119B
onboarding.js    1820B
home.html        1539B
home.js          5170B
settings.html    1877B
settings.js      4832B
history.html      508B
history.js        375B
i18n.js          3752B
app.js           4187B
style.css        4187B
```

**1.0 决策**：
- 文件路径 `backend/static/`（与 Go 二进制同目录）
- 浏览器直连（不走 CDN）
- 不做 SSR（无 SEO 需求）
- 不做 PWA（1.0 不需要离线）

**Phase 5 不做**（留 P1/P2）：
- Vue 工程化升级
- 真正的 5 页面 SPA router
- Vant/Element Plus 设计系统
- PWA manifest
- 暗色模式（CSS 已留好变量位置）

## 端到端验证（Phase 8 部署与文档）

```bash
# docker compose 配置校验（沙箱无网络，不能 build）
$ docker compose config --quiet
error while interpolating ... AIIO_MASTER_KEY is missing
# → 正确：env 校验生效，强制用户设 master key

# 本地 go run 跑（生产实际是 docker compose up）
$ go run ./cmd/aiio &
$ curl /health         # → 200
$ curl /onboarding.html  # → 200
$ curl /api/v1/models    # → 10 models 5 providers
$ curl /metrics          # → aiio_chat_total 0
```

**新增/修改文件**：
- `backend/Dockerfile` — 2 阶段：golang:1.22-alpine → distroless/static:nonroot
  - binary + static/ 一起 COPY 进镜像
  - CGO_ENABLED=0 纯静态
  - trimpath + ldflags 缩小体积
- `backend/docker-compose.yml` — 单 master 服务
  - env 校验（必填 AIIO_MASTER_KEY / AIIO_JWT_SECRET）
  - healthcheck
  - memory 200M 限制
  - 持久化 volume
- `backend/docker-compose.simple.yml` — 极简版（无 healthcheck / 资源限制）
- `backend/.env.example` — 模板（含 openssl 生成命令）
- `docs/deploy.md` — 5 分钟快速启动 + 详细步骤 + nginx 反代 + FAQ
- `docs/user-guide.md` — 3 分钟上手 + 3 种模式 + Settings + 快捷键 + 隐私
- `mkdocs.yml` — nav 加"运维"分组

**部署验证**：
- 沙箱里 docker build 因网络限制无法做（registry-1.docker.io 不可达）
- `docker compose config` 语法校验通过
- `go run` 替代方案端到端通过
- 用户在自己机器上 `docker compose up -d` 应当 5 分钟内跑通

**docker-compose 简化决策**：
- 1.0 阶段：只跑 master（worker 进程内占位）
- 2.0 阶段：可拆出 worker-cn / worker-us 到不同 VPS
- 完整版 compose（含 worker）保留为参考

## 端到端验证（Phase 6 文件上传）

```bash
# 上传图片
curl -X POST http://localhost:18080/api/v1/files \
  -H "Authorization: Bearer devtoken" \
  -F "file=@test.png;type=image/png"
# → 201 {
#     "id":"file_203eb767ec94de7c979cec9d",
#     "owner_user_id":"default",
#     "filename":"test-upload.png",
#     "mime_type":"image/png",
#     "size":14,"processed_size":14,
#     "truncated":false,
#     "source":"user_upload",
#     "created_at":"..."
#   }

# 列出
curl http://localhost:18080/api/v1/files -H "Authorization: Bearer devtoken"
# → {"files":[...]}

# 拒绝不支持的 mime
curl -X POST .../files -F "file=@virus.exe;type=application/x-msdownload"
# → 400 {"error":{"code":"file_unsupported",...}}

# 磁盘结构
ls /data/files/                  # → file_xxx.bin 0600
cat /data/file_index.json         # → _files 索引
```

**核心实现**：
- **本地文件系统**：`data/files/{id}.bin` + `data/file_index.json`
- **大小限制**：图片 5MB / 文档 50KB
- **截断策略**：超限 → 头 30K + 中间省略 + 尾 20K
- **mime 校验**：`image/*` / `text/*` / `application/pdf` / `application/json`
- **owner 隔离**：1.0 单用户（"default"），2.0 接 JWT 后用真实 user_id
- **拒绝 office/二进制**（1.0 简化，不引入 PDF 抽文本库）

**测试统计**：
- 累计 96 tests passed（+18）
  - 之前 78
  - storage 10 + api 8 = 18 新增

**Phase 6 决策**：
- 1.0 不做 PDF 抽文本 / 图片压缩（仅截断）
- 1.0 不做文件级权限（所有上传归 "default"）
- 2.0 升级：图片压缩、PDF 抽文本、多用户隔离、Office 支持

**已知 1.0 限制**（待 Phase 1.1+）：
- ~~chat 不读 attachments~~ ✅ Phase 1.1 已解决
- ~~file_id 注入 messages~~ ✅ Phase 1.1 已解决
- 历史会话：未实现（前端占位）
- 1.0 简化：图片类附件只标注 mime，不做 vision 抽取（让用户描述）

## 端到端验证（Phase 1.1 附件注入）

```bash
# 1. 上传文件
curl -X POST .../files -F "file=@test.txt;type=text/plain"
# → 201 + file_id

# 2. chat with attachment
curl -X POST .../chat/completions -d '{
  "model":"mock-echo",
  "messages":[{
    "role":"user",
    "content":"看下这个文件",
    "attachments":["file_xxx"]
  }]
}'
# → 200 + AI echo 出来看到完整附件内容
```

**实际 mock echo 收到**：

```
echo: [附件: test.txt (text/plain, 37 字节)]
This is the file content for testing.

---

看下这个文件说了什么
```

格式约定：
```
[附件: filename (mime, size 字节, → 截断到 size 字节)]
<文件内容>

---

<原用户消息>
```

**核心实现**：
- `internal/capabilities/chat/preprocessing/preprocessor.go`
  - 遍历 messages 找 `attachments` 字段
  - 从 FileStore 读 file_id 对应的内容和元信息
  - 注入到 message content 前面
  - system 消息不动（保护 system prompt）
  - owner 校验：跨用户访问被跳过 + 警告
- `internal/api/chat.go` 在路由前调用 Preprocessor
  - `req.Messages` 被替换为处理后的版本
  - 后续 Provider 拿到的就是带附件内容的 messages

**测试覆盖**（6 tests）：
- 小文件 → 完整内容
- 大文件 → 截断 + 警告
- 多附件 → 全部注入
- 无附件 → 不变
- 跨用户 → 跳过 + 警告
- system 消息保护

**1.0 简化**：
- 图片附件不抽取（标注 mime + 让用户描述）
- PDF 附件 1.0 阶段按文本注入（FileStore 已存原文）
- 不做 token 估算（让 Provider 自己处理）
- 不做 PDF 抽文本（留 2.0）

## 端到端验证（Phase 7 移动端打磨）

```bash
# 1. PWA manifest
curl http://localhost:18080/manifest.webmanifest | jq
# {
#   "name": "AI All-in-One",
#   "short_name": "AIIO",
#   "start_url": "/home.html",
#   "display": "standalone",
#   "theme_color": "#4f46e5",
#   "icons": [{"src": "...","sizes":"192x192"}, {"src":"...","sizes":"512x512","purpose":"any maskable"}]
# }

# 2. 暗色 CSS 变量
curl -s http://localhost:18080/style.css | grep 'data-theme="dark"'
# → :root[data-theme="dark"] {
# →   --bg: #0f0f0f;
# →   --bg-card: #1a1a1a;
# →   ...

# 3. a11y 属性
curl -s http://localhost:18080/home.html | grep 'aria-'
# → role="log" aria-live="polite" aria-label="消息列表"
# → aria-label="停止生成" / "发送消息"
# → aria-label="主题切换" / "语言切换"

# 4. 主题色 meta（移动浏览器状态栏）
curl -s http://localhost:18080/home.html | grep theme-color
# → <meta name="theme-color" content="#4f46e5" />
```

**核心增强**：

### 暗色模式
- 全部颜色改为 CSS 变量（`--bg` `--text` `--primary` 等 20+ 变量）
- 3 模式：auto（跟随系统）/ light / dark
- localStorage 持久化用户选择
- 暗色配色专门调过（背景 #0f0f0f 不是纯黑，护眼）

### a11y 改进
- 所有交互元素加 `aria-label`
- 消息区 `role="log" aria-live="polite"`（屏幕阅读器自动播报新消息）
- 主题/语言选择器加 `<label>` + visually-hidden
- 焦点环：`outline: 2px solid var(--focus-ring)` 替代浏览器默认
- `prefers-reduced-motion` 尊重动画偏好

### 触摸目标
- 所有 button / select / input `min-height: 44px`（Apple HIG 标准）
- 表单元素 padding 加大
- 关键按钮 `min-width: 44px`

### PWA
- `manifest.webmanifest` 合法（name/short_name/start_url/icons/display/theme_color/background_color）
- 2 个 icon（192 + 512 maskable，纯 inline SVG，无外部资源）
- 顶栏加 `<link rel="manifest">` + `<meta name="theme-color">`
- 1.0 不做 service worker 离线缓存（保持简单）

**端到端验证**：
- 4 个 HTML 文件 200 + 含 theme-color meta
- /manifest.webmanifest 200 + 合法 JSON
- style.css 320 行（之前 200 行）+ 暗色 + 触摸目标 + a11y
- 浏览器 DevTools 模拟暗色：CSS 变量自动切换

**1.0 不做**（留 P1/P2）：
- service worker 离线缓存（基础网络假设始终可用）
- 推送通知
- 桌面快捷方式（macOS dock / Windows taskbar）
- 长按 / 滑动 / 拖拽等高级手势

## 端到端验证（Phase 1.1.1 历史会话）

```bash
# 1. POST /api/v1/conversations
curl -X POST .../conversations -d '{"model":"mock-echo"}'
# → 201 {"id":"conv_1a77a2dca1c5fa0b","title":"新对话","model":"mock-echo",...}

# 2. GET /api/v1/conversations
curl .../conversations
# → {"conversations":[{...}]}

# 3. GET /api/v1/conversations/{id}
curl .../conversations/conv_1a77a2dca1c5fa0b
# → {"conversation":{...},"messages":null}

# 4. PATCH 改标题
curl -X PATCH .../conversations/conv_xxx -d '{"title":"我的新对话"}'
# → 200 {"title":"我的新对话",...}

# 5. DELETE
curl -X DELETE .../conversations/conv_xxx
# → 204

# 6. GET after delete
curl .../conversations/conv_xxx
# → 404 {"error":{"code":"conv_not_found","message":"conversation not found"}}
```

**磁盘结构**：
```
/data/conv_index.json   # 会话索引（0600）
/data/msg_index.json    # 消息索引（0600）
```

**前端 history.js 真实功能**：
- 列出 conv（按 pinned + updated_at 排序）
- 新建 conv 按钮（顶部）
- 打开 conv（跳转 home.html 带 ?conv=ID，存到 localStorage）
- 改名（PATCH title，弹 prompt）
- 删除（DELETE，弹 confirm）
- 相对时间显示（刚刚 / N 分钟前 / N 小时前 / N 天前）
- 置顶图标（📌）
- 错误时 showToast 提示

**核心实现**：
- `internal/storage/conv.go` — ConvRepo（CRUD + Pin + List 排序）
  - JSON 文件索引（0600）
  - 跨用户访问返回 ErrConvNotFound
  - 置顶优先 + 按 updated_at 倒序
- `internal/storage/msg.go` — MsgRepo
  - 按 conv_id 过滤
  - ListByConv 顺时序
  - 级联删除（Delete conv 时调 deleteByConv）
- `internal/api/convs.go` — ConvsHandler + ConvItemHandler
  - 5 端点：GET 列表 / POST 新建 / GET 详情 / PATCH 改标题或置顶 / DELETE
  - 路由解析：TrimPrefix("/api/v1/conversations/", id)
- `static/history.js` — 完整 CRUD UI
- `static/i18n.js` — 加 4 条 history 字符串（open / rename / confirmDelete）

**测试统计**：
- 累计 116 tests passed（+14）
  - 之前 102
  - storage 7 + api 6 + ... = 13 convs 相关（另加 1 修复 404）

**已知 1.0 限制**（留 1.2）：
- chat 路由不自动落消息：1.0 阶段会话创建了但 chat 时不自动 Append 到 MsgRepo
- 1.2 计划：chat handler 调完 Provider 后，Append user msg + assistant msg
- 1.2 计划：home.html 启动时读 ?conv=ID 加载历史消息
- 1.2 计划：完整 streaming 续接（流中断时保存 partial msg）

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
| 2026-07-29 | d965f82 | 4 | Key 管理：AES-256-GCM 加密 + JSON 文件 Keyring + CRUD API |
| 2026-07-29 | d64c589 | 4.5 | Keyring ↔ chat 集成：userKeyFor + router 接受 keyFor 回调 |
| 2026-07-29 | fea0b47 | 3 | Worker + 豆包/DeepSeek/Kimi OpenAI 兼容 Provider |
| 2026-07-29 | 651db93 | 5 | 前端 MVP：4 HTML + 5 JS + 1 CSS，零构建工具，i18n zh/en |
| 2026-07-29 | fb086c7 | 8 | 部署：Dockerfile + docker-compose.yml + .env.example + deploy.md + user-guide.md |
| 2026-07-29 | 0efe4c0 | 6 | 文件上传：本地文件系统 + 截断 + mime 校验 + 端到端 upload/list/reject |
| 2026-07-29 | 9db371f | 6.5 | 附件注入 chat：preprocessing 把 file_id 解析为文本注入 messages |
| 2026-07-29 | (pending) | 1.1.1 | 历史会话：conv/msg repo + 5 端点 + history.js 真实 CRUD + i18n 4 字符串 |
| 2026-07-29 | (pending) | 7 | 移动端打磨：暗色模式 + a11y + PWA manifest + 44pt 触摸目标 |

