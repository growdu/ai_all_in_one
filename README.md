# AI All-in-One

> 一句话：把豆包、ChatGPT、DeepSeek、Kimi 等大模型封装成「开箱即用」的产品，未来可平滑扩展音乐、视频、图片等生成式 AI 能力。

## 项目目标

- 面向普通用户的统一 AI 入口，用户无需关心 API Key、代理、协议差异
- 1.0 用户自配 Key（后端仅做透传代理 + 协议适配），后续可平滑升级为聚合计费
- 一套前端，同时支持 Web / 移动端 H5 / 小程序
- 后端只做"代理 + 适配 + 抽象"，不绑定任何一家厂商

## 目录约定

```
ai_all_in_one/
├── docs/                   # 设计文档（先写代码前先看这里）
│   ├── architecture/       # 顶层架构
│   ├── api/                # 接口契约
│   ├── frontend/           # 前端设计
│   ├── backend/            # 后端设计
│   └── roadmap/            # 扩展路线
├── backend/                # 后端实现（Python · FastAPI）
└── frontend/
    ├── web/                # Web + 移动端 H5（Vue3 · 优先实现）
    └── mobile/             # 原生 App 占位（后续）
```

## 阅读顺序

1. docs/architecture/00-overview.md    — 顶层架构与核心抽象
2. docs/api/01-protocol.md             — 统一接口契约
3. docs/backend/02-provider.md         — Provider 适配层设计
4. docs/frontend/03-web.md             — Web 前端设计
5. docs/roadmap/04-extensibility.md    — 音乐/视频/图片扩展路线
