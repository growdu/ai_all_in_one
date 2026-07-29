# 统一接口契约

> 目标：前端只认这套协议，后端 Provider 适配器负责把请求转成各厂商私有协议。

## 一、对外 REST 接口（前端 ↔ 后端）

所有接口前缀 `/api/v1`，鉴权用 `Authorization: Bearer <user_token>`，user_token 在 1.0 由前端用用户自配的 master key 派生（详见后端文档）。

### 1.1 列模型（能力发现）

```
GET /api/v1/models
```

返回前端所需的所有可用模型 + 厂商 + 模态，前端据此渲染"模型选择器"。

```json
{
  "models": [
    {
      "id": "doubao-1-5-pro-32k",
      "display_name": "豆包 1.5 Pro",
      "provider": "doubao",
      "modality": "chat",
      "capabilities": ["text", "stream", "vision", "file"],
      "context_window": 32000,
      "input_price_per_1k": 0.0008,
      "output_price_per_1k": 0.002
    },
    {
      "id": "gpt-4o",
      "display_name": "GPT-4o",
      "provider": "openai",
      "modality": "chat",
      "capabilities": ["text", "stream", "vision", "file", "tools"],
      "context_window": 128000
    }
  ]
}
```

**为什么需要这个接口**：
- 前端不需要硬编码任何模型清单
- 后端新增 Provider 后前端自动可见
- 价格、上下文窗口等元数据从 Provider 配置读出，未来可改

### 1.1.1 能力字段定义

`capabilities` 是字符串数组，每个值取自下方枚举。后端在 `routing` 层做 `capability_match` 判分时按枚举比对，不接受未在枚举内的值。

**基础能力（`capabilities[]` 元素）**：

| 值 | 含义 | 适用模态 |
|----|------|---------|
| `text` | 基础文本生成 | chat / music (歌词) / image (caption) |
| `stream` | 支持流式 SSE 输出 | chat / image (partial preview) |
| `vision` | 图像理解（输入图） | chat |
| `file` | 文档解析（输入文件） | chat |
| `tools` | function calling | chat |
| `json_mode` | JSON 结构化输出 | chat |
| `reasoning` | 推理增强（o1 类深度思考） | chat |

**复杂能力（Model 对象下的子字段，**不**放在 `capabilities` 数组里）**：

```yaml
# function calling 详细配置
model.tools:
  parallel_calls: bool       # 是否支持并行多 tool
  max_steps: int             # 单轮最多 tool 调用次数

# 推理增强详细配置
model.reasoning:
  effort: low | medium | high   # 推理力度

# 图像生成详细配置（2.0+）
model.image:
  sizes: ["1024x1024", "1024x1792"]
  quality: ["standard", "hd"]
  edit: bool                  # 是否支持 inpaint / outpaint
```

**示例**（o1 类模型）：

```json
{
  "id": "o1-preview",
  "display_name": "OpenAI o1 Preview",
  "provider": "openai",
  "modality": "chat",
  "capabilities": ["text", "stream", "reasoning"],
  "reasoning": { "effort": "high" },
  "context_window": 128000
}
```

**示例**（支持并行 tools）：

```json
{
  "id": "gpt-4o",
  "display_name": "GPT-4o",
  "provider": "openai",
  "modality": "chat",
  "capabilities": ["text", "stream", "vision", "file", "tools"],
  "tools": { "parallel_calls": true, "max_steps": 10 },
  "context_window": 128000
}
```

### 1.2 Chat 对话（OpenAI 兼容）

```
POST /api/v1/chat/completions
```

请求体**完全遵循 OpenAI Chat Completions 规范**：

```json
{
  "model": "doubao-1-5-pro-32k",
  "messages": [
    {"role": "user", "content": "总结一下这篇文章", "attachments": ["file-uuid-1"]},
    {"role": "assistant", "content": "好的..."}
  ],
  "stream": true,
  "temperature": 0.7,
  "max_tokens": 2000
}
```

