# 后端设计

## 一、目录结构

```
backend/
├── app/
│   ├── main.py                    # FastAPI 入口
│   ├── config.py                  # 配置（环境变量、用户配置）
│   │
│   ├── core/                      # 核心抽象
│   │   ├── capability.py          # Modality / Capability 枚举
│   │   ├── provider_base.py       # Provider 基类与 Protocol
│   │   ├── registry.py            # ProviderRegistry
│   │   ├── models.py              # 统一数据模型（Pydantic）
│   │   └── errors.py              # 统一异常体系
│   │
│   ├── capabilities/              # 能力路由（按模态分）
│   │   ├── chat/
│   │   │   ├── router.py          # /api/v1/chat/completions
│   │   │   ├── service.py         # chat capability 业务逻辑
│   │   │   └── adapters/          # 协议转换（统一↔厂商）
│   │   ├── music/                 # 2.0 音乐
│   │   ├── video/                 # 2.0 视频
│   │   ├── image/                 # 2.0 图片
│   │   └── tts/                   # 2.0 语音
│   │
│   ├── providers/                 # 各厂商适配器
│   │   ├── openai/                # OpenAI 兼容（doubao/deepseek/kimi 大多兼容）
│   │   │   ├── provider.py
│   │   │   └── config.py
│   │   ├── doubao/                # 豆包特有功能（如果有）
│   │   ├── deepseek/
│   │   └── kimi/
│   │
│   ├── api/                       # 顶层路由聚合
│   │   ├── v1/
│   │   │   ├── models.py          # GET /api/v1/models
│   │   │   ├── files.py           # 文件上传
│   │   │   ├── chat.py            # chat 路由挂载
│   │   │   └── usage.py           # 用量（预留）
│   │   └── deps.py                # 依赖注入（鉴权、Provider 工厂）
│   │
│   ├── infra/                     # 基础设施
│   │   ├── keyring.py             # 用户 Key 加密存储
│   │   ├── storage.py             # 文件存储抽象（local / s3）
│   │   ├── http_client.py         # httpx 客户端工厂
│   │   └── logging.py             # 结构化日志
│   │
│   └── db/                        # 数据访问（1.0 用 SQLite + SQLModel）
│       ├── models.py
│       └── session.py
│
├── tests/
├── pyproject.toml
├── Dockerfile
└── docker-compose.yml
```

## 二、核心抽象详解

### 2.1 Modality & Capability 枚举

```python
# app/core/capability.py
from enum import Enum

class Modality(str, Enum):
    CHAT = "chat"
    MUSIC = "music"
    VIDEO = "video"
    IMAGE = "image"
    TTS = "tts"
    EMBEDDING = "embedding"

class ChatCapability(str, Enum):
    TEXT = "text"
    STREAM = "stream"
    VISION = "vision"          # 图片理解
    FILE = "file"              # 文档解析
    TOOLS = "tools"            # function calling
    REASONING = "reasoning"    # 深度思考
```

### 2.2 Provider Protocol

```python
# app/core/provider_base.py
from typing import Protocol, AsyncIterator, runtime_checkable
from app.core.models import ModelInfo, ChatRequest, ChatChunk, ChatResponse

@runtime_checkable
class ChatProvider(Protocol):
    """所有 chat 厂商必须实现的接口"""
    name: str
    modality: Modality = Modality.CHAT

    async def list_models(self) -> list[ModelInfo]: ...
    async def chat(self, req: ChatRequest) -> AsyncIterator[ChatChunk]: ...
    async def chat_complete(self, req: ChatRequest) -> ChatResponse: ...

@runtime_checkable
class MusicProvider(Protocol):
    name: str
    modality: Modality = Modality.MUSIC

    async def list_models(self) -> list[ModelInfo]: ...
    async def generate(self, req: "MusicRequest") -> AsyncIterator["MusicProgress"]: ...
```

### 2.3 Provider Registry（注册表）

```python
# app/core/registry.py
class ProviderRegistry:
    """单例，所有 Provider 启动时注册到这里"""
    _chat: dict[str, ChatProvider] = {}
    _music: dict[str, MusicProvider] = {}

    @classmethod
    def register_chat(cls, provider: ChatProvider):
        cls._chat[provider.name] = provider

    @classmethod
    def get_chat(cls, name: str) -> ChatProvider:
        if name not in cls._chat:
            raise ProviderNotFound(name)
        return cls._chat[name]

    @classmethod
    def all_models(cls) -> list[ModelInfo]:
        # 给 /api/v1/models 用，聚合所有 chat provider 的模型
        ...
```

启动时在 `app/providers/*/provider.py` 的模块顶层调用 `register_chat(...)`，无需改 main。

### 2.4 统一数据模型

```python
# app/core/models.py
from pydantic import BaseModel, Field
from typing import Literal

class ModelInfo(BaseModel):
    id: str                          # 在我们系统内的唯一 ID
    display_name: str
    provider: str                    # 对应 ProviderRegistry 的 key
    modality: Modality
    capabilities: list[str]
    context_window: int
    input_price_per_1k: float | None = None
    output_price_per_1k: float | None = None

class ChatMessage(BaseModel):
    role: Literal["system", "user", "assistant", "tool"]
    content: str
    attachments: list[str] = []      # 我们系统的 file_id 列表

class ChatRequest(BaseModel):
    model: str
    messages: list[ChatMessage]
    stream: bool = False
    temperature: float = 1.0
    max_tokens: int | None = None
    # ... 透传字段，OpenAI 兼容

class ChatChunk(BaseModel):
    id: str
    delta: str
    finish_reason: str | None = None
```

