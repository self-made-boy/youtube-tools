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

// TestParseVideoFormatID 测试ParseVideoFormatID方法
func TestParseVideoFormatID(t *testing.T) {
	// 创建测试服务实例
	cfg := &config.Config{}
	logger := zap.NewNop()
	service := New(cfg, logger)

	tests := []struct {
		name               string
		formatID           string
		expectedExt        string
		expectedResolution string
		expectedVFormat    string
		expectedAFormat    string
		expectedError      bool
	}{
		{
			name:               "valid video format ID with audio",
			formatID:           "v__mp4__1920x1080__137+140",
			expectedExt:        "mp4",
			expectedResolution: "1920x1080",
			expectedVFormat:    "137",
			expectedAFormat:    "140",
			expectedError:      false,
		},
		{
			name:               "valid video format ID without audio",
			formatID:           "v__webm__1280x720__136",
			expectedExt:        "webm",
			expectedResolution: "1280x720",
			expectedVFormat:    "136",
			expectedAFormat:    "",
			expectedError:      false,
		},
		{
			name:          "invalid format ID - wrong prefix",
			formatID:      "a__mp4__1920x1080__137+140",
			expectedError: true,
		},
		{
			name:          "invalid format ID - wrong parts count",
			formatID:      "v__mp4__1920x1080",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, resolution, vFormatID, aFormatID, err := service.ParseVideoFormatID(tt.formatID)
			if tt.expectedError {
				if err == nil {
					t.Errorf("ParseVideoFormatID() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseVideoFormatID() unexpected error: %v", err)
				return
			}
			if ext != tt.expectedExt {
				t.Errorf("ParseVideoFormatID() ext = %v, want %v", ext, tt.expectedExt)
			}
			if resolution != tt.expectedResolution {
				t.Errorf("ParseVideoFormatID() resolution = %v, want %v", resolution, tt.expectedResolution)
			}
			if vFormatID != tt.expectedVFormat {
				t.Errorf("ParseVideoFormatID() vFormatID = %v, want %v", vFormatID, tt.expectedVFormat)
			}
			if aFormatID != tt.expectedAFormat {
				t.Errorf("ParseVideoFormatID() aFormatID = %v, want %v", aFormatID, tt.expectedAFormat)
			}
		})
	}
}

// TestParseAudioFormatID 测试ParseAudioFormatID方法
func TestParseAudioFormatID(t *testing.T) {
	// 创建测试服务实例
	cfg := &config.Config{}
	logger := zap.NewNop()
	service := New(cfg, logger)

	tests := []struct {
		name           string
		formatID       string
		expectedExt    string
		expectedAsr    int64
		expectedFormat string
		expectedError  bool
	}{
		{
			name:           "valid audio format ID",
			formatID:       "a__m4a__44100__140",
			expectedExt:    "m4a",
			expectedAsr:    44100,
			expectedFormat: "140",
			expectedError:  false,
		},
		{
			name:           "valid audio format ID with webm",
			formatID:       "a__webm__48000__251",
			expectedExt:    "webm",
			expectedAsr:    48000,
			expectedFormat: "251",
			expectedError:  false,
		},
		{
			name:          "invalid format ID - wrong prefix",
			formatID:      "v__m4a__44100__140",
			expectedError: true,
		},
		{
			name:          "invalid format ID - wrong parts count",
			formatID:      "a__m4a__44100",
			expectedError: true,
		},
		{
			name:          "invalid format ID - invalid asr",
			formatID:      "a__m4a__invalid__140",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext, asr, formatID, err := service.ParseAudioFormatID(tt.formatID)
			if tt.expectedError {
				if err == nil {
					t.Errorf("ParseAudioFormatID() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseAudioFormatID() unexpected error: %v", err)
				return
			}
			if ext != tt.expectedExt {
				t.Errorf("ParseAudioFormatID() ext = %v, want %v", ext, tt.expectedExt)
			}
			if asr != tt.expectedAsr {
				t.Errorf("ParseAudioFormatID() asr = %v, want %v", asr, tt.expectedAsr)
			}
			if formatID != tt.expectedFormat {
				t.Errorf("ParseAudioFormatID() formatID = %v, want %v", formatID, tt.expectedFormat)
			}
		})
	}
}
