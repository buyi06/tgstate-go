#!/usr/bin/env bash
set -euo pipefail

# ==================== 一键安装脚本 ====================
# 适用于 tgstate-go (Go 版本)

# --- 默认配置 ---
NAME="tgstate-go"
PORT=""
BASE_URL=""
DATA_DIR="./data"
VERSION="latest"

# --- 颜色定义 ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# --- 检查依赖 ---
check_dependencies() {
    local missing=()
    
    for cmd in curl tar; do
        if ! command -v $cmd >/dev/null 2>&1; then
            missing+=($cmd)
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "缺少依赖: ${missing[*]}"
        exit 1
    fi
}

# --- 获取版本/提交 ---
get_version() {
    if [ -d ".git" ]; then
        VERSION=$(git rev-parse --short HEAD 2>/dev/null || echo "latest")
    fi
}

# --- 下载预编译二进制 ---
download_binary() {
    local arch=$(uname -m)
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    
    # 确定架构
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) arch="amd64" ;;
    esac
    
    local filename="tgstate-${os}-${arch}"
    
    log_info "下载预编译二进制: ${filename}"
    
    # 从 GitHub Releases 下载
    local download_url="https://github.com/buyi06/tgstate-go/releases/${VERSION}/download/${filename}"
    
    if curl -sfL "$download_url" -o "$NAME" 2>/dev/null; then
        chmod +x "$NAME"
        log_info "下载成功"
        return 0
    fi
    
    # 如果 Release 不存在，尝试从 Actions Artifacts 下载
    log_warn "Release 不存在，尝试从 Actions 下载..."
    local artifact_url="https://github.com/buyi06/tgstate-go/releases/download/${VERSION}/${filename}"
    
    if curl -sfL "$artifact_url" -o "$NAME" 2>/dev/null; then
        chmod +x "$NAME"
        log_info "下载成功"
        return 0
    fi
    
    return 1
}

# --- 端口交互逻辑 ---
get_port() {
    if [ -z "${PORT:-}" ]; then
        if [ -t 0 ]; then
            read -r -p "请输入端口 (回车默认 8000): " PORT < /dev/tty || true
        fi
    fi
    PORT="${PORT:-8000}"
    
    case "$PORT" in
        ''|*[!0-9]* ) PORT=8000 ;;
        * ) if [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then PORT=8000; fi ;;
    esac
}

# --- 自动获取公网 IP ---
get_public_ip() {
    local ip
    ip=$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null) || true
    echo "${ip:-127.0.0.1}"
}

# --- BASE_URL 自动推导 ---
get_base_url() {
    if [ -z "${BASE_URL:-}" ]; then
        local pub
        pub=$(get_public_ip)
        BASE_URL="http://${pub}:${PORT}"
    fi
}

# --- 创建数据目录和配置 ---
setup() {
    log_info "初始化环境..."
    
    # 创建目录
    mkdir -p "$DATA_DIR"
    
    # 创建配置文件
    if [ ! -f "${NAME}/.env" ]; then
        mkdir -p "$NAME"
        cp .env.example "${NAME}/.env" 2>/dev/null || true
        log_warn "请编辑 ${NAME}/.env 填入 BOT_TOKEN 和 CHANNEL_NAME"
    fi
}

# --- 运行 ---
run_server() {
    # 检查二进制文件
    if [ ! -f "./${NAME}" ]; then
        log_error "二进制文件不存在"
        exit 1
    fi
    
    # 停止旧进程
    pkill -f "./${NAME}" 2>/dev/null || true
    sleep 1
    
    # 后台运行
    cd "$NAME"
    nohup ../${NAME} --port "${PORT}" > ../tgstate.log 2>&1 &
    cd ..
    
    sleep 2
    
    log_info "✅ tgState 已启动!"
    log_info "访问地址: ${BASE_URL}"
    log_info "日志: tail -f tgstate.log"
}

# --- 主流程 ---
main() {
    echo "========================================"
    echo "   tgState-Go 一键安装脚本"
    echo "========================================"
    echo ""
    
    check_dependencies
    get_version
    get_port
    get_base_url
    
    # 尝试下载预编译版本
    if ! download_binary; then
        log_warn "无法下载预编译版本，尝试本地构建..."
        
        if ! command -v git >/dev/null 2>&1; then
            log_error "需要安装 git 才能本地构建"
            exit 1
        fi
        
        # 克隆代码
        if [ ! -d "$NAME" ]; then
            log_info "克隆代码中..."
            git clone https://github.com/buyi06/tgstate-go.git "$NAME"
        fi
        
        cd "$NAME"
        go mod download
        go build -ldflags="-s -w" -o tgstate ./cmd/server
        cd ..
    fi
    
    setup
    run_server
    
    echo ""
    log_info "首次使用请访问 ${BASE_URL}/welcome 设置密码"
}

# --- 命令行参数解析 ---
case "${1:-}" in
    --download-only)
        check_dependencies
        get_version
        download_binary
        ;;
    --build)
        check_dependencies
        if [ ! -d "$NAME" ]; then
            git clone https://github.com/buyi06/tgstate-go.git "$NAME"
        fi
        cd "$NAME"
        go mod download
        go build -ldflags="-s -w" -o tgstate ./cmd/server
        ;;
    --run)
        get_port
        get_base_url
        run_server
        ;;
    --help|-h)
        echo "用法: $0 [选项]"
        echo ""
        echo "选项:"
        echo "  --download-only  仅下载二进制"
        echo "  --build         本地构建"
        echo "  --run           仅运行"
        echo "  --help          显示帮助"
        ;;
    *)
        main
        ;;
esac
