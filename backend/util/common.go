package util

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"platform/config"
)

// Claims JWT声明（供内部使用）
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// 错误定义
var (
	ErrUserNotFound    = errors.New("用户不存在")
	ErrInvalidPassword = errors.New("密码错误")
	ErrUnauthorized    = errors.New("未授权")
	ErrForbidden       = errors.New("权限不足")
)

// JWTBlacklist 令牌黑名单（内存实现，生产环境建议使用Redis）
type JWTBlacklist struct {
	tokens  map[string]time.Time
	mu      sync.RWMutex
	maxAge  time.Duration // 黑名单中令牌的最大保留时间
	maxSize int           // 最大容量限制，防止内存泄漏
}

var (
	// 全局黑名单实例
	globalBlacklist = &JWTBlacklist{
		tokens:  make(map[string]time.Time),
		maxAge:  24 * time.Hour, // 黑名单保留24小时
		maxSize: 10000,          // 最大容量限制
	}
)

// NewJWTBlacklist 创建新的令牌黑名单
func NewJWTBlacklist(maxAge time.Duration) *JWTBlacklist {
	bl := &JWTBlacklist{
		tokens:  make(map[string]time.Time),
		maxAge:  maxAge,
		maxSize: 10000, // 默认最大容量
	}

	// 定期清理过期的黑名单条目
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			bl.cleanup()
		}
	}()

	return bl
}

// Add 将令牌添加到黑名单
func (bl *JWTBlacklist) Add(tokenString string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	// 检查容量限制
	if len(bl.tokens) >= bl.maxSize {
		// 清理最旧的条目
		bl.cleanupExcess()
	}

	bl.tokens[tokenString] = time.Now()
}

// IsBlacklisted 检查令牌是否在黑名单中
func (bl *JWTBlacklist) IsBlacklisted(tokenString string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	_, exists := bl.tokens[tokenString]
	return exists
}

// cleanup 清理过期的黑名单条目
func (bl *JWTBlacklist) cleanup() {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	now := time.Now()
	for token, addedAt := range bl.tokens {
		if now.Sub(addedAt) > bl.maxAge {
			delete(bl.tokens, token)
		}
	}
}

// cleanupExcess 清理超出容量的最旧条目
func (bl *JWTBlacklist) cleanupExcess() {
	var oldestToken string
	var oldestTime time.Time

	// 找出最旧的条目
	for token, addedAt := range bl.tokens {
		if oldestToken == "" || addedAt.Before(oldestTime) {
			oldestToken = token
			oldestTime = addedAt
		}
	}

	if oldestToken != "" {
		delete(bl.tokens, oldestToken)
	}
}

// AddToBlacklist 将令牌添加到全局黑名单
func AddToBlacklist(tokenString string) {
	globalBlacklist.Add(tokenString)
}

// IsTokenBlacklisted 检查令牌是否在全局黑名单中
func IsTokenBlacklisted(tokenString string) bool {
	return globalBlacklist.IsBlacklisted(tokenString)
}

// HashPassword 对密码进行哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash 验证密码
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT 生成JWT令牌 - 增强版本
func GenerateJWT(userID uint) (string, error) {
	return GenerateJWTWithClaims(userID, "", "user")
}

// GenerateJWTWithClaims 生成带有自定义声明的JWT令牌
func GenerateJWTWithClaims(userID uint, username, role string) (string, error) {
	// 使用配置中的过期时间
	expirationTime := time.Duration(config.AppConfig.JWTExpireHour) * time.Hour

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expirationTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "platform-audit-system",
			// 添加唯一ID用于令牌撤销
			ID: fmt.Sprintf("%d-%d", userID, time.Now().UnixNano()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

// ValidateTokenWithBlacklist 验证令牌并检查黑名单
func ValidateTokenWithBlacklist(tokenString string) (*Claims, error) {
	// 检查是否在黑名单中
	if IsTokenBlacklisted(tokenString) {
		return nil, errors.New("令牌已失效")
	}

	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.AppConfig.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// 验证令牌是否过期
		if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
			return nil, errors.New("令牌已过期")
		}
		return claims, nil
	}

	return nil, errors.New("无效的令牌")
}

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(data interface{}) *Response {
	return &Response{
		Code:    200,
		Message: "操作成功",
		Data:    data,
	}
}

// Error 错误响应
func Error(code int, message string) *Response {
	return &Response{
		Code:    code,
		Message: message,
	}
}

// HandleError 处理错误并返回响应
func HandleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		c.JSON(http.StatusNotFound, Error(404, "用户不存在"))
	case errors.Is(err, ErrInvalidPassword):
		c.JSON(http.StatusUnauthorized, Error(401, "密码错误"))
	case errors.Is(err, ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, Error(401, "未授权"))
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, Error(403, "权限不足"))
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, Error(404, "记录不存在"))
	default:
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
	}
}

// ValidateFileSize 验证文件大小
func ValidateFileSize(fileSize int64, maxSize int64) error {
	if fileSize > maxSize {
		return fmt.Errorf("文件大小超过限制，最大允许 %d 字节", maxSize)
	}
	return nil
}

// ValidateFileType 验证文件类型
func ValidateFileType(filename string, allowedTypes []string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, allowedType := range allowedTypes {
		if ext == allowedType {
			return nil
		}
	}
	return fmt.Errorf("不支持的文件类型: %s", ext)
}

// StringToInt 字符串转整数
func StringToInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

// StringToUint 字符串转无符号整数
func StringToUint(s string) uint {
	var result uint
	fmt.Sscanf(s, "%d", &result)
	return result
}

// DeleteFile 删除文件
func DeleteFile(filePath string) error {
	return os.Remove(filePath)
}

// GetUserID 从gin.Context中获取用户ID
func GetUserID(c *gin.Context) uint {
	userID, exists := c.Get("userID")
	if !exists {
		return 0
	}
	// 类型断言处理浮点数情况（JWT中数字可能作为float64存储）
	switch v := userID.(type) {
	case uint:
		return v
	case float64:
		return uint(v)
	case int:
		return uint(v)
	case int64:
		return uint(v)
	default:
		return 0
	}
}

// GetUploadPath 获取上传目录的绝对路径
func GetUploadPath() string {
	return config.GetUploadPath()
}

// EnsureUploadDir 确保上传目录存在
func EnsureUploadDir() error {
	uploadPath := GetUploadPath()
	return os.MkdirAll(uploadPath, 0755)
}
