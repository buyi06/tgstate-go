package main

import (
	"flag"
	"log"
	"os"

	"tgstate-go/internal/api"
	"tgstate-go/internal/bot"
	"tgstate-go/internal/config"
	"tgstate-go/internal/database"
)

func main() {
	// 解析命令行参数
	port := flag.String("port", "8000", "Server port")
	configPath := flag.String("config", ".env", "Config file path")
	flag.Parse()

	// 加载配置
	if err := config.Load(*configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库
	if err := database.Init(); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 设置日志
	if os.Getenv("LOG_LEVEL") == "DEBUG" {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// 启动 Telegram Bot
	cfg := config.Get()
	tgBot := bot.NewBot(cfg.BotToken, cfg.ChannelName)
	
	// 设置全局 Bot 实例
	api.SetBot(tgBot)
	
	go func() {
		if err := tgBot.Start(); err != nil {
			log.Printf("Bot error: %v", err)
		}
	}()

	// 启动 HTTP 服务器
	server := api.NewServer()
	log.Printf("Server starting on port %s", *port)
	if err := server.Run(":" + *port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
