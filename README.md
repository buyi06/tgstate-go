# tgState Go

基于 Telegram 的无限私有云存储 & 永久图床系统（Go 版本）

## 功能特性

- Telegram Bot 集成
- 文件上传和管理
- 短链接分享
- 图床模式
- 用户认证

## 快速开始

### 1. 克隆并构建

```bash
git clone https://github.com/buyi06/tgstate-go.git
cd tgstate-go
go mod download
go build -o tgstate ./cmd/server
```

### 2. 配置

复制 `.env.example` 为 `.env` 并修改配置：

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```env
BOT_TOKEN=your_telegram_bot_token
CHANNEL_NAME=your_channel_id
BASE_URL=http://your-domain:8000
PORT=8000
PASSWORD=your_admin_password
```

### 3. 运行

```bash
./tgstate --port 8000
```

## 配置说明

- `BOT_TOKEN`: Telegram Bot Token（从 @BotFather 获取）
- `CHANNEL_NAME`: 频道 ID（用于存储文件）
- `BASE_URL`: 公开访问的域名
- `PORT`: 服务端口
- `PASSWORD`: 管理密码

## 使用方法

1. 启动服务后，访问 `http://localhost:8000/welcome` 设置管理员密码
2. 配置 BOT_TOKEN 和 CHANNEL_NAME
3. 将 Bot 添加到频道并设为管理员
4. 向频道发送文件即可自动保存

## License

MIT
