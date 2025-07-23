package config

import (
	"os"
	"strconv"
)

// Config 保存应用程序配置
type Config struct {
	// 服务器配置
	Port int

	// 日志配置
	LogLevel  string
	LogFormat string

	// yt-dlp 配置
	YtdlpPath    string
	FfmpegPath   string
	DownloadDir  string
	CookiesPath  string // cookies.txt 文件路径
	MaxDownloads int
	MaxFileSize  int64 // 单位：字节

	AudioFormats []string
	VideoFormats []string

	// 其他配置
	Env string
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		port = 8080
	}

	maxDownloads, err := strconv.Atoi(getEnv("MAX_DOWNLOADS", "5"))
	if err != nil {
		maxDownloads = 5
	}

	maxFileSize, err := strconv.ParseInt(getEnv("MAX_FILE_SIZE", "1073741824"), 10, 64) // 默认 1GB
	if err != nil {
		maxFileSize = 1073741824 // 1GB
	}

	return &Config{
		Port:         port,
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		LogFormat:    getEnv("LOG_FORMAT", "json"),
		YtdlpPath:    getEnv("YTDLP_PATH", "/usr/bin/yt-dlp"),
		FfmpegPath:   getEnv("FFMPEG_PATH", "/usr/bin/ffmpeg"),
		DownloadDir:  getEnv("DOWNLOAD_DIR", "/app/downloads"),
		CookiesPath:  getEnv("COOKIES_PATH", "/app/cookies.txt"),
		MaxDownloads: maxDownloads,
		MaxFileSize:  maxFileSize,
		Env:          getEnv("ENV", "development"),
	}, nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
