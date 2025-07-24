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
// @Tags youtube
// @Produce json
// @Param url query string true "视频 URL"
// @Success 200 {object} response.Response{data=ytdlp.VideoInfo}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /info [get]
func (h *Handler) GetVideoInfo(c *gin.Context) {
	var req GetVideoInfoRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, response.INVALID_REQUEST, err)
		return
	}
	// 检查URL是否有效
	url, _, err := h.ytdlp.CheckUrl(req.URL)
	if err != nil {
		response.BadRequest(c, response.INVALID_REQUEST, err)
		return
	}

	// 获取视频信息
	info, err := h.ytdlp.GetVideoInfo(url)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.VIDEO_INFO_ERROR, err)
		return
	}

	// 直接返回完整的视频信息
	response.Success(c, info)
}

// StartDownloadRequest 表示开始下载的请求
type StartDownloadRequest struct {
	// 下载的url
	URL string `json:"url" binding:"required"`
	// 下载的格式
	FormatId string `json:"format_id" binding:"omitempty"`
}

// StartDownloadResp 表示开始下载的响应
type StartDownloadResp struct {
	TaskID string `json:"task_id"`
}

// StartDownload 处理开始下载请求
// @Summary 开始下载视频
// @Description 开始下载指定 URL 的视频
// @Tags youtube
// @Accept json
// @Produce json
// @Param request body StartDownloadRequest true "下载请求"
// @Success 200 {object} response.Response{data=StartDownloadResp}
// @Failure 400 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /download [post]
func (h *Handler) StartDownload(c *gin.Context) {
	var req StartDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, response.INVALID_REQUEST, err)
		return
	}

	// 检查URL是否有效
	_, _, err := h.ytdlp.CheckUrl(req.URL)
	if err != nil {
		response.BadRequest(c, response.INVALID_REQUEST, err)
		return
	}
	_, _, _, audioErr := h.ytdlp.ParseAudioFormatID(req.FormatId)
	_, _, _, videoErr := h.ytdlp.ParseVideoFormatID(req.FormatId)
	if audioErr != nil && videoErr != nil {
		response.BadRequest(c, response.INVALID_REQUEST, videoErr)
		return
	}
	// 开始下载
	taskID, err := h.ytdlp.StartDownload(req.URL, req.FormatId)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.DOWNLOAD_ERROR, err)
		return
	}

	response.Success(c, StartDownloadResp{
		TaskID: taskID,
	})
}

// DownloadTaskStatusResp 表示下载任务状态的响应
type DownloadTaskStatusResp struct {
	// 任务ID
	TaskID string `json:"task_id" example:"123456"`
	// 下载状态
	State string `json:"state" example:"pending, downloading, completed, failed"`
	// 下载进度
	Progress float64 `json:"progress" example:"0.5"`
	// 预计时间
	ETA string `json:"eta" example:"10s"`
	// 下载文件路径
	DownloadUrl string `json:"download_url" example:"https://xxx.com/123456.m4a"`
}

// GetDownloadStatus 处理获取下载状态请求
// @Summary 获取下载状态
// @Description 获取指定任务 ID 的下载状态
// @Tags youtube
// @Produce json
// @Param task_id query string true "任务 ID"
// @Success 200 {object} response.Response{data=DownloadTaskStatusResp}
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /download/status [get]
func (h *Handler) GetDownloadStatus(c *gin.Context) {
	taskID := c.Query("task_id")

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

	response.Success(c, DownloadTaskStatusResp{
		TaskID:      task.ID,
		State:       task.State,
		Progress:    task.Progress,
		ETA:         task.ETA,
		DownloadUrl: task.DownloadUrl,
	})
}
