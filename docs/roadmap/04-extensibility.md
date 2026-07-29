# 扩展路线：音乐 / 视频 / 图片 / TTS

> 核心论点：因为我们从 Day 1 就把 Modality 和 Provider 抽象清楚了，新增模态的成本是「写一个新 capability 目录 + 写 provider 适配器」，不需要改 chat 的一行代码。

## 一、能力矩阵演进

| 版本 | Chat | Image | Music | Video | TTS |
|------|:----:|:-----:|:-----:|:-----:|:-:|
| 1.0  | ✅   | ⚠️ 仅 vision 理解 | ❌ | ❌ | ❌ |
| 2.0  | ✅   | ✅ 生图 | ✅ Suno/Udio | ❌ | ✅ |
| 3.0  | ✅   | ✅     | ✅          | ✅ 可灵/Sora | ✅ |

## 二、新增音乐模态的具体步骤

以 Suno 为例，看从 0 到能用的全流程。

### Step 1: 后端加能力枚举

```python
# app/core/capability.py
class Modality(str, Enum):
    CHAT = "chat"
    MUSIC = "music"   # 新增
    ...
```

### Step 2: 新增路由

```go
// internal/api/v1/music/router.go
package music

import (
    "github.com/gin-gonic/gin"
    "aiio/internal/core"
)

func Register(r *gin.RouterGroup) {
    g := r.Group("/music")
    g.GET("/models", listModels)
    g.POST("/generate", generate)
}

func listModels(c *gin.Context) { /* ... */ }
func generate(c *gin.Context)   {
    // 流式返回进度，最后返回音频 URL
}
```

### Step 3: 实现 Provider

Suno 是异步任务模式（提交任务 → 轮询/回调 → 拿音频），跟 chat 的流式模式不同。代码量比 OpenAI 兼容 chat 多。

```go
// internal/providers/suno/provider.go
package suno

import (
    "context"
    "aiio/internal/core"
)

type Provider struct {
    name  string
    apiKey string
}

func (p *Provider) Name() string { return "suno" }

func (p *Provider) Generate(ctx context.Context, req core.MusicRequest) <-chan core.MusicProgress {
    out := make(chan core.MusicProgress, 8)
    go func() {
        defer close(out)
        taskID := p.submit(ctx, req)
        for {
            status := p.poll(ctx, taskID)
            out <- core.MusicProgress{Status: status.State, Progress: status.Progress, AudioURL: status.AudioURL}
            if status.State == "succeeded" || status.State == "failed" {
                return
            }
        }
    }()
    return out
}
```

每个 Provider 实现对应模态的接口。Capability 层只依赖接口，不 import 任何具体实现。

### Step 4: 前端新页面

```
src/pages/Music.vue
src/components/music/{LyricsInput.vue, StyleSelector.vue, Player.vue}
```

**前端变化量 ≈ 2 个新页面 + 3 个新组件**，完全不影响 Chat 页面。

### Step 5: 注册到导航

主页加底部 Tab：`聊天 | 音乐 | 设置`（移动端），桌面端加左侧导航。

## 三、关键设计约束：避免模态间耦合

1. **不要在 chat capability 里塞音乐逻辑**。哪怕技术上可以（比如让 chat 调音乐工具），也走独立的 Tool Calling 机制（OpenAI tools）而不是混在 capability 里。
2. **统一的"任务"概念**。音乐/视频是异步任务，chat 是流式，需要在前端抽象一个 `Job` 概念：
   - 提交 → 拿到 job_id
   - 订阅进度（SSE 或轮询）
   - 完成后取结果
3. **文件存储复用**。生成出来的音频/视频 URL 走同一个 `/api/v1/files/{id}` 体系，前端不关心是用户上传的还是 AI 生成的。

## 四、Provider 增加 vs Modality 增加

| 想做的事 | 改哪些文件 | 工作量 |
|---------|-----------|--------|
| 加一个 chat 厂商（如 Claude） | 1 个 provider 文件 | 0.5 人天 |
| 加一个 image 厂商（如 Midjourney） | 1 个 capability（如果没有）+ 1 个 provider | 1 人天 |
| 加一个 image 模态 | 1 个 capability 目录 + 1 个 provider + 前端新页面 | 2-3 人天 |
| 加一个 music 模态 | 1 个 capability 目录 + 1 个 provider + 前端新页面 + 异步任务调度 | 3-5 人天 |

## 五、未来特别关注的扩展点

### Function Calling / Tools

OpenAI 协议已经有 `tools` 字段。我们的 chat capability 直接透传即可，前端在高级设置里开放。

### 联网搜索

- 新增 `SearchProvider` Protocol
- chat capability 里 `tools` 注册 `web_search` 工具
- 前端无需改，模型自动决定何时调

### 多模态对话（图+文+语音）

1.0 chat 已经支持 vision（图片理解）。后续：
- 上传音频 → 后端 ASR 转文字 → 注入 messages
- TTS 回复 → 后端合成 → 流式回前端
- 1 个 chat 模态搞定，无需新 capability

### 多人协作 / 共享对话

数据库加 `conversation.share_token`，后端加个 `GET /api/v1/share/{token}` 即可，前端加个"分享"按钮。

## 六、不要做的事（YAGNI 提醒）

- ❌ 1.0 不要做"AI 智能推荐模型"，模型选择器是显式 UI
- ❌ 1.0 不要做"多轮 function calling 编排"，等真有需求再说
- ❌ 1.0 不要做"Prompt 市场"，那是社区运营的事
- ❌ 1.0 不要做 SSO / 团队管理，先单用户跑通
