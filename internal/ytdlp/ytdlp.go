package ytdlp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/self-made-boy/youtube-tools/internal/config"
)

// Service 提供 yt-dlp 相关操作
type Service struct {
	config    *config.Config
	logger    *zap.Logger
	downloads map[string]*DownloadTask
	mutex     sync.RWMutex
}

// DownloadTask 表示一个下载任务
type DownloadTask struct {
	ID        string             `json:"id"`
	URL       string             `json:"url"`
	Format    string             `json:"format"`
	OutputDir string             `json:"output_dir"`
	Filename  string             `json:"filename"`
	State     string             `json:"state"` // pending, downloading, completed, failed
	Progress  float64            `json:"progress"`
	Speed     string             `json:"speed"`
	ETA       string             `json:"eta"`
	Error     string             `json:"error,omitempty"`
	StartTime time.Time          `json:"start_time"`
	EndTime   time.Time          `json:"end_time,omitempty"`
	FilePath  string             `json:"file_path,omitempty"`
	FileSize  int64              `json:"file_size,omitempty"`
	Cmd       *exec.Cmd          `json:"-"`
	Ctx       context.Context    `json:"-"`
	Cancel    context.CancelFunc `json:"-"`
}

// VideoInfo 表示视频信息
type VideoInfo struct {
	// 视频ID
	ID string `json:"id" example:"dQw4w9WgXcQ"`
	// 视频网页URL
	WebpageURL string `json:"webpage_url" example:"https://www.youtube.com/watch?v=dQw4w9WgXcQ"`
	// 视频标题
	Title string `json:"title" example:"Rick Astley - Never Gonna Give You Up"`
	// 视频描述
	Description string `json:"description" example:"Official video for Never Gonna Give You Up"`
	// 视频时长
	Duration int `json:"duration" example:"213"`
	// 视频缩略图
	Thumbnail string `json:"thumbnail" example:"https://i.ytimg.com/vi/dQw4w9WgXcQ/maxresdefault.jpg"`
	// 观看次数
	ViewCount int64 `json:"view_count" example:"1000000"`
	// 评论数量
	CommentCount int64 `json:"comment_count" example:"50000"`
	// 点赞数量
	LikeCount int64 `json:"like_count" example:"80000"`
	// 上传日期
	UploadDate string `json:"upload_date" example:"20091025"`
	// 上传者
	Uploader string `json:"uploader" example:"Rick Astley"`
	// 分类
	Categories []string `json:"categories" example:"[\"Music\"]"`
	// 标签
	Tags []string `json:"tags" example:"[\"rick astley\", \"never gonna give you up\", \"music\"]"`
	// 频道名称
	ChannelName string `json:"channel" example:"Rick Astley"`
	// 频道URL
	ChannelURL string `json:"channel_url" example:"https://www.youtube.com/channel/UCuAXFkgsw1L7xaCfnd5JJOw"`
	// 频道订阅数
	ChannelFollowerCount int64 `json:"channel_follower_count" example:"2500000"`
	// 音频格式
	Audio []VideoFormatGroup `json:"audio"`
	// 视频格式
	Video []VideoFormatGroup `json:"video"`
}

// VideoFormatGroup 表示视频按照后缀名分组格式
type VideoFormatGroup struct {
	// 文件扩展名
	Ext string `json:"ext" example:"mp4"`
	// 格式列表
	Formats []VideoFormat `json:"formats"`
}

// AudioFormatGroup 表示音频按照后缀名分组格式
type AudioFormatGroup struct {
	// 音频文件扩展名
	Ext string `json:"ext" example:"m4a"`
	// 音频格式列表
	Formats []AudioFormat `json:"formats"`
}

// VideoFormat 表示视频格式
type VideoFormat struct {
	// 格式ID
	FormatID string `json:"format_id" example:"137"`
	// 文件扩展名
	Ext string `json:"ext" example:"mp4"`
	// 分辨率
	Resolution string `json:"resolution" example:"1920x1080"`
	// 文件大小
	Filesize int64 `json:"filesize" example:"52428800"`
}

// AudioFormat 表示音频格式
type AudioFormat struct {
	// 音频格式ID
	FormatID string `json:"format_id" example:"140"`
	// 音频文件扩展名
	Ext string `json:"ext" example:"m4a"`
	// 音频文件大小
	Filesize int64 `json:"filesize" example:"3145728"`
}

// New 创建一个新的 yt-dlp 服务
func New(cfg *config.Config, logger *zap.Logger) *Service {
	return &Service{
		config:    cfg,
		logger:    logger,
		downloads: make(map[string]*DownloadTask),
		mutex:     sync.RWMutex{},
	}
}

