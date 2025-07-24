package utils

import (
	"encoding/hex"
)

// 将字符串转为 16进制
func ToHex(s string) string {
	return hex.EncodeToString([]byte(s))
}

// 将 16进制字符串转为普通字符串
func FromHex(s string) (string, error) {
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
