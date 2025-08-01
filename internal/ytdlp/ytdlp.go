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

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/self-made-boy/youtube-tools/internal/config"
	"github.com/self-made-boy/youtube-tools/internal/utils"
)

// Service 提供 yt-dlp 相关操作
type Service struct {
	config    *config.Config
	logger    *zap.Logger
	downloads map[string]*DownloadTask
	mutex     sync.RWMutex
	// group 用于确保同一videoID只执行一次
	group     singleflight.Group
}

// DownloadTask 表示一个下载任务
type DownloadTask struct {
	ID          string             `json:"id"`
	URL         string             `json:"url"`
	Format      string             `json:"format"`
	State       string             `json:"state"` // pending, downloading, completed, failed
	Progress    float64            `json:"progress"`
	Speed       string             `json:"speed"`
	ETA         string             `json:"eta"`
	DownloadUrl string             `json:"download_url,omitempty"`
	Error       string             `json:"error,omitempty"`
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time,omitempty"`
	Cmd         *exec.Cmd          `json:"-"`
	Ctx         context.Context    `json:"-"`
	Cancel      context.CancelFunc `json:"-"`
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
	Audio []AudioFormatGroup `json:"audio"`
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
}

// AudioFormat 表示音频格式
type AudioFormat struct {
	// 音频格式ID
	FormatID string `json:"format_id" example:"140"`
	// 音频文件扩展名
	Ext string `json:"ext" example:"m4a"`

	// 采样率
	Asr int64 `json:"asr" example:"44100"`
}

// New 创建一个新的 yt-dlp 服务
func New(cfg *config.Config, logger *zap.Logger) *Service {
	s := &Service{
		config:    cfg,
		logger:    logger,
		downloads: make(map[string]*DownloadTask),
		mutex:     sync.RWMutex{},
	}

	// 启动清理 goroutine
	go s.startCleanupRoutine()

	return s
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
func (s *Service) getVideoJsonPath(videoID string) string {
	return filepath.Join(s.config.S3Mount, fmt.Sprintf("%s/%s.json", videoID, videoID))
}

// executeYtdlpCommand 执行yt-dlp命令获取视频信息
func (s *Service) executeYtdlpCommand(url string) (string, error) {
	_, videoID, err := s.CheckUrl(url)
	if err != nil {
		return "", err
	}
	
	// 使用singleflight确保同一videoID只执行一次
	result, err, _ := s.group.Do(videoID, func() (interface{}, error) {
		return s.doExecuteYtdlpCommand(url, videoID)
	})
	
	if err != nil {
		return "", err
	}
	
	return result.(string), nil
}

// doExecuteYtdlpCommand 实际执行yt-dlp命令的逻辑
func (s *Service) doExecuteYtdlpCommand(url, videoID string) (string, error) {
	videoJsonPath := s.getVideoJsonPath(videoID)
	
	// 检查文件是否已存在
	if _, statErr := os.Stat(videoJsonPath); statErr == nil {
		// 文件存在，读取内容
		content, readErr := os.ReadFile(videoJsonPath)
		if readErr == nil {
			return string(content), nil
		}
	}

	// 构建命令参数
	cmdArgs := []string{
		"--dump-json",
		"--no-playlist",
	}

	// 添加 cookies 文件
	if s.config.Ytdlp.CookiesPath != "" {
		cmdArgs = append(cmdArgs, "--cookies", s.config.Ytdlp.CookiesPath)
	}

	// 添加代理配置
	if s.config.Ytdlp.Proxy != "" {
		cmdArgs = append(cmdArgs, "--proxy", s.config.Ytdlp.Proxy)
	}

	// 添加 URL
	cmdArgs = append(cmdArgs, url)

	// 构建命令
	cmd := exec.Command(s.config.Ytdlp.Path, cmdArgs...)

	// 记录要执行的命令详情
	s.logger.Info("Executing yt-dlp command for video info",
		zap.String("full_command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))

	// 执行命令并获取输出
	start := time.Now()
	output, err := cmd.Output()
	duration := time.Since(start)

	if err != nil {
		// 记录命令执行失败的详细信息
		if exitError, ok := err.(*exec.ExitError); ok {
			s.logger.Error("yt-dlp command failed",
				zap.Error(err),
				zap.String("stderr", string(exitError.Stderr)),
				zap.Int("exit_code", exitError.ExitCode()),
				zap.Duration("duration", duration),
				zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))
		} else {
			s.logger.Error("Failed to execute yt-dlp command",
				zap.Error(err),
				zap.Duration("duration", duration),
				zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))
		}
		return "", fmt.Errorf("failed to get video info: %w", err)
	}

	// 记录命令执行成功的信息
	s.logger.Info("yt-dlp command executed successfully",
		zap.Duration("duration", duration),
		zap.Int("output_size", len(output)),
		zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))

	// 记录输出内容（仅在debug级别，因为可能很长）
	s.logger.Debug("yt-dlp command output", zap.String("output", string(output)))

	// 将结果写入到 videoJsonPath 中
	writeErr := os.WriteFile(videoJsonPath, output, 0644)
	if writeErr != nil {
		s.logger.Error("Failed to write video info to file", zap.Error(writeErr))
	}
	return string(output), nil
}