// CheckUrl 检查URL是否为有效的YouTube视频链接,返回纯净的链接和视频 Id
func (s *Service) CheckUrl(urlStr string) (string, string, error) {
	// 解析URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", "", err
	}

	// Check and normalize URL scheme
	if parsedURL.Scheme == "" || parsedURL.Scheme == "http" {
		parsedURL.Scheme = "https"
	} else if parsedURL.Scheme != "https" {
		return "", "", fmt.Errorf("invalid URL scheme: %s", parsedURL.Scheme)
	}

	// Check if hostname is www.youtube.com, youtube.com or m.youtube.com
	// Convert hostname to www.youtube.com if valid
	switch parsedURL.Host {
	case "youtube.com", "m.youtube.com":
		parsedURL.Host = "www.youtube.com"
	case "www.youtube.com":
		// Already correct format
	default:
		return "", "", fmt.Errorf("invalid URL host: %s", parsedURL.Host)
	}

	// 检查路径是否为 /watch
	if parsedURL.Path != "/watch" {
		return "", "", fmt.Errorf("invalid URL path: %s", parsedURL.Path)
	}

	// 检查是否包含 v 参数
	queryParams := parsedURL.Query()
	videoID := queryParams.Get("v")
	if videoID == "" {
		return "", "", fmt.Errorf("missing video ID in URL")
	}

	return parsedURL.String(), videoID, nil
}

