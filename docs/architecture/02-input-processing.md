# 输入处理：附件预处理 + Prompt 增强（review C）

> 解决"用户输入很可能不是 AI 友好的"。默认**字面透传**，所有增强**可关可看**。

## 一、设计原则

1. **字面透传为默认** — 用户输入的文本一字不改，原样发给 AI
2. **所有增强可关** — Settings 里一个总开关，关闭后回到 1.0 行为
3. **所有增强可看** — 用户能在 chat 页看到实际发给 AI 的 prompt（"查看增强后"）
4. **附件特殊处理** — 文件/PDF/图片预处理用户明知道会发生，不算"改输入"
5. **失败不阻塞** — 任何增强失败，fallback 到字面透传，不影响可用性

## 二、三层处理

```
用户输入 (text + attachments)
        ↓
   ┌────┴────┐
   ▼         ▼
文本增强   附件预处理
（可选）    （必走）
   │         │
   └────┬────┘
        ▼
   增强后 messages
        ↓
   发给 AI
```

| 层 | 默认 | 失败 fallback | 可关 |
|----|------|---------------|------|
| 附件预处理 | 必走 | 跳过预处理，原文件引用发给 AI | 不可关 |
| Prompt 增强 | 开启 | 字面透传 | 可关 |
| 意图识别 | 关闭（2.0+） | 字面透传 | 关闭 |

## 三、附件预处理（必走，1.0）

### 3.1 各类型处理策略

| 附件类型 | 预处理 | 不支持时 fallback |
|---------|--------|-----------------|
| 纯文本 (.txt .md .log) | 字符/行数统计，超长截断 + 提示 | 原样 |
| 代码 (.py .go .js .ts ...) | 语法高亮 token 数估算；超长截断 | 原样 |
| PDF | 抽文本（`unidoc/unipdf` 纯 Go），保留页码 | 错误码 `pdf_extract_failed`，原文件引用 |
| 图片 (.png .jpg) | 尺寸检查 + EXIF 去除（隐私）；> 模型限制时压缩 | 原样 |
| Office (.docx .xlsx) | 2.0 加，1.0 错误码 `unsupported_office` | 原样 |
| 其他 | 错误码 `unsupported_file_type` | 仅保留文件名提示 |

**实现位置**：`internal/capabilities/chat/preprocess/`

```go
type Preprocessor interface {
    CanHandle(mime string) bool
    Process(ctx context.Context, file *File) (ProcessedFile, error)
}

type ProcessedFile struct {
    Type        string  // "text" | "image" | "binary"
    TextContent string  // 文本提取结果（图片为空）
    Truncated   bool    // 是否被截断
    OriginalSize int
    ProcessedSize int
    Warnings    []string
}
```

### 3.2 截断策略

| 文件大小 | 行为 |
|---------|------|
| < 50KB | 全量发送 |
| 50-200KB | 截断到 50KB，前后各 25KB + 中间省略 |
| > 200KB | 截断到 50KB（首 30KB + 末 20KB），UI 提示"文件过大已截断" |

截断后 system message 注入：

```
[System: 文件 X 已被截断。原 150KB → 50KB。
如需全文请分段上传或使用更具体的文件]
```

### 3.3 隐私保护

- 图片预处理**必须**去除 EXIF（GPS、相机型号等元数据）
- 文档作者/公司信息字段，**默认不发送**到 AI
- 1.0 不做"全部元数据可见性设置"，2.0 加

## 四、Prompt 增强（可选，1.0）

### 4.1 增强规则集

1.0 默认开启一个**最弱**的增强规则，行为接近字面透传：

| 规则 | 默认 | 行为 |
|------|------|------|
| 注入"识别问题"指令 | 开启 | 在 system prompt 末尾追加一句话 |
| 注入"语言一致"指令 | 开启 | 让 AI 尽量用用户问题的语言回答 |
| 注入"格式提示" | 关闭 | 让 AI 用 markdown/code block 等 |
| 注入"代码要求" | 关闭 | 强制 AI 写完整可运行代码，不要省略号 |
| 自定义 system 模板 | 关闭 | 用户写的 system prompt 增强模板（高级） |

### 4.2 注入位置

```
增强前 messages:
[
  { role: "system", content: "<用户 system prompt>" },   // 用户自己写的
  { role: "user", content: "<原始文本>", attachments: [file_xxx] },
]

增强后:
[
  { role: "system", content: "<用户 system prompt>\n\n---\n<系统增强指令>" },
  { role: "user", content: "<附件预处理结果>\n\n---\n<原始文本>" },
]
```

**用户 system prompt 永远在前面**，增强指令在 system prompt 末尾用 `---` 分隔。

### 4.3 默认增强指令（开启时）

