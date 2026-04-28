# ============================================================
# Stage 1: Build frontend and backend
# ============================================================
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder

WORKDIR /build/frontend
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ── Backend ─────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend-builder

# 预装构建依赖
RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GIT_SHA=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w -X electricquery/internal/config.appVersion=${GIT_SHA}" \
    -o electricquery ./cmd/server

# ============================================================
# Stage 2: Runtime image（前后端合一容器）
# ============================================================
FROM alpine:3.20

LABEL org.opencontainers.image.source="https://github.com/nxygen/ElectricQuery"
LABEL org.opencontainers.image.description="ElectricQuery 宿舍水电查询系统"

# 安装运行时依赖（ca-certificates 用于 HTTPS 请求）
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户（安全最佳实践）
RUN adduser -D -g '' appuser

WORKDIR /app

# 复制前端构建产物
COPY --from=frontend-builder /build/frontend/dist ./frontend/dist

# 复制后端二进制
COPY --from=backend-builder /build/electricquery .

# 复制配置文件（使用默认文件名，运行时通过环境变量覆盖）
COPY application.conf.example application.conf

# 切换为非 root 用户
RUN chown -R appuser:appuser /app
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

# 启动命令
# 默认从 application.conf 读取配置，可通过环境变量覆盖
ENTRYPOINT ["/app/electricquery"]
CMD []
