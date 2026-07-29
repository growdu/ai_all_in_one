# AI All-in-One

> 一句话：把豆包、ChatGPT、DeepSeek、Kimi 等大模型封装成「开箱即用」的产品，未来可平滑扩展音乐、视频、图片等生成式 AI 能力。

## 项目目标

- 面向普通用户的统一 AI 入口，用户无需关心 API Key、代理、协议差异
- 1.0 用户自配 Key（后端仅做透传代理 + 协议适配），后续可平滑升级为聚合计费
- 一套前端，同时支持 Web / 移动端 H5 / 小程序
- 后端只做"代理 + 适配 + 抽象"，不绑定任何一家厂商

## 文档导航

- [顶层架构](architecture/00-overview.md) — Modality / Capability / Provider 三层抽象
- [统一协议](api/01-protocol.md) — OpenAI Chat Completions 兼容契约
- [后端 Provider 抽象](backend/02-provider.md) — Go + Master/Worker 双角色设计
- [前端 Web 设计](frontend/03-web.md) — Vue3 + 移动端 H5 适配
- [扩展路线](roadmap/04-extensibility.md) — 音乐 / 视频 / 图片扩展步骤
- [1.0 任务拆分](roadmap/05-mvp-tasks.md) — TDD 风格任务清单

## 仓库结构

```
ai_all_in_one/
├── docs/                 # 本文档站（部署到 GitHub Pages）
├── backend/              # 后端实现（Go · Gin · Master/Worker 双角色）
├── frontend/web/         # 前端实现（Vue3 · Web + 移动 H5）
├── mkdocs.yml            # 文档站配置
└── requirements-docs.txt # 文档构建依赖
```

## 快速开始

```bash
# 文档站本地预览
pip install -r requirements-docs.txt
mkdocs serve

# 打开 http://127.0.0.1:8000
```

## 文档站

本文档已部署到 GitHub Pages：https://growdu.github.io/ai_all_in_one/

每次 push 到 main 触发 GitHub Actions 自动部署。
