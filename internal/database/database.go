package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tgstate-go/internal/config"
	"tgstate-go/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func Init() error {
	cfg := config.Get()
	// 创建数据目录
	dir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// 打开数据库
	database, err := sql.Open("sqlite3", cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	db = database

	// 创建表
	if err := createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Println("Database initialized successfully")
	return nil
}

func createTables() error {
	// 文件元数据表
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS file_metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			short_id TEXT UNIQUE NOT NULL,
			filename TEXT NOT NULL,
			file_id TEXT NOT NULL,
			filesize INTEGER NOT NULL,
			upload_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 创建索引
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_short_id ON file_metadata(short_id)`)
	if err != nil {
		return err
	}

	// 应用设置表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)

	return err
}

// AddFileMetadata 添加文件元数据
func AddFileMetadata(filename, fileID string, filesize int64) (string, error) {
	shortID := generateShortID()
	
	_, err := db.Exec(`
		INSERT INTO file_metadata (short_id, filename, file_id, filesize, upload_date)
		VALUES (?, ?, ?, ?, ?)
	`, shortID, filename, fileID, filesize, time.Now())
	
	if err != nil {
		return "", err
	}
	
	return shortID, nil
}

// GetFileByShortID 通过短ID获取文件
func GetFileByShortID(shortID string) (*models.FileMetadata, error) {
	row := db.QueryRow(`
		SELECT id, short_id, filename, file_id, filesize, upload_date
		FROM file_metadata
		WHERE short_id = ?
	`, shortID)

	var file models.FileMetadata
	err := row.Scan(&file.ID, &file.ShortID, &file.Filename, &file.FileID, &file.Filesize, &file.UploadDate)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

// GetFileByMessageID 通过消息ID获取文件
func GetFileByMessageID(messageID int) (*models.FileMetadata, error) {
	row := db.QueryRow(`
		SELECT id, short_id, filename, file_id, filesize, upload_date
		FROM file_metadata
		WHERE file_id LIKE ?
	`, fmt.Sprintf("%%:%d", messageID))

	var file models.FileMetadata
	err := row.Scan(&file.ID, &file.ShortID, &file.Filename, &file.FileID, &file.Filesize, &file.UploadDate)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

// DeleteFileByShortID 通过短ID删除文件
func DeleteFileByShortID(shortID string) error {
	_, err := db.Exec(`DELETE FROM file_metadata WHERE short_id = ?`, shortID)
	return err
}

// GetAllFiles 获取所有文件
func GetAllFiles() ([]models.FileMetadata, error) {
	rows, err := db.Query(`
		SELECT id, short_id, filename, file_id, filesize, upload_date
		FROM file_metadata
		ORDER BY upload_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.FileMetadata
	for rows.Next() {
		var file models.FileMetadata
		if err := rows.Scan(&file.ID, &file.ShortID, &file.Filename, &file.FileID, &file.Filesize, &file.UploadDate); err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	return files, nil
}

// GetSetting 获取设置
func GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetSetting 设置设置
func SetSetting(key, value string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO app_settings (key, value) VALUES (?, ?)
	`, key, value)
	return err
}

// GetPassword 获取密码
func GetPassword() (string, error) {
	return GetSetting("password")
}

// SetPassword 设置密码
func SetPassword(password string) error {
	return SetSetting("password", password)
}

func generateShortID() string {
	data := make([]byte, 6)
	rand.Read(data)
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}
