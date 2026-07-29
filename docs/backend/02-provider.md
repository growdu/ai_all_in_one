# 后端设计（Go · 多区域）

> 2026-07 更新：原 Python/FastAPI 方案性能与并发不满足海外代理场景，迁移到 Go。
> 关键变更：
>   - 运行时：Go 1.22+ · Gin/Fiber
>   - 部署模型：**单二进制双角色**（同一仓库，同一可执行文件，启动时按 `AIIO_ROLE` 决定）
>   - 新增角色：Master（用户面、鉴权、Key 存储、路由）与 Worker（区域代理，仅转发厂商 API）
>   - **对外接口完全不变**，前端零改动

## 一、为什么 Go

| 维度 | Python (FastAPI) | Go (Gin/Fiber) |
|------|------------------|----------------|
| 单机并发 | ~千级（受 GIL 间接影响） | 十万级 goroutine |
| 内存占用 | 100MB+ / worker | 10MB / 实例 |
| 部署 | Python 运行时 + 依赖包 | 单二进制（含 UI 资源时也单文件） |
| 跨区域网络代理 | 需要 gevent/uvloop 调优 | 原生 net/http · context 取消干净 |
| 流式 (SSE) | OK | 原生更顺，零拷贝 bufio.Scanner 友好 |
| 类型系统 | 运行时 | 编译期（与 Pydantic 同等保障） |

性能不是选 Go 的唯一原因。**真正原因是多区域 Worker 需要轻量、可静态交叉编译、能用同一份代码做不同的事**。

## 二、核心抽象保持不变

Modality / Capability / Provider 三层架构不变，只是从 Python Protocol 改成 Go interface：

```go
// internal/core/capability.go
type Modality string
const (
    ModalityChat  Modality = "chat"
    ModalityMusic Modality = "music"
    ModalityVideo Modality = "video"
    ModalityImage Modality = "image"
    ModalityTTS   Modality = "tts"
)

type ModelInfo struct {
    ID              string   `json:"id"`
    DisplayName     string   `json:"display_name"`
    Provider        string   `json:"provider"`
    Modality        Modality `json:"modality"`
    Capabilities    []string `json:"capabilities"`
    ContextWindow   int      `json:"context_window"`
    InputPricePer1k *float64 `json:"input_price_per_1k,omitempty"`
    OutputPricePer1k *float64 `json:"output_price_per_1k,omitempty"`
}

// internal/core/provider.go
type ChatProvider interface {
    Name() string
    ListModels() []ModelInfo
    Chat(ctx context.Context, req ChatRequest, userKey string) (ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest, userKey string) (<-chan ChatChunk, <-chan error)
}
```

ProviderRegistry / Modality / Capability 全部从 Python 重写到 Go，**接口契约 1:1 对齐**。

## 三、目录结构

