package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/self-made-boy/youtube-tools/internal/config"
	"github.com/self-made-boy/youtube-tools/internal/ytdlp"
)

// Handler 处理 API 请求
type Handler struct {
	config    *config.Config
	logger    *zap.Logger
	ytdlp     *ytdlp.Service
	version   string
	startTime time.Time
}

// New 创建一个新的处理器
func New(cfg *config.Config, logger *zap.Logger, ytdlpService *ytdlp.Service) *Handler {
	return &Handler{
		config:    cfg,
		logger:    logger,
		ytdlp:     ytdlpService,
		version:   "1.0.0",
		startTime: time.Now(),
	}
}

// Response 表示 API 响应
type Response struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ErrorInfo 表示错误信息
type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// HealthCheck 处理健康检查请求
// @Summary 健康检查
// @Description 获取 API 服务的健康状态
// @Tags 系统
// @Produce json
// @Success 200 {object} Response
// @Router /health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	uptime := time.Since(h.startTime).String()

	c.JSON(http.StatusOK, Response{
		Status: "success",
		Data: map[string]string{
			"version": h.version,
			"uptime":  uptime,
		},
	})
}

// GetVideoInfoRequest 表示获取视频信息的请求
type GetVideoInfoRequest struct {
	URL    string `form:"url" binding:"required"`
	Format string `form:"format" binding:"omitempty,oneof=json simple"`
}

// GetVideoInfo 处理获取视频信息请求
// @Summary 获取视频信息
// @Description 获取指定 URL 的视频信息
// @Tags 视频
// @Produce json
// @Param url query string true "视频 URL"
// @Param format query string false "输出格式 (json, simple)"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /info [get]
func (h *Handler) GetVideoInfo(c *gin.Context) {
	var req GetVideoInfoRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		return
	}

	// 获取视频信息
	info, err := h.ytdlp.GetVideoInfo(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "VIDEO_INFO_ERROR",
				Message: "Failed to get video information",
				Details: err.Error(),
			},
		})
		return
	}

	// 根据格式返回结果
	if req.Format == "simple" {
		c.JSON(http.StatusOK, Response{
			Status: "success",
			Data: map[string]interface{}{
				"id":       info.ID,
				"title":    info.Title,
				"uploader": info.Uploader,
				"duration": info.Duration,
			},
		})
	} else {
		c.JSON(http.StatusOK, Response{
			Status: "success",
			Data:   info,
		})
	}
}

// StartDownloadRequest 表示开始下载的请求
type StartDownloadRequest struct {
	URL       string `json:"url" binding:"required"`
	Format    string `json:"format" binding:"omitempty"`
	OutputDir string `json:"output_dir" binding:"omitempty"`
	Filename  string `json:"filename" binding:"omitempty"`
}

// StartDownload 处理开始下载请求
// @Summary 开始下载视频
// @Description 开始下载指定 URL 的视频
// @Tags 下载
// @Accept json
// @Produce json
// @Param request body StartDownloadRequest true "下载请求"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /download [post]
func (h *Handler) StartDownload(c *gin.Context) {
	var req StartDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "INVALID_REQUEST",
				Message: "Invalid request parameters",
				Details: err.Error(),
			},
		})
		return
	}

	// 开始下载
	task, err := h.ytdlp.StartDownload(req.URL, req.Format, req.OutputDir, req.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "DOWNLOAD_ERROR",
				Message: "Failed to start download",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status: "success",
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"filename": task.Filename,
			"state":    task.State,
		},
	})
}

// GetDownloadStatus 处理获取下载状态请求
// @Summary 获取下载状态
// @Description 获取指定任务 ID 的下载状态
// @Tags 下载
// @Produce json
// @Param task_id path string true "任务 ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /status/{task_id} [get]
func (h *Handler) GetDownloadStatus(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "INVALID_TASK_ID",
				Message: "Task ID is required",
			},
		})
		return
	}

	// 获取下载状态
	task, err := h.ytdlp.GetDownloadStatus(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "TASK_NOT_FOUND",
				Message: "Download task not found",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status: "success",
		Data:   task,
	})
}

// CancelDownload 处理取消下载请求
// @Summary 取消下载
// @Description 取消指定任务 ID 的下载
// @Tags 下载
// @Produce json
// @Param task_id path string true "任务 ID"
// @Success 200 {object} Response
// @Failure 400 {object} Response
// @Failure 404 {object} Response
// @Router /download/{task_id} [delete]
func (h *Handler) CancelDownload(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "INVALID_TASK_ID",
				Message: "Task ID is required",
			},
		})
		return
	}

	// 取消下载
	err := h.ytdlp.CancelDownload(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Status: "error",
			Error: &ErrorInfo{
				Code:    "TASK_NOT_FOUND",
				Message: "Download task not found",
				Details: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status:  "success",
		Message: "Download cancelled successfully",
	})
}
