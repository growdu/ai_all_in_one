# --- builder ---
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aiio ./cmd/aiio

# --- runtime ---
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/aiio /usr/local/bin/aiio
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/aiio"]