```
backend/
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile                   # 多阶段：build → scratch/distroless
├── docker-compose.yml           # master + worker 编排示例
│
├── cmd/
│   └── aiio/
│       └── main.go              # 唯一入口，按 AIIO_ROLE 决定启动什么
│
├── internal/
│   ├── core/                    # 跨角色共享
│   │   ├── capability.go        # Modality / Capability
│   │   ├── provider.go          # interface 定义
│   │   ├── registry.go          # ProviderRegistry
│   │   ├── models.go            # ChatRequest / ChatResponse / ChatChunk
│   │   └── errors.go            # 统一错误
│   │
│   ├── providers/               # 厂商适配器
│   │   ├── openai_compat/       # OpenAI 兼容基类（豆包/DeepSeek/Kimi 复用）
│   │   │   └── provider.go
│   │   ├── doubao/
│   │   ├── openai/
│   │   ├── deepseek/
│   │   ├── kimi/
│   │   └── claude/              # 非 OpenAI 兼容，单独写
│   │
│   ├── transport/               # HTTP 客户端与流式
│   │   ├── http_client.go       # fasthttp/nethttp + 连接池 + 重试
│   │   ├── sse.go               # SSE 解析（厂商→ChatChunk）
│   │   └── sse_cache.go         # 流缓存 + 续传（review 2.2）
│   │
│   ├── security/                # 鉴权 / 加密
│   │   ├── aesgcm.go          # AES-256-GCM，Key 加密与通道封装共用
│   │   ├── jwt.go               # 用户 token（Master 签，Worker 验）
│   │   └── mtls.go              # Master↔Worker 双向 TLS（可选）
│   │
│   ├── storage/                 # 存储抽象
│   │   ├── keyring.go           # 用户 Key 存取
│   │   ├── files.go             # 文件存储
│   │   └── sqlite.go            # 1.0 嵌入式数据库（modernc.org/sqlite 纯 Go）
│   │
│   ├── role/                    # 角色逻辑
│   │   ├── master.go            # Master 模式启动
│   │   └── worker.go            # Worker 模式启动
│   │
│   ├── api/                     # 路由层
│   │   ├── v1/
│   │   │   ├── models.go        # GET /api/v1/models
│   │   │   ├── chat.go          # POST /api/v1/chat/completions
│   │   │   ├── keys.go          # 用户 Key CRUD
│   │   │   ├── files.go         # 文件上传/下载
│   │   │   └── usage.go         # 预留
│   │   └── middleware/
│   │       ├── auth.go
│   │       ├── ratelimit.go     # 令牌桶
│   │       └── recovery.go
│   │
│   ├── routing/                 # Master 专用：决定请求发到哪个 Provider
│   │   ├── router.go            # single/auto/compare 三模式分发
│   │   ├── strategy.go          # by-region / by-provider / failover
│   │   ├── signals.go           # 滑动窗口信号收集
│   │   ├── scoring.go           # auto 模式打分公式
│   │   └── compare.go           # 并行 compare 实现
│   │                            # 详见 docs/architecture/01-routing-strategy.md
│   │
│   └── config/
│       └── config.go            # YAML + env 加载
│
├── configs/
│   ├── master.yaml              # Master 端配置示例
│   ├── worker-cn.yaml           # 国内 Worker
│   ├── worker-us.yaml           # 美国 Worker（代理 OpenAI/Claude）
│   └── worker-eu.yaml           # 欧洲 Worker
│
└── test/
    ├── e2e/                     # 端到端
    └── fixtures/
```

## 四、双角色模型

### 4.1 角色定义

| 角色 | 部署位置 | 职责 | 不做什么 |
|------|---------|------|---------|
| **Master** | 用户就近（国内/任一区域） | 用户鉴权、Key 存储、文件存储、模型发现、请求路由、用量统计、对外 API 入口 | 不直接调厂商 API |
| **Worker** | 厂商就近（豆包 Worker 放国内，OpenAI Worker 放美西/东亚，Claude 放美国） | 接受 Master 转发、调对应厂商、流式回传 | 不存用户数据（除临时转发外）、不对外暴露用户 API |

### 4.2 启动模式

```bash
# Master
AIIO_ROLE=master AIIO_CONFIG=configs/master.yaml ./aiio

# Worker（区域国内）
AIIO_ROLE=worker AIIO_REGION=cn AIIO_CONFIG=configs/worker-cn.yaml ./aiio

# Worker（美国，代理 OpenAI/Claude）
AIIO_ROLE=worker AIIO_REGION=us-west AIIO_CONFIG=configs/worker-us.yaml ./aiio
```

同一个二进制读环境变量分支到 `role.Master()` 或 `role.Worker()` 启动。

### 4.3 Master ↔ Worker 通信

```
                    ┌──────────────────────────────┐
                    │           Master             │
 用户 ──HTTPS──▶    │  • 鉴权                      │
                    │  • 选 Worker (routing)        │
                    │  • 注入用户 Key               │
                    │  • 流式回包给用户              │
                    └──────────┬───────────────────┘
                               │  mTLS / JWT
              ┌────────────────┼──────────────────┐
              ▼                ▼                  ▼
     ┌────────────────┐ ┌────────────┐ ┌────────────────┐
     │ Worker (cn)    │ │ Worker (us)│ │ Worker (eu)    │
     │ 豆包/DeepSeek  │ │ OpenAI     │ │ Mistral        │
     │ Kimi           │ │ Claude     │ │ (预留)         │
     │                │ │ Gemini     │ │                │
     └────────────────┘ └────────────┘ └────────────────┘
```

- Master→Worker：HTTP/2 + mTLS（自签 CA 即可，1.0 简化版用共享 HMAC 签名）
- 请求体：Master 把解密后的用户 Key 注入到内部协议头，Worker 不落盘
- 流式：Worker → Master → 用户 全程 SSE

### 4.4 内部协议（Master ↔ Worker）

复用 OpenAI Chat Completions 格式作为内部协议（避免重复造轮子）：

