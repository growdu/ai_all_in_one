# 路由策略：单 Provider · 自动选 · 多 Provider 对比

> 解决"同一个请求怎么发、给用户看什么"的问题。这是 1.0 的核心交互模式。

## 一、三种模式总览

| 模式 | 触发 | 后端行为 | 前端展示 |
|------|------|---------|---------|
| **single** | 默认 | 选 1 个 Provider 调 | 单条回答 |
| **auto** | 用户开启"自动选最佳" | 选 N 个候选并发起，从中筛 1 个 | 只显示胜出那条 |
| **compare** | 用户开启"对比模式" | 并行发 N 个 Provider | 并排/卡片展示 N 条 |

1.0 默认 `single`；`auto` 和 `compare` 通过聊天输入框顶部的开关切换，**会话级**（不用每次重设）。

```
┌─────────────────────────────────────────────────┐
│ [模型: 豆包 Pro ▼] [模式: 单个 ▼] [⚙️]         │  ← 顶栏
├─────────────────────────────────────────────────┤
│ ...                                            │
```

模式选项：
- 单个（默认，1 个 Provider）
- 自动选（多 Provider 取最佳）
- 对比（多 Provider 并列）

## 二、Provider 候选池怎么算

无论 auto 还是 compare，都需要先从用户**已配置**的 Provider 里选一个子集。

```
用户已配 Key 的 Provider：{豆包, OpenAI, DeepSeek, Kimi, Claude}
用户当前选了 "豆包 Pro"
候选池 = {豆包 Pro}                  ← 显式选择优先
```

候选池收敛规则（按顺序）：

1. **显式锁定**：用户在 UI 上选了一个 Provider → 候选池 = {该 Provider}
2. **能力匹配**：根据请求特性（vision / file / tools）过滤掉不支持的
3. **健康过滤**：剔除最近 5 分钟内有错误的 Provider（见四、信号系统）
4. **限流过滤**：剔除当前 RPM 已用尽的 Provider

**auto 与 compare 的差异**：
- `single`：候选池可能就 1 个，直接发
- `auto`：候选池 = 1 个时退化为 single；> 1 个时进入"打分筛选"流程
- `compare`：候选池 = 1 个时报错提示"未配置多 Provider"；> 1 个时并行发

## 三、auto 模式：怎么筛"最可靠"

### 3.1 1.0 评分公式（轻信号驱动）

不靠客观 benchmark（成本高、过期快）。用**用户实际使用数据**做实时打分：

```
score(provider, request) =
    w1 * success_rate_5m
  + w2 * (1 - normalized_latency_5m)
  + w3 * user_preference_score
  + w4 * capability_match_score
```

| 因子 | 含义 | 1.0 默认权重 | 数据来源 |
|------|------|------------|---------|
| success_rate_5m | 近 5 分钟成功率（成功次数/总次数） | 0.4 | Master 本地滑动窗口 |
| normalized_latency_5m | 近 5 分钟 P50 延迟，线性归一到 0-1（越快越高） | 0.2 | Master 本地滑动窗口 |
| user_preference_score | 用户对这个 Provider 的偏好（0-1，初始 0.5） | 0.3 | 用户手动调 + 用户每次手动"换一家重答"事件自动调 |
| capability_match_score | 该 Provider 对此请求特性的支持度（0/0.5/1） | 0.1 | 静态元数据 |

`auto` 模式选 score 最高的那个 Provider 发请求。**失败则按 score 降序 fallback，最多重试 1 次**（避免一个不行换一个换来换去）。

### 3.2 为什么这个公式够用

- **零冷启动**：用户刚配好 Key 时 success_rate/latency 是 0 → fallback 到 user_preference + capability，user_preference 默认 0.5，capability_match 至少 0.5，输出 ≈ 0.5，**所有 Provider 平手**，按注册顺序选第一个。能用。
- **自学习**：用户多聊几次，success_rate 立刻有信号，公式开始有分辨力
- **无偏见**：不预设"OpenAI 比豆包好"等
- **可解释**：前端能展示"为什么是它"，方便用户调权重或加 Provider

### 3.3 2.0 升级路线（不破坏 1.0 接口）

- 引入 `ai_score` 维度：让一个轻 LLM（如 GPT-4o-mini）当 judge，对历史回答打分
- 引入 `cost_score`：把 token 价格纳入权重
- 引入 `task_embedding_match`：按问题类型（编程/写作/翻译）匹配擅长该任务的 Provider
- **核心原则**：每次只增加一个 w_new 维度，老维度权重按比例缩放，永远不做破坏式升级