```
---
[System Note]
1. 请先识别用户问题的核心意图，再回答
2. 默认使用用户提问的语言回复
3. 如果附件已截断，明示用户并建议分段
4. 不要编造截断或未提供的内容
5. 涉及代码时用对应语言的标准 markdown code block
```

每条都是**可单独关闭**的（Settings → 高级 → Prompt 增强）：

```
☑ 识别问题意图
☑ 语言一致
☑ 附件截断提示
☑ 不编造
☐ 代码格式（关）
☐ 完整代码（关）
```

### 4.4 关键设计：用户能看增强后

**Chat 页 → 任意消息 → "查看增强后 prompt"**：

```
┌──────────────────────────────────────────┐
│ 实际发送给 AI 的内容（折叠）             │
├──────────────────────────────────────────┤
│ System:                                  │
│ 你是一个 Go 专家。                       │
│ ---                                      │
│ [System Note]                            │
│ 1. 请先识别用户问题的核心意图            │
│ 2. 默认使用用户提问的语言回复            │
│ ...                                      │
│                                          │
│ User:                                    │
│ [附件: main.go (2KB, 完整)]              │
│ ---                                      │
│ 帮我看看这段代码有什么问题               │
│                                          │
│ 附件内容：                               │
│ package main                             │
│ ...                                      │
└──────────────────────────────────────────┘
```

**为什么必须能看**：
- 用户信任基础：能审计 AI 看到的到底什么
- 调试关键：AI 答错时，先看 prompt 是否对
- 高级用户调优：知道关哪条规则能改行为

### 4.5 关闭增强（用户开关）

Settings → 高级：

```
┌──────────────────────────────────────────┐
│ Prompt 增强                              │
│                                          │
│  ☑ 启用 prompt 增强（默认开启）           │
│  ☑ 注入"识别问题"指令                    │
│  ☑ 注入"语言一致"指令                    │
│  ...                                     │
│                                          │
│  ⚠️ 关闭后 AI 可能答非所问                │
└──────────────────────────────────────────┘
```

关闭"启用 prompt 增强" → 全部规则失效 → 字面透传。

## 五、协议层扩展

### 5.1 请求加 `input_processing` 字段

```json
{
  "model": "doubao-1-5-pro-32k",
  "messages": [...],
  "input_processing": {
    "preprocess_attachments": true,     // 1.0 强制 true，保留供未来扩展
    "enhance_prompt": true,             // 用户开关
    "rules": ["identify_intent", "lang_match"]  // 用户可关指定规则
  }
}
```

字段：
- `preprocess_attachments`：1.0 永远 true，未来可加"原始文件模式"（如 vision 直传图）
- `enhance_prompt`：用户总开关
- `rules`：用户选择的子规则（[] = 全部用默认）

### 5.2 响应加 `processing_info`

```json
{
  "id": "chatcmpl-xxx",
  "processing_info": {
    "attachments": [
      {
        "file_id": "file_xxx",
        "type": "pdf",
        "original_size": 204800,
        "processed_size": 18432,
        "truncated": false,
        "warnings": []
      }
    ],
    "prompt_enhanced": true,
    "rules_applied": ["identify_intent", "lang_match"]
  }
}
```

前端用 `processing_info` 在 chat 页展示"附件已处理 X KB → Y KB"。

### 5.3 错误码（处理失败）

| code | 含义 | 用户感知 |
|------|------|---------|
| `pdf_extract_failed` | PDF 抽文本失败 | "PDF 解析失败，AI 看不到文件内容" |
| `image_too_large` | 图片超过模型限制 | "图片过大，已压缩到 5MB" |
| `file_unsupported` | 文件类型不支持 | "该文件类型暂不支持，仅可下载" |
| `preprocess_timeout` | 预处理超时 | "文件处理超时，已跳过预处理" |

## 六、后端实现

### 6.1 目录

```
internal/capabilities/chat/
├── preprocess/
│   ├── preprocessor.go      # 接口定义
│   ├── registry.go          # Preprocessor 注册表
│   ├── text.go              # 纯文本 / 代码
│   ├── pdf.go               # unidoc/unipdf
│   ├── image.go             # 图像压缩 + EXIF
│   └── office.go            # 2.0 预留
├── enhance/
│   ├── enhancer.go          # 增强器主逻辑
│   ├── rules.go             # 5 条默认规则
│   └── template.go          # system note 模板
└── pipeline.go              # 串联：preprocess → enhance → forward
```

### 6.2 Pipeline 流程