// GetVideoInfo 获取视频信息
func (s *Service) GetVideoInfo(url string) (*VideoInfo, error) {
	s.logger.Info("Getting video info", zap.String("url", url))

	// 构建命令
	cmd := exec.Command(
		s.config.YtdlpPath,
		"--dump-json",
		"--no-playlist",
		url,
	)

	// 执行命令并获取输出
	output, err := cmd.Output()
	if err != nil {
		s.logger.Error("Failed to get video info", zap.Error(err))
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	// 解析 JSON 输出
	var rawInfo map[string]interface{}
	if err := json.Unmarshal(output, &rawInfo); err != nil {
		s.logger.Error("Failed to parse video info", zap.Error(err))
		return nil, fmt.Errorf("failed to parse video info: %w", err)
	}

	// 提取所需信息
	info := &VideoInfo{
		ID:           getStringValue(rawInfo, "id"),
		WebpageURL:   getStringValue(rawInfo, "webpage_url"),
		Title:        getStringValue(rawInfo, "title"),
		Description:  getStringValue(rawInfo, "description"),
		Duration:     getIntValue(rawInfo, "duration"),
		Thumbnail:    getStringValue(rawInfo, "thumbnail"),
		ViewCount:    getInt64Value(rawInfo, "view_count"),
		CommentCount: getInt64Value(rawInfo, "comment_count"),
		LikeCount:    getInt64Value(rawInfo, "like_count"),
		UploadDate:   getStringValue(rawInfo, "upload_date"),
		Uploader:     getStringValue(rawInfo, "uploader"),
	}

	// 处理分类信息
	if categories, ok := rawInfo["categories"].([]interface{}); ok && len(categories) > 0 {
		if category, ok := categories[0].(string); ok {
			info.Categories = append(info.Categories, category)
		}
	}

	// 提取标签信息
	info.Tags = getStringArrayValue(rawInfo, "tags")

	// 提取频道信息
	info.ChannelName = getStringValue(rawInfo, "channel")
	info.ChannelURL = getStringValue(rawInfo, "channel_url")

	// 尝试获取频道订阅数
	info.ChannelFollowerCount = getInt64Value(rawInfo, "channel_follower_count")
	// 如果没有 channel_follower_count 字段，尝试 subscriber_count 字段
	if info.ChannelFollowerCount == 0 {
		info.ChannelFollowerCount = getInt64Value(rawInfo, "subscriber_count")
	}

	// 提取格式信息
	audioFormats := make(map[string][]AudioFormat)
	videoFormats := make(map[string][]VideoFormat)

	if formatsRaw, ok := rawInfo["formats"].([]interface{}); ok {
		for _, formatRaw := range formatsRaw {
			if formatMap, ok := formatRaw.(map[string]interface{}); ok {
				formatID := getStringValue(formatMap, "format_id")
				ext := getStringValue(formatMap, "ext")
				filesize := getInt64Value(formatMap, "filesize")
				vcodec := getStringValue(formatMap, "vcodec")
				acodec := getStringValue(formatMap, "acodec")

				// 跳过 storyboard 格式
				if strings.Contains(getStringValue(formatMap, "format_note"), "storyboard") {
					continue
				}

				// 纯音频格式 (vcodec == "none" && acodec != "none")
				if vcodec == "none" && acodec != "none" && acodec != "" {
					audioFormat := AudioFormat{
						FormatID: formatID,
						Ext:      ext,
						Filesize: filesize,
					}
					audioFormats[ext] = append(audioFormats[ext], audioFormat)
				}

				// 纯视频格式 (acodec == "none" && vcodec != "none")
				if acodec == "none" && vcodec != "none" && vcodec != "" {
					resolution := getResolution(formatMap)
					videoFormat := VideoFormat{
						FormatID:   formatID,
						Ext:        ext,
						Resolution: resolution,
						Filesize:   filesize,
					}
					videoFormats[ext] = append(videoFormats[ext], videoFormat)
				}
			}
		}
	}

	// 构建音频格式组列表
	for ext, formats := range audioFormats {
		if len(formats) > 0 {
			audioGroup := VideoFormatGroup{
				Ext:     ext,
				Formats: convertAudioToVideoFormats(formats),
			}
			info.Audio = append(info.Audio, audioGroup)
		}
	}

	// 构建视频格式组列表
	for ext, formats := range videoFormats {
		if len(formats) > 0 {
			videoGroup := VideoFormatGroup{
				Ext:     ext,
				Formats: formats,
			}
			info.Video = append(info.Video, videoGroup)
		}
	}

	return info, nil
}

// StartDownload 开始下载视频
func (s *Service) StartDownload(url, formatID string) (*DownloadTask, error) {
	s.logger.Info("Starting download", zap.String("url", url), zap.String("format", formatID))

	outputDir := filepath.Join(s.config.DownloadDir, url)
	filename := url

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		s.logger.Error("Failed to create output directory", zap.Error(err))
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// 生成任务 ID
	taskID := uuid.New().String()

	// 创建上下文，用于取消下载
	ctx, cancel := context.WithCancel(context.Background())

	// 创建下载任务
	task := &DownloadTask{
		ID:        taskID,
		URL:       url,
		Format:    formatID,
		OutputDir: outputDir,
		Filename:  filename,
		State:     "pending",
		Progress:  0,
		Speed:     "0 B/s",
		ETA:       "unknown",
		StartTime: time.Now(),
		Ctx:       ctx,
		Cancel:    cancel,
	}

	// 添加到下载列表
	s.mutex.Lock()
	s.downloads[taskID] = task
	s.mutex.Unlock()

	// 在后台启动下载
	go s.runDownload(task)

	return task, nil
}

// GetDownloadStatus 获取下载状态
func (s *Service) GetDownloadStatus(taskID string) (*DownloadTask, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	task, ok := s.downloads[taskID]
	if !ok {
		return nil, errors.New("download task not found")
	}

	return task, nil
}

// CancelDownload 取消下载
func (s *Service) CancelDownload(taskID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	task, ok := s.downloads[taskID]
	if !ok {
		return errors.New("download task not found")
	}

	// 取消下载
	if task.State == "downloading" && task.Cancel != nil {
		task.Cancel()
		task.State = "failed"
		task.Error = "Download cancelled by user"
		task.EndTime = time.Now()
	}

	return nil
}

// runDownload 执行下载任务
func (s *Service) runDownload(task *DownloadTask) {
	s.logger.Info("Running download task", zap.String("task_id", task.ID))

	// 更新任务状态
	task.State = "downloading"

	// 构建输出文件名
	outputTemplate := filepath.Join(task.OutputDir, "%(title)s.%(ext)s")
	if task.Filename != "" {
		outputTemplate = filepath.Join(task.OutputDir, task.Filename+".%(ext)s")
	}

	// 构建命令
	cmdArgs := []string{
		"--newline",
		"--progress",
		"--no-playlist",
		"--restrict-filenames",
	}

	// 添加格式
	if task.Format != "" {
		cmdArgs = append(cmdArgs, "-f", task.Format)
	}

	// 添加输出模板
	cmdArgs = append(cmdArgs, "-o", outputTemplate)

	// 添加 URL
	cmdArgs = append(cmdArgs, task.URL)

	// 创建命令
	cmd := exec.CommandContext(task.Ctx, s.config.YtdlpPath, cmdArgs...)
	task.Cmd = cmd

	// 获取标准输出和错误
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		s.logger.Error("Failed to get stdout pipe", zap.Error(err))
		task.State = "failed"
		task.Error = fmt.Sprintf("Failed to start download: %v", err)
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		s.logger.Error("Failed to get stderr pipe", zap.Error(err))
		task.State = "failed"
		task.Error = fmt.Sprintf("Failed to start download: %v", err)
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		s.logger.Error("Failed to start download", zap.Error(err))
		task.State = "failed"
		task.Error = fmt.Sprintf("Failed to start download: %v", err)
		return
	}

	// 处理输出
	go s.processOutput(task, stdoutPipe, stderrPipe)

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		// 检查是否是因为取消而失败
		if task.Ctx.Err() == context.Canceled {
			s.logger.Info("Download cancelled", zap.String("task_id", task.ID))
			task.State = "failed"
			task.Error = "Download cancelled by user"
		} else {
			s.logger.Error("Download failed", zap.Error(err))
			task.State = "failed"
			task.Error = fmt.Sprintf("Download failed: %v", err)
		}
	} else if task.State != "failed" {
		// 下载成功
		s.logger.Info("Download completed", zap.String("task_id", task.ID))
		task.State = "completed"
		task.Progress = 100
		task.Speed = "0 B/s"
		task.ETA = "00:00"
	}

	task.EndTime = time.Now()
}

