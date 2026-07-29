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

流式响应：SSE 格式，与 OpenAI 一致，前端可直接用 openai SDK 风格的解析。

### 1.3 文件上传（多模态准备）

```
POST /api/v1/files      multipart/form-data
GET  /api/v1/files/{id}
```

1.0 chat 主要用图片/文档，返回 `file_id` 后在 chat 时通过 `attachments` 引用。

### 1.4 预留接口（1.0 不实现，2.0 启用）

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

## 四、错误约定

统一错误结构：

```json
{
  "error": {
    "code": "provider_rate_limit",
    "message": "豆包 API 限流，请稍后重试",
    "provider": "doubao",
    "retry_after": 30
  }
}
```

错误码规范：
- `auth_missing` / `auth_invalid` — 鉴权问题
- `model_not_found` — 模型不存在
- `provider_<name>_error` — 厂商层错误（透传）
- `provider_rate_limit` — 限流
- `upstream_timeout` — 超时
- `internal_error` — 兜底

前端按 `code` 做差异化提示，详见 frontend 文档。
