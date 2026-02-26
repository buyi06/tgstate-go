#!/usr/bin/env bash
set -euo pipefail

# ==================== 一键安装脚本 ====================
# 适用于 tgstate-go (Go 版本)

# --- 默认配置 ---
NAME="tgstate-go"
PORT=""
BASE_URL=""
DATA_DIR="./data"

# --- 颜色定义 ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# --- 检查依赖 ---
check_dependencies() {
    local missing=()
    
    if ! command -v git >/dev/null 2>&1; then
        missing+=("git")
    fi
    
    if [ ${#missing[@]} -gt 0 ]; then
        log_error "缺少依赖: ${missing[*]}"
        log_info "请先安装: apt install ${missing[*]}"
        exit 1
    fi
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

# --- 克隆/更新代码 ---
update_code() {
    if [ -d "${NAME}" ]; then
        log_info "更新代码中..."
        cd "${NAME}"
        git pull origin main || git pull
        cd ..
    else
        log_info "克隆代码中..."
        git clone https://github.com/buyi06/tgstate-go.git "${NAME}"
        cd "${NAME}"
    fi
}

# --- 构建 ---
build() {
    log_info "构建中..."
    
    # 检查 Go
    if ! command -v go >/dev/null 2>&1; then
        log_info "安装 Go..."
        if command -v apt-get >/dev/null 2>&1; then
            # Debian/Ubuntu
            apt-get update -qq
            apt-get install -y -qq golang-go
        elif command -v yum >/dev/null 2>&1; then
            # CentOS/RHEL
            yum install -y golang
        fi
    fi
    
    cd "${NAME}"
    go mod download
    go build -o tgstate ./cmd/server
    cd ..
}

# --- 运行 ---
run_server() {
    log_info "配置环境中..."
    
    # 创建数据目录
    mkdir -p "${DATA_DIR}"
    
    # 检查二进制文件
    if [ ! -f "${NAME}/tgstate" ]; then
        log_error "请先构建: bash install.sh --build"
        exit 1
    fi
    
    # 创建配置文件
    if [ ! -f "${NAME}/.env" ]; then
        cp "${NAME}/.env.example" "${NAME}/.env"
        log_warn "请编辑 ${NAME}/.env 填入 BOT_TOKEN 和 CHANNEL_NAME"
    fi
    
    # 停止旧进程
    pkill -f "./tgstate" 2>/dev/null || true
    
    # 后台运行
    cd "${NAME}"
    nohup ./tgstate --port "${PORT}" > ../tgstate.log 2>&1 &
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
    get_port
    get_base_url
    
    update_code
    build
    run_server
    
    echo ""
    log_info "首次使用请访问 ${BASE_URL}/welcome 设置密码"
}

# --- 命令行参数解析 ---
case "${1:-}" in
    --build)
        check_dependencies
        get_port
        get_base_url
        update_code
        build
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
        echo "  --build    仅构建"
        echo "  --run      仅运行"
        echo "  --help     显示帮助"
        echo ""
        echo "环境变量:"
        echo "  PORT       端口"
        echo "  BASE_URL   公开访问地址"
        ;;
    *)
        main
        ;;
esac
