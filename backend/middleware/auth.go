package middleware

import (
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"platform/config"
	"platform/model"
	"platform/util"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT声明（使用util包中的定义）
type Claims = util.Claims

// sanitizeErrorMessage 对错误消息进行脱敏处理
func sanitizeErrorMessage(message string) string {
	// 替换可能包含敏感信息的模式
	sensitivePatterns := map[string]string{
		// 密码相关
		"password": "***",
		"pwd":      "***",
		// JWT/Token相关
		"token":  "***",
		"bearer": "***",
		// 数据库连接信息
		"password='[^']*'": "password='***'",
		"pwd='[^']*'":      "pwd='***'",
		// 文件路径中的用户名
		"/home/[^/]+":             "/home/***",
		"C:\\\\Users\\\\[^\\\\]+": "C:\\\\Users\\\\***",
		// API密钥
		"api[_-]?key": "api_key=***",
		"secret":      "***",
	}

	result := message
	for pattern, replacement := range sensitivePatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		result = re.ReplaceAllString(result, replacement)
	}

	// 限制错误消息长度，防止日志注入
	if len(result) > 500 {
		result = result[:500] + "..."
	}

	return result
}

// RequestLogger 请求日志中间件（安全版本，避免记录敏感信息）
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		clientIP := c.ClientIP()

		// 避免记录敏感路径
		sensitivePaths := []string{
			"/api/auth/login",
			"/api/auth/register",
			"/api/auth/change-password",
			"/api/auth/refresh",
		}

		// 检查是否为敏感路径
		shouldLog := true
		for _, sensitivePath := range sensitivePaths {
			if strings.HasPrefix(path, sensitivePath) {
				shouldLog = false
				break
			}
		}

		// 只记录非敏感路径的请求
		if shouldLog {
			// 结构化日志，避免记录敏感参数
			log.Printf("[请求] %s %s from %s", method, path, clientIP)
		}

		// 捕获响应错误进行脱敏处理
		var errMsg string
		c.Next()

		// 检查是否有错误发生
		if len(c.Errors) > 0 {
			errMsg = c.Errors.Last().Error()
			// 对错误消息进行脱敏处理
			errMsg = sanitizeErrorMessage(errMsg)
		}

		duration := time.Since(startTime)
		statusCode := c.Writer.Status()

		// 只记录非敏感路径的响应
		if shouldLog {
			// 避免记录敏感响应内容，对错误消息进行脱敏
			if errMsg != "" {
				log.Printf("[响应] %s %s - %d - %v - 错误: %s", method, path, statusCode, duration, errMsg)
			} else {
				log.Printf("[响应] %s %s - %d - %v", method, path, statusCode, duration)
			}
		}
	}
}

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证认证头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证信息"})
			c.Abort()
			return
		}

		// 提取Bearer token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证格式，请使用Bearer Token"})
			c.Abort()
			return
		}

		// 解析token
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// 验证签名算法
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "认证令牌解析失败: " + err.Error()})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(*Claims); ok && token.Valid {
			// 验证token是否过期
			if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "认证令牌已过期"})
				c.Abort()
				return
			}

			// 存储用户信息到context
			c.Set("userID", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证令牌"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserID 从context获取用户ID
func GetUserID(c *gin.Context) uint {
	userID, exists := c.Get("userID")
	if !exists {
		return 0
	}

	// 类型断言处理各种数字类型
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

// GetUsername 获取用户名
func GetUsername(c *gin.Context) string {
	username, exists := c.Get("username")
	if !exists {
		return ""
	}
	if s, ok := username.(string); ok {
		return s
	}
	return ""
}

// GetUserRole 获取用户角色
func GetUserRole(c *gin.Context) string {
	role, exists := c.Get("role")
	if !exists {
		return ""
	}
	if s, ok := role.(string); ok {
		return s
	}
	return ""
}

// GetUserFromContext 从context获取用户信息
func GetUserFromContext(c *gin.Context) (*model.User, error) {
	userID := GetUserID(c)
	if userID == 0 {
		return nil, util.ErrUnauthorized
	}

	var user model.User
	if err := util.DB.First(&user, userID).Error; err != nil {
		return nil, util.ErrUserNotFound
	}

	return &user, nil
}

// RequireRole 角色权限检查中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := GetUserRole(c)

		for _, role := range roles {
			if userRole == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		c.Abort()
	}
}
