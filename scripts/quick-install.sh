#!/usr/bin/env bash
set -euo pipefail

NAME="tgstate"
PORT="${PORT:-8000}"
DATA_DIR="./data"

echo "=== 下载二进制 ==="
curl -L -o "$NAME" "https://agent-cdn.minimax.io/cdn_upload/20260226/483082029558124544/370598626463973/153020_f1b8/workspace/tgstate-go/tgstate"
chmod +x "$NAME"

echo "=== 初始化 ==="
mkdir -p "$DATA_DIR"

if [ ! -f ".env" ]; then
    cat > .env << 'EOF'
BOT_TOKEN=
CHANNEL_NAME=
PORT=8000
EOF
    echo "请编辑 .env 填入 BOT_TOKEN 和 CHANNEL_NAME"
fi

echo "=== 启动 ==="
nohup ./$NAME --port "$PORT" > tgstate.log 2>&1 &
sleep 2

echo "完成！访问 http://你的IP:$PORT"
