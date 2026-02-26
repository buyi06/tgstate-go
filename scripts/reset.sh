#!/usr/bin/env bash
set -euo pipefail

# ==================== 一键重建容器脚本 ====================
# 保留数据，仅重启服务

NAME="tgstate-go"
DATA_DIR="./data"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# 停止旧进程
log_info "停止旧进程..."
pkill -f "./tgstate" 2>/dev/null || true
pkill -f "tgstate-go" 2>/dev/null || true
sleep 1

# 重新运行
if [ ! -f "${NAME}/tgstate" ]; then
    log_warn "二进制文件不存在，请先运行: bash scripts/install.sh"
    exit 1
fi

# 获取端口
PORT=$(grep "^PORT=" "${NAME}/.env" 2>/dev/null | cut -d= -f2)
PORT="${PORT:-8000}"

# 获取 BASE_URL
BASE_URL=$(grep "^BASE_URL=" "${NAME}/.env" 2>/dev/null | cut -d= -f2)
if [ -z "${BASE_URL}" ]; then
    PUB=$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null) || PUB="127.0.0.1"
    BASE_URL="http://${PUB}:${PORT}"
fi

log_info "启动服务..."
cd "${NAME}"
nohup ./tgstate --port "${PORT}" > ../tgstate.log 2>&1 &
cd ..

sleep 2

log_info "✅ 服务已重启!"
log_info "访问地址: ${BASE_URL}"
log_info "日志: tail -f tgstate.log"