// GetVideoInfo 获取视频信息
func (s *Service) GetVideoInfo(url string) (*VideoInfo, error) {
	s.logger.Info("Getting video info", zap.String("url", url))

	// 执行yt-dlp命令获取输出
	outputStr, err := s.executeYtdlpCommand(url)
	if err != nil {
		return nil, err
	}

	output := []byte(outputStr)
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
	info.Categories = getStringArrayValue(rawInfo, "categories")

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
	optimalAudioFormats, optimalVideoFormats := s.extractOptimalFormats(rawInfo)

	maxAsr := int64(0)
	maxAFormatId := ""

	// 构建音频格式组列表
	for _, afe := range s.config.Ytdlp.AudioFormats {
		formats := []AudioFormat{}
		for _, af := range optimalAudioFormats {
			formats = append(formats, AudioFormat{
				FormatID: buildAudioFormatID(afe, af.Asr, af.FormatID),
				Ext:      afe,
				Asr:      af.Asr,
			})
			if af.Asr > maxAsr {
				maxAsr = af.Asr
				maxAFormatId = af.FormatID
			}
		}
		if len(formats) > 0 {
			info.Audio = append(info.Audio, AudioFormatGroup{
				Ext:     afe,
				Formats: formats,
			})
		}
	}

	// 构建视频格式组列表
	for _, vfe := range s.config.Ytdlp.VideoFormats {
		formats := []VideoFormat{}
		for _, vf := range optimalVideoFormats {
			formats = append(formats, VideoFormat{
				FormatID:   buildVideoFormatID(vfe, vf.Resolution, vf.FormatID, maxAFormatId),
				Ext:        vfe,
				Resolution: vf.Resolution,
			})
		}
		if len(formats) > 0 {
			videoGroup := VideoFormatGroup{
				Ext:     vfe,
				Formats: formats,
			}
			info.Video = append(info.Video, videoGroup)
		}
	}

	return info, nil
}

// buildAudioFormatID 构建音频格式 ID，格式为 a__ext__asr__formatID
func buildAudioFormatID(ext string, asr int64, formatID string) string {
	return utils.ToHex(fmt.Sprintf("a__%s__%d__%s", ext, asr, formatID))
}

// buildVideoFormatID 构建视频格式 ID，格式为 v__ext__resolution__vFormatID+aFormatID
func buildVideoFormatID(ext string, resolution string, vFormatID string, aFormatID string) string {
	if aFormatID != "" {
		aFormatID = "+" + aFormatID
	}
	return utils.ToHex(fmt.Sprintf("v__%s__%s__%s%s", ext, resolution, vFormatID, aFormatID))
}

// ParseAudioFormatID 解析音频格式 ID，格式为 a__ext__asr__formatID
func (s *Service) ParseAudioFormatID(formatID string) (ext string, asr int64, originalFormatID string, err error) {
	formatID, err = utils.FromHex(formatID)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid audio format ID: %s", formatID)
	}
	parts := strings.Split(formatID, "__")
	if len(parts) != 4 || parts[0] != "a" {
		return "", 0, "", fmt.Errorf("invalid audio format ID: %s", formatID)
	}

	ext = parts[1]
	asr, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid asr value in format ID: %s", formatID)
	}
	originalFormatID = parts[3]
	return ext, asr, originalFormatID, nil
}

// ParseVideoFormatID 解析视频格式 ID，格式为 v__ext__resolution__vFormatID+aFormatID
func (s *Service) ParseVideoFormatID(formatID string) (ext string, resolution string, vaFormatID string, err error) {
	formatID, err = utils.FromHex(formatID)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid video format ID: %s", formatID)
	}
	parts := strings.Split(formatID, "__")
	if len(parts) != 4 || parts[0] != "v" {
		return "", "", "", fmt.Errorf("invalid video format ID: %s", formatID)
	}

	ext = parts[1]
	resolution = parts[2]

	// 处理 vFormatID+aFormatID 部分
	vaFormatID = parts[3]

	return ext, resolution, vaFormatID, nil
}

// IsVideoFormatID 检查格式 ID 是否为视频格式
func (s *Service) IsVideoFormatID(formatID string) bool {
	formatID, err := utils.FromHex(formatID)
	if err != nil {
		return false
	}
	return strings.HasPrefix(formatID, "v__")
}

func (s *Service) getTaskId(url, formatID string) (string, error) {
	_, videoID, err := s.CheckUrl(url)
	if err != nil {
		return "", err
	}

	task_id := ""
	// 添加格式
	if s.IsVideoFormatID(formatID) {
		ext, resolution, _, _ := s.ParseVideoFormatID(formatID)
		task_id = fmt.Sprintf("%s/video/%s/%s.%s", videoID, resolution, videoID, ext)
	} else {
		ext, asr, _, _ := s.ParseAudioFormatID(formatID)

		task_id = fmt.Sprintf("%s/audio/%d/%s.%s", videoID, asr, videoID, ext)
	}
	return utils.ToHex(task_id), nil

}

