# 前端设计（Web 优先）

## 一、技术选型与理由

| 维度 | 选型 | 理由 |
|------|------|------|
| 框架 | Vue 3 (Composition API + `<script setup>`) | 国内生态最成熟，移动端 H5 适配成本低 |
| 构建 | Vite 5 | 启动快、HMR 顺 |
| 路由 | Vue Router 4 | 标准 |
| 状态 | Pinia | 替代 Vuex，类型友好 |
| UI 组件 | Vant 5（移动）+ Element Plus（桌面）双栈 | 一套代码同时覆盖两端 |
| HTTP | ofetch（Nuxt 同款，轻量） | 比 axios 体积小 60%，原生支持 SSE |
| 样式 | Tailwind CSS + 设计变量 | 与 Vant/Element Plus 主题打通 |
| TypeScript | 全量 | 与后端 Go struct tag 派生类型对齐，IDE 自动补全 |

**移动端 vs 桌面端如何一套代码覆盖**：
- 用 Vant 5 作为基础组件库（移动端体验好，且在桌面浏览器也能用）
- 关键页面（设置、历史）用 Element Plus 增强桌面体验
- 通过 `useMediaQuery` 切换主题/布局，不做两套代码

## 二、目录结构

```
frontend/web/
├── src/
│   ├── main.ts
│   ├── App.vue
│   ├── router/
│   │   └── index.ts
│   ├── stores/                # Pinia
│   │   ├── auth.ts            # 鉴权
│   │   ├── chat.ts            # chat 状态
│   │   ├── models.ts          # 模型清单
│   │   └── settings.ts        # 用户设置
│   ├── pages/
│   │   ├── Home.vue           # 主页（聊天）
│   │   ├── Settings.vue       # API Key 配置
│   │   ├── History.vue        # 对话历史
│   │   └── Music.vue          # 2.0 音乐生成
│   ├── components/
│   │   ├── chat/
│   │   │   ├── MessageList.vue
│   │   │   ├── MessageItem.vue
│   │   │   ├── InputBox.vue
│   │   │   ├── ModelSelector.vue
│   │   │   └── FileUpload.vue
│   │   └── common/
│   ├── api/
│   │   ├── client.ts          # ofetch 实例 + 拦截器
│   │   ├── chat.ts
│   │   ├── models.ts
│   │   └── files.ts
│   ├── composables/
│   │   ├── useChat.ts         # chat 核心逻辑
│   │   ├── useSSE.ts          # 流式响应封装
│   │   └── useUpload.ts
│   ├── types/
│   │   ├── api.ts             # 与后端 Go struct 对齐
│   ├── styles/
│   └── utils/
├── public/
├── index.html
├── vite.config.ts
├── tailwind.config.ts
└── package.json
```

## 三、核心页面

### 3.1 Home（Chat）— 1.0 核心

布局（移动端从上到下，桌面端左右分栏）：

```
┌─────────────────────────────┐
│  [☰] AI 助手    [⚙️ 设置]   │  ← 顶栏
├─────────────────────────────┤
│  [模型: 豆包 Pro ▼] [模式: 单个 ▼]   │  ← 模型+模式
├─────────────────────────────┤
│                             │
│  ┌─ 消息列表（流式）──────┐  │
│  │ 👤 你好               │  │
│  │ 🤖 你好！...           │  │
│  │                       │  │
│  └──────────────────────┘  │
│                             │
├─────────────────────────────┤
│  [📎 附件] [输入框........] │  ← 输入区
│                  [发送 ➤]   │
└─────────────────────────────┘
```

