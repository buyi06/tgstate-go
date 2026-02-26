package config

import (
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BotToken     string `yaml:"bot_token" env:"BOT_TOKEN"`
	ChannelName  string `yaml:"channel_name" env:"CHANNEL_NAME"`
	BaseURL      string `yaml:"base_url" env:"BASE_URL"`
	Port         string `yaml:"port" env:"PORT"`
	Password     string `yaml:"password" env:"PASSWORD"`
	DatabasePath string `yaml:"database_path" env:"DATABASE_PATH"`
	LogLevel     string `yaml:"log_level" env:"LOG_LEVEL"`
}

var cfg *Config

func Load(path string) error {
	// 尝试读取 .env 文件
	cfg = &Config{}

	// 从环境变量加载
	if token := os.Getenv("BOT_TOKEN"); token != "" {
		cfg.BotToken = token
	}
	if channel := os.Getenv("CHANNEL_NAME"); channel != "" {
		cfg.ChannelName = channel
	}
	if baseURL := os.Getenv("BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if port := os.Getenv("PORT"); port != "" {
		cfg.Port = port
	}
	if password := os.Getenv("PASSWORD"); password != "" {
		cfg.Password = password
	}
	if dbPath := os.Getenv("DATABASE_PATH"); dbPath != "" {
		cfg.DatabasePath = dbPath
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	// 尝试加载 YAML 配置
	if data, err := os.ReadFile(path); err == nil {
		yamlCfg := &Config{}
		if err := yaml.Unmarshal(data, yamlCfg); err == nil {
			if yamlCfg.BotToken != "" && cfg.BotToken == "" {
				cfg.BotToken = yamlCfg.BotToken
			}
			if yamlCfg.ChannelName != "" && cfg.ChannelName == "" {
				cfg.ChannelName = yamlCfg.ChannelName
			}
			if yamlCfg.BaseURL != "" && cfg.BaseURL == "" {
				cfg.BaseURL = yamlCfg.BaseURL
			}
			if yamlCfg.Port != "" && cfg.Port == "" {
				cfg.Port = yamlCfg.Port
			}
			if yamlCfg.Password != "" && cfg.Password == "" {
				cfg.Password = yamlCfg.Password
			}
		}
	}

	// 默认值
	if cfg.Port == "" {
		cfg.Port = "8000"
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "./data/file_metadata.db"
	}

	// 验证必要配置 - 测试时可以注释掉
	// if cfg.BotToken == "" {
	// 	return fmt.Errorf("BOT_TOKEN is required")
	// }
	// if cfg.ChannelName == "" {
	// 	return fmt.Errorf("CHANNEL_NAME is required")
	// }

	log.Printf("Config loaded: BotToken=%s, ChannelName=%s, BaseURL=%s",
		maskToken(cfg.BotToken), cfg.ChannelName, cfg.BaseURL)

	return nil
}

func Get() *Config {
	return cfg
}

func maskToken(token string) string {
	if len(token) < 10 {
		return "***"
	}
	return strings.Repeat("*", len(token)-10) + token[len(token)-10:]
}
