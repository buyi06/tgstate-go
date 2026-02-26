package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"tgstate-go/internal/config"
	"tgstate-go/internal/database"
)

const (
	ChunkSize = 19 * 1024 * 1024 // 19.5MB
)

// Bot 结构体
type Bot struct {
	token       string
	channelName string
	httpClient  *http.Client
}

// Update Telegram 更新
type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

// Message Telegram 消息
type Message struct {
	MessageID      int       `json:"message_id"`
	Chat           Chat      `json:"chat"`
	Text           string    `json:"text"`
	Photo          *Photo    `json:"photo"`
	Document       *Document `json:"document"`
	Date           int       `json:"date"`
	ReplyToMessage *Message  `json:"reply_to_message"`
}

// Chat Telegram 聊天
type Chat struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Title    string `json:"title"`
	Type     string `json:"type"`
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
	FileSize int64  `json:"file_size"`
}

// File Telegram 文件
type File struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int64  `json:"file_size"`
	FilePath     string `json:"file_path"`
}

// MessageResponse 发送消息响应
type MessageResponse struct {
	OK     bool    `json:"ok"`
	Result Message `json:"result"`
}

// FileResponse 获取文件响应
type FileResponse struct {
	OK     bool  `json:"ok"`
	Result File  `json:"result"`
}

// NewBot 创建新 Bot
func NewBot(token, channelName string) *Bot {
	return &Bot{
		token:       token,
		channelName: channelName,
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
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
}

// getUpdates 获取更新
func (b *Bot) getUpdates(offset, limit int, timeout int) ([]Update, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&limit=%d&timeout=%d",
		b.token, offset, limit, timeout)

	resp, err := b.httpGet(apiURL)
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

	if !result.OK {
		return nil, fmt.Errorf("telegram API error")
	}

	return result.Result, nil
}

// handleUpdate 处理更新
func (b *Bot) handleUpdate(update Update) {
	if update.Message.Text != "" && update.Message.Text == "get" && update.Message.ReplyToMessage != nil {
		b.handleGetCommand(update.Message)
		return
	}

	// 检查是否来自授权频道
	channelIdentifier := config.Get().ChannelName
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

	// 发送确认消息
	cfg := config.Get()
	if cfg.BaseURL != "" {
		downloadLink := fmt.Sprintf("%s/d/%s/%s", strings.TrimRight(cfg.BaseURL, "/"), shortID, url.QueryEscape(fileName))
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("文件已保存: %s", downloadLink))
	}
}

// handleNewDocument 处理新文档
func (b *Bot) handleNewDocument(msg Message) {
	if msg.Document == nil {
		return
	}

	doc := msg.Document
	fileID := fmt.Sprintf("%d:%s", msg.MessageID, doc.FileID)
	fileName := doc.FileName
	if fileName == "" {
		fileName = fmt.Sprintf("document_%d", msg.MessageID)
	}
	fileSize := doc.FileSize

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

	// 发送确认消息
	cfg := config.Get()
	if cfg.BaseURL != "" {
		downloadLink := fmt.Sprintf("%s/d/%s/%s", strings.TrimRight(cfg.BaseURL, "/"), shortID, url.QueryEscape(fileName))
		b.sendMessage(msg.Chat.ID, fmt.Sprintf("文件已保存: %s", downloadLink))
	}
}

// handleGetCommand 处理 get 命令
func (b *Bot) handleGetCommand(msg Message) {
	cfg := config.Get()

	if msg.ReplyToMessage == nil {
		return
	}

	replyMsg := msg.ReplyToMessage
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

	b.httpPostForm(apiURL, data.Encode())
}

// sendDocument 发送文档
func (b *Bot) sendDocument(chatID int64, fileName string, fileData []byte, replyToID int) (*Document, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", b.token)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加 chat_id
	writer.WriteField("chat_id", strconv.FormatInt(chatID, 10))

	// 添加 document
	part, err := writer.CreateFormFile("document", fileName)
	if err != nil {
		return nil, err
	}
	part.Write(fileData)

	// 添加 reply_to_message_id
	if replyToID > 0 {
		writer.WriteField("reply_to_message_id", strconv.Itoa(replyToID))
	}

	writer.Close()

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result MessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API error: %s", string(respBody))
	}

	return result.Result.Document, nil
}