```http
POST /internal/chat HTTP/2
Authorization: Bearer <master-worker-shared-token>
X-AIIO-Region: us-west
X-AIIO-User-Key: <aesgcm-encrypted-by-master, only-in-transit>  # 不落盘
Content-Type: application/json

{ "model": "gpt-4o", "messages": [...], "stream": true }
```

返回 SSE（OpenAI 兼容），Worker 不做协议转换，Master 也不做——**端到端透传**，Master 只做鉴权、Key 注入、计量。

## 五、单二进制双角色的关键工程

### 5.1 main.go 分支

```go
package main

import (
    "log"
    "os"
    "aiio/internal/role"
)

func main() {
    cfg, err := config.Load(os.Getenv("AIIO_CONFIG"))
    if err != nil { log.Fatal(err) }

    switch os.Getenv("AIIO_ROLE") {
    case "master":
        role.RunMaster(cfg)
    case "worker":
        region := os.Getenv("AIIO_REGION")
        if region == "" { log.Fatal("AIIO_REGION required for worker") }
        role.RunWorker(cfg, region)
    default:
        log.Fatal("AIIO_ROLE must be 'master' or 'worker'")
    }
}
```

### 5.2 配置示例

```yaml
# configs/master.yaml
server:
  listen: ":8080"
  tls:
    enabled: true
    cert_file: ./certs/master.crt
    key_file: ./certs/master.key

storage:
  sqlite_path: ./data/master.db

security:
  master_key_env: AIIO_MASTER_KEY
  jwt_secret_env: AIIO_JWT_SECRET

workers:
  - id: cn-hangzhou
    region: cn
    endpoint: https://worker-cn.internal:8443
    providers: [doubao, deepseek, kimi]
  - id: us-west
    region: us-west
    endpoint: https://worker-us.internal:8443
    providers: [openai, claude]

routing:
  strategy: by-provider    # by-provider / by-latency / failover
  health_check_interval: 10s
```

```yaml
# configs/worker-us.yaml
server:
  listen: ":8443"
  tls:
    enabled: true
    cert_file: ./certs/worker.crt
    key_file: ./certs/worker.key
    client_ca_file: ./certs/master-ca.crt  # mTLS

providers:
  openai:
    base_url: https://api.openai.com/v1
    upstream_timeout: 60s
  claude:
    base_url: https://api.anthropic.com
    upstream_timeout: 60s
```

## 六、与原 Python 设计的关键差异

| 维度 | 原 Python | 新 Go |
|------|----------|--------|
| Web 框架 | FastAPI | **Gin**（生态最广）或 Fiber（更快） |
| ASGI vs HTTP | ASGI | net/http + Gin |
| 异步 | asyncio | goroutine（更轻量） |
| 加密 | cryptography.fernet | 自实现 AES-256-GCM（~80 行，零依赖） |
| 数据库 | SQLModel + SQLite | modernc.org/sqlite（**纯 Go 驱动，无需 CGO**） |
| HTTP 客户端 | httpx | net/http + 自封装连接池（够用）/ fasthttp（极致） |
| 流式解析 | sse_starlette | bufio.Scanner 手动解析 |
| 配置 | pyyaml + pydantic-settings | yaml.v3 + envconfig |
| 类型校验 | Pydantic | struct tag + go-playground/validator |

**外部接口 100% 兼容**：URL、Header、Body 一字不差。前端 SDK、文档示例、客户端测试全部复用。

## 七、Provider 实现：OpenAI 兼容基类

```go
// internal/providers/openai_compat/provider.go
type OpenAICompat struct {
    name    string
    baseURL string
    models  map[string]ModelInfo
    client  *http.Client
}

func New(name, baseURL string, models []ModelInfo) *OpenAICompat {
    return &OpenAICompat{
        name: name, baseURL: baseURL,
        models: sliceToMap(models),
        client: &http.Client{Timeout: 60 * time.Second},
    }
}

func (p *OpenAICompat) Name() string { return p.name }

func (p *OpenAICompat) ListModels() []ModelInfo { /* ... */ }

func (p *OpenAICompat) ChatStream(ctx context.Context, req ChatRequest, userKey string) (<-chan ChatChunk, <-chan error) {
    out := make(chan ChatChunk, 16)
    errs := make(chan error, 1)
    go func() {
        defer close(out); defer close(errs)
        // 用 net/http 发起 SSE 流式，逐 chunk 推到 out
    }()
    return out, errs
}

// 注册
func Register(name, baseURL string, models []ModelInfo) {
    core.Registry.RegisterChat(New(name, baseURL, models))
}
```

新增厂商 = 一个新文件 + 一个 `init()` 或显式注册调用。

## 八、安全：Key 不出 Master

```
用户 ──Key──▶ Master 加密落库
                │
                ▼
         Master 解密
                │
                ▼
         转发时用 AES-GCM 临时封装，塞 X-AIIO-User-Key header
                │
                ▼
         Worker 解出明文 → 调厂商 API → 用完丢弃
                │
                ▼
         Worker 进程内存中永不持久化
```

1.0 用 AES-GCM + 共享密钥；2.0 上 mTLS 后可考虑去掉这个 header，靠 mTLS 通道本身保证安全。

## 九点五、长对话自动截断 {#chat-truncation}

> 解决 review 3.4："用户聊到 100 轮正嗨，boom — 上下文超限"。

### 9.5.1 触发条件

Master 在调用 Provider 之前估算 token 数：

```go
// internal/capabilities/chat/truncator.go
type Truncator struct {
    model    ModelInfo
    counter  func(string) int   // 简单字符/token 估算
}

func (t *Truncator) Fit(messages []ChatMessage) ([]ChatMessage, TruncatedInfo) {
    budget := t.model.ContextWindow * 90 / 100   // 留 10% 给输出
    // ...
}
```

**token 估算**：
- 1.0 用字符/4 粗估（中文/英文混用 ≈ chars/3）
- 2.0 接入 tiktoken-go 精确算
- 估算误差 20% 没关系，反正留 10% buffer

### 9.5.2 截断策略（3 步降级）

1. **丢最早的 user/assistant 轮**（保留 system）
2. **仍超限**：丢到只保留 system + 最近 10 轮
3. **仍超限**：返回 422 + `code: context_too_long`，前端弹"建议开启新对话"

**保留规则**：
- system 消息永远保留
- 最近的 user 消息永远保留（用户当前问题）
- tool 消息与对应 assistant tool_call 一起保留（不能拆）

### 9.5.3 响应

正常截断时响应头：

```
X-AIIO-Messages-Truncated: 5    # 丢了 5 条
X-AIIO-Context-Usage: 0.87      # 截断后上下文使用率
```

前端按 X-AIIO-Context-Usage 在顶栏显示进度条：
- < 70%：绿色
- 70-90%：黄色
- > 90%：红色，提示"建议开启新对话"

### 9.5.4 高级用户

Settings 加开关 "锁定 system prompt"，开启后 system 消息不被截断，可手动编辑。

### 9.5.5 为什么在 Master 而非 Provider

- Provider 限流 / 超时由 Provider 自己判
- 但**输入消息的截断决策**涉及 system 保留、tool 配对等业务规则
- 放 Master 才能跨 Provider 一致
- 也方便后续接 AI summarization 进一步压缩（2.0）

## 九点六、SQLite 可靠性（review 2.4）

1.0 用 modernc.org/sqlite 纯 Go 驱动。必须开的 PRAGMA 与备份策略：

**启动时 PRAGMA**（`internal/storage/sqlite.go` 初始化时执行）：

```sql
PRAGMA journal_mode = WAL;           -- 读写并发，写不阻塞读
PRAGMA synchronous = NORMAL;         -- WAL 模式下安全，崩溃最多丢最后一次事务
PRAGMA foreign_keys = ON;            -- 启用外键（SQLite 默认关闭）
PRAGMA busy_timeout = 5000;          -- 5s 忙等，避免 SQLITE_BUSY
PRAGMA temp_store = MEMORY;          -- 临时表放内存，提速
```

**WAL checkpoint**：
- 默认 SQLite 在 WAL 满 1000 页时自动 checkpoint
- 1.0 用户少可接受
- 2.0 上线后加手动 checkpoint：每天低峰期调 `PRAGMA wal_checkpoint(TRUNCATE)`

**备份策略**（`internal/storage/backup.go`）：

| 周期 | 方式 | 保留 | 实现 |
|------|------|------|------|
| 每小时 | `VACUUM INTO '/backup/db-YYYYMMDDHH.db'` | 24 份 | 定时 goroutine |
| 每天 | 整库 cp 到 `/backup/daily-YYYYMMDD.db` | 30 份 | cron / docker 外部脚本 |
| 启动时 | 上次备份完整性 check（`PRAGMA integrity_check`） | N/A | 启动流程 |

