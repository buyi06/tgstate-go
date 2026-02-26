#!/usr/bin/env bash
set -euo pipefail

# ==================== Docker 一键部署脚本 ====================

: "${PORT:=}"
: "${BASE_URL:=}"
: "${NAME:=tgstate-go}"
: "${IMG:=tgstate-go:latest}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 检查 Docker
if ! command -v docker >/dev/null 2>&1; then
    log_error "Docker 未安装"
    exit 1
fi

# 获取端口
if [ -z "${PORT:-}" ]; then
    if [ -t 0 ]; then
        read -r -p "请输入端口 (回车默认 8000): " PORT < /dev/tty || true
    fi
fi
PORT="${PORT:-8000}"

# 获取 BASE_URL
if [ -z "${BASE_URL:-}" ]; then
    PUB=$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null) || PUB="127.0.0.1"
    BASE_URL="http://${PUB}:${PORT}"
fi

# 停止旧容器
log_info "清理旧容器..."
docker rm -f "${NAME}" 2>/dev/null || true

# 拉取/构建镜像
if docker image inspect "${IMG}" >/dev/null 2>&1; then
    log_info "使用本地镜像: ${IMG}"
else
    log_warn "镜像不存在，请先构建: docker build -t ${IMG} ."
    exit 1
fi

# 运行
log_info "启动容器..."
docker run -d --name "${NAME}" --restart unless-stopped \
    -p "${PORT}:8000" \
    -v "$(pwd)/data:/app/data" \
    -e "PORT=8000" \
    -e "BASE_URL=${BASE_URL}" \
    "${IMG}"

log_info "✅ tgState-Go 已启动!"
log_info "访问地址: ${BASE_URL}"
log_info "查看日志: docker logs -f ${NAME}"
