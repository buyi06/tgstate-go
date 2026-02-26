package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"tgstate-go/internal/bot"
	"tgstate-go/internal/config"
	"tgstate-go/internal/database"
	"tgstate-go/internal/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router  *gin.Engine
	server  *http.Server
	sseHub  *SSEHub
}

type SSEHub struct {
	clients map[chan []byte]bool
	mutex   sync.RWMutex
}

func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[chan []byte]bool),
	}
}

func (h *SSEHub) Register(client chan []byte) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.clients[client] = true
}

func (h *SSEHub) Unregister(client chan []byte) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	delete(h.clients, client)
}

func (h *SSEHub) Broadcast(message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	for client := range h.clients {
		select {
		case client <- message:
		default:
		}
	}
}

var layoutTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; min-height: 100vh; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header { background: #fff; padding: 20px; margin-bottom: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .card { background: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 20px; }
        .btn { padding: 10px 20px; background: #007bff; color: #fff; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; }
        .btn:hover { background: #0056b3; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #c82333; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: 500; }
        .form-group input { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; }
        .alert { padding: 15px; border-radius: 4px; margin-bottom: 15px; }
        .alert-success { background: #d4edda; color: #155724; }
        .alert-error { background: #f8d7da; color: #721c24; }
        .nav { display: flex; gap: 20px; margin-top: 15px; }
        .nav a { color: #007bff; text-decoration: none; }
        .nav a:hover { text-decoration: underline; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #eee; }
        th { background: #f8f9fa; font-weight: 600; }
        .file-item { display: flex; justify-content: space-between; align-items: center; padding: 12px; border-bottom: 1px solid #eee; }
        .file-info { flex: 1; }
        .file-name { font-weight: 500; }
        .file-meta { font-size: 12px; color: #999; margin-top: 4px; }
        .actions { display: flex; gap: 8px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>tgState Go</h1>
            <div class="nav">
                <a href="/">📁 文件管理</a>
                <a href="/image_hosting">🖼️ 图床</a>
                <a href="/settings">⚙️ 设置</a>
                <a href="/login">🔐 登录</a>
            </div>
        </div>
        {{.Content}}
    </div>
</body>
</html>`

func renderPage(title string, content string) string {
	tmpl, _ := template.New("page").Parse(layoutTemplate)
	var buf strings.Builder
	tmpl.Execute(&buf, map[string]string{
		"Title":   title,
		"Content": content,
	})
	return buf.String()
}

func NewServer() *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 安全头中间件
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	})

	// 静态文件
	r.Static("/static", "./web/static")

	// 路由
	setupRoutes(r, NewSSEHub())

	server := &Server{
		router: r,
		sseHub:  NewSSEHub(),
		server: &http.Server{
			Addr:    ":8000",
			Handler: r,
		},
	}

	return server
}

func (s *Server) Run(addr string) error {
	s.server.Addr = addr
	return s.server.ListenAndServe()
}

func setupRoutes(r *gin.Engine, sseHub *SSEHub) {
	// 页面路由
	r.GET("/", authMiddleware(), homePage)
	r.GET("/login", loginPage)
	r.GET("/files", authMiddleware(), filesPage)
	r.GET("/settings", authMiddleware(), settingsPage)
	r.GET("/welcome", welcomePage)
	r.GET("/image_hosting", authMiddleware(), imageHostingPage)
	r.GET("/d/:short_id/*filepath", downloadPage)
	r.GET("/f/:short_id", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/d/"+c.Param("short_id"))
	})

	// API 路由
	api := r.Group("/api")
	{
		api.POST("/auth/login", loginAPI)
		api.POST("/auth/logout", logoutAPI)

		// 公开 API - 欢迎页设置需要
		api.POST("/set-password", setPasswordAPI)
		api.POST("/app-config", setConfigAPI)

		// 认证 API
		protected := api.Group("")
		protected.Use(authMiddleware())
		{
			protected.GET("/files", getFilesAPI)
			protected.DELETE("/files/:short_id", deleteFileAPI)
			protected.POST("/batch_delete", batchDeleteAPI)
			protected.POST("/upload", uploadAPI)
			protected.GET("/app-config", getConfigAPI)
			protected.POST("/reset-config", resetConfigAPI)
		}

		api.GET("/sse", sseAPI(sseHub))
	}
}

// 中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		password, err := database.GetPassword()
		if err != nil || password == "" {
			if c.FullPath() != "/welcome" && c.FullPath() != "/pwd" &&
			   !strings.HasPrefix(c.FullPath(), "/api/auth") &&
			   !strings.HasPrefix(c.FullPath(), "/static") &&
			   !strings.HasPrefix(c.FullPath(), "/d/") {
				c.Redirect(http.StatusTemporaryRedirect, "/welcome")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		if c.FullPath() == "/welcome" || c.FullPath() == "/pwd" || c.FullPath() == "/login" {
			c.Next()
			return
		}

		cookie, err := c.Cookie("tgstate_session")
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			c.Abort()
			return
		}

		hash := sha256.Sum256([]byte(password))
		sessionHash := hex.EncodeToString(hash[:])

		if cookie != sessionHash && cookie != password {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

// 页面处理函数
func homePage(c *gin.Context) {
	files, _ := database.GetAllFiles()
	cfg := config.Get()

	content := `<div class="card">
    <h2>文件管理</h2>
    <p style="margin: 10px 0;">Bot状态: ` + fmt.Sprintf("%v", cfg.BotToken != "") + `</p>
    <p>频道: ` + cfg.ChannelName + `</p>
</div>`

	if len(files) == 0 {
		content += `<div class="card"><p>暂无文件</p></div>`
	} else {
		content += `<div class="card"><table><thead><tr><th>文件名</th><th>大小</th><th>日期</th><th>操作</th></tr></thead><tbody>`
		for _, f := range files {
			content += fmt.Sprintf(`<tr>
				<td>%s</td>
				<td>%s</td>
				<td>%s</td>
				<td><a href="/d/%s">下载</a></td>
			</tr>`, f.Filename, formatSize(f.Filesize), f.UploadDate.Format("2006-01-02"), f.ShortID)
		}
		content += `</tbody></table></div>`
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderPage("文件管理 - tgState", content)))
}

func filesPage(c *gin.Context) {
	homePage(c)
}

func loginPage(c *gin.Context) {
	content := `<div class="card">
    <h2>登录</h2>
    <form id="login-form">
        <div class="form-group">
            <label>密码</label>
            <input type="password" name="password" required>
        </div>
        <button type="submit" class="btn">登录</button>
    </form>
</div>
<script>
document.getElementById('login-form').onsubmit = async (e) => {
    e.preventDefault();
    const formData = new FormData(e.target);
    const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({password: formData.get('password')})
    });
    const data = await res.json();
    if (data.code === 'success') {
        location.href = '/';
    } else {
        alert(data.message);
    }
};
</script>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderPage("登录 - tgState", content)))
}

func welcomePage(c *gin.Context) {
	content := `<div class="card">
    <h2>欢迎使用 tgState Go</h2>
    <p>首次使用，请完成以下配置</p>
    <form id="welcome-form" style="margin-top: 20px;">
        <div class="form-group">
            <label>管理员密码 *</label>
            <input type="password" name="password" required placeholder="设置管理后台密码">
        </div>
        <div class="form-group">
            <label>Bot Token *</label>
            <input type="text" name="bot_token" required placeholder="从 @BotFather 获取">
        </div>
        <div class="form-group">
            <label>频道 ID *</label>
            <input type="text" name="channel_name" required placeholder="-1001234567890">
        </div>
        <div class="form-group">
            <label>Base URL (可选)</label>
            <input type="text" name="base_url" placeholder="https://pan.example.com">
        </div>
        <button type="submit" class="btn">开始使用</button>
    </form>
</div>
<script>
document.getElementById('welcome-form').onsubmit = async (e) => {
    e.preventDefault();
    const formData = new FormData(e.target);
    const password = formData.get('password');
    const bot_token = formData.get('bot_token');
    const channel_name = formData.get('channel_name');
    const base_url = formData.get('base_url');
    
    // 先设置密码
    const res1 = await fetch('/api/set-password', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({password})
    });
    const data1 = await res1.json();
    if (data1.code !== 'success') {
        alert(data1.message || '设置密码失败');
        return;
    }
    
    // 再设置配置
    const res2 = await fetch('/api/app-config', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({bot_token, channel_name, base_url})
    });
    const data2 = await res2.json();
    if (data2.code === 'success') {
        location.href = '/';
    } else {
        alert(data2.message || '配置失败');
    }
};
</script>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderPage("欢迎 - tgState", content)))
}

func settingsPage(c *gin.Context) {
	cfg := config.Get()
	content := `<div class="card">
    <h2>系统设置</h2>
    <form id="settings-form">
        <div class="form-group">
            <label>Bot Token</label>
            <input type="text" name="bot_token" value="` + maskToken(cfg.BotToken) + `" placeholder="从 @BotFather 获取">
        </div>
        <div class="form-group">
            <label>频道 ID</label>
            <input type="text" name="channel_name" value="` + cfg.ChannelName + `" placeholder="-1001234567890">
        </div>
        <div class="form-group">
            <label>Base URL</label>
            <input type="text" name="base_url" value="` + cfg.BaseURL + `" placeholder="https://pan.example.com">
        </div>
        <button type="submit" class="btn">保存配置</button>
    </form>
    <hr style="margin: 20px 0;">
    <button onclick="resetAll()" class="btn btn-danger">重置所有数据</button>
</div>
<script>
document.getElementById('settings-form').onsubmit = async (e) => {
    e.preventDefault();
    const formData = new FormData(e.target);
    const res = await fetch('/api/app-config', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            bot_token: formData.get('bot_token'),
            channel_name: formData.get('channel_name'),
            base_url: formData.get('base_url')
        })
    });
    const data = await res.json();
    alert(data.code === 'success' ? '保存成功' : data.message);
};
async function resetAll() {
    if (!confirm('确定要重置所有数据吗？此操作不可恢复！')) return;
    await fetch('/api/reset-config', {method: 'POST'});
    location.href = '/welcome';
}
</script>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderPage("设置 - tgState", content)))
}

func imageHostingPage(c *gin.Context) {
	content := `<div class="card">
    <h2>图床模式</h2>
    <p>选择图片获取 Markdown/HTML 链接</p>
    <div id="image-list" style="margin-top: 20px;">
        <p>加载中...</p>
    </div>
</div>
<script>
async function loadImages() {
    const res = await fetch('/api/files');
    const data = await res.json();
    const list = document.getElementById('image-list');
    if (!data.data || data.data.length === 0) {
        list.innerHTML = '<p>暂无图片</p>';
        return;
    }
    const images = data.data.filter(f => /\\.(jpg|jpeg|png|gif|webp|svg)$/i.test(f.filename));
    if (images.length === 0) {
        list.innerHTML = '<p>暂无图片</p>';
        return;
    }
    list.innerHTML = images.map(f => '<div class="file-item"><span>' + f.filename + '</span><button onclick="copyLink(\'/d/' + f.short_id + '\')">复制链接</button></div>').join('');
}
function copyLink(path) {
    navigator.clipboard.writeText(location.origin + path);
    alert('已复制');
}
loadImages();
</script>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderPage("图床 - tgState", content)))
}

// 全局 Bot 实例
var tgBot *bot.Bot

func SetBot(b *bot.Bot) {
	tgBot = b
}

func downloadPage(c *gin.Context) {
	shortID := c.Param("short_id")

	file, err := database.GetFileByShortID(shortID)
	if err != nil {
		c.String(http.StatusNotFound, "文件不存在")
		return
	}

	download := c.Query("download") == "1"

	contentType := getContentType(file.Filename)
	if download || !isInlineContentType(contentType) {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", urlEncode(file.Filename)))
	} else {
		c.Header("Content-Disposition", "inline")
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(file.Filesize, 10))
	c.Header("Accept-Ranges", "bytes")

	// 从 Telegram 下载
	if tgBot != nil {
		fileData, err := tgBot.DownloadFile(file.FileID)
		if err != nil {
			log.Printf("Failed to download file: %v", err)
			c.String(http.StatusInternalServerError, "下载失败")
			return
		}
		c.Data(http.StatusOK, contentType, fileData)
	} else {
		c.String(http.StatusOK, "File ID: %s\n请配置 Telegram Bot", file.FileID)
	}
}

// API 处理函数
func loginAPI(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Error("请输入密码", "invalid_input"))
		return
	}

	password, err := database.GetPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Error("获取密码失败", "error"))
		return
	}

	if password != req.Password {
		c.JSON(http.StatusUnauthorized, models.Error("密码错误", "invalid_password"))
		return
	}

	hash := sha256.Sum256([]byte(password))
	sessionHash := hex.EncodeToString(hash[:])
	c.SetCookie("tgstate_session", sessionHash, 86400*7, "/", "", false, true)

	c.JSON(http.StatusOK, models.Success(nil))
}

func logoutAPI(c *gin.Context) {
	c.SetCookie("tgstate_session", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, models.Success(nil))
}

func getFilesAPI(c *gin.Context) {
	files, err := database.GetAllFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Error("获取文件列表失败", "error"))
		return
	}
	c.JSON(http.StatusOK, models.Success(files))
}

func deleteFileAPI(c *gin.Context) {
	shortID := c.Param("short_id")
	if err := database.DeleteFileByShortID(shortID); err != nil {
		c.JSON(http.StatusInternalServerError, models.Error("删除失败", "error"))
		return
	}
	c.JSON(http.StatusOK, models.Success(nil))
}

func batchDeleteAPI(c *gin.Context) {
	var req struct {
		FileIDs []string `json:"file_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Error("参数错误", "invalid_input"))
		return
	}

	for _, shortID := range req.FileIDs {
		database.DeleteFileByShortID(shortID)
	}

	c.JSON(http.StatusOK, models.Success(nil))
}

