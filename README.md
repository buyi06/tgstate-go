# tgState Go

基于 Telegram 的无限私有云存储 & 永久图床系统（Go 版本）

将您的 Telegram 频道或群组瞬间变身为功能强大的私有网盘与图床。无需服务器存储空间，借助 Telegram 的无限云端能力，实现文件管理、外链分享、图片托管等功能。

---

## 一键部署（推荐）

```bash
bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/install.sh)'
```

或指定参数：

```bash
PORT=8080 BASE_URL=http://1.2.3.4:8080 bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/install.sh)'
```

---

## 一键脚本说明

### 1. 一键安装 / 一键更新
```bash
bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/install.sh)'
```

### 2. 一键重启（保留数据）
```bash
bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/reset.sh)'
```

### 3. 一键清理（清空数据）
```bash
bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/purge.sh)'
```

---

## 手动部署

### 1. 下载预编译版本
```bash
# Linux x86_64
curl -L https://github.com/buyi06/tgstate-go/releases/latest/download/tgstate-linux-amd64 -o tgstate-go
chmod +x tgstate-go

# Linux ARM64
curl -L https://github.com/buyi06/tgstate-go/releases/latest/download/tgstate-linux-arm64 -o tgstate-go
chmod +x tgstate-go

# macOS
curl -L https://github.com/buyi06/tgstate-go/releases/latest/download/tgstate-darwin-amd64 -o tgstate-go
chmod +x tgstate-go

# macOS Apple Silicon
curl -L https://github.com/buyi06/tgstate-go/releases/latest/download/tgstate-darwin-arm64 -o tgstate-go
chmod +x tgstate-go
```

### 2. 配置
```bash
cp .env.example .env
nano .env
```

### 3. 运行
```bash
./tgstate-go --port 8000
```

---

## Docker 部署

```bash
docker build -t tgstate-go .
docker run -d -p 8000:8000 -v $(pwd)/data:/app/data tgstate-go
```

---

## 配置说明

### .env 配置项

| 变量 | 必需 | 说明 |
|------|------|------|
| BOT_TOKEN | ✅ | Telegram Bot Token（从 @BotFather 获取） |
| CHANNEL_NAME | ✅ | 频道 ID（用于存储文件） |
| BASE_URL | ❌ | 公开访问的域名 |
| PORT | ❌ | 服务端口（默认 8000） |
| PASSWORD | ❌ | 管理员密码 |

---

## 首次配置

### 1. 获取 BOT_TOKEN
在 Telegram 搜索 **[@BotFather](https://t.me/BotFather)**，发送 `/newbot` 创建机器人，获取 Token。

### 2. 获取 CHANNEL_NAME
- 创建频道，将机器人添加为管理员
- 发送消息后访问：`https://api.telegram.org/bot<TOKEN>/getUpdates`
- 找到 chat 下的 id（通常以 `-100` 开头）

### 3. 访问网页设置
部署后访问 `http://your-ip:port/welcome` 设置密码，然后配置 BOT_TOKEN 和 CHANNEL_NAME。

---

## 功能特性

- 无限存储（Telegram 频道）
- 短链接分享
- 拖拽上传
- 大文件分块上传
- 图床模式
- 密码保护

---

## License

MIT
