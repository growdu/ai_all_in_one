# 1.0 实施路线图

> 把架构落到代码的最小切片，按 TDD 风格拆分任务。每个任务 2-5 分钟。

## Phase 0：项目脚手架（半天）

- [ ] Task 0.1：后端 pyproject.toml + FastAPI 启动
- [ ] Task 0.2：后端 Dockerfile + docker-compose.yml
- [ ] Task 0.3：前端 Vite + Vue3 初始化
- [ ] Task 0.4：前端 Tailwind + Vant 接入
- [ ] Task 0.5：前端 Proxy 配置 /api → 后端

## Phase 1：核心抽象（1 天）

- [ ] Task 1.1：Modality / Capability 枚举
- [ ] Task 1.2：统一数据模型（Pydantic + TS）
- [ ] Task 1.3：ProviderRegistry 实现
- [ ] Task 1.4：OpenAICompatibleProvider 基类
- [ ] Task 1.5：Provider 协议 Protocol 定义

## Phase 2：Chat 端到端打通（2 天）

- [ ] Task 2.1：GET /api/v1/models 路由
- [ ] Task 2.2：豆包 Provider 适配（首个）
- [ ] Task 2.3：POST /api/v1/chat/completions 流式
- [ ] Task 2.4：POST /api/v1/chat/completions 非流式
- [ ] Task 2.5：DeepSeek Provider
- [ ] Task 2.6：Kimi Provider
- [ ] Task 2.7：OpenAI Provider
- [ ] Task 2.8：统一错误处理中间件

## Phase 3：用户 Key 管理（1 天）

- [ ] Task 3.1：SQLModel + SQLite 初始化
- [ ] Task 3.2：Fernet 加密工具
- [ ] Task 3.3：Keyring 服务（存 / 取 / 删）
- [ ] Task 3.4：POST /api/v1/keys 路由
- [ ] Task 3.5：GET /api/v1/keys 路由（脱敏）

## Phase 4：前端 Chat MVP（2 天）

- [ ] Task 4.1：路由 + 主页布局
- [ ] Task 4.2：模型选择器（下拉）
- [ ] Task 4.3：useSSE composable
- [ ] Task 4.4：useChat composable
- [ ] Task 4.5：MessageList + MessageItem
- [ ] Task 4.6：InputBox（含停止按钮）
- [ ] Task 4.7：Settings 页面（Key 管理）
- [ ] Task 4.8：错误 toast 集成

## Phase 5：文件上传 + 多模态（1 天）

- [ ] Task 5.1：POST /api/v1/files
- [ ] Task 5.2：GET /api/v1/files/{id}
- [ ] Task 5.3：本地文件存储
- [ ] Task 5.4：FileUpload 组件
- [ ] Task 5.5：chat 注入 attachments 流程

## Phase 6：移动端适配 + 打磨（1 天）

- [ ] Task 6.1：Vant 主题统一
- [ ] Task 6.2：响应式布局（媒体查询）
- [ ] Task 6.3：安全区适配
- [ ] Task 6.4：暗色模式
- [ ] Task 6.5：PWA manifest（可装到桌面）

## Phase 7：部署与文档（半天）

- [ ] Task 7.1：docker-compose 一键起
- [ ] Task 7.2：环境变量 .env.example
- [ ] Task 7.3：用户文档 docs/user-guide.md
- [ ] Task 7.4：贡献文档 docs/contributing.md

**总计 ≈ 9 人天**（不含架构设计文档）

## 2.0+ 暂列待办

- [ ] 图片生成 capability（image）
- [ ] 音乐生成 capability（music）
- [ ] TTS capability
- [ ] 视频生成 capability
- [ ] 自营聚合计费
- [ ] 原生 App（Capacitor 套壳）