## 三、Provider 适配器示例（OpenAI 兼容族）

**关键洞察**：豆包、DeepSeek、Kimi 的 chat 接口都是 OpenAI 兼容的（base_url 不同而已）。我们写一个 `OpenAICompatibleProvider` 基类，子类只配 base_url 和 model 列表即可。

```python
# app/providers/openai/provider.py
class OpenAICompatibleProvider:
    """豆包/DeepSeek/Kimi/GPT 等的 OpenAI 兼容适配器基类"""

    def __init__(self, name: str, base_url: str, model_list: list[ModelInfo]):
        self.name = name
        self.base_url = base_url
        self._models = {m.id: m for m in model_list}
        self._client_factory = lambda key: httpx.AsyncClient(
            base_url=base_url,
            headers={"Authorization": f"Bearer {key}"},
            timeout=httpx.Timeout(60.0, connect=10.0),
        )

    async def list_models(self) -> list[ModelInfo]:
        return list(self._models.values())

    async def chat(self, req: ChatRequest, user_key: str) -> AsyncIterator[ChatChunk]:
        async with self._client_factory(user_key) as client:
            async with client.stream(
                "POST", "/chat/completions",
                json=req.model_dump(exclude_none=True),
            ) as resp:
                async for line in resp.aiter_lines():
                    if line.startswith("data: "):
                        data = line[6:]
                        if data == "[DONE]":
                            return
                        # 解析 OpenAI chunk → 我们的 ChatChunk
                        yield self._convert_chunk(json.loads(data))
```

**新增一个厂商 = 一个文件**：

```python
# app/providers/doubao/provider.py
from .base import OpenAICompatibleProvider

DOUBAO_MODELS = [
    ModelInfo(id="doubao-1-5-pro-32k", display_name="豆包 1.5 Pro", ...),
    ModelInfo(id="doubao-1-5-vision-pro", display_name="豆包视觉", ...),
]

provider = OpenAICompatibleProvider(
    name="doubao",
    base_url="https://ark.cn-beijing.volces.com/api/v3",
    model_list=DOUBAO_MODELS,
)
ProviderRegistry.register_chat(provider)
```

加一个厂商的边际成本 ≈ 30 行代码 + 一个配置。

## 四、鉴权与用户 Key 管理

### 1.0 模式（自配 Key）

```
┌──────────┐         ┌──────────────┐         ┌──────────┐
│ Frontend │────────▶│ POST /keys   │────────▶│ Keyring  │
│ Settings │  HTTPS  │ {provider,   │ 加密入库 │ (Fernet) │
└──────────┘         │  api_key}    │         └──────────┘
                     └──────────────┘
```

- 用户在前端 Settings 页面填入各大厂的 API Key
- 后端用 `cryptography.fernet` + 从环境变量派生的主密钥加密存 SQLite
- 真正发起 chat 请求时解密取出，**绝不返回给前端**
- 前端用从 master key 派生的 user_token 调用业务接口

**为什么 Key 不能直接给前端代理**：
- 浏览器侧明文 Key 会被 devtools 看到
- 没法做集中的限流/审计
- 没法做 Provider 故障转移

## 五、能力路由示例（chat）

```python
# app/capabilities/chat/router.py
from fastapi import APIRouter, Depends
from app.core.models import ChatRequest, ChatChunk
from app.core.registry import ProviderRegistry
from app.api.deps import get_user_key_for_provider

router = APIRouter()

@router.post("/chat/completions")
async def chat_completions(
    req: ChatRequest,
    user_key: str = Depends(get_user_key_for_provider),
):
    provider = ProviderRegistry.get_chat_by_model(req.model)
    if req.stream:
        return StreamingResponse(
            provider.chat(req, user_key),
            media_type="text/event-stream",
        )
    return await provider.chat_complete(req, user_key)
```

## 六、扩展点示例

### 加一个 Claude chat 厂商

1. `app/providers/claude/provider.py` — 写一个 Anthropic 协议的 Provider（不是 OpenAI 兼容族，要重写 chat 方法）
2. `CLAUDE_MODELS` 列表
3. `ProviderRegistry.register_chat(provider)`

前端无需改动（除非要展示 Claude 专属字段）。

### 加音乐模态

1. `app/core/capability.py` 加 `Modality.MUSIC`
2. `app/capabilities/music/router.py` 新路由
3. `app/providers/suno/provider.py` Suno 适配器

## 七、为什么 1.0 用 SQLite + SQLModel

- 零部署门槛，`uv run` 起来就能用
- 用户量 < 1000 时性能完全够
- 后续切 Postgres 改一个连接串

## 八、配置示例

```yaml
# config.yaml
server:
  port: 8000
  master_key_env: AI_ALL_IN_ONE_MASTER_KEY  # Fernet 主密钥来源

providers:
  doubao:
    enabled: true
  openai:
    enabled: true
    base_url_override: https://api.openai.com/v1
  deepseek:
    enabled: true
  kimi:
    enabled: true
```