// StartDownload 开始下载视频
func (s *Service) StartDownload(url, formatID string) (string, error) {
	s.logger.Info("Starting download", zap.String("url", url), zap.String("format", formatID))

	// 生成任务 ID
	taskID, err := s.getTaskId(url, formatID)
	if err != nil {
		return "", err
	}

	// 使用读锁检查任务是否已存在
	s.mutex.RLock()
	if _, ok := s.downloads[taskID]; ok {
		s.mutex.RUnlock()
		return taskID, nil
	}
	s.mutex.RUnlock()

	// 使用写锁进行双重检查并创建任务
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 双重检查：在获取写锁后再次检查任务是否存在
	// 防止在读锁释放到写锁获取之间有其他goroutine创建了相同的任务
	if _, ok := s.downloads[taskID]; ok {
		return taskID, nil
	}

	// 创建上下文，用于取消下载
	ctx, cancel := context.WithCancel(context.Background())

	// 创建下载任务
	task := &DownloadTask{
		ID:        taskID,
		URL:       url,
		Format:    formatID,
		State:     "pending",
		Progress:  0,
		Speed:     "0 B/s",
		ETA:       "unknown",
		StartTime: time.Now(),
		Ctx:       ctx,
		Cancel:    cancel,
	}

	s.downloads[taskID] = task

	// 在后台启动下载
	go s.runDownload(task)

	return taskID, nil
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

// GetActiveTasksCount 获取当前活跃的下载任务数量
func (s *Service) GetActiveTasksCount() (total, pending, downloading, completed, failed int) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	total = len(s.downloads)
	for _, task := range s.downloads {
		switch task.State {
		case "pending":
			pending++
		case "downloading":
			downloading++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}
	return
}

// runDownload 执行下载任务
//
// 实际执行的 yt-dlp 命令示例:
//
// 基本下载命令:
//
//	yt-dlp --newline --progress --no-playlist --restrict-filenames \
//	       --cookies /app/cookies.txt \
//	       -o "/path/to/output/%(title)s.%(ext)s" \
//	       "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
//
// 指定格式下载:
//
//	yt-dlp --newline --progress --no-playlist --restrict-filenames \
//	       --cookies /app/cookies.txt \
//	       -f "best[height<=720]" \
//	       -o "/path/to/output/%(title)s.%(ext)s" \
//	       "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
//
// 自定义文件名下载:
//
//	yt-dlp --newline --progress --no-playlist --restrict-filenames \
//	       --cookies /app/cookies.txt \
//	       -o "/path/to/output/my_video.%(ext)s" \
//	       "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
//
// 参数说明:
//
//	--newline: 每行输出进度信息
//	--progress: 显示下载进度
//	--no-playlist: 只下载单个视频，不下载播放列表
//	--restrict-filenames: 限制文件名字符，避免特殊字符
//	--cookies: 指定cookies文件路径，用于访问需要登录的内容
//	-f: 指定视频格式和质量
//	-o: 指定输出文件路径和命名模板
func (s *Service) runDownload(task *DownloadTask) {
	s.logger.Info("Running download task", zap.String("task_id", task.ID))

	decodedTaskID, err := utils.FromHex(task.ID)
	if err != nil {
		task.State = "failed"
		task.Error = err.Error()
		task.EndTime = time.Now()
		return
	}
	location := filepath.Join(s.config.S3Mount, decodedTaskID)
	// 判断 location 文件是否存在，如果存在直接返回成功
	if _, statErr := os.Stat(location); statErr == nil {
		task.State = "completed"
		task.EndTime = time.Now()
		task.Progress = 100
		task.Speed = "0 B/s"
		task.ETA = "00:00"
		task.DownloadUrl = s.getDownloadUrl(decodedTaskID)
		return
	}

	// 更新任务状态
	task.State = "downloading"

	// 构建输出文件名
	outputDir := s.config.Ytdlp.DownloadDir
	outputTemplate := outputDir

	// 构建命令
	cmdArgs := []string{
		"--newline",
		"--progress",
		"--no-playlist",
		"--restrict-filenames",
	}

	// 添加 cookies 文件
	if s.config.Ytdlp.CookiesPath != "" {
		cmdArgs = append(cmdArgs, "--cookies", s.config.Ytdlp.CookiesPath)
	}

	// 添加代理配置
	if s.config.Ytdlp.Proxy != "" {
		cmdArgs = append(cmdArgs, "--proxy", s.config.Ytdlp.Proxy)
	}

	_, videoID, _ := s.CheckUrl(task.URL)

	s3Location := ""
	// 添加格式
	if s.IsVideoFormatID(task.Format) {
		ext, resolution, vaFormatID, _ := s.ParseVideoFormatID(task.Format)
		cmdArgs = append(cmdArgs, "-f", vaFormatID)
		cmdArgs = append(cmdArgs, "--merge-output-format", ext)
		cmdArgs = append(cmdArgs, "--postprocessor-args", getFfmpegArgs(ext))

		s3Location = fmt.Sprintf("%s/video/%s/%s.%s", videoID, resolution, videoID, ext)
		outputTemplate = filepath.Join(outputDir, s3Location)
	} else {
		ext, asr, aFormatID, _ := s.ParseAudioFormatID(task.Format)
		cmdArgs = append(cmdArgs, "-f", aFormatID)
		cmdArgs = append(cmdArgs, "-x")
		cmdArgs = append(cmdArgs, "--audio-format", ext)
		cmdArgs = append(cmdArgs, "--postprocessor-args", getFfmpegArgs(ext))
		s3Location = fmt.Sprintf("%s/audio/%d/%s.%s", videoID, asr, videoID, ext)
		outputTemplate = filepath.Join(outputDir, s3Location)
	}

	// 添加输出模板
	cmdArgs = append(cmdArgs, "-o", outputTemplate)

	// 添加 URL
	cmdArgs = append(cmdArgs, task.URL)

	// 创建命令
	cmd := exec.CommandContext(task.Ctx, s.config.Ytdlp.Path, cmdArgs...)
	task.Cmd = cmd

	// 记录要执行的下载命令详情
	s.logger.Info("Executing yt-dlp command for download",
		zap.String("task_id", task.ID),
		zap.String("full_command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))

	// 获取标准输出和错误
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		s.logger.Error("Failed to get stdout pipe",
			zap.String("task_id", task.ID),
			zap.Error(err),
			zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))
		task.State = "failed"
		task.Error = fmt.Sprintf("Failed to start download: %v", err)
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		s.logger.Error("Failed to get stderr pipe",
			zap.String("task_id", task.ID),
			zap.Error(err),
			zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))
		task.State = "failed"
		task.Error = fmt.Sprintf("Failed to start download: %v", err)
		return
	}

	// 启动命令
	commandStartTime := time.Now()
	if err := cmd.Start(); err != nil {
		s.logger.Error("Failed to start download command",
			zap.String("task_id", task.ID),
			zap.Error(err),
			zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))
		task.State = "failed"
		task.Error = fmt.Sprintf("Failed to start download: %v", err)
		return
	}

	s.logger.Info("yt-dlp download command started successfully",
		zap.String("task_id", task.ID),
		zap.Int("process_id", cmd.Process.Pid))

	// 处理输出
	go s.processOutput(task, stdoutPipe, stderrPipe)

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		commandDuration := time.Since(commandStartTime)
		// 检查是否是因为取消而失败
		if task.Ctx.Err() == context.Canceled {
			s.logger.Info("Download cancelled",
				zap.String("task_id", task.ID),
				zap.Duration("command_duration", commandDuration))
			task.State = "failed"
			task.Error = "Download cancelled by user"
		} else {
			// 记录命令执行失败的详细信息
			if exitError, ok := err.(*exec.ExitError); ok {
				s.logger.Error("yt-dlp download command failed",
					zap.String("task_id", task.ID),
					zap.Error(err),
					zap.Int("exit_code", exitError.ExitCode()),
					zap.Duration("command_duration", commandDuration),
					zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))
			} else {
				s.logger.Error("Download command execution failed",
					zap.String("task_id", task.ID),
					zap.Error(err),
					zap.Duration("command_duration", commandDuration),
					zap.String("command", fmt.Sprintf("%s %s", s.config.Ytdlp.Path, strings.Join(cmdArgs, " "))))
			}
			task.State = "failed"
			task.Error = fmt.Sprintf("Download failed: %v", err)
		}
	} else if task.State != "failed" {
		commandDuration := time.Since(commandStartTime)
		// 将文件 outputTemplate mv 到 s3Location
		destinationPath := filepath.Join(s.config.S3Mount, s3Location)
		if err := s.moveFile(outputTemplate, destinationPath); err != nil {
			s.logger.Error("Failed to move file to S3 location",
				zap.String("task_id", task.ID),
				zap.Error(err),
				zap.String("source", outputTemplate),
				zap.String("destination", destinationPath))
			task.State = "failed"
			task.Error = fmt.Sprintf("Failed to move file to S3 location: %v", err)
			return
		}
		// 下载成功
		downloadUrl := s.getDownloadUrl(s3Location)
		s.logger.Info("Download completed successfully",
			zap.String("task_id", task.ID),
			zap.Duration("command_duration", commandDuration),
			zap.String("download_url", downloadUrl))
		task.State = "completed"
		task.Progress = 100
		task.Speed = "0 B/s"
		task.ETA = "00:00"
		task.DownloadUrl = downloadUrl
	}

	task.EndTime = time.Now()
}

func getFfmpegArgs(ext string) string {
	switch ext {
	case "mp4":
		return "ffmpeg:-c:v libx264 -c:a aac"
	case "webm":
		return "ffmpeg:-c:v libvpx-vp9 -c:a libopus"
	case "avi":
		return "ffmpeg:-c:v libx264 -c:a libmp3lame"
	case "mov":
		return "ffmpeg:-c:v libx264 -c:a aac"
	case "flv":
		return "ffmpeg:-c:v libx264 -c:a aac"
	case "mp3":
		return "ffmpeg:-c:a libmp3lame"
	case "m4a":
		return "ffmpeg:-c:a aac"
	case "aac":
		return "ffmpeg:-c:a aac"
	case "opus":
		return "ffmpeg:-c:a libopus"
	case "flac":
		return "ffmpeg:-c:a flac"
	case "wav":
		return "ffmpeg:-c:a pcm_s16le"
	default:
		return "ffmpeg:-c copy"
	}
}
func (s *Service) getDownloadUrl(s3Location string) string {
	return s.config.S3Prefix + s3Location
}

// moveFile 安全地移动文件，支持跨文件系统操作
func (s *Service) moveFile(src, dst string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// 复制文件内容
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// 确保数据写入磁盘
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// 复制文件权限
	if srcInfo, err := srcFile.Stat(); err == nil {
		if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
			s.logger.Warn("Failed to copy file permissions",
				zap.String("dst", dst),
				zap.Error(err))
		}
	}

	// 删除源文件
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source file: %w", err)
	}

	return nil
}

