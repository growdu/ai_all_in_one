# 顶层架构设计

## 一、核心抽象：Modality + Provider + Capability

为了支持 chat / 音乐 / 视频 / 图片等不同模态，且不被具体厂商锁死，我们定义三个核心抽象：

```
┌──────────────────────────────────────────────────────────────┐
│                       Frontend (Web / H5)                    │
│   ChatPage  MusicPage  VideoPage  ImagePage  SettingsPage   │
└──────────────┬───────────────────────────────────────────────┘
               │  统一协议（见 docs/api/01-protocol.md）
               ▼
┌──────────────────────────────────────────────────────────────┐
│                    API Gateway (Gin)                          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────────┐ │
│  │ Auth/KeyMgr │  │ RateLimit    │  │ Usage/Billing 预留   │ │
│  └─────────────┘  └──────────────┘  └──────────────────────┘ │
└──────────────┬───────────────────────────────────────────────┘
               │  内部接口
               ▼
┌──────────────────────────────────────────────────────────────┐
│             Capability Layer  (能力抽象层)                    │
│                                                              │
│  chat     music     video     image     tts     embedding   │
│   │        │         │         │         │         │        │
│   ▼        ▼         ▼         ▼         ▼         ▼        │
│ ┌─────────────────────────────────────────────────────────┐  │
│ │            Provider Registry (适配器注册表)              │  │
│ │  doubao  openai  deepseek  kimi  suno  luma  midjourney │  │
│ └─────────────────────────────────────────────────────────┘  │
└──────────────┬───────────────────────────────────────────────┘
               │  各厂商私有协议
               ▼
        各大模型厂商 API
```

### 三个抽象的职责

| 抽象 | 职责 | 示例 |
|------|------|------|
| **Modality（模态）** | 用户可见的功能大类，决定前端页面和交互 | chat / music / video / image / tts |
| **Capability（能力）** | 模态下的具体动作，跨厂商可对标 | chat→text/stream, music→generate/extend, image→generate/edit |
| **Provider（提供商）** | 具体厂商的协议适配器，1:1 映射真实 API | openai / doubao / deepseek / kimi / suno / luma |

**关键设计原则**：前端只认 Modality + Capability，不感知 Provider。Provider 是后端可插拔的实现细节。

## 二、为什么这套抽象能撑住扩展

| 未来需求 | 扩展点 | 影响面 |
|---------|--------|--------|
| 加一个 chat 厂商（如 Claude） | 新增 Provider，无需改前端 | 后端一个文件 |
| 加一个新模态（如音乐） | 新增 Capability 接口 + 前端新页面 | 前后端各一文件 |
| 加一种 chat 能力（如联网） | 在 chat capability 下加 sub-capability | 后端小改 |
| 加新的鉴权方式（OAuth、扫码登录） | KeyMgr 抽象扩展 | 后端一处 |

## 三、关键非功能性需求

- **可观测性**：所有 Provider 调用记录 latency / token / cost（预留 billing 接口）
- **可降级**：单个 Provider / Worker 不可用时前端可看到状态并切换
- **可灰度**：通过 Provider Registry 的优先级配置做灰度；Worker 维度天然支持区域灰度
- **安全**：用户 Key 在 Master 用 AES-GCM 加密落库，永不出 Master 到前端；Worker 收到的 Key 仅在内存中转发
- **多区域**：Master 与 Worker 拆分，Worker 部署在厂商就近区域（豆包国内、OpenAI 美西等），单二进制按 `AIIO_ROLE` 切换角色
- **协议兼容**：对外接口遵循 OpenAI Chat Completions 规范，前端可零成本切到任何 OpenAI 兼容客户端
- **首次使用门槛**：≤ 30 秒配 1 个 Key 即可开始聊天（onboarding 设计见 frontend 文档）

## 四、技术选型

| 层 | 选型 | 理由 |
|---|------|------|
| 前端 Web | Vue 3 + Vite + Pinia + Vue Router | 国内生态成熟，移动端 H5 适配成本低 |
| 前端 UI | Vant 5（移动）+ Element Plus（桌面）双栈 | 一套设计语言覆盖两端 |
| 后端运行时 | **Go 1.22+** | 静态二进制、轻量、并发高、跨区域部署成本低 |
| 后端 Web | **Gin** | 生态成熟、中间件丰富、流式 SSE 支持好 |
| 后端 HTTP | **net/http + fasthttp（按场景）** | 标准库即够用；极致性能场景切 fasthttp |
| 后端架构 | **单二进制双角色** | 同一仓库同一可执行文件，按 `AIIO_ROLE` 启动为 Master 或 Worker；Master↔Worker 走 mTLS/共享 HMAC |
| 数据 | **modernc.org/sqlite（1.0 纯 Go 驱动）→ PostgreSQL（后续）** | 零 CGO 部署；后续切 PG 不改业务代码 |
| 加密 | AES-GCM（自实现，~80 行） | 不引入额外依赖，Key 加密 + Master↔Worker 通道封装共用 |
| 部署 | Docker Compose（1.0）→ k8s（2.0） | 1.0 自部署，2.0 多 Worker 区域化 |

## 五、详细设计入口

- 接口协议：docs/api/01-protocol.md
- 后端 Provider 抽象：docs/backend/02-provider.md
- 前端 Web：docs/frontend/03-web.md
- 扩展路线：docs/roadmap/04-extensibility.md
