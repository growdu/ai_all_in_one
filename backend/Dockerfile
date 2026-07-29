# --- builder ---
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aiio ./cmd/aiio

# --- runtime ---
# distroless/static:nonroot 极简镜像（无 shell、无包管理器、~2MB）
# 1.0 阶段不挂载证书（TLS 由前置 nginx/caddy 处理）
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/aiio /usr/local/bin/aiio
COPY --from=builder /src/static /usr/local/share/aiio/static
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/aiio"]