**磁盘空间预估**（1.0）：

- 1 个用户 1000 条对话 × 5KB/条 ≈ 5MB
- 1000 用户 ≈ 5GB
- WAL 文件额外 ~10%
- 备份 24h × 5GB ≈ 120GB，**1.0 不可能达到**，策略可降到每 6h 备份 + 保留 4 份

**为什么不开 FULL 同步**：
- WAL + NORMAL 已能保证崩溃后数据库一致
- FULL 性能差 10x，1.0 不值得
- 2.0 真有合规需求时再切

**为什么 1.0 就要 WAL**：
- 不用 WAL → 写阻塞读 → 聊天时存对话历史会卡住当前响应
- 开 WAL 几乎零成本（写多一行配置）
- 不用等需要才加

**故障恢复流程**（写到 deploy.md）：

```
1. 启动时 integrity_check 失败
2. 报警：DB corrupted at <path>
3. 尝试用最近 hourly 备份恢复：
   cp /backup/db-latest.db /data/master.db
4. 仍失败 → 用 daily 备份
5. 全部失败 → 联系用户，全新初始化（Key 全部要重配）
```

## 九点四、可观测性（review 2.7）

> 解决"用户报用着用着变慢，Master 端看不到"。

1.0 最小可观测：JSON 行日志 + Prometheus 指标端点。**不上 OpenTelemetry**（2.0 引入）。

### 9.4.1 结构化日志

**每次 chat 请求记录**（写到 stdout JSON）：

```json
{"ts":"2026-07-29T10:00:00Z","level":"info","event":"chat","user_id":"u_xxx","provider":"doubao","model":"doubao-1-5-pro-32k","latency_ms":1240,"prompt_tokens":12,"completion_tokens":234,"status":"succeeded","error":null}
```

字段：
- `event`：固定 `chat` / `chat_failed` / `user_switched` / `rate_limited` / `auth_failed`
- `latency_ms`：从接到请求到响应结束
- `prompt_tokens` / `completion_tokens`：从厂商响应里抓
- `status`：`succeeded` / `failed`
- `error`：失败时填 code

**实现**：`internal/observability/log.go`
- 用 `log/slog`（Go 1.21+ 标准库）
- 默认 JSON handler，stdout
- 1.0 不引第三方日志库

### 9.4.2 Prometheus 指标

```
GET /metrics
```

**1.0 暴露的指标**：

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `aiio_chat_total` | counter | provider, model, status | chat 调用总数 |
| `aiio_chat_latency_seconds` | histogram | provider, model | 端到端延迟 |
| `aiio_chat_tokens_total` | counter | provider, model, type (prompt/completion) | token 用量 |
| `aiio_rate_limit_hits_total` | counter | layer (user/provider/global) | 限流命中次数 |
| `aiio_sse_cache_hits_total` | counter | result (hit/miss/gone) | SSE 续传缓存命中 |
| `aiio_active_streams` | gauge | provider | 当前活跃流数 |

**实现**：`internal/observability/metrics.go`
- 用 `github.com/prometheus/client_golang/prometheus`
- 注册到 `/metrics` 端点
- 1.0 裸指标，无认证（仅本机或反代后访问）

### 9.4.3 健康检查

```
GET /health
```

返回：

```json
{"status": "ok", "version": "0.1.0", "uptime_sec": 3600, "db_ok": true}
```

- `db_ok` = `PRAGMA quick_check` 通过
- 2.0 加 workers_ok（每个 Worker 健康检查状态）

**实现**：`internal/observability/health.go`

### 9.4.4 为什么 1.0 不上 OpenTelemetry

- 1.0 单二进制单 Master，分布式 trace 价值不大
- Prometheus 指标 + 日志够定位 80% 问题
- 2.0 引入 OTel 的成本 ≈ 0.5 人天，但收益要在多 Master 横向扩展后才显现

## 九点八、输入处理 Pipeline（review C）

> 实现细节见 [输入处理设计](../architecture/02-input-processing.md)。这里只列接口与流程。

**Pipeline 顺序**：

```
用户请求 → [1. 鉴权] → [2. 限流] → [3. 附件预处理] → [4. Prompt 增强] → [5. 长对话截断] → [6. 路由到 Provider]
```

