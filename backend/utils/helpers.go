package utils

import (
	"os"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

///// 环境变量函数 /////

// GetEnv 获取环境变量（兼容config包的getEnv）
func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetEnvAsInt 获取环境变量并转换为int
func GetEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if result, err := strconv.Atoi(value); err == nil {
			return result
		}
	}
	return defaultValue
}

// GetEnvAsInt64 获取环境变量并转换为int64
func GetEnvAsInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if result, err := strconv.ParseInt(value, 10, 64); err == nil {
			return result
		}
	}
	return defaultValue
}

///// Unicode 解码函数 /////

// DecodeUnicodeString 解码 Unicode 转义序列
// 此函数用于处理编码后的Unicode字符串，支持\uXXXX和\UXXXXXXXX格式
func DecodeUnicodeString(s string) string {
	re := regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	result := re.ReplaceAllStringFunc(s, func(match string) string {
		hex := match[2:]
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return match
		}
		return string(rune(code))
	})

	re2 := regexp.MustCompile(`\\U([0-9a-fA-F]{8})`)
	result = re2.ReplaceAllStringFunc(result, func(match string) string {
		hex := match[3:]
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return match
		}
		return string(rune(code))
	})

	if !utf8.ValidString(result) {
		return s
	}

	return result
}

///// 标准库替换函数 /////

// Min 返回两个整数中较小的值
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max 返回两个整数中较大的值
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

///// 用户上下文函数 /////

// GetUserID 从Gin上下文获取用户ID
func GetUserID(c *gin.Context) uint {
	userID, exists := c.Get("userID")
	if !exists {
		return 0
	}
	id, ok := userID.(uint)
	if !ok {
		return 0
	}
	return id
}
