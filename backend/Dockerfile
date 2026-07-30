# --- builder ---
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aiio ./cmd/aiio

# --- runtime ---
# 1.0 阶段：Alpine 镜像（gcr.io 在国内/沙箱环境常不可达）
# 2.0 阶段：可换 distroless/static 进一步缩体积
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/aiio /usr/local/bin/aiio
COPY --from=builder /src/static /usr/local/share/aiio/static
COPY --from=builder /src/configs /usr/local/share/aiio/configs
USER root
# 1.0 简化：以 root 运行以保证 /data volume 写权限
# 2.0+ 可通过 named volume + 非 root UID 收紧
WORKDIR /usr/local/share/aiio
EXPOSE 8080
# 默认 master 角色，配置路径 /usr/local/share/aiio/configs/master.yaml
# docker-compose 覆盖为 /configs/master.yaml
ENTRYPOINT ["/usr/local/bin/aiio"]