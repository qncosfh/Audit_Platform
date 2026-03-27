package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	JWTExpireHour int // JWT过期小时数
	UploadPath    string
	MaxUploadSize int64
	Environment   string // "development" or "production"
}

var AppConfig Config

// generateSecureSecret 生成安全的随机密钥
func generateSecureSecret() string {
	bytes := make([]byte, 32) // 256位
	if _, err := rand.Read(bytes); err != nil {
		// 如果生成失败，使用时间戳作为后备
		return fmt.Sprintf("dev-secret-%d", os.Getpid())
	}
	return hex.EncodeToString(bytes)
}

func LoadConfig() error {
	// 加载环境变量（优先加载当前目录的.env，如果不存在则尝试上级目录）
	_ = godotenv.Load()
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		// 当前目录没有.env，尝试从上级目录加载
		_ = godotenv.Load("../.env")
	}

	// 判断环境
	env := getEnv("ENVIRONMENT", "development")

	// 根据环境设置默认值
	var databaseURL string
	if env == "production" {
		databaseURL = getEnv("DATABASE_URL", "")
		if databaseURL == "" {
			return fmt.Errorf("生产环境必须设置DATABASE_URL环境变量")
		}
	} else {
		databaseURL = getEnv("DATABASE_URL", "./platform.db")
	}

	// JWT密钥处理
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		if env == "production" {
			return fmt.Errorf("生产环境必须设置JWT_SECRET环境变量")
		}
		// 开发环境生成临时密钥（不打印实际密钥）
		jwtSecret = generateSecureSecret()
		fmt.Printf("[开发环境] 已生成临时JWT密钥（开发环境使用）\n")
	}

	// 确保上传目录存在
	uploadPath := getEnv("UPLOAD_PATH", "./sandbox")
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return fmt.Errorf("创建上传目录失败: %v", err)
	}

	// JWT过期时间配置
	jwtExpireHour := getEnvAsInt("JWT_EXPIRE_HOUR", 24) // 默认24小时
	if env == "production" {
		jwtExpireHour = getEnvAsInt("JWT_EXPIRE_HOUR", 2) // 生产环境默认2小时
	}

	AppConfig = Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   databaseURL,
		JWTSecret:     jwtSecret,
		JWTExpireHour: jwtExpireHour,
		UploadPath:    uploadPath,
		MaxUploadSize: getEnvAsInt64("MAX_UPLOAD_SIZE", 100*1024*1024), // 默认100MB，更安全的默认值
		Environment:   env,
	}

	// 验证配置
	if err := validateConfig(); err != nil {
		return err
	}

	return nil
}

func validateConfig() error {
	// 验证端口
	if AppConfig.Port == "" {
		return fmt.Errorf("端口未配置")
	}

	// 验证数据库URL
	if AppConfig.DatabaseURL == "" {
		return fmt.Errorf("数据库URL未配置")
	}

	// 验证JWT密钥强度（生产环境）
	if AppConfig.Environment == "production" {
		if len(AppConfig.JWTSecret) < 32 {
			return fmt.Errorf("生产环境JWT密钥长度必须至少为32字符")
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		var result int64
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		var result int
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return defaultValue
}

// GetUploadPath 返回绝对上传路径
func GetUploadPath() string {
	if filepath.IsAbs(AppConfig.UploadPath) {
		return AppConfig.UploadPath
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, AppConfig.UploadPath)
}
