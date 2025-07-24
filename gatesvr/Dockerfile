# --- 多阶段构建 ---
FROM golang:1.24.3-alpine AS builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN cd gatesvr && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gatesvr cmd/main.go

# --- 运行阶段 ---
FROM alpine:3.19
WORKDIR /app
# 安装wget用于健康检查
RUN apk add --no-cache wget

# 只拷贝可执行文件和必要的证书/配置
COPY --from=builder /app/gatesvr/gatesvr .
COPY --from=builder /app/gatesvr/config.yaml ./config.yaml
COPY --from=builder /app/gatesvr/config-local.yaml ./config-local.yaml
COPY --from=builder /app/gatesvr/only-4096-ca-cert ./only-4096-ca-cert

# 暴露端口
EXPOSE 6001 6002 50051 8082

# 健康检查接口
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --spider --quiet http://localhost:8083/health || exit 1

# 默认使用生产配置，可通过环境变量覆盖
ENV GATESVR_CONFIG=config.yaml

CMD ["sh", "-c", "./gatesvr -config ${GATESVR_CONFIG}"]
