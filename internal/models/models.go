package models

import "time"

type FileMetadata struct {
	ID         int64     `json:"id"`
	ShortID    string    `json:"short_id"`
	Filename   string    `json:"filename"`
	FileID     string    `json:"file_id"`
	Filesize   int64     `json:"filesize"`
	UploadDate time.Time `json:"upload_date"`
}

type AppSettings struct {
	Password     string `json:"password"`
	BotToken     string `json:"bot_token"`
	ChannelName  string `json:"channel_name"`
	BaseURL      string `json:"base_url"`
}

type APIResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) APIResponse {
	return APIResponse{
		Code:    "success",
		Message: "操作成功",
		Data:    data,
	}
}

func Error(message string, code string) APIResponse {
	return APIResponse{
		Code:    code,
		Message: message,
	}
}