| 步骤 | 必走 | 失败 fallback |
|------|------|---------------|
| 1-2 | 是 | 返回 401/429 |
| 3 附件预处理 | 是 | 单附件失败 → 跳过该附件，注入警告；全部失败 → 错误码 |
| 4 Prompt 增强 | 用户开关 | 关闭或异常 → 字面透传 |
| 5 截断 | 是 | 超限 → 422 context_too_long |
| 6 路由 | 是 | 失败 → 502 + Provider 错误码 |

**核心数据结构**（`internal/capabilities/chat/pipeline.go`）：

```go
type Pipeline struct {
    preprocessors map[string]Preprocessor
    enhancer      *Enhancer
    truncator     *Truncator
}

func (p *Pipeline) Process(ctx context.Context, req ChatRequest, userID string) (ChatRequest, ProcessingInfo, error) {
    // 见输入处理 §6.2 完整实现
}
```

**注册表**（`internal/capabilities/chat/preprocess/registry.go`）：

```go
var DefaultRegistry = &Registry{
    Processors: map[string]Preprocessor{
        "text/plain":      NewTextProcessor(50 * 1024),
        "text/x-python":   NewTextProcessor(50 * 1024),
        "application/pdf": NewPDFProcessor(),
        "image/png":       NewImageProcessor(5 * 1024 * 1024),
        "image/jpeg":      NewImageProcessor(5 * 1024 * 1024),
    },
}
```

**YAGNI 边界**：
- 1.0 不做 Office（.docx .xlsx）
- 1.0 不做音频/视频转写
- 1.0 不做用户自定义增强模板

## 九点七、历史会话 schema（review 1.4）

> 解决 review 1.4：1.0 最小可用版历史持久化。

### 9.7.1 数据库表

```sql
-- 会话表
CREATE TABLE conversation (
  id           TEXT PRIMARY KEY,        -- conv_xxx
  owner_id     TEXT NOT NULL,           -- user_xxx
  title        TEXT NOT NULL DEFAULT '新对话',
  model        TEXT NOT NULL,           -- 用的 model id
  mode         TEXT NOT NULL DEFAULT 'single',  -- single/auto/compare
  system_prompt TEXT,                   -- 会话级 system prompt（可空）
  pinned       INTEGER NOT NULL DEFAULT 0,
  created_at   TIMESTAMP NOT NULL,
  updated_at   TIMESTAMP NOT NULL,
  FOREIGN KEY (owner_id) REFERENCES user(id)
);
CREATE INDEX idx_conv_owner_updated ON conversation(owner_id, updated_at DESC);

-- 消息表
CREATE TABLE message (
  id              TEXT PRIMARY KEY,    -- msg_xxx
  conversation_id TEXT NOT NULL,
  role            TEXT NOT NULL,       -- user/assistant/system/tool
  content         TEXT NOT NULL,
  attachments     TEXT NOT NULL DEFAULT '[]',  -- JSON array of file_id
  model           TEXT,                -- 仅 assistant 有
  latency_ms      INTEGER,             -- 仅 assistant 有
  created_at      TIMESTAMP NOT NULL,
  FOREIGN KEY (conversation_id) REFERENCES conversation(id) ON DELETE CASCADE
);
CREATE INDEX idx_msg_conv ON message(conversation_id, created_at);
```

### 9.7.2 仓储接口

```go
// internal/db/conv_repo.go
type ConvRepo struct { db *sql.DB }

func (r *ConvRepo) Create(ctx context.Context, ownerID string, model string) (*Conversation, error)
func (r *ConvRepo) List(ctx context.Context, ownerID string, limit int, offset int) ([]*Conversation, error)
func (r *ConvRepo) Get(ctx context.Context, id string, ownerID string) (*Conversation, error)
func (r *ConvRepo) UpdateTitle(ctx context.Context, id string, ownerID string, title string) error
func (r *ConvRepo) Pin(ctx context.Context, id string, ownerID string, pinned bool) error
func (r *ConvRepo) Delete(ctx context.Context, id string, ownerID string) error  // 级联删消息
```

```go
// internal/db/msg_repo.go
type MsgRepo struct { db *sql.DB }

func (r *MsgRepo) Append(ctx context.Context, convID string, msg *Message) error
func (r *MsgRepo) ListByConv(ctx context.Context, convID string, ownerID string) ([]*Message, error)
```

### 9.7.3 路由

