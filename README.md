# AI All-in-One

> 把豆包、DeepSeek、Kimi 等大模型封装成「配 Key 即用」的产品，
> 未来平滑扩展音乐 / 视频 / 图片等生成式 AI。

## 1.0 现状

- 用户自配 API Key（豆包 / DeepSeek / Kimi / OpenAI 兼容）
- 1 个二进制同时充当 Master + Worker
- 一套前端 HTML（移动端优先），覆盖 Web + 移动 H5
- 后端只做透传代理 + 协议适配，不绑任何厂商
- 14 Phase 全部完成，121 tests 全过
- 已实现：chat + 文件上传 + 历史会话 + Keyring + auto/compare 路由 + SSE 流式

## 5 分钟上手

```bash
git clone https://github.com/growdu/ai_all_in_one.git
cd ai_all_in_one/backend

# 编译
go build -o aiio ./cmd/aiio

# 生成密钥
export AIIO_MASTER_KEY=$(openssl rand -base64 32)
export AIIO_JWT_SECRET=$(openssl rand -hex 32)

# 启动 master
AIIO_ROLE=master ./aiio

# 浏览器打开
open http://localhost:8080
```

按 onboarding 引导：选 Provider → 填 API Key → 开始聊天。

## 目录约定

```
ai_all_in_one/
├── docs/                   设计文档
│   ├── architecture/       顶层架构
│   ├── api/                接口契约
│   ├── backend/            后端设计
│   ├── frontend/           前端设计
│   ├── implementation/     实施跟踪
│   ├── roadmap/            扩展路线
│   ├── deploy.md           部署指南
│   └── user-guide.md       用户手册
├── backend/                Go 后端（单 binary 双角色）
│   ├── cmd/aiio/           master / worker 入口
│   ├── internal/
│   │   ├── api/            5 个 HTTP handler
│   │   ├── core/           Modality / Registry / ChatProvider
│   │   ├── providers/      mock + doubao + deepseek + kimi + openai 兼容
│   │   ├── routing/        4 因子打分 + auto/compare
│   │   ├── security/       AES-GCM Keyring
│   │   ├── storage/        FileStore + ConvRepo + MsgRepo
│   │   ├── capabilities/   chat 附件预处理
│   │   ├── config/         YAML 配置加载
│   │   ├── observability/  health / metrics / log
│   │   └── role/           Master + Worker 启动逻辑
│   └── static/             12 个前端静态文件（HTML + JS + CSS）
└── frontend/
    └── web/                Vue/Vant 脚手架（备用，1.0 用 backend/static/）
```

## 阅读顺序

1. docs/user-guide.md            — 用户视角使用说明
2. docs/deploy.md                — 自部署指南
3. docs/architecture/00-overview — 顶层架构与核心抽象
4. docs/api/01-protocol.md       — 统一接口契约
5. docs/backend/02-provider.md   — Provider 适配层
6. docs/frontend/03-web.md       — 前端设计
7. docs/implementation/00-progress — 15 Phase 实施进度
8. docs/roadmap/04-extensibility — 音乐 / 视频 / 图片扩展路线

## License

暂未指定（私有 / 内部项目）。

## 下一步（1.1 增量）

详见 [docs/roadmap/05-mvp-tasks.md](docs/roadmap/05-mvp-tasks.md) §1.1：

- **minimax Provider** — Anthropic 兼容端点，独立实现
- **Claude Provider** — 与 minimax 共享 Anthropic 基类
- **共享 key 池** — 项目维护者充值免费 key，按 IP 限流，降低用户门槛
- 历史会话搜索 / 导出对话