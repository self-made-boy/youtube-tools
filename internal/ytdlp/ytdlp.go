package ytdlp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Uploader   string   `json:"uploader"`
	Duration   int      `json:"duration"`
	UploadDate string   `json:"upload_date"`
	Formats    []Format `json:"formats"`
}

// Format 表示视频格式
type Format struct {
	FormatID   string `json:"format_id"`
	Ext        string `json:"ext"`
	Resolution string `json:"resolution"`
	Filesize   int64  `json:"filesize"`
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
		ID:         getStringValue(rawInfo, "id"),
		Title:      getStringValue(rawInfo, "title"),
		Uploader:   getStringValue(rawInfo, "uploader"),
		Duration:   getIntValue(rawInfo, "duration"),
		UploadDate: getStringValue(rawInfo, "upload_date"),
		Formats:    []Format{},
	}

	// 提取格式信息
	if formatsRaw, ok := rawInfo["formats"].([]interface{}); ok {
		for _, formatRaw := range formatsRaw {
			if formatMap, ok := formatRaw.(map[string]interface{}); ok {
				format := Format{
					FormatID:   getStringValue(formatMap, "format_id"),
					Ext:        getStringValue(formatMap, "ext"),
					Resolution: getResolution(formatMap),
					Filesize:   getInt64Value(formatMap, "filesize"),
				}
				info.Formats = append(info.Formats, format)
			}
		}
	}

	return info, nil
}

// StartDownload 开始下载视频
func (s *Service) StartDownload(url, format, outputDir, filename string) (*DownloadTask, error) {
	s.logger.Info("Starting download", zap.String("url", url), zap.String("format", format))

	// 验证输出目录
	if outputDir == "" {
		outputDir = s.config.DownloadDir
	}

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
		Format:    format,
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