func uploadAPI(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.Error("请通过 Telegram 频道上传文件", "not_implemented"))
}

func getConfigAPI(c *gin.Context) {
	cfg := config.Get()
	c.JSON(http.StatusOK, models.Success(map[string]string{
		"bot_token":    maskToken(cfg.BotToken),
		"channel_name": cfg.ChannelName,
		"base_url":     cfg.BaseURL,
	}))
}

func setConfigAPI(c *gin.Context) {
	var req struct {
		BotToken    string `json:"bot_token"`
		ChannelName string `json:"channel_name"`
		BaseURL     string `json:"base_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Error("参数错误", "invalid_input"))
		return
	}

	if req.BotToken != "" {
		config.Get().BotToken = req.BotToken
		database.SetSetting("bot_token", req.BotToken)
	}
	if req.ChannelName != "" {
		config.Get().ChannelName = req.ChannelName
		database.SetSetting("channel_name", req.ChannelName)
	}
	if req.BaseURL != "" {
		config.Get().BaseURL = req.BaseURL
		database.SetSetting("base_url", req.BaseURL)
	}

	c.JSON(http.StatusOK, models.Success(nil))
}

func setPasswordAPI(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.Error("请输入密码", "invalid_input"))
		return
	}

	if err := database.SetPassword(req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, models.Error("设置密码失败", "error"))
		return
	}

	hash := sha256.Sum256([]byte(req.Password))
	sessionHash := hex.EncodeToString(hash[:])
	c.SetCookie("tgstate_session", sessionHash, 86400*7, "/", "", false, true)

	c.JSON(http.StatusOK, models.Success(nil))
}

func resetConfigAPI(c *gin.Context) {
	database.SetSetting("bot_token", "")
	database.SetSetting("channel_name", "")
	database.SetSetting("base_url", "")
	database.SetSetting("password", "")

	c.SetCookie("tgstate_session", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, models.Success(nil))
}

func sseAPI(hub *SSEHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		client := make(chan []byte)
		hub.Register(client)
		defer hub.Unregister(client)

		c.Stream(func(w io.Writer) bool {
			select {
			case msg := <-client:
				c.SSEvent("message", msg)
				return true
			case <-c.Request.Context().Done():
				return false
			}
		})
	}
}

func urlEncode(filename string) string {
	result := ""
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ' ' {
			result += string(r)
		} else {
			result += fmt.Sprintf("%%%02X", r)
		}
	}
	return result
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/1024/1024/1024)
}

func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".zip":
		return "application/zip"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func isInlineContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "audio/") ||
		contentType == "application/pdf" ||
		contentType == "text/plain" ||
		contentType == "text/html"
}

func maskToken(token string) string {
	if len(token) < 10 {
		return "***"
	}
	return "***" + token[len(token)-10:]
}