扩展点：
- `messages[].attachments[]`：本系统的文件 ID（前端上传后拿到），后端负责拉取并注入
- 后续加 `tools`、`response_format` 等 OpenAI 字段时直接透传
- `compare` 字段：对比模式扩展，详见 [路由策略](../architecture/01-routing-strategy.md#compare-mode)

流式响应：SSE 格式，与 OpenAI 一致，前端可直接用 openai SDK 风格的解析。

### 1.3 文件上传（多模态准备）

```
POST   /api/v1/files       multipart/form-data
GET    /api/v1/files/{id}
DELETE /api/v1/files/{id}     1.0 用户上传的；2.0+ AI 生成的也支持
```

1.0 chat 主要用图片/文档，返回 `file_id` 后在 chat 时通过 `attachments` 引用。

**File 对象 schema**：

```json
{
  "id": "file_xxx",
  "source": "user_upload",
  "modality": "image",
  "owner_user_id": "user_xxx",
  "filename": "photo.png",
  "size": 102400,
  "mime_type": "image/png",
  "created_at": "2026-07-29T10:00:00Z",
  "expires_at": null,
  "url": "/api/v1/files/file_xxx"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `source` | 是 | `user_upload`（用户上传）/ `ai_generated`（AI 产出） |
| `modality` | 是 | `image` / `audio` / `video` / `document` |
| `owner_user_id` | 是 | 归属用户（仅本人可读/删，admin 例外） |
| `expires_at` | 否 | 过期时间，详见生命周期 |

**生命周期与归属规则**：

| source | 默认保留 | 永不过期 | 关联 Job 任务 |
|--------|---------|---------|--------------|
| `user_upload` | 永久（用户主动删） | N/A | 不关联 |
| `ai_generated` | 30 天 | `?permanent=true` 可永久 | 关联到产出它的 `job_id` |

**删除规则**：
- 用户只能删自己的 file
- `ai_generated` 关联到 job 时，job 删除/过期会自动联动删除
- DELETE 请求返回 204，幂等

**为什么 1.0 就定义**：
- 2.0+ 加音乐/视频/图片生成时立刻用上
- 现在补这一条 ≈ 1 个 schema 描述
- 2.0 临时加就要改 3 处（API、存储、UI）

### 1.4 异步任务（Job）

> 1.0 chat 是流式就够了，但 2.0+ 加音乐/视频/图片生成都是异步任务。
> 现在就把 Job 协议定下来，2.0 加新模态时只写 Provider 实现，不动协议层。

**端点**：

```
POST   /api/v1/jobs                  提交任务
GET    /api/v1/jobs/{id}             查状态
GET    /api/v1/jobs/{id}/result      拿结果（生成出的 file_id）
GET    /api/v1/jobs/{id}/events      SSE 实时进度（可选，polling 也行）
DELETE /api/v1/jobs/{id}             取消
GET    /api/v1/jobs                  列表（按用户过滤，分页）
```

**Job 状态机**：

```
           submit
   ┌──────────────────►
   │                   │
 [pending]        [running] ───► [succeeded] → 返回 result_file_ids
   │                   │      └► [failed]    → 携带 error
   │                   │      └► [cancelled] → 用户主动
   │                   ▼
   └───────── cancel ┘
```

**Job 对象 schema**：

```json
{
  "id": "job_xxx",
  "type": "music.generate",
  "status": "running",
  "progress": 0.45,
  "input": {
    "prompt": "一首关于星空的电子乐",
    "duration_sec": 60
  },
  "result_file_ids": [],
  "error": null,
  "created_at": "2026-07-29T10:00:00Z",
  "updated_at": "2026-07-29T10:00:30Z",
  "owner_user_id": "user_xxx"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | `music.generate` / `image.generate` / `video.generate` 等 |
| `status` | 是 | 枚举：`pending` / `running` / `succeeded` / `failed` / `cancelled` |
| `progress` | 否 | 0-1 之间，succeeded 时恒为 1 |
| `result_file_ids` | 否 | succeeded 时填充，引用 `file_id`（见 1.3） |
| `error` | 否 | failed 时填充，与 chat 错误同结构（见四、错误约定） |

**提交示例**：

```http
POST /api/v1/jobs
Content-Type: application/json

{
  "type": "music.generate",
  "provider": "suno",
  "input": {
    "prompt": "lo-fi 钢琴曲，节奏舒缓",
    "duration_sec": 60
  }
}
```

响应 202 Accepted：

```json
{
  "id": "job_xxx",
  "type": "music.generate",
  "status": "pending",
  "created_at": "..."
}
```

**SSE 进度流**（可选订阅）：

```
GET /api/v1/jobs/{id}/events

data: {"status": "running", "progress": 0.1}
data: {"status": "running", "progress": 0.45}
data: {"status": "running", "progress": 0.92}
data: {"status": "succeeded", "progress": 1.0, "result_file_ids": ["file_xxx"]}
data: [DONE]
```

**为什么 1.0 就定义**：
- 音乐/视频/图片生成 2.0+ 全部异步
- chat 流式和能力级 Job 是两套模式，但前端要统一处理
- 现在写 = 一次到位；2.0 临时加 = 改 4 处（API、routing、UI、状态机）

**1.0 落地**：协议层完整定义，路由 501 占位返回。Phase X 实施时实现 Job 调度器（goroutine pool + 状态机 + SQLite 持久化）。

### 1.5 预留接口（1.0 不实现，2.0 启用）

| 接口 | 用途 | 触发场景 |
|------|------|---------|
| `POST /api/v1/usage` 查询 | 用量统计 | 切到自营模式 |
| `POST /api/v1/topup` 充值 | 余额 | 切到自营模式 |
| `POST /api/v1/admin/providers` | 动态注册 Provider | 多租户场景 |

后端路由文件里这些路径已占位，返回 501 Not Implemented，避免后期破坏性变更。

## 二、内部接口（后端 Capability ↔ Provider）

后端内部走"能力接口"，不是 HTTP，是 Go interface：

```go
type ChatProvider interface {
    Name() string
    SupportsStream() bool

    ListModels() []ModelInfo
    Chat(ctx context.Context, req ChatRequest) <-chan ChatChunk   // 流式
    ChatComplete(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type MusicProvider interface {
    Name() string

    ListModels() []ModelInfo
    Generate(ctx context.Context, req MusicRequest) <-chan MusicProgress
}
```

每个 Provider 实现对应模态的 Protocol。Capability 层只调接口，不 import 任何具体 Provider。

## 三、为什么用 OpenAI 兼容协议

1. 生态最大，前端 SDK 几乎开箱即用
2. 用户认知成本低，看到的字段就是熟悉的字段
3. 后续要加 tools / function calling 直接透传即可
4. 万一哪天想接第三方 OpenAI 兼容客户端（如 NextChat、LobeChat），接口一致

## 四、错误约定 {#error-format}

统一错误结构：

```json
{
  "error": {
    "code": "provider_rate_limit",
    "message": "豆包 API 限流，请稍后重试",
    "user_message_key": "errors.provider_rate_limit",
    "provider": "doubao",
    "retry_after": 30
  }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `code` | 是 | 机器可读错误码，前端按此做差异化提示 |
| `message` | 否 | 后端原始 message（开发调试用，前端不直接展示） |
| `user_message_key` | 否 | 前端 i18n key，1.0 默认不传，前端降级用 `message` |
| `retry_after` | 否 | 限流/排队场景下提示多少秒后可重试 |
| `provider` | 否 | 触发错误的 Provider 名 |

**错误码规范**：

| code | 含义 | 触发场景 |
|------|------|---------|
| `auth_missing` | 未登录 | 请求无 token |
| `auth_invalid` | 登录失效 | token 过期/伪造 |
| `model_not_found` | 模型不存在 | 用户选了一个不存在的 model |
| `provider_<name>_error` | 厂商层错误 | 透传厂商错误，`<name>` 是 Provider id |
| `provider_rate_limit` | 厂商限流 | Provider 公开 RPM 用尽 |
| `user_rate_limit` | 用户限流 | 单用户在单位时间内请求过多 |
| `system_overload` | 系统过载 | Master 并发达到上限 |
| `upstream_timeout` | 上游超时 | 调用厂商超时 |
| `no_provider_configured` | 未配 Key | 用户没配任何 Provider |
| `only_one_provider` | 对比模式 Provider 不足 | compare 模式但只配了 1 个 |
| `all_providers_failed` | 全部失败 | compare 模式所有 Provider 都报错 |
| `no_capable_provider` | 无能力匹配 | 请求需要 vision 但没 Provider 支持 |
| `internal_error` | 兜底 | 未知错误 |

**429 响应**（限流专属）：

```
HTTP/1.1 429 Too Many Requests
Retry-After: 30
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1690521600

{
  "error": {
    "code": "user_rate_limit",
    "retry_after": 30
  }
}
```

**为什么用 code + 可选 user_message_key**：
- 前端按 lang 渲染：i18n 表 → 用户感知
- 后端不背 i18n 包袱：只发 code
- i18n key 找不到时降级用 `message` 字段（开发友好）

**i18n key 命名建议**：`errors.<code>`（如 `errors.user_rate_limit`）。1.0 由前端定义，2.0 由后端 + 前端共同维护。
