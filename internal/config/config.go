package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 保存应用程序配置
type Config struct {
	// 服务器配置
	Server ServerConfig `yaml:"server"`

	// 日志配置
	Log LogConfig `yaml:"log"`

	// yt-dlp 配置
	Ytdlp YtdlpConfig `yaml:"ytdlp"`

	// 其他配置
	Env string `yaml:"env"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int `yaml:"port"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// YtdlpConfig yt-dlp 配置
type YtdlpConfig struct {
	Path         string   `yaml:"path"`
	FfmpegPath   string   `yaml:"ffmpeg_path"`
	DownloadDir  string   `yaml:"download_dir"`
	CookiesPath  string   `yaml:"cookies_path"` // cookies.txt 文件路径
	Proxy        string   `yaml:"proxy"`        // HTTP/HTTPS/SOCKS代理，例如：http://proxy.example.com:8080
	MaxDownloads int      `yaml:"max_downloads"`
	MaxFileSize  int64    `yaml:"max_file_size"` // 单位：字节
	AudioFormats []string `yaml:"audio_formats"` // aac, alac, flac, m4a, mp3, opus, vorbis, wav
	VideoFormats []string `yaml:"video_formats"` // avi, flv, mkv, mov, mp4, webm
}

// Load 从YAML配置文件加载配置
func Load() (*Config, error) {
	// 获取配置文件路径，默认为当前目录下的config.yaml
	configPath := getEnv("CONFIG_PATH", "config.yaml")

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// 读取配置文件
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// 解析YAML配置
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// 设置默认值
	setDefaults(&config)

	return &config, nil
}

// setDefaults 设置默认值
func setDefaults(config *Config) {
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Log.Level == "" {
		config.Log.Level = "info"
	}
	if config.Log.Format == "" {
		config.Log.Format = "json"
	}
	if config.Ytdlp.Path == "" {
		config.Ytdlp.Path = "/usr/bin/yt-dlp"
	}
	if config.Ytdlp.FfmpegPath == "" {
		config.Ytdlp.FfmpegPath = "/usr/bin/ffmpeg"
	}
	if config.Ytdlp.DownloadDir == "" {
		config.Ytdlp.DownloadDir = "/tmp"
	}

	if config.Ytdlp.MaxDownloads == 0 {
		config.Ytdlp.MaxDownloads = 5
	}
	if config.Ytdlp.MaxFileSize == 0 {
		config.Ytdlp.MaxFileSize = 1073741824 // 1GB
	}
	if len(config.Ytdlp.AudioFormats) == 0 {
		config.Ytdlp.AudioFormats = []string{"mp3", "m4a", "aac", "opus", "flac", "wav"}
	}
	if len(config.Ytdlp.VideoFormats) == 0 {
		config.Ytdlp.VideoFormats = []string{"mp4", "webm", "mkv", "avi", "mov", "flv"}
	}
	if config.Env == "" {
		config.Env = "development"
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
