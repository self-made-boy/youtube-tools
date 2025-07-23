package ytdlp

import (
	"testing"

	"go.uber.org/zap"

	"github.com/self-made-boy/youtube-tools/internal/config"
)

// TestService_CheckUrl 测试CheckUrl方法
func TestService_CheckUrl(t *testing.T) {
	// 创建测试服务实例
	cfg := &config.Config{}
	logger := zap.NewNop() // 使用无操作日志器避免测试输出
	service := New(cfg, logger)

	// 测试有效URL
	validURL := "https://www.youtube.com/watch?v=1234567890"
	normalizedURL, videoID, err := service.CheckUrl(validURL)
	if err != nil {
		t.Errorf("CheckUrl for valid URL returned error: %v", err)
	}
	if normalizedURL != validURL {
		t.Errorf("CheckUrl for valid URL normalized URL, want %s, got %s", validURL, normalizedURL)
	}
	if videoID != "1234567890" {
		t.Errorf("CheckUrl for valid URL video ID, want 1234567890, got %s", videoID)
	}

	// 测试无效URL
	invalidURL := "https://www1.youtube.com/watch?v=invalidID"
	_, _, err = service.CheckUrl(invalidURL)
	if err == nil {
		t.Errorf("CheckUrl for invalid URL did not return error")
	}
}
