# tgState Go

基于 Telegram 的无限私有云存储 & 永久图床系统（Go 版本）

将您的 Telegram 频道或群组瞬间变身为功能强大的私有网盘与图床。无需服务器存储空间，借助 Telegram 的无限云端能力，实现文件管理、外链分享、图片托管等功能。

---

## 一键脚本（推荐）

### 1. 一键安装 / 一键更新（保留数据，推荐）
```bash
bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/install.sh)'
```

### 2. 一键重建（保留数据，专治服务崩溃）
```bash
bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/reset.sh)'
```

### 3. 一键彻底清理（清空数据，不可逆）
```bash
bash -lc 'bash <(curl -fsSL https://raw.githubusercontent.com/buyi06/tgstate-go/main/scripts/purge.sh)'
```

> 💡 运行脚本时会提示输入端口（回车默认 8000），也可通过环境变量跳过交互：`PORT=15767 BASE_URL=https://...`

---

## Docker 部署

### 1. 构建镜像
```bash
docker build -t tgstate-go:latest .
```

### 2. 运行
```bash
bash scripts/install-docker.sh
```

---

## 手动部署

### 1. 克隆
```bash
git clone https://github.com/buyi06/tgstate-go.git
cd tgstate-go
```

### 2. 构建
```bash
go mod download
go build -o tgstate ./cmd/server
```

### 3. 配置
```bash
cp .env.example .env
nano .env
```

编辑 `.env` 文件，填入：
- `BOT_TOKEN`: Telegram Bot Token
- `CHANNEL_NAME`: 频道 ID

### 4. 运行
```bash
./tgstate --port 8000
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
| DATABASE_PATH | ❌ | 数据库路径 |
| LOG_LEVEL | ❌ | 日志级别 |

---

## 首次配置教程

部署后首次访问网页，会进入"引导页"设置管理员密码。之后请进入 **"系统设置"** 完成核心配置。

### 第一步：获取 BOT_TOKEN
1. 在 Telegram 搜索 **[@BotFather](https://t.me/BotFather)** 并点击"开始"。
2. 发送指令 `/newbot` 创建新机器人。
3. 按提示输入 Name（名字）和 Username（用户名，必须以 `bot` 结尾）。
4. 成功后，BotFather 会发送一条消息，其中 `Use this token to access the HTTP API:` 下方的那串字符就是 **BOT_TOKEN**。

### 第二步：获取 Chat ID (CHANNEL_NAME)
1. **准备群组/频道**：
   - 您可以新建一个群组或频道（公开或私密均可）。
   - **关键操作**：必须将您的机器人拉入该群组/频道，并设为**管理员**（给予读取消息和发送消息的权限）。
2. **获取 ID**：
   - 在群组/频道内随便发送一条文本消息。
   - 在浏览器访问：`https://api.telegram.org/bot<您的Token>/getUpdates`
   - 查看返回的 JSON，找到 `chat` 字段下的 `id`。
   - 通常是以 `-100` 开头的数字（例如 `-1001234567890`）。

### 第三步：填写配置
回到网页的"系统设置"，填入：
- **BOT_TOKEN**: 第一步获取的 Token
- **CHANNEL_NAME**: 第二步获取的 Chat ID

保存后即可开始使用！

---

## 功能特性

- **无限存储**：依赖 Telegram 频道，容量无上限
- **短链接分享**：生成简洁的分享链接（`/d/AbC123`）
- **拖拽上传**：支持批量拖拽上传，大文件自动分块
- **图床模式**：支持 Markdown/HTML 格式一键复制
- **隐私安全**：所有数据存储在您的私有频道，Web 端支持密码保护

---

## 常见问题

### Q: 登录后跳转回登录页？
- 检查密码设置是否正确
- 清除浏览器 Cookie 重试

### Q: 文件上传失败？
- 确保 Bot 已添加到频道并设为管理员
- 检查 BOT_TOKEN 是否正确

---

## License

MIT
