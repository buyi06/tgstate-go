#!/usr/bin/env bash
set -euo pipefail

NAME="tgstate"
PORT="${PORT:-8000}"
DATA_DIR="./data"

echo "=== 下载二进制 ==="
curl -L -o "$NAME" "https://agent-cdn.minimax.io/cdn_upload/20260226/483082029558124544/370598626463973/153410_54d8/workspace/tgstate-go/tgstate"
chmod +x "$NAME"

echo "=== 初始化 ==="
mkdir -p "$DATA_DIR"

# 不需要预先配置 .env，服务启动后再通过网页设置

echo "=== 启动 ==="
nohup ./$NAME --port "$PORT" > tgstate.log 2>&1 &
sleep 2

echo ""
echo "========================================"
echo "  启动成功!"
echo "========================================"
echo ""
echo "请访问 http://你的IP:$PORT/welcome"
echo "在引导页设置: 密码、Bot Token、频道ID"
echo ""
echo "查看日志: tail -f tgstate.log"
