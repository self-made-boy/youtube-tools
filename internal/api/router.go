package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/self-made-boy/youtube-tools/internal/api/handlers"
	"github.com/self-made-boy/youtube-tools/internal/api/middleware"
	"github.com/self-made-boy/youtube-tools/internal/config"
	"github.com/self-made-boy/youtube-tools/internal/ytdlp"

	_ "github.com/self-made-boy/youtube-tools/docs" // 导入 Swagger 文档
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRouter 设置 API 路由
func SetupRouter(cfg *config.Config, logger *zap.Logger) *gin.Engine {
	// 设置 Gin 模式
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建 Gin 路由器
	router := gin.New()

	// 添加中间件
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())

	// 创建 yt-dlp 服务
	ytdlpService := ytdlp.New(cfg, logger)

	// 创建处理器
	h := handlers.New(cfg, logger, ytdlpService)

	// API 路由组
	api := router.Group("/api/yt/")
	{
		// 健康检查
		api.GET("/health", h.HealthCheck)

		api.GET("/info", h.GetVideoInfo)
		api.POST("/download", h.StartDownload)
		api.GET("/download/status", h.GetDownloadStatus)
	}

	// Swagger 文档
	router.GET("/api/yt/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}
