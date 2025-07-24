package utils

import (
	"testing"
)

func TestToHex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: "",
		},
		{
			name:     "简单字符串",
			input:    "hello",
			expected: "68656c6c6f",
		},
		{
			name:     "包含数字的字符串",
			input:    "hello123",
			expected: "68656c6c6f313233",
		},
		{
			name:     "包含特殊字符的字符串",
			input:    "hello@world!",
			expected: "68656c6c6f40776f726c6421",
		},
		{
			name:     "中文字符串",
			input:    "你好",
			expected: "e4bda0e5a5bd",
		},
		{
			name:     "单个字符",
			input:    "a",
			expected: "61",
		},
		{
			name:     "空格字符",
			input:    " ",
			expected: "20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToHex(tt.input)
			if result != tt.expected {
				t.Errorf("ToHex(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFromHex(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:        "空字符串",
			input:       "",
			expected:    "",
			expectError: false,
		},
		{
			name:        "简单十六进制字符串",
			input:       "68656c6c6f",
			expected:    "hello",
			expectError: false,
		},
		{
			name:        "包含数字的十六进制字符串",
			input:       "68656c6c6f313233",
			expected:    "hello123",
			expectError: false,
		},
		{
			name:        "包含特殊字符的十六进制字符串",
			input:       "68656c6c6f40776f726c6421",
			expected:    "hello@world!",
			expectError: false,
		},
		{
			name:        "中文字符的十六进制字符串",
			input:       "e4bda0e5a5bd",
			expected:    "你好",
			expectError: false,
		},
		{
			name:        "单个字符的十六进制",
			input:       "61",
			expected:    "a",
			expectError: false,
		},
		{
			name:        "空格字符的十六进制",
			input:       "20",
			expected:    " ",
			expectError: false,
		},
		{
			name:        "大写十六进制字符串",
			input:       "48454C4C4F",
			expected:    "HELLO",
			expectError: false,
		},
		{
			name:        "混合大小写十六进制字符串",
			input:       "48656C6c6F",
			expected:    "Hello",
			expectError: false,
		},
		{
			name:        "无效的十六进制字符串 - 包含非法字符",
			input:       "68656c6c6g",
			expected:    "",
			expectError: true,
		},
		{
			name:        "无效的十六进制字符串 - 奇数长度",
			input:       "68656c6c6",
			expected:    "",
			expectError: true,
		},
		{
			name:        "无效的十六进制字符串 - 包含空格",
			input:       "68 65 6c 6c 6f",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FromHex(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("FromHex(%q) expected error, but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("FromHex(%q) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("FromHex(%q) = %q, expected %q", tt.input, result, tt.expected)
				}
			}
		})
	}
}

// TestToHexUpperAndFromHexRoundTrip 测试往返转换
func TestToHexUpperAndFromHexRoundTrip(t *testing.T) {
	tests := []string{
		"",
		"hello",
		"hello world",
		"Hello World!",
		"123456789",
		"!@#$%^&*()",
		"你好世界",
		"🚀🌟💻",
		"\n\t\r",
	}

	for _, original := range tests {
		t.Run("roundtrip_"+original, func(t *testing.T) {
			// 转换为十六进制
			hexStr := ToHex(original)

			// 再转换回原始字符串
			result, err := FromHex(hexStr)
			if err != nil {
				t.Errorf("FromHex failed for hex string %q: %v", hexStr, err)
			}

			if result != original {
				t.Errorf("Round trip failed: original=%q, hex=%q, result=%q", original, hexStr, result)
			}
		})
	}
}

// BenchmarkToHex 性能测试
func BenchmarkToHex(b *testing.B) {
	testStr := "hello world this is a test string for benchmarking"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToHex(testStr)
	}
}

// BenchmarkFromHex 性能测试
func BenchmarkFromHex(b *testing.B) {
	hexStr := "68656c6c6f20776f726c642074686973206973206120746573742073747269696e6720666f722062656e63686d61726b696e67"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FromHex(hexStr)
	}
}