// getFile 获取文件信息
func (b *Bot) getFile(fileID string) (*File, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", b.token, fileID)

	resp, err := b.httpGet(apiURL)
	if err != nil {
		return nil, err
	}

	var result FileResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API error")
	}

	return &result.Result, nil
}

// GetDownloadURL 获取下载链接
func (b *Bot) GetDownloadURL(fileID string) (string, error) {
	file, err := b.getFile(fileID)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.token, file.FilePath), nil
}

// DownloadFile 下载文件
func (b *Bot) DownloadFile(fileID string) ([]byte, error) {
	downloadURL, err := b.GetDownloadURL(fileID)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Get(downloadURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// DeleteMessage 删除消息
func (b *Bot) DeleteMessage(chatID int64, messageID int) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", b.token)

	data := url.Values{}
	data.Set("chat_id", strconv.FormatInt(chatID, 10))
	data.Set("message_id", strconv.Itoa(messageID))

	resp, err := b.httpPostForm(apiURL, data.Encode())
	if err != nil {
		return err
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("failed to delete message")
	}

	return nil
}

// UploadFile 上传文件到 Telegram
func (b *Bot) UploadFile(filePath string, fileName string) (string, int64, error) {
	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", 0, err
	}

	fileSize := int64(len(data))

	// 大文件分块上传
	if fileSize >= ChunkSize {
		return b.uploadAsChunks(data, fileName)
	}

	// 小文件直接上传
	channelID, _ := strconv.ParseInt(b.channelName, 10, 64)
	if strings.HasPrefix(b.channelName, "@") {
		channelID = 0 // 群组/频道用 username
	}

	doc, err := b.sendDocument(channelID, fileName, data, 0)
	if err != nil {
		return "", 0, err
	}

	compositeID := fmt.Sprintf("%d:%s", 0, doc.FileID) // message_id 暂时为 0
	return compositeID, fileSize, nil
}

// uploadAsChunks 分块上传
func (b *Bot) uploadAsChunks(data []byte, originalFileName string) (string, int64, error) {
	channelID, _ := strconv.ParseInt(b.channelName, 10, 64)
	if strings.HasPrefix(b.channelName, "@") {
		channelID = 0
	}

	chunkIDs := []string{}
	var firstMessageID int

	// 分块上传
	chunkNum := 1
	for i := 0; i < len(data); i += ChunkSize {
		end := i + ChunkSize
		if end > len(data) {
			end = len(data)
		}

		chunkData := data[i:end]
		chunkName := fmt.Sprintf("%s.part%d", originalFileName, chunkNum)

		log.Printf("Uploading chunk %d: %s", chunkNum, chunkName)

		doc, err := b.sendDocument(channelID, chunkName, chunkData, firstMessageID)
		if err != nil {
			log.Printf("Failed to upload chunk %d: %v", chunkNum, err)
			continue
		}

		chunkIDs = append(chunkIDs, fmt.Sprintf("%d:%s", 0, doc.FileID)) // message_id 暂时为 0
		if firstMessageID == 0 {
			firstMessageID = 1 // 标记第一个消息
		}
		chunkNum++
	}

	// 上传清单文件
	manifestContent := fmt.Sprintf("tgstate-blob\n%s\n%s", originalFileName, strings.Join(chunkIDs, "\n"))
	manifestName := originalFileName + ".manifest"

	log.Println("Uploading manifest file")

	doc, err := b.sendDocument(channelID, manifestName, []byte(manifestContent), firstMessageID)
	if err != nil {
		return "", 0, err
	}

	compositeID := fmt.Sprintf("%d:%s", 0, doc.FileID)
	return compositeID, int64(len(data)), nil
}

// httpGet 发送 GET 请求
func (b *Bot) httpGet(apiURL string) ([]byte, error) {
	resp, err := b.httpClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// httpPostForm 发送 POST 请求
func (b *Bot) httpPostForm(apiURL string, data string) ([]byte, error) {
	resp, err := b.httpClient.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