// processOutput 处理命令输出
func (s *Service) processOutput(task *DownloadTask, stdout, stderr io.ReadCloser) {
	// 处理标准输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			s.parseProgressLine(task, line)
		}
	}()

	// 处理标准错误
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			s.logger.Debug("yt-dlp stderr", zap.String("line", line))

			// 检查是否包含文件名信息
			if strings.Contains(line, "Destination:") {
				parts := strings.SplitN(line, "Destination: ", 2)
				if len(parts) == 2 {
					task.FilePath = strings.TrimSpace(parts[1])
				}
			}
		}
	}()
}

// parseProgressLine 解析进度行
func (s *Service) parseProgressLine(task *DownloadTask, line string) {
	s.logger.Debug("yt-dlp stdout", zap.String("line", line))

	// 解析进度信息
	if strings.Contains(line, "% of") {
		// 提取进度百分比
		progressRegex := regexp.MustCompile(`(\d+\.\d+)%`)
		matches := progressRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			progress, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				task.Progress = progress
			}
		}

		// 提取下载速度
		speedRegex := regexp.MustCompile(`at\s+([\d\.]+\s*[KMGTP]?i?B/s)`)
		matches = speedRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			task.Speed = matches[1]
		}

		// 提取剩余时间
		etaRegex := regexp.MustCompile(`ETA\s+(\d+:\d+)`)
		matches = etaRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			task.ETA = matches[1]
		}
	}

	// 检查是否包含文件大小信息
	if strings.Contains(line, "Destination:") && strings.Contains(line, "has already been downloaded") {
		parts := strings.SplitN(line, "Destination: ", 2)
		if len(parts) == 2 {
			filePath := strings.Split(parts[1], " has already")[0]
			task.FilePath = strings.TrimSpace(filePath)

			// 获取文件大小
			if info, err := os.Stat(task.FilePath); err == nil {
				task.FileSize = info.Size()
			}
		}
	}

	// 检查是否下载完成
	if strings.Contains(line, "has already been downloaded") || strings.Contains(line, "Merging formats") {
		// 获取文件大小
		if task.FilePath != "" {
			if info, err := os.Stat(task.FilePath); err == nil {
				task.FileSize = info.Size()
			}
		}
	}
}

// 辅助函数

func getStringValue(data map[string]interface{}, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

func getIntValue(data map[string]interface{}, key string) int {
	switch value := data[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	case string:
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return 0
}

func getInt64Value(data map[string]interface{}, key string) int64 {
	switch value := data[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case string:
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func getResolution(data map[string]interface{}) string {
	// 尝试从 resolution 字段获取
	if res, ok := data["resolution"].(string); ok && res != "" {
		return res
	}

	// 尝试从 width 和 height 字段构建
	width := getIntValue(data, "width")
	height := getIntValue(data, "height")
	if width > 0 && height > 0 {
		return fmt.Sprintf("%dx%d", width, height)
	}

	// 尝试从格式描述中提取
	if format, ok := data["format"].(string); ok {
		resRegex := regexp.MustCompile(`(\d+x\d+|\d+p)`)
		matches := resRegex.FindStringSubmatch(format)
		if len(matches) > 0 {
			return matches[0]
		}
	}

	return "unknown"
}

// getStringArrayValue 从数据中提取字符串数组
func getStringArrayValue(data map[string]interface{}, key string) []string {
	var result []string

	if value, ok := data[key].([]interface{}); ok {
		for _, item := range value {
			if strItem, ok := item.(string); ok {
				result = append(result, strItem)
			}
		}
	}

	return result
}

// convertAudioToVideoFormats 将AudioFormat转换为VideoFormat
func convertAudioToVideoFormats(audioFormats []AudioFormat) []VideoFormat {
	var videoFormats []VideoFormat
	for _, af := range audioFormats {
		vf := VideoFormat{
			FormatID:   af.FormatID,
			Ext:        af.Ext,
			Resolution: "audio only",
			Filesize:   af.Filesize,
		}
		videoFormats = append(videoFormats, vf)
	}
	return videoFormats
}
