package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
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

	// 加载模板
	r.SetHTMLTemplate(loadTemplates())

	// SSE Hub
	sseHub := NewSSEHub()

	// 路由
	setupRoutes(r, sseHub)

	server := &Server{
		router: r,
		sseHub: sseHub,
		server: &http.Server{
			Addr:    ":8000",
			Handler: r,
		},
	}

	return server
}

func loadTemplates() *template.Template {
	funcs := template.FuncMap{
		"split":   strings.Split,
		"splitN":  func(s, sep string, n int) []string { return strings.SplitN(s, sep, n) },
		"replace": strings.ReplaceAll,
		"json": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}

	// 加载所有模板文件
	templateDir := "web/templates"
	templateNames := []string{
		"base.html",
		"index.html",
		"settings.html",
		"image_hosting.html",
		"welcome.html",
		"download.html",
		"pwd.html",
	}

	var tmpl *template.Template
	for i, name := range templateNames {
		var err error
		content, err := os.ReadFile(templateDir + "/" + name)
		if err != nil {
			log.Printf("Warning: could not read template %s: %v", name, err)
			continue
		}
		if i == 0 {
			tmpl, err = template.New(name).Funcs(funcs).Parse(string(content))
		} else {
			tmpl, err = tmpl.New(name).Parse(string(content))
		}
		if err != nil {
			log.Printf("Warning: could not parse template %s: %v", name, err)
		}
	}

	return tmpl
}

func init() {
	// 这个在运行时会被替换
}

func (s *Server) Run(addr string) error {
	s.server.Addr = addr
	return s.server.ListenAndServe()
}

func setupRoutes(r *gin.Engine, sseHub *SSEHub) {
	// 页面路由
	r.GET("/", authMiddleware(), homePage)
	r.GET("/login", loginPage)
	r.GET("/pwd", loginPage)
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
		// 公开 API
		api.POST("/auth/login", loginAPI)
		api.POST("/auth/logout", logoutAPI)

		// 认证 API
		protected := api.Group("")
		protected.Use(authMiddleware())
		{
			protected.GET("/files", getFilesAPI)
			protected.DELETE("/files/:short_id", deleteFileAPI)
			protected.POST("/batch_delete", batchDeleteAPI)
			protected.POST("/upload", uploadAPI)
			protected.GET("/app-config", getConfigAPI)
			protected.POST("/app-config", setConfigAPI)
			protected.POST("/set-password", setPasswordAPI)
			protected.POST("/reset-config", resetConfigAPI)
			protected.GET("/image_hosting", imageHostingAPI)
		}

		// SSE
		api.GET("/sse", sseAPI(sseHub))
	}
}

// 中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要引导
		password, err := database.GetPassword()
		if err != nil || password == "" {
			if c.FullPath() != "/welcome" && 
			   c.FullPath() != "/pwd" &&
			   !strings.HasPrefix(c.FullPath(), "/api/auth") &&
			   !strings.HasPrefix(c.FullPath(), "/static") &&
			   !strings.HasPrefix(c.FullPath(), "/d/") &&
			   !strings.HasPrefix(c.FullPath(), "/f/") {
				c.Redirect(http.StatusTemporaryRedirect, "/welcome")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// 已设置密码，检查登录
		if c.FullPath() == "/welcome" || c.FullPath() == "/pwd" || c.FullPath() == "/login" {
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
	files, _ := database.GetAllFiles()
	cfg := config.Get()
	
	c.HTML(http.StatusOK, "base.html", gin.H{
		"Request": c.Request,
		"files":   files,
		"cfg":     cfg,
	})
}

func filesPage(c *gin.Context) {
	files, _ := database.GetAllFiles()
	cfg := config.Get()
	
	c.HTML(http.StatusOK, "index.html", gin.H{
		"Request": c.Request,
		"files":   files,
		"cfg":     cfg,
	})
}

func loginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "pwd.html", gin.H{
		"Request": c.Request,
	})
}

func welcomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "welcome.html", gin.H{
		"Request": c.Request,
	})
}

func settingsPage(c *gin.Context) {
	cfg := config.Get()
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"Request": c.Request,
		"config":  cfg,
	})
}

func imageHostingPage(c *gin.Context) {
	c.HTML(http.StatusOK, "image_hosting.html", gin.H{
		"Request": c.Request,
	})
}

// 全局 Bot 实例
var tgBot *bot.Bot

// SetBot 设置全局 Bot 实例
func SetBot(b *bot.Bot) {
	tgBot = b
}

func downloadPage(c *gin.Context) {
	shortID := c.Param("short_id")
	_ = c.Param("filepath")

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

	// Range 请求支持
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" && tgBot != nil {
		// 处理 Range 请求
		parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
		start, _ := strconv.ParseInt(parts[0], 10, 64)
		end := file.Filesize - 1
		if len(parts) > 1 && parts[1] != "" {
			end, _ = strconv.ParseInt(parts[1], 10, 64)
		}
		
		// 从 Telegram 获取文件（需要支持 Range）
		// 暂时返回全部内容
		fileData, err := tgBot.DownloadFile(file.FileID)
		if err != nil {
			log.Printf("Failed to download file from Telegram: %v", err)
			c.String(http.StatusInternalServerError, "下载失败")
			return
		}
		
		// 发送部分内容
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Filesize))
		c.Header("Content-Length", strconv.FormatInt(end-start+1, 10))
		c.Status(http.StatusPartialContent)
		
		if start > 0 && int(start) < len(fileData) {
			if end >= int64(len(fileData)) {
				end = int64(len(fileData)) - 1
			}
			c.Data(http.StatusOK, contentType, fileData[start:end+1])
		} else {
			c.Data(http.StatusOK, contentType, fileData)
		}
		return
	}

	// 完整文件下载
	if tgBot != nil {
		fileData, err := tgBot.DownloadFile(file.FileID)
		if err != nil {
			log.Printf("Failed to download file from Telegram: %v", err)
			c.String(http.StatusInternalServerError, "下载失败")
			return
		}
		c.Data(http.StatusOK, contentType, fileData)
	} else {
		// 如果没有 Bot 实例，返回提示
		c.String(http.StatusOK, "文件ID: %s\n请配置 Telegram Bot", file.FileID)
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
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.Error("请选择文件", "invalid_file"))
		return
	}
	defer file.Close()

	// 读取文件内容
	fileData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Error("读取文件失败", "error"))
		return
	}

	// TODO: 通过 Bot 上传到 Telegram
	_ = header
	_ = fileData

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

	// 设置 Cookie
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

func imageHostingAPI(c *gin.Context) {
	// 获取图片文件用于图床
	files, err := database.GetAllFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.Error("获取文件列表失败", "error"))
		return
	}
	
	// 过滤图片文件
	var imageFiles []models.FileMetadata
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Filename))
		for _, imgExt := range imageExts {
			if ext == imgExt {
				imageFiles = append(imageFiles, f)
				break
			}
		}
	}
	
	c.JSON(http.StatusOK, models.Success(imageFiles))
}

func sseAPI(hub *SSEHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")

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