**关键交互**：
- 顶栏第二行是模型选择器 + 模式选择器（**single / auto / compare**）
- compare 模式下，消息列表切换为多 Provider 并排/堆叠展示，详见 [路由策略](../architecture/01-routing-strategy.md#compare-mode)
- 切换模型后输入框清空（不同模型上下文不共享）
- 长按消息可复制 / 重新生成 / 切换模型再生成
- 移动端用 Vant 的 `PullRefresh` + 触底加载历史

### 3.2 Settings — API Key 管理

```
我的 API Key
┌─────────────────────────────┐
│ 豆包      [••••••] [👁️] [✏️] │
│ OpenAI    [未配置]   [添加]  │
│ DeepSeek  [••••••] [👁️] [✏️] │
│ Kimi      [未配置]   [添加]  │
└─────────────────────────────┘
```

Key 提交到后端，**前端不存明文**（用 HttpOnly Cookie 存 user_token）。

### 3.3 Onboarding 流程（review 3.1）

> 解决"99% 用户配 1 个 Key 就用得很好，1.0 一进来就看到 4 个红色未配提示"。

**核心原则**：1 个 Key 就能开始用，**不是 0 警告也不是 N 警告**。

**四步流程**：

```
1. 首次进入 → 看到 "选择你想用的 AI" 卡片
   ┌──────────────────────────────┐
   │ 选择你想用的 AI              │
   │                              │
   │ [豆包]  [OpenAI]             │
   │ [DeepSeek] [Kimi]            │
   │                              │
   │        [跳过]                 │
   └──────────────────────────────┘
   ↓ 选了 1 个
2. 引导去配 Key 页面（不是设置页浮窗，是 onboarding 流内）
   输入 Key → 实时验证 → 成功
   ↓
3. 立刻进聊天页，模型选择器预选刚配的那个
4. 顶栏非干扰提示："再配 1 个可解锁对比模式"（P1）
```

**模型选择器分级**：

| 状态 | 渲染 | 交互 |
|------|------|------|
| 已配 Key | 正常显示 | 可选 |
| 未配 Key | 半透明 + 锁图标 | hover "点此配 Key"（点击跳到该 Provider 的 onboarding 步骤） |
| 选中但未配 | 灰底 | 不可选 |

**关键设计**：
- onboarding 状态存 localStorage：`aiio.onboarded = true` 后不再展示
- 顶栏非干扰提示（N 配 N+1 提示）也要存已阅状态
- 高级用户可点 onboarding 任何步骤的"跳过"，跳过 = 标记为已 onboarded
- "再配 1 个可解锁对比模式" 提示在已配 1 个时出现，已配 ≥ 2 个时消失

**为什么这是产品问题不是技术问题**：
- 后端无感知，纯前端 UX
- 但 UX 设计错误会让 1.0 自部署用户首日流失 50%+
- 1.0 投入 ≈ 1 人天，回报是首日留存

**实施位置**：`src/pages/Onboarding.vue` + `src/composables/useOnboarding.ts`（4 步骤状态机 + 进度保存）。

## 四、流式聊天核心实现

### 4.1 useSSE 封装

```typescript
// composables/useSSE.ts
export function useSSE() {
  const stream = async function* (
    url: string,
    body: any,
    signal?: AbortSignal,
  ) {
    const resp = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal,
    });
    const reader = resp.body!.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop()!;
      for (const line of lines) {
        if (line.startsWith('data: ') && line !== 'data: [DONE]') {
          yield JSON.parse(line.slice(6));
        }
      }
    }
  };
  return { stream };
}
```

### 4.1.1 断线重连（review 2.2）

按 [统一协议 §1.2.1](../api/01-protocol.md#sse-resume) 实现：

**状态机**：

```
  connect
     ↓
  streaming ───disconnect──→ retrying (短抖动 1s)
     ↑                            │
     │                            ├──success──→ streaming
     │                            │
     │                            └──fail────→ resuming (调续传端点)
     │                                          │
     │                                          ├──success──→ streaming
     │                                          │
     │                                          └──fail────→ failed (UI 提示)
     │
  done
```

**实现要点**：

```typescript
// composables/useSSE.ts 增强版
export function useSSEWithResume() {
  // 1. fetch 失败时记录 lastChunkIndex
  // 2. 短抖动（1s）后重试
  // 3. 重试仍失败 → GET /api/v1/chat/completions/{id}?from_chunk=N
  // 4. 续传失败 → 提示"对话已断开，建议开启新对话"
  // 5. 移动端 network change 事件触发主动重连
  // 6. 按 id 去重：相同 id 的 chunk 不重复 append
}
```

**为什么是协议层的责任**：
- 不靠前端 hardcode 重试逻辑
- Master 端能控缓存窗口（5min）避免无限续传
- 续传端点是公开 API，未来其它客户端（LobeChat 等）也能用

**UI 反馈**：
- 重连过程中显示 "正在恢复对话..."
- 重连成功隐藏
- 续传失败显示 "对话已断开" + "开启新对话" 按钮

### 4.2 useChat 状态机

```typescript
// composables/useChat.ts
export function useChat() {
  const messages = ref<ChatMessage[]>([]);
  const isStreaming = ref(false);
  const currentModel = ref<ModelInfo | null>(null);
  const abortRef = ref<AbortController | null>(null);

  async function send(content: string, attachments: string[] = []) {
    if (!currentModel.value) return;
    messages.value.push({ role: 'user', content, attachments });
    const assistantMsg = { role: 'assistant', content: '' };
    messages.value.push(assistantMsg);

    isStreaming.value = true;
    abortRef.value = new AbortController();

    try {
      const { stream } = useSSE();
      for await (const chunk of stream('/api/v1/chat/completions', {
        model: currentModel.value.id,
        messages: messages.value.slice(0, -1),
        stream: true,
      }, abortRef.value.signal)) {
        assistantMsg.content += chunk.delta;
      }
    } finally {
      isStreaming.value = false;
    }
  }

  function stop() { abortRef.value?.abort(); }
  function clear() { messages.value = []; }

  return { messages, isStreaming, currentModel, send, stop, clear };
}
```

## 五、与后端类型对齐

```typescript
// types/api.ts
export type Modality = 'chat' | 'music' | 'video' | 'image' | 'tts';

export interface ModelInfo {
  id: string;
  display_name: string;
  provider: string;
  modality: Modality;
  capabilities: string[];
  context_window: number;
  input_price_per_1k?: number;
  output_price_per_1k?: number;
}

export interface ChatMessage {
  role: 'system' | 'user' | 'assistant' | 'tool';
  content: string;
  attachments?: string[];
}

export interface ChatRequest {
  model: string;
  messages: ChatMessage[];
  stream?: boolean;
  temperature?: number;
  max_tokens?: number;
}
```

后端 Go struct 导出 JSON Schema，前端用 `json-schema-to-typescript` 自动生成，避免漂移。

## 六、错误处理

```typescript
// api/client.ts
export const $api = ofetch.create({
  baseURL: '/api/v1',
  onResponseError({ response }) {
    const err = response._data?.error;
    if (!err) return;
    // 统一 toast / 上报到 Sentry
    switch (err.code) {
      case 'auth_missing': return router.push('/settings');
      case 'provider_rate_limit': return showToast(`限流，${err.retry_after}s 后重试`);
      case 'model_not_found': return showToast('模型不可用');
      default: return showToast(err.message);
    }
  },
});
```

### 6.1 错误 i18n 映射（review 3.2）

> 后端只发 `code` + 可选 `user_message_key`，前端按 lang 渲染。

**映射表**（`src/i18n/zh.ts`）：

```ts
export const errors: Record<string, string> = {
  // 鉴权
  auth_missing: '请先登录',
  auth_invalid: '登录已过期，请重新登录',
  // 资源
  model_not_found: '该模型暂不可用',
  no_provider_configured: '请先在设置里配置至少一个 AI 服务的 Key',
  no_capable_provider: '当前问题需要的能力（如识图）没有可用的模型',
  // 限流
  user_rate_limit: '请求过于频繁，请稍后再试',
  provider_rate_limit: '服务商繁忙，请稍后重试',
  system_overload: '服务暂时繁忙，请稍后重试',
  // 网络
  upstream_timeout: '网络超时，请检查连接后重试',
  // 对比模式
  only_one_provider: '对比模式需要至少配置 2 个 AI 服务',
  all_providers_failed: '所有 AI 服务都失败了，请稍后重试',
  // 兜底
  internal_error: '服务暂时不可用，请稍后重试',
}
```

**英文版**（`src/i18n/en.ts`）：

```ts
export const errors: Record<string, string> = {
  auth_missing: 'Please sign in first',
  auth_invalid: 'Your session has expired, please sign in again',
  model_not_found: 'This model is currently unavailable',
  no_provider_configured: 'Please configure at least one AI service key in Settings',
  no_capable_provider: 'No available model supports the required capability (e.g. vision)',
  user_rate_limit: 'Too many requests, please slow down',
  provider_rate_limit: 'The AI service is busy, please retry later',
  system_overload: 'The service is busy, please retry later',
  upstream_timeout: 'Network timeout, please check your connection',
  only_one_provider: 'Compare mode requires at least 2 AI services',
  all_providers_failed: 'All AI services failed, please retry later',
  internal_error: 'Service temporarily unavailable',
}
```

**接入 useChat 错误处理**：

```typescript
import { errors } from '@/i18n/zh'

function showError(err: ApiError) {
  // 优先级：i18n 表 → 后端 user_message_key → 后端 message
  let msg = errors[err.code]
  if (!msg && err.user_message_key) msg = errors[err.user_message_key]
  if (!msg) msg = err.message || 'Unknown error'
  showToast(msg)
}
```

**关键原则**：
- 1.0 前端维护 i18n 表，2.0 后端可在错误里直接带 user_message_key 覆盖
- i18n key 找不到时降级用 `message` 字段（开发友好）
- **前端永远不直接展示 `message` 字段**（后端 message 是开发用的，可能带堆栈）

## 七、移动端 H5 适配

- viewport meta：`width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no`
- 安全区：`padding-bottom: env(safe-area-inset-bottom)`
- 输入法弹起：用 `visualViewport.height` 动态计算可用高度，避免布局抖动
- 桌面浏览器访问：媒体查询 ≥ 1024px 时切到左右分栏布局

## 八、后续原生 App 预留

- `frontend/web` 完全用 Web 技术，理论上 Capacitor 套壳即可生成 iOS/Android
- 因此 1.0 不投入原生开发，等 Web 体验稳定后再决定
- `frontend/mobile` 目录先占位
