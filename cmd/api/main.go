package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/self-made-boy/youtube-tools/internal/api"
	"github.com/self-made-boy/youtube-tools/internal/config"
	"github.com/self-made-boy/youtube-tools/internal/logger"
)

// @title           YouTube Tools API
// @version         1.0
// @description     A RESTful API service for YouTube video operations
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @host      localhost:8080
// @BasePath  /api/yt/

// @securityDefinitions.basic  BasicAuth
func main() {
	// 初始化配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// 初始化日志
	logger, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting YouTube Tools API service")
	logger.Info(fmt.Sprintf("Server will run on port %d", cfg.Server.Port))

	// 打印实际加载的配置信息
	logger.Info("Loaded configuration details",
		zap.String("env", cfg.Env),
		zap.Int("server_port", cfg.Server.Port),
		zap.String("log_level", cfg.Log.Level),
		zap.String("log_format", cfg.Log.Format),
		zap.String("ytdlp_path", cfg.Ytdlp.Path),
		zap.String("ffmpeg_path", cfg.Ytdlp.FfmpegPath),
		zap.String("download_dir", cfg.Ytdlp.DownloadDir),
		zap.String("cookies_path", cfg.Ytdlp.CookiesPath),
		zap.String("proxy", cfg.Ytdlp.Proxy),
		zap.Int("max_downloads", cfg.Ytdlp.MaxDownloads),
		zap.Int64("max_file_size", cfg.Ytdlp.MaxFileSize),
		zap.Strings("audio_formats", cfg.Ytdlp.AudioFormats),
		zap.Strings("video_formats", cfg.Ytdlp.VideoFormats),
		zap.String("s3_mount", cfg.S3Mount),
		zap.String("s3_prefix", cfg.S3Prefix),
	)

	// 初始化路由
	router := api.SetupRouter(cfg, logger)

	// 创建 HTTP 服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// 在单独的 goroutine 中启动服务器
	go func() {
		logger.Info(fmt.Sprintf("Server is running on port %d", cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// 设置 5 秒的超时时间来关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")
}