见 [统一协议 §1.5 历史会话](../api/01-protocol.md#15)。5 个端点对应 5 个 service 方法。

### 9.7.4 权限

所有查询都带 `WHERE owner_id = ?`，**强制隔离**。SQL 层防御，不依赖业务层。

### 9.7.5 写入时机

- 用户发消息 → `convRepo` 更新 `updated_at`，`msgRepo.Append(user)`
- AI 响应完成 → `msgRepo.Append(assistant)`，`convRepo` 更新 `updated_at`
- 流式场景：assistant 消息先建空 record（status=streaming），流结束后 PATCH content

### 9.7.6 为什么 1.0 不做全文搜索

- 1.0 用户量小，列表翻页够用
- FTS5 索引 2.0 加，几行 SQL
- 提前加会让 1.0 写入路径复杂 20%

### 9.7.7 实施时间

- 2 张表 + 迁移：0.2 d
- 5 个仓储方法：0.3 d
- 5 个路由 + service：0.3 d
- 前端 History 页面：0.2 d
- **合计 ≈ 1 d**（review 计划口径）

## 七、为什么这套设计能撑住扩展

| 未来需求 | 扩展点 | 影响面 |
|---------|--------|--------|
| 加 chat 厂商 | 新增 provider 文件 + 注册 | 1 文件 |
| 加 Worker 区域 | docker-compose 加 service + configs/worker-*.yaml | 1 文件 + 1 service |
| 加新模态（音乐/视频） | core 加 interface + providers 加实现 | 后端 1 capability 目录 |
| Master 切到 k8s | Dockerfile 不变，yaml 改 | 部署层 |
| 切到自营计费 | 已有 usage 路由直接启用 | 已有预留 |

## 九点三、限流策略

1.0 三层独立令牌桶（`golang.org/x/time/rate`），命中后等 0.3s 再判（避免突发）：

| 层 | 维度 | 默认 | 错误码 | 备注 |
|----|------|------|--------|------|
| 用户层 | IP + user_token | 100 req/min，burst 20 | `user_rate_limit` | 防止单用户/单 IP 打爆 |
| Provider 层 | 该 Provider 名 | 公开 RPM × 0.8 | `provider_rate_limit` | 不同 Provider 配额不同，按官方文档调 |
| 全局并发 | Master 进程 | 500 并发流 | `system_overload` | 实测调，保护 Master 内存/CPU |

**配置项**（`configs/master.yaml`）：

```yaml
rate_limit:
  user:
    requests_per_minute: 100
    burst: 20
  provider:
    openai: 4500      # gpt-4o 公开 5000/min，0.8 系数
    doubao: 2400      # 豆包公开 3000/min
    deepseek: 2400
    kimi: 2400
    claude: 4000      # claude 公开 5000/min
  global:
    concurrent_streams: 500
```

**实现要点**：
- 用户层 key = `user:<user_id>`，未登录用户用 IP
- Provider 层 key = `provider:<name>`，跨用户共享
- 全局并发用 `chan struct{}` 模拟信号量
- 命中限流后 retry_after 由令牌桶自动算
- Worker 调 Provider 失败时也记 Provider 配额消耗

**响应**（详见 [统一协议 §四](../api/01-protocol.md#error-format)）：

```
HTTP/1.1 429 Too Many Requests
Retry-After: 30
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
```

**为什么三层独立**：
- 1 个用户刷爆不影响其他用户
- 1 个 Provider 限流不影响其他 Provider
- 全局兜底保护 Master 进程不被 OOM

**YAGNI**：1.0 不上分布式限流（Redis 计数），单进程够用；2.0 Master 横向扩展时再切。

## 十、为什么不用 Service Mesh / Kong / 其他

1. 1.0 用户量小，自己写 routing + mTLS 比引入一套基础设施简单
2. Worker 是长连接的逻辑角色，不是物理节点，Service Mesh 解决的是 L4 问题，我们是 L7 路由
3. 后续真上规模时，Master 可以保持自研 routing，外面套一层 envoy/istio 做更复杂的流量管理

## 十一、对外接口不变（再次强调）

```
GET  /api/v1/models
POST /api/v1/chat/completions
POST /api/v1/files
GET  /api/v1/files/{id}
POST /api/v1/keys
GET  /api/v1/keys
```

URL、Header、Body JSON 字段、错误结构、Streaming 格式 **全部保持**。前端代码一行不改。