```go
// internal/capabilities/chat/pipeline.go
type Pipeline struct {
    preprocessors map[string]Preprocessor
    enhancer      *Enhancer
    model         ModelInfo
}

func (p *Pipeline) Process(ctx context.Context, req ChatRequest, userID string) (ChatRequest, ProcessingInfo, error) {
    info := ProcessingInfo{}

    // 1. 附件预处理（必走，失败 fallback）
    for i, att := range req.Messages {
        file, _ := fileRepo.Get(ctx, att.Attachments[j], userID)
        proc, _ := p.preprocessors[file.MimeType].Process(ctx, file)
        req.Messages[i].Content = injectAttachmentText(req.Messages[i].Content, proc)
        info.Attachments = append(info.Attachments, ...)
    }

    // 2. Prompt 增强（可选，失败 fallback）
    if req.InputProcessing.EnhancePrompt {
        req.Messages, info.RulesApplied = p.enhancer.Apply(req.Messages, req.InputProcessing.Rules)
        info.PromptEnhanced = true
    }

    return req, info, nil
}
```

### 6.3 Enhance.Apply 行为

```go
func (e *Enhancer) Apply(messages []ChatMessage, rules []string) ([]ChatMessage, []string) {
    if len(rules) == 0 {
        rules = defaultRules  // ["identify_intent", "lang_match", "truncation_note", "no_hallucinate"]
    }
    applied := []string{}
    noteParts := []string{}
    for _, r := range rules {
        if t, ok := e.templates[r]; ok {
            noteParts = append(noteParts, t)
            applied = append(applied, r)
        }
    }
    if len(noteParts) == 0 {
        return messages, nil
    }
    // 注入到 system prompt 末尾
    sysMsg := findOrCreateSystem(&messages)
    sysMsg.Content += "\n---\n[System Note]\n" + strings.Join(noteParts, "\n")
    return messages, applied
}
```

## 七、前端 UX

### 7.1 Settings 集成

在 §3.4 AI 角色 下加一个折叠区：

```
AI 角色 (System Prompt)
[多行文本框]
[保存] [重置]
☑ 锁定

▼ Prompt 增强（review C）
☑ 启用 prompt 增强
  ☑ 识别问题
  ☑ 语言一致
  ☑ 截断提示
  ☑ 不编造
  ☐ 代码格式
  ☐ 完整代码
```

### 7.2 Chat 页"查看增强后"

每条 user 消息下加 "ℹ️ 增强后" 链接：

```
👤 帮我看看这段代码
   [附件: main.go (2KB, 完整)]              [ℹ️ 增强后]
   ↓ 点开
   ┌────────────────────────────────────┐
   │ System: 你是一个 Go 专家。         │
   │ ---                                 │
   │ [System Note]                       │
   │ 1. 请先识别用户问题的核心意图       │
   │ ...                                 │
   │                                     │
   │ User:                               │
   │ ---                                 │
   │ 帮我看看这段代码                    │
   │                                     │
   │ 附件内容：                          │
   │ package main                        │
   │ ...                                 │
   └────────────────────────────────────┘
```

### 7.3 附件处理提示

上传 PDF/图片后，UI 显示"处理中" → "已处理 200KB → 18KB"。

失败时显示：
```
⚠️ PDF 解析失败，AI 将看不到文件内容
   [重试] [上传纯文本]
```

## 八、安全与隐私

| 风险 | 缓解 |
|------|------|
| 增强 prompt 让 AI 偏离用户意图 | 5 条规则**全部可关** |
| 附件预处理泄露用户数据给第三方库 | 用纯 Go 库（unidoc，不调云 API） |
| 用户系统 prompt 被覆盖 | 用户 prompt 永远在 system 消息**最前**，增强在末尾用 `---` 分隔 |
| 调试困难 | 增强后 prompt **可看** |

## 九、1.0 vs 2.0

| 功能 | 1.0 | 2.0 |
|------|:---:|:---:|
| 附件预处理（文本/代码/PDF/图片） | ✅ | ✅ |
| 截断 + 提示 | ✅ | ✅ |
| EXIF 去除 | ✅ | ✅ |
| 5 条默认增强规则 | ✅ | ✅ |
| 用户子规则开关 | ✅ | ✅ |
| 增强后 prompt 可看 | ✅ | ✅ |
| 用户自定义增强模板 | ❌ | ✅ |
| 意图识别（auto-pick mode） | ❌ | ✅ |
| Office 文件 | ❌ | ✅ |
| 音频/视频转文字 | ❌ | ✅ |

## 十、为什么这套设计平衡

- **保守**：默认字面透传，不强制增强
- **可调**：5 条规则可单独开关
- **可看**：增强后 prompt 用户能审计
- **可降级**：任何增强失败，fallback 到字面透传
- **可扩展**：2.0 加意图识别、自定义模板、Office 等不影响 1.0 协议

## 十一、为什么不做完整意图识别

- 1.0 用户量小，意图识别错一次就劝退
- 路由策略的 mode 字段（single/auto/compare）已经是手动意图选择
- 等有用户反馈"我希望 AI 帮我选模式"再做
- 详见路由策略文档的 §二候选池