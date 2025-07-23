package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/self-made-boy/youtube-tools/internal/api/response"
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

// 使用 response 包中的 Response 结构体

// HealthCheck 处理健康检查请求
// @Summary 健康检查
// @Description 获取 API 服务的健康状态
// @Tags 系统
// @Produce json
// @Success 200 {object} response.Response
// @Router /health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	uptime := time.Since(h.startTime).String()

	response.Success(c, map[string]string{
		"version": h.version,
		"uptime":  uptime,
	})
}

// GetVideoInfoRequest 表示获取视频信息的请求
type GetVideoInfoRequest struct {
	URL string `form:"url" binding:"required"`
}

// GetVideoInfo 处理获取视频信息请求
// @Summary 获取视频信息
// @Description 获取指定 URL 的视频信息
// @Tags 视频
// @Produce json
// @Param url query string true "视频 URL"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /info [get]
func (h *Handler) GetVideoInfo(c *gin.Context) {
	var req GetVideoInfoRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, response.INVALID_REQUEST, err)
		return
	}

	// 获取视频信息
	info, err := h.ytdlp.GetVideoInfo(req.URL)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.VIDEO_INFO_ERROR, err)
		return
	}

	// 直接返回完整的视频信息
	response.Success(c, info)
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
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /download [post]
func (h *Handler) StartDownload(c *gin.Context) {
	var req StartDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, response.INVALID_REQUEST, err)
		return
	}

	// 开始下载
	task, err := h.ytdlp.StartDownload(req.URL, req.Format, req.OutputDir, req.Filename)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.DOWNLOAD_ERROR, err)
		return
	}

	response.Success(c, map[string]interface{}{
		"task_id":  task.ID,
		"filename": task.Filename,
		"state":    task.State,
	})
}

// GetDownloadStatus 处理获取下载状态请求
// @Summary 获取下载状态
// @Description 获取指定任务 ID 的下载状态
// @Tags 下载
// @Produce json
// @Param task_id path string true "任务 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /status/{task_id} [get]
func (h *Handler) GetDownloadStatus(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		response.FailWithMessage(c, http.StatusBadRequest, response.INVALID_TASK_ID, "Task ID is required")
		return
	}

	// 获取下载状态
	task, err := h.ytdlp.GetDownloadStatus(taskID)
	if err != nil {
		response.NotFound(c, response.TASK_NOT_FOUND, err)
		return
	}

	response.Success(c, task)
}

// CancelDownload 处理取消下载请求
// @Summary 取消下载
// @Description 取消指定任务 ID 的下载
// @Tags 下载
// @Produce json
// @Param task_id path string true "任务 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /download/{task_id} [delete]
func (h *Handler) CancelDownload(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		response.FailWithMessage(c, http.StatusBadRequest, response.INVALID_TASK_ID, "Task ID is required")
		return
	}

	// 取消下载
	err := h.ytdlp.CancelDownload(taskID)
	if err != nil {
		response.NotFound(c, response.TASK_NOT_FOUND, err)
		return
	}

	response.SuccessWithMessage(c, "Download cancelled successfully", nil)
}
