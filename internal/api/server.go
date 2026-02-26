package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tgstate-go/internal/config"
	"tgstate-go/internal/database"
	"tgstate-go/internal/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router *gin.Engine
	server *http.Server
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

	// 模板
	r.SetHTMLTemplate(htmlTemplate())

	// 路由
	setupRoutes(r)

	server := &Server{
		router: r,
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

func setupRoutes(r *gin.Engine) {
	// 页面路由
	r.GET("/", authMiddleware(), homePage)
	r.GET("/login", loginPage)
	r.GET("/files", authMiddleware(), filesPage)
	r.GET("/settings", authMiddleware(), settingsPage)
	r.GET("/welcome", welcomePage)
	r.GET("/image_hosting", authMiddleware(), imageHostingPage)

	// API 路由
	api := r.Group("/api")
	{
		// 公开 API
		api.POST("/auth/login", loginAPI)
		api.POST("/auth/logout", logoutAPI)

		// 认证 API
		protected := api.Group("")
		protected.Use(authMiddleware())
		{
			protected.GET("/files", getFilesAPI)
			protected.DELETE("/files/:short_id", deleteFileAPI)
			protected.POST("/upload", uploadAPI)
			protected.GET("/app-config", getConfigAPI)
			protected.POST("/app-config", setConfigAPI)
			protected.POST("/set-password", setPasswordAPI)
		}

		// 公开下载
		r.GET("/d/:short_id/*filepath", downloadPage)
		r.GET("/f/:short_id", fileAPI)
	}

	// SSE
	r.GET("/api/sse", sseAPI)
}

// 中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要引导
		password, err := database.GetPassword()
		if err != nil || password == "" {
			if c.FullPath() != "/welcome" && 
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

		// 已设置密码，检查登录
		if c.FullPath() == "/welcome" || c.FullPath() == "/login" {
			c.Next()
			return
		}

		// 检查 Cookie
		cookie, err := c.Cookie("tgstate_session")
		if err != nil {
			c.Redirect(http.StatusTemporaryRedirect, "/login")
			c.Abort()
			return
		}

		// 验证密码
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
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "tgState - 文件管理",
	})
}

func loginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "登录 - tgState",
	})
}

func welcomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "welcome.html", gin.H{
		"title": "欢迎 - tgState",
	})
}

func filesPage(c *gin.Context) {
	c.HTML(http.StatusOK, "files.html", gin.H{
		"title": "文件列表 - tgState",
	})
}

func settingsPage(c *gin.Context) {
	cfg := config.Get()
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"title":  "设置 - tgState",
		"config": cfg,
	})
}

func imageHostingPage(c *gin.Context) {
	c.HTML(http.StatusOK, "image_hosting.html", gin.H{
		"title": "图床 - tgState",
	})
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

	// 设置 Cookie
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

func uploadAPI(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Error("请选择文件", "invalid_file"))
		return
	}
	defer file.Close()

	_ = header // 暂时不用

	// 这里需要通过 Bot 上传到 Telegram
	// 暂时返回错误
	c.JSON(http.StatusNotImplemented, models.Error("请通过 Telegram 频道上传文件", "not_implemented"))
}

func getConfigAPI(c *gin.Context) {
	cfg := config.Get()
	c.JSON(http.StatusOK, models.Success(map[string]string{
		"bot_token":    maskToken(cfg.BotToken),
		"channel_name": cfg.ChannelName,
		"base_url":    cfg.BaseURL,
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
	}
	if req.ChannelName != "" {
		config.Get().ChannelName = req.ChannelName
	}
	if req.BaseURL != "" {
		config.Get().BaseURL = req.BaseURL
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

	// 设置 Cookie
	hash := sha256.Sum256([]byte(req.Password))
	sessionHash := hex.EncodeToString(hash[:])
	c.SetCookie("tgstate_session", sessionHash, 86400*7, "/", "", false, true)

	c.JSON(http.StatusOK, models.Success(nil))
}

func downloadPage(c *gin.Context) {
	shortID := c.Param("short_id")
	filepath := c.Param("filepath")
	filepath = strings.TrimPrefix(filepath, "/")

	file, err := database.GetFileByShortID(shortID)
	if err != nil {
		c.String(http.StatusNotFound, "文件不存在")
		return
	}

	download := c.Query("download") == "1"
	
	// 设置响应头
	contentType := getContentType(file.Filename)
	if download || !isInlineContentType(contentType) {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", urlEncode(file.Filename)))
	} else {
		c.Header("Content-Disposition", "inline")
	}
	
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(file.Filesize, 10))
	c.Header("Accept-Ranges", "bytes")

	// TODO: 从 Telegram 获取文件
	c.String(http.StatusOK, "文件下载功能开发中")
}

func fileAPI(c *gin.Context) {
	shortID := c.Param("short_id")
	c.Redirect(http.StatusTemporaryRedirect, "/d/"+shortID)
}

func sseAPI(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// TODO: 实现 SSE
	c.SSEvent("message", map[string]string{"status": "ok"})
	c.Writer.Flush()
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

// HTML 模板
func htmlTemplate() *template.Template {
	html := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.title}}</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    <div class="container">
        {{.content}}
    </div>
    <script src="/static/js/app.js"></script>
</body>
</html>
	`
	return template.Must(template.New("html").Parse(html))
}