// processOutput 处理命令输出
func (s *Service) processOutput(task *DownloadTask, stdout, stderr io.ReadCloser) {
	// 处理标准输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			s.logger.Info("yt-dlp download task stdout",
				zap.String("task_id", task.ID),
				zap.String("line", line))
			s.parseProgressLine(task, line)
		}
	}()

	// 处理标准错误
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			s.logger.Info("yt-dlp download task stderr",
				zap.String("task_id", task.ID),
				zap.String("line", line))
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

// extractOptimalFormats 提取音频和视频的最优格式
// 音频按采样率分组，视频按分辨率分组，相同条件下选择最高质量的
func (s *Service) extractOptimalFormats(rawInfo map[string]interface{}) ([]AudioFormat, []VideoFormat) {
	// 用于存储按采样率分组的音频格式原始数据
	audioByAsr := make(map[string]map[string]interface{})
	// 用于存储按分辨率分组的视频格式原始数据
	videoByResolution := make(map[string]map[string]interface{})

	if formatsRaw, ok := rawInfo["formats"].([]interface{}); ok {
		for _, formatRaw := range formatsRaw {
			if formatMap, ok := formatRaw.(map[string]interface{}); ok {
				vcodec := getStringValue(formatMap, "vcodec")
				acodec := getStringValue(formatMap, "acodec")

				// 跳过 storyboard 格式
				if strings.Contains(getStringValue(formatMap, "format_note"), "storyboard") {
					continue
				}

				// 处理纯音频格式 (vcodec == "none" && acodec != "none")
				if vcodec == "none" && acodec != "none" && acodec != "" {
					asr := getInt64Value(formatMap, "asr")
					if asr == 0 {
						// asr字段为0时跳过该格式
						continue
					}

					// 使用asr数值作为key
					asrKey := fmt.Sprintf("%d", asr)
					// 检查是否已存在相同采样率的格式
					if existingMap, exists := audioByAsr[asrKey]; exists {
						// 比较质量，选择更好的格式
						if s.isAudioFormatMapBetter(formatMap, existingMap) {
							audioByAsr[asrKey] = formatMap
						}
					} else {
						audioByAsr[asrKey] = formatMap
					}
				}

				// 处理纯视频格式 (acodec == "none" && vcodec != "none")
				if acodec == "none" && vcodec != "none" && vcodec != "" {
					resolution := getResolution(formatMap)
					if resolution == "" {
						resolution = "unknown"
					}

					// 检查是否已存在相同分辨率的格式
					if existingMap, exists := videoByResolution[resolution]; exists {
						// 比较质量，选择更好的格式
						if s.isVideoFormatMapBetter(formatMap, existingMap) {
							videoByResolution[resolution] = formatMap
						}
					} else {
						videoByResolution[resolution] = formatMap
					}
				}
			}
		}
	}

	// 将map转换为slice，同时转换为目标结构体
	var audioFormats []AudioFormat
	for _, formatMap := range audioByAsr {
		audioFormat := AudioFormat{
			FormatID: getStringValue(formatMap, "format_id"),
			Ext:      getStringValue(formatMap, "ext"),
			Asr:      getInt64Value(formatMap, "asr"),
		}
		audioFormats = append(audioFormats, audioFormat)
	}

	var videoFormats []VideoFormat
	for _, formatMap := range videoByResolution {
		videoFormat := VideoFormat{
			FormatID:   getStringValue(formatMap, "format_id"),
			Ext:        getStringValue(formatMap, "ext"),
			Resolution: getResolution(formatMap),
		}
		videoFormats = append(videoFormats, videoFormat)
	}

	return audioFormats, videoFormats
}

