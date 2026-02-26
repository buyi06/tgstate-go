package bot

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tgstate-go/internal/config"
	"tgstate-go/internal/database"
)

// Bot 结构体
type Bot struct {
	token   string
	updates chan interface{}
}

// Update Telegram 更新
type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

// Message Telegram 消息
type Message struct {
	MessageID       int       `json:"message_id"`
	Chat            Chat      `json:"chat"`
	Text            string    `json:"text"`
	Photo           *Photo    `json:"photo"`
	Document        *Document `json:"document"`
	Date            int       `json:"date"`
	ReplyToMessage  *Message  `json:"reply_to_message"`
}

// Chat Telegram 聊天
type Chat struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Title     string `json:"title"`
	Type      string `json:"type"`
}

// Photo Telegram 照片
type Photo struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size"`
}

// Document Telegram 文档
type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int    `json:"file_size"`
}

// File Telegram 文件
type File struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size"`
	FilePath     string `json:"file_path"`
}

// NewBot 创建新 Bot
func NewBot(token string) *Bot {
	return &Bot{
		token:   token,
		updates: make(chan interface{}, 100),
	}
}

// Start 启动 Bot
func (b *Bot) Start() error {
	offset := 0

	log.Println("Bot started, polling...")

	for {
		updates, err := b.getUpdates(offset, 100, 30)
		if err != nil {
			log.Printf("Error getting updates: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			go b.handleUpdate(update)
		}
	}

	return nil
}

// getUpdates 获取更新
func (b *Bot) getUpdates(offset, limit int, timeout int) ([]Update, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&limit=%d&timeout=%d",
		b.token, offset, limit, timeout)

	resp, err := httpGet(apiURL)
	if err != nil {
		return nil, err
	}

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Result, nil
}

// handleUpdate 处理更新
func (b *Bot) handleUpdate(update Update) {
	cfg := config.Get()

	if update.Message.Text != "" && update.Message.Text == "get" && update.Message.ReplyToMessage != nil {
		b.handleGetCommand(update.Message)
		return
	}

	// 检查是否来自授权频道
	channelIdentifier := cfg.ChannelName
	isAllowed := false

	chat := update.Message.Chat
	if strings.HasPrefix(channelIdentifier, "@") {
		if chat.Username == strings.TrimPrefix(channelIdentifier, "@") {
			isAllowed = true
		}
	} else {
		channelID, _ := strconv.ParseInt(channelIdentifier, 10, 64)
		if chat.ID == channelID {
			isAllowed = true
		}
	}

	if !isAllowed {
		return
	}

	// 处理新文件
	if update.Message.Photo != nil {
		b.handleNewPhoto(update.Message)
	} else if update.Message.Document != nil {
		b.handleNewDocument(update.Message)
	}
}

// handleNewPhoto 处理新照片
func (b *Bot) handleNewPhoto(msg Message) {
	cfg := config.Get()

	if msg.Photo == nil {
		return
	}

	photo := msg.Photo
	fileID := fmt.Sprintf("%d:%s", msg.MessageID, photo.FileID)
	fileName := fmt.Sprintf("photo_%d.jpg", msg.MessageID)
	fileSize := int64(photo.FileSize)

	// 跳过大于20MB的文件
	if fileSize > 20*1024*1024 {
		return
	}

	// 保存到数据库
	shortID, err := database.AddFileMetadata(fileName, fileID, fileSize)
	if err != nil {
		log.Printf("Failed to add photo metadata: %v", err)
		return
	}

	log.Printf("New photo: %s -> %s", fileName, shortID)

	// 发送确认消息（可选）
	if cfg.BaseURL != "" {
		downloadLink := fmt.Sprintf("%s/d/%s/%s", strings.TrimRight(cfg.BaseURL, "/"), shortID, url.QueryEscape(fileName))
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("文件已保存: %s", downloadLink))
	}
}

// handleNewDocument 处理新文档
func (b *Bot) handleNewDocument(msg Message) {
	cfg := config.Get()

	if msg.Document == nil {
		return
	}

	doc := msg.Document
	fileID := fmt.Sprintf("%d:%s", msg.MessageID, doc.FileID)
	fileName := doc.FileName
	if fileName == "" {
		fileName = fmt.Sprintf("document_%d", msg.MessageID)
	}
	fileSize := int64(doc.FileSize)

	// 跳过大于20MB的文件
	if fileSize > 20*1024*1024 {
		return
	}

	// 保存到数据库
	shortID, err := database.AddFileMetadata(fileName, fileID, fileSize)
	if err != nil {
		log.Printf("Failed to add document metadata: %v", err)
		return
	}

	log.Printf("New document: %s -> %s", fileName, shortID)

	// 发送确认消息（可选）
	if cfg.BaseURL != "" {
		downloadLink := fmt.Sprintf("%s/d/%s/%s", strings.TrimRight(cfg.BaseURL, "/"), shortID, url.QueryEscape(fileName))
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("文件已保存: %s", downloadLink))
	}
}

// handleGetCommand 处理 get 命令
func (b *Bot) handleGetCommand(msg Message) {
	cfg := config.Get()

	replyMsg := msg.ReplyToMessage
	if replyMsg == nil {
		return
	}

	var fileID, fileName string

	if replyMsg.Photo != nil {
		fileID = fmt.Sprintf("%d:%s", replyMsg.MessageID, replyMsg.Photo.FileID)
		fileName = fmt.Sprintf("photo_%d.jpg", replyMsg.MessageID)
	} else if replyMsg.Document != nil {
		fileID = fmt.Sprintf("%d:%s", replyMsg.MessageID, replyMsg.Document.FileID)
		fileName = replyMsg.Document.FileName
	}

	if fileID == "" {
		return
	}

	// 从数据库获取短ID
	files, err := database.GetAllFiles()
	if err != nil {
		b.sendMessage(msg.Chat.ID, "获取文件失败")
		return
	}

	var shortID string
	for _, f := range files {
		if strings.HasPrefix(f.FileID, fileID) {
			shortID = f.ShortID
			fileName = f.Filename
			break
		}
	}

	if shortID == "" {
		b.sendMessage(msg.Chat.ID, "文件未找到")
		return
	}

	downloadPath := fmt.Sprintf("/d/%s/%s", shortID, url.QueryEscape(fileName))

	var replyText string
	if cfg.BaseURL != "" {
		replyText = fmt.Sprintf("文件 '%s' 的下载链接:\n%s%s", fileName, strings.TrimRight(cfg.BaseURL, "/"), downloadPath)
	} else {
		replyText = fmt.Sprintf("文件 '%s' 的下载路径 (请自行拼接域名):\n%s", fileName, downloadPath)
	}

	b.sendMessage(msg.Chat.ID, replyText)
}

// sendMessage 发送消息
func (b *Bot) sendMessage(chatID int64, text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.token)
	
	data := url.Values{}
	data.Set("chat_id", strconv.FormatInt(chatID, 10))
	data.Set("text", text)

	httpPostForm(apiURL, data.Encode())
}

// httpGet 发送 GET 请求
func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// httpPostForm 发送 POST 请求
func httpPostForm(url string, data string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/x-www-form-urlencoded", strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