## 四、信号系统：Master 怎么收集数据

### 4.1 滑动窗口

Master 内存里维护每个 Provider 的环形 buffer：

```go
// internal/routing/signals.go
type Signal struct {
    Provider    string
    Timestamp   time.Time
    LatencyMs   int
    Success     bool
    ErrorCode   string    // 失败时的标准化错误码
    UserPicked  bool      // auto 模式返回后用户是否手动切了别的
}

type Window struct {
    mu       sync.Mutex
    capacity int               // 默认 200 条
    items    []Signal
}

func (w *Window) Record(s Signal)   { /* 环形写入 */ }
func (w *Window) SuccessRate() float64 { /* 成功/总 */ }
func (w *Window) P50Latency() int      { /* 排序取中位 */ }
```

- 1.0 内存存储，进程重启清零
- 2.0 落 SQLite（已预留 `usage` 表）

### 4.2 关键事件

| 事件 | 触发 | 影响 |
|------|------|------|
| `chat_started` | 开始调用 | 启动 latency 计时 |
| `chat_succeeded` | 收到完整响应 | success_rate ↑, latency 记录 |
| `chat_failed` | 错误（任意 code） | success_rate ↓, 记 error_code |
| `user_switched` | auto 模式下用户点击"重答用 X" | **user_preference[被替换的] -= 0.05**, **user_preference[用户选的] += 0.05** |
| `user_pinned` | 用户在设置里手动调整"偏好"滑块 | 覆盖 user_preference 长期值 |

## 五、compare 模式：怎么展示 {#compare-mode}

### 5.1 协议层

请求体加 `compare` 字段：

```json
{
  "model": "auto",                        // compare 模式下 model 字段为 "auto"
  "messages": [...],
  "compare": {
    "providers": ["doubao", "openai", "deepseek"],  // 显式指定
    // 或 "providers": "all_configured"             // 用全部已配
    "format": "side_by_side"               // side_by_side | stacked
  }
}
```

响应（非流式）：

```json
{
  "id": "chatcmpl-xxx",
  "compare": {
    "providers_requested": ["doubao", "openai", "deepseek"],
    "results": [
      {
        "provider": "doubao",
        "status": "succeeded",
        "latency_ms": 1240,
        "content": "...",
        "usage": {"prompt_tokens": 12, "completion_tokens": 234}
      },
      {
        "provider": "openai",
        "status": "succeeded",
        "latency_ms": 2100,
        "content": "...",
        "usage": {"prompt_tokens": 12, "completion_tokens": 198}
      },
      {
        "provider": "deepseek",
        "status": "failed",
        "error": {"code": "upstream_timeout", "message": "..."}
      }
    ]
  }
}
```

流式 compare：每个 chunk 带 `provider` 字段，前端按 provider 分别 buffer 渲染。

### 5.2 前端展示

**side_by_side 模式（桌面端默认）**：

```
┌──────────────────────────────────────────────────────────┐
│ 你的问题：什么是 Go interface？                          │
├──────────────────────┬───────────────────────────────────┤
│  豆包 Pro   1.24s    │  OpenAI GPT-4o   2.10s           │
│                      │                                   │
│  Go interface 是     │  Go 的 interface 是一组方法签名   │
│  一组方法签名...      │  的集合，类型通过实现这些方法...  │
│                      │                                   │
│  [👍] [👎] [📋] [⤴️] │  [👍] [👎] [📋] [⤴️]            │
├──────────────────────┴───────────────────────────────────┤
│  DeepSeek   ❌ 上游超时                                  │
│  [重试]                                                  │
└──────────────────────────────────────────────────────────┘
```

**stacked 模式（移动端默认）**：

```
┌─────────────────────────┐
│ 你的问题：什么是 Go...  │
├─────────────────────────┤
│ 豆包 Pro   1.24s        │
│ Go interface 是...      │
│ [👍] [👎] [📋] [⤴️]    │
├─────────────────────────┤
│ OpenAI GPT-4o  2.10s   │
│ Go 的 interface...     │
│ [👍] [👎] [📋] [⤴️]    │
├─────────────────────────┤
│ DeepSeek   ❌ 超时      │
│ [重试]                  │
└─────────────────────────┘
```

**关键交互**：
- 每个 Provider 卡片有 👍/👎：影响 user_preference 分数
- 📋 复制
- ⤴️ "用这条继续"：把这条的回答作为新对话的 assistant 历史，后续用 single 模式继续聊
- 失败的 Provider 显示 [重试]：单点重发，不影响其他
- 顶部按 latency 排序（快的在上面），用户最关心响应快