// isAudioFormatMapBetter 比较两个音频格式的质量（基于原始formatMap）
// 返回 true 表示 a 比 b 更好
func (s *Service) isAudioFormatMapBetter(a, b map[string]interface{}) bool {
	// 1. 优先比较比特率（abr字段）
	aAbr := getInt64Value(a, "abr")
	bAbr := getInt64Value(b, "abr")
	if aAbr != bAbr {
		return aAbr > bAbr
	}

	// 2. 比较文件大小（更大通常意味着更高质量）
	aFilesize := getInt64Value(a, "filesize")
	bFilesize := getInt64Value(b, "filesize")
	if aFilesize != bFilesize {
		return aFilesize > bFilesize
	}

	return true
}

// isVideoFormatMapBetter 比较两个视频格式的质量（基于原始formatMap）
// 返回 true 表示 a 比 b 更好
func (s *Service) isVideoFormatMapBetter(a, b map[string]interface{}) bool {
	// 1. 优先比较比特率（vbr字段）
	aVbr := getInt64Value(a, "vbr")
	bVbr := getInt64Value(b, "vbr")
	if aVbr != bVbr {
		return aVbr > bVbr
	}

	// 2. 比较帧率（fps字段）
	aFps := getFloat64Value(a, "fps")
	bFps := getFloat64Value(b, "fps")
	if aFps != bFps {
		return aFps > bFps
	}

	// 3. 比较文件大小（更大通常意味着更高质量）
	aFilesize := getInt64Value(a, "filesize")
	bFilesize := getInt64Value(b, "filesize")
	if aFilesize != bFilesize {
		return aFilesize > bFilesize
	}

	return true
}

// getFloat64Value 从数据中提取float64值
func getFloat64Value(data map[string]interface{}, key string) float64 {
	switch value := data[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return 0
}

// startCleanupRoutine 启动清理例程，定期清理已完成的下载任务
func (s *Service) startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupCompletedTasks()
		}
	}
}

// cleanupCompletedTasks 清理已完成超过10分钟的下载任务
func (s *Service) cleanupCompletedTasks() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	var tasksToDelete []string

	for taskID, task := range s.downloads {
		// 检查任务是否已完成（completed 或 failed）且超过10分钟
		if (task.State == "completed" || task.State == "failed") &&
			!task.EndTime.IsZero() &&
			now.Sub(task.EndTime) > 10*time.Minute {
			tasksToDelete = append(tasksToDelete, taskID)
		}
	}

	// 删除过期的任务
	for _, taskID := range tasksToDelete {
		s.logger.Info("Cleaning up completed download task",
			zap.String("task_id", taskID),
			zap.String("state", s.downloads[taskID].State),
			zap.Duration("age", now.Sub(s.downloads[taskID].EndTime)))
		delete(s.downloads, taskID)
	}

	if len(tasksToDelete) > 0 {
		s.logger.Info("Cleaned up download tasks", zap.Int("count", len(tasksToDelete)))
	}
}
