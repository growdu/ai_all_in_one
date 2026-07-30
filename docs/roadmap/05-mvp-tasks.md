# 1.0 实施路线图

> 把架构落到代码的最小切片。已从 Python 切换为 Go，按 TDD 风格拆分任务，每个任务 2-5 分钟。

## Phase 0：项目脚手架（半天）

- [ ] Task 0.1：后端 `go mod init` + Gin 启动 + /health
- [ ] Task 0.2：Dockerfile 多阶段（builder → distroless）
- [ ] Task 0.3：docker-compose 编排：master + worker-cn + worker-us
- [ ] Task 0.4：configs/{master,worker-cn,worker-us}.yaml 示例
- [ ] Task 0.5：Makefile（build / test / lint / run-master / run-worker）

## Phase 1：核心抽象（1 天）

- [ ] Task 1.1：Modality / Capability 常量
- [ ] Task 1.2：ModelInfo / ChatRequest / ChatResponse 结构体
- [ ] Task 1.3：ChatProvider / MusicProvider interface
- [ ] Task 1.4：Registry（线程安全 map）
- [ ] Task 1.5：OpenAI 兼容基类实现 + 单测

## Phase 2：Master 角色（2 天）

- [ ] Task 2.1：GET /api/v1/models（聚合所有 Worker 暴露的 provider）
- [ ] Task 2.2：POST /api/v1/chat/completions（single 模式非流式）
- [ ] Task 2.3：POST /api/v1/chat/completions（single 模式流式 SSE 透传）
- [ ] Task 2.4：Routing 单模式分发（router.go）
- [ ] Task 2.5：Worker 健康检查（周期性 ping）
- [ ] Task 2.6：JWT 签发与校验
- [ ] Task 2.7：统一错误中间件

## Phase 2.5：Routing 进阶（2 天，单列阶段）

- [ ] Task R.1：Signal 滑动窗口实现（signals.go）
- [ ] Task R.2：Scoring 打分公式（scoring.go）+ 4 因子权重
- [ ] Task R.3：auto 模式：候选池收敛 + 选 1 + 失败 fallback 1 次
- [ ] Task R.4：compare 模式：并行发 N 个 Provider（compare.go）
- [ ] Task R.5：compare 模式：流式 SSE 多 provider chunk
- [ ] Task R.6：响应扩展：compare 包装 + 错误码
- [ ] Task R.7：用户事件采集（user_switched / user_pinned → 调 user_preference）
- [ ] Task R.8：前端 compare UI（side_by_side + stacked）
- [ ] Task R.9：用户 👍/👎 反馈入口
- [ ] Task R.10：单测（scoring 公式 + 信号衰减 + 候选池过滤）

## Phase 3：Worker 角色（1 天）

- [ ] Task 3.1：Worker 启动与 Provider 注册
- [ ] Task 3.2：豆包 Provider（用 OpenAI 兼容基类）
- [ ] Task 3.3：DeepSeek Provider
- [ ] Task 3.4：Kimi Provider
- [ ] Task 3.5：OpenAI Provider（海外 Worker）
- [ ] Task 3.6：Claude Provider（非 OpenAI 兼容，独立实现）
- [ ] Task 3.7：mTLS / 共享 HMAC 鉴权

## Phase 4：用户 Key 管理（1 天）

- [ ] Task 4.1：modernc.org/sqlite 初始化（纯 Go，无 CGO）
- [ ] Task 4.2：AES-GCM 加密工具
- [ ] Task 4.3：Keyring 存取服务
- [ ] Task 4.4：POST/GET/DELETE /api/v1/keys 路由
- [ ] Task 4.5：Key 注入到 Worker 转发头

## Phase 5：前端 Chat MVP（2 天）

- [ ] Task 5.1：路由 + 主页布局
- [ ] Task 5.2：模型选择器（下拉）
- [ ] Task 5.3：useSSE composable
- [ ] Task 5.4：useChat composable
- [ ] Task 5.5：MessageList + MessageItem
- [ ] Task 5.6：InputBox（含停止按钮）
- [ ] Task 5.7：Settings 页面（Key 管理）
- [ ] Task 5.8：错误 toast 集成

## Phase 6：文件上传 + 多模态（1 天）

- [ ] Task 6.1：POST /api/v1/files
- [ ] Task 6.2：GET /api/v1/files/{id}
- [ ] Task 6.3：本地文件存储
- [ ] Task 6.4：FileUpload 组件
- [ ] Task 6.5：chat 注入 attachments 流程

## Phase 7：移动端适配 + 打磨（1 天）

- [ ] Task 7.1：Vant 主题统一
- [ ] Task 7.2：响应式布局（媒体查询）
- [ ] Task 7.3：安全区适配
- [ ] Task 7.4：暗色模式
- [ ] Task 7.5：PWA manifest

## Phase 8：部署与文档（半天）

- [ ] Task 8.1：docker-compose 一键起
- [ ] Task 8.2：自签 mTLS CA 脚本（scripts/gen-mtls.sh）
- [ ] Task 8.3：环境变量 .env.example
- [ ] Task 8.4：用户文档 docs/user-guide.md
- [ ] Task 8.5：部署文档 docs/deploy.md（master + 多 worker）

**总计 ≈ 13 人天**（不含架构设计文档）

## 1.1 增量（v1.0.1 之后的小迭代）

- [ ] **minimax Provider**（Anthropic 兼容端点 `https://api.minimaxi.com/anthropic`）
    - 协议：Anthropic Messages API（与 OpenAI Chat Completions 不同，需独立实现）
    - 模型：MiniMax-M3（claude 风格）
    - 文件：`backend/internal/providers/minimax/provider.go` + `provider_test.go`
    - 模板：复用 `openaicompat` 的结构，但替换请求/响应协议
    - 测试：mock HTTP server 验证消息格式转换
    - 关联：env var `ANTHROPIC_BASE_URL=https://api.minimaxi.com/anthropic` 已存在（系统层使用），用户应用层需要单独配置 key
- [ ] **Claude Provider**（同一 Anthropic 协议，可与 minimax 共享基类）
- [ ] **历史会话搜索**（按 title / content 过滤）
- [ ] **导出对话**（Markdown / JSON）
- [ ] **共享 key 池**（项目维护者充值的免费 key，按 IP 限流，降低用户门槛）
    - 详见 docs/user-guide.md §三 提到的"豆包/DeepSeek/Kimi 不都需要 key 吗"反馈
    - 风险：滥用 / 成本失控
    - 设计要点：每用户每分钟请求数 + 每用户 token 配额 + 月度总预算

## 2.0+ 暂列待办

- [ ] 图片生成 capability（image）
- [ ] 音乐生成 capability（music）
- [ ] TTS capability
- [ ] 视频生成 capability
- [ ] 自营聚合计费
- [ ] 原生 App（Capacitor 套壳）
