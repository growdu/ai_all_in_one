# 部署指南 — AI All-in-One 1.0

> 适用：自部署 / 私有云 / 单机测试
> 详见 docs/architecture/00-overview.md 与 docs/backend/02-provider.md

## 一、前置要求

| 资源 | 最低 | 推荐 |
|------|------|------|
| CPU | 1 vCPU | 2 vCPU |
| 内存 | 256 MB | 512 MB |
| 磁盘 | 500 MB | 2 GB |
| OS | Linux / macOS / Windows | Linux (Ubuntu 22.04+) |
| Docker | 20.10+ | 24.0+ |
| Docker Compose | v2.0+ | v2.20+ |

## 二、5 分钟快速启动（最简路径）

### 方式 A — Docker Compose（推荐）

```bash
# 1. 克隆代码
git clone https://github.com/growdu/ai_all_in_one.git
cd ai_all_in_one/backend

# 2. 准备环境变量
cp .env.example .env
# 编辑 .env，至少设 AIIO_MASTER_KEY 和 AIIO_JWT_SECRET
# 生成 key：
#   openssl rand -base64 32      → 32 字节 master key
#   openssl rand -hex 32         → JWT secret

# 3. 一键起服务
docker compose up -d

# 4. 验证
curl http://localhost:8080/health
# → {"status":"ok","version":"0.1.0","uptime_sec":3,"db_ok":true}

# 5. 打开浏览器
open http://localhost:8080
# 走 onboarding → 选 provider → 配 Key → 开始聊天
```

### 方式 B — 直接跑二进制（开发/调试）

```bash
cd ai_all_in_one/backend

# 编译
go build -o aiio ./cmd/aiio

# 生成密钥
export AIIO_MASTER_KEY=$(openssl rand -base64 32)
export AIIO_JWT_SECRET=$(openssl rand -hex 32)

# 启动 master（数据存到 ./data/）
mkdir -p ./data
AIIO_ROLE=master AIIO_CONFIG=./configs/master.yaml \
  ./aiio

# 浏览器打开
open http://localhost:8080
```

### 方式 C — 临时试用（mock provider，免配 key）

mock provider 不需要真 API Key，1.0 已内置。

启动后 onboarding 选 **mock**，直接可以聊（返回 echo）。
适合先体验 UI / 验证部署，再去申请真 Key。

## 三、详细步骤

### 3.1 生成密钥

```bash
# Master key (32 字节，base64 编码)
AIIO_MASTER_KEY=$(openssl rand -base64 32)

# JWT secret (32 字节 hex)
AIIO_JWT_SECRET=$(openssl rand -hex 32)

# 填到 .env
echo "AIIO_MASTER_KEY=$AIIO_MASTER_KEY" >> .env
echo "AIIO_JWT_SECRET=$AIIO_JWT_SECRET" >> .env
echo "AIIO_AUTH_TOKEN=devtoken" >> .env
```

### 3.2 启动

```bash
# 一键起
docker compose up -d

# 查看日志
docker compose logs -f master

# 看到这条说明启动成功：
# {"time":"...","level":"INFO","msg":"master role started",
#  "addr":":8080","storage":"/data/master.db","providers":5}
```

### 3.3 配 API Key

浏览器打开 `http://localhost:8080/onboarding.html`，按提示走：

1. 选择你想用的 AI 服务（豆包/OpenAI/DeepSeek/Kimi）
2. 跳到 Settings 页面，填入 API Key
3. 点击保存 → 状态显示"已配"
4. 回 Home 页开始聊天

### 3.4 数据持久化

| 路径 | 内容 | 备份建议 |
|------|------|---------|
| `/data/keyring.json` | 加密的 API Key | **重要**——丢了要重配所有 Key |
| `/data/master.db` | 未来会话历史（2.0+） | 2.0 上线后定期备份 |

容器用 `aiio-data` volume 持久化数据：

```bash
# 查看
docker volume inspect aiio-data

# 备份
docker run --rm -v aiio-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/aiio-data-$(date +%Y%m%d).tar.gz /data

# 恢复
docker run --rm -v aiio-data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/aiio-data-20260729.tar.gz -C /
```

## 四、生产部署建议

### 4.1 必备

- **HTTPS**：用 nginx / caddy / traefik 反代，提供 TLS 终止
- **防火墙**：8080 仅监听 localhost 或内网
- **Master Key 备份**：丢了不可恢复，存密码管理器
- **定期备份** keyring.json

### 4.2 推荐

- **监控**：scrape `/metrics`（Prometheus 格式）到 Grafana
- **日志收集**：用 Loki / ELK 收 stdout JSON
- **告警**：当 `aiio_chat_failed_total` 持续增长时告警

### 4.3 高可用（2.0+）

1.0 阶段 Master 是单点。可用性 = 你的部署 SLA。
2.0 计划：
- Master Active-Pass（systemd 自动重启足够个人使用）
- 2.0+ Master Active-Active + session sticky

### 4.4 nginx 反代示例

```nginx
server {
  listen 443 ssl http2;
  server_name ai.example.com;

  ssl_certificate     /etc/letsencrypt/live/ai.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/ai.example.com/privkey.pem;

  client_max_body_size 20M;  # 1.0 阶段无大文件

  location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_buffering off;  # SSE 必须关
    proxy_cache off;      # SSE 必须关
    proxy_read_timeout 120s;  # SSE 长连接
  }
}
```

## 五、常见问题

### Q1: 启动报 "AIIO_MASTER_KEY must be set"

`.env` 没设。运行 `openssl rand -base64 32` 生成 32 字节 base64 字符串，填到 `.env`。

### Q2: 报 "AIIO_JWT_SECRET must be set"

同上。1.0 简化版也会校验这个，2.0 接真 JWT 后放开。

### Q3: 浏览器打不开 8080

检查容器：`docker compose ps` 看 master 状态。
看日志：`docker compose logs master`。
看端口：`netstat -tlnp | grep 8080`。

### Q4: 配了 API Key 但 chat 报 400

`AIIO_AUTH_TOKEN` 没设。设了之后浏览器要刷新（localStorage 的 token 才会更新）。

### Q5: chat 报 "no_provider_configured"

没配该 provider 的 Key。回 Settings 页面添加。

### Q6: 配了 Key 但 chat 报 "upstream xxx"

OpenAI 兼容 provider（豆包/DeepSeek/Kimi）的真上游错误。检查：
- API Key 有效性
- 余额/额度
- 网络能否访问上游（国内访问 OpenAI 可能要代理）

## 六、升级

```bash
git pull
docker compose build --pull
docker compose up -d
```

## 七、卸载

```bash
docker compose down          # 停服务
docker volume rm aiio-data   # 删数据（连 Keyring 一起）
```

## 八、参考

- [架构概览](architecture/00-overview.md)
- [后端设计](backend/02-provider.md)
- [统一协议](api/01-protocol.md)
- [用户手册](user-guide.md)
- [路由策略](architecture/01-routing-strategy.md)
- [输入处理](architecture/02-input-processing.md)
