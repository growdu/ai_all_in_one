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
| TypeScript | 全量 | 与后端 Pydantic 类型对齐，IDE 自动补全 |

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
│   │   └── api.ts             # 与后端 Pydantic 对齐
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
│  [☰] AI 助手    [⚙️ 设置]   │  ← 顶栏，含模型选择
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
- 顶部"模型选择器"是核心入口，点开看到所有可用模型（来自 `GET /api/v1/models`）
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

后端 Pydantic 模型导出 JSON Schema，前端用 `json-schema-to-typescript` 自动生成，避免漂移。

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

## 七、移动端 H5 适配

- viewport meta：`width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no`
- 安全区：`padding-bottom: env(safe-area-inset-bottom)`
- 输入法弹起：用 `visualViewport.height` 动态计算可用高度，避免布局抖动
- 桌面浏览器访问：媒体查询 ≥ 1024px 时切到左右分栏布局

## 八、后续原生 App 预留

- `frontend/web` 完全用 Web 技术，理论上 Capacitor 套壳即可生成 iOS/Android
- 因此 1.0 不投入原生开发，等 Web 体验稳定后再决定
- `frontend/mobile` 目录先占位