## 六、Master 内部实现

### 6.1 路由决策树

```
                    ChatRequest
                         │
                         ▼
               ┌─────────────────┐
               │ 解析 mode       │
               │  (single/auto/  │
               │   compare)      │
               └────────┬────────┘
                        │
          ┌─────────────┼─────────────┐
          ▼             ▼             ▼
       single         auto         compare
          │             │             │
          ▼             ▼             ▼
    显式 Provider   候选池+打分   候选池(>=2)
          │             │             │
          ▼             ▼             ▼
       直接发      选 1 个发     并行发 N 个
                       │
                  失败 fallback
```

### 6.2 并行 compare（关键实现）

```go
// internal/routing/compare.go
func (r *Router) Compare(ctx context.Context, req ChatRequest) CompareResponse {
    results := make([]ProviderResult, len(req.Compare.Providers))
    var wg sync.WaitGroup
    for i, p := range req.Compare.Providers {
        wg.Add(1)
        go func(i int, name string) {
            defer wg.Done()
            rctx, cancel := context.WithTimeout(ctx, 60*time.Second)
            defer cancel()
            start := time.Now()
            content, usage, err := r.callProvider(rctx, name, req)
            latency := time.Since(start)
            sig := Signal{Provider: name, LatencyMs: int(latency.Milliseconds()), Success: err == nil}
            r.signals[name].Record(sig)
            if err != nil {
                results[i] = ProviderResult{Provider: name, Status: "failed", Error: toErrorResp(err)}
            } else {
                results[i] = ProviderResult{Provider: name, Status: "succeeded", LatencyMs: int(latency.Milliseconds()), Content: content, Usage: usage}
            }
        }(i, p)
    }
    wg.Wait()
    return CompareResponse{Results: results}
}
```

每个 goroutine 独立的 `context.WithTimeout`，单点超时不影响其他。

### 6.3 流式 compare

SSE 格式扩展：

```
data: {"provider": "doubao", "delta": "Go "}
data: {"provider": "openai", "delta": "Go "}
data: {"provider": "doubao", "delta": "interface "}
data: {"provider": "doubao", "delta": "是"}
data: {"provider": "openai", "delta": "的 "}
data: {"provider": "deepseek", "error": "upstream_timeout"}
data: [DONE]
```

前端用 `Map<provider, content[]>` buffer，渐进渲染。

## 七、错误处理

| 错误 | 触发 | 行为 |
|------|------|------|
| `no_provider_configured` | 用户没配任何 Key | 前端引导去设置页 |
| `only_one_provider` | compare 模式但只配了 1 个 | 前端提示"对比模式需至少 2 个 Provider" |
| `all_providers_failed` | compare 全挂 | 展示每个错误 + 整体重试按钮 |
| `no_capable_provider` | 请求需要 vision 但没 Provider 支持 | 前端显示 + 推荐模型 |

## 八、配置项

```yaml
# configs/master.yaml
routing:
  strategy: by-provider           # 默认 single 模式的选 Provider 策略
  signals:
    window_size: 200              # 滑动窗口容量
    decay: linear                 # 衰减方式：linear / exponential
  weights:
    success_rate: 0.4
    latency: 0.2
    user_preference: 0.3
    capability: 0.1
  auto:
    max_fallback: 1               # 失败最多换几家
  compare:
    max_concurrent: 5             # 一次最多对比 5 个 Provider
    default_format: stacked       # stacked / side_by_side
```

## 九、为什么这样设计

- **单 Provider 仍是默认**：99% 的对话用户不需要对比
- **auto 是"懒人模式"**：用户懒得选模型时给个够用的默认值
- **compare 是"专业模式"**：写代码/写文案/做决策时多看几家
- **三种模式共享同一信号系统**：用户在 single 模式下踩的坑会被 auto 学到，反之亦然
- **信号全在本地**：不上报任何云端，隐私无忧
- **失败 fallback 而非多发**：避免 token 浪费
- **可解释**：每个选择都能说出"为什么是它"

## 十、扩展点（不影响协议）

- 1.0 → 2.0：auto 引入 cost_score，把 token 价格纳入权重
- 1.0 → 2.0：signal 落 SQLite，跨进程保留
- 1.0 → 2.0：引入 "auto + 2 个备选" 模式（先发 1 个，如果信号不够自信再补 1-2 个）
- 1.0 → 2.0：AI judge（用小模型给历史回答打分）
