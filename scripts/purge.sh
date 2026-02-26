#!/usr/bin/env bash
set -euo pipefail

# ==================== 一键清理脚本 ====================
# 清空所有数据，不可逆！

NAME="tgstate-go"
DATA_DIR="./data"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "========================================"
echo "   ⚠️  警告：数据将被清空 ⚠️"
echo "========================================"
echo ""
log_warn "此操作将删除:"
echo "  - 所有文件索引数据"
echo "  - 管理员密码"
echo "  - 所有配置"
echo ""
read -r -p "确定继续吗? (输入 'yes' 确认): " confirm

if [ "${confirm}" != "yes" ]; then
    log_info "已取消"
    exit 0
fi

# 停止服务
log_info "停止服务..."
pkill -f "./tgstate" 2>/dev/null || true
pkill -f "tgstate-go" 2>/dev/null || true
sleep 1

# 删除数据
log_info "删除数据目录..."
rm -rf "${DATA_DIR}"
rm -f tgstate.log

# 恢复默认配置
if [ -f "${NAME}/.env" ]; then
    rm "${NAME}/.env"
fi

log_info "✅ 清理完成!"
log_info "重新运行: bash scripts/install.sh"
