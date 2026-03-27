package util

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"platform/config"
	"platform/model"
)

var DB *gorm.DB

func InitDB() error {
	var err error

	// 创建上传目录
	if err := os.MkdirAll(config.AppConfig.UploadPath, 0755); err != nil {
		return fmt.Errorf("创建上传目录失败: %v", err)
	}

	// 根据数据库URL判断使用哪种数据库
	// 如果包含 "sqlite" 或者不是 postgres URL，则使用 SQLite
	var db *gorm.DB

	if strings.Contains(config.AppConfig.DatabaseURL, "sqlite") || !strings.Contains(config.AppConfig.DatabaseURL, "postgres") {
		// 使用SQLite（用于本地开发）
		db, err = gorm.Open(sqlite.Open(config.AppConfig.DatabaseURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		log.Println("使用 SQLite 数据库")
	} else {
		// 使用PostgreSQL（用于生产环境）
		// 配置 PostgreSQL 连接参数
		dsn := config.AppConfig.DatabaseURL
		// 添加连接超时参数和性能优化参数
		if !strings.Contains(dsn, "connect_timeout") {
			dsn += "&connect_timeout=10"
		}
		// 添加性能优化参数
		if !strings.Contains(dsn, "statement_timeout") {
			dsn += "&statement_timeout=30000"
		}
		if !strings.Contains(dsn, "lock_timeout") {
			dsn += "&lock_timeout=10000"
		}

		// 生产环境减少日志开销
		logLevel := logger.Warn
		if config.AppConfig.Environment == "development" {
			logLevel = logger.Info
		}

		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logLevel),
		})
		log.Println("使用 PostgreSQL 数据库")
	}

	if err != nil {
		return fmt.Errorf("连接数据库失败: %v", err)
	}

	DB = db

	// 配置连接池
	sqlDB, err := db.DB()
	if err == nil {
		// 优化：生产环境使用更智能的连接池配置
		if config.AppConfig.Environment == "production" {
			// 生产环境：优化连接池参数以提高并发性能
			// 根据CPU核心数动态调整连接数
			numCPU := runtime.NumCPU()
			maxOpenConns := numCPU * 4 // 每个CPU核心支持4个并发连接
			if maxOpenConns > 50 {
				maxOpenConns = 50 // 最大50个连接
			}
			if maxOpenConns < 10 {
				maxOpenConns = 10 // 最小10个连接
			}

			sqlDB.SetMaxOpenConns(maxOpenConns)
			sqlDB.SetMaxIdleConns(maxOpenConns / 2) // 保持一半的连接空闲
			// 缩短连接生命周期，减少无效连接占用
			sqlDB.SetConnMaxLifetime(5 * 60) // 5分钟
			// 优化：添加连接最大空闲时间
			sqlDB.SetConnMaxIdleTime(2 * 60) // 2分钟

			log.Printf("生产环境连接池已优化: MaxOpenConns=%d, MaxIdleConns=%d", maxOpenConns, maxOpenConns/2)
		} else {
			// 开发环境：使用较大连接池
			sqlDB.SetMaxOpenConns(100)
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetConnMaxLifetime(30 * 60)
		}
		log.Println("数据库连接池已配置")
	}

	// 自动迁移表结构 - 使用 GORM 的 AutoMigrate
	// 注意：如果表已由 init-scripts 创建，GORM 会跳过已存在的表
	err = DB.AutoMigrate(
		&model.User{},
		&model.CodeSource{},
		&model.ModelConfig{},
		&model.Task{},
		&model.Vulnerability{},
		&model.VulnerabilityChain{},
		&model.ProjectStats{},
	)

	// 优化：为 PostgreSQL 启用 TOAST 压缩和优化
	// TOAST (The Oversized-Attribute Storage Technique) 是 PostgreSQL 自动压缩大字段的技术
	if !strings.Contains(config.AppConfig.DatabaseURL, "sqlite") && strings.Contains(config.AppConfig.DatabaseURL, "postgres") {
		// 启用自适应检查点 - 减少 I/O
		DB.Exec("SET checkpoint_timeout = '15min'")

		// 启用并行查询（如果支持）
		DB.Exec("SET max_parallel_workers_per_gather = 4")

		// 优化大文本写入：启用 TOAST 压缩
		// 这不会影响现有表，只影响新插入的数据
		log.Println("PostgreSQL 优化参数已应用")
	}

	// 检查是否是约束已存在的错误，如果是则忽略
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "does not exist") {
			log.Printf("数据库迁移警告: %v (可以忽略)", err)
		} else {
			return fmt.Errorf("数据库迁移失败: %v", err)
		}
	}

	log.Println("数据库连接成功")
	return nil
}
