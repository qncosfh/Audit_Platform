package router

import (
	"log"
	"net/http"
	"os"
	"time"

	"platform/config"
	"platform/handler"
	"platform/middleware"
	"platform/util"
	"platform/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS 配置 - 根据环境配置
	allowOrigins := []string{"*"}
	if env := os.Getenv("ENVIRONMENT"); env == "production" {
		// 生产环境限制来源
		allowOrigins = []string{"*"}
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * 60 * 60,
	}))

	// 安全中间件
	r.Use(gin.Recovery())

	// 请求日志中间件（生产环境可关闭调试日志）
	if env := os.Getenv("ENVIRONMENT"); env != "production" {
		r.Use(gin.Logger())
	}
	r.Use(middleware.RequestLogger())

	// 健康检查 - 增强版
	r.GET("/health", func(c *gin.Context) {
		// 检查数据库连接
		sqlDB, err := util.DB.DB()
		dbStatus := "ok"
		dbError := ""
		if err != nil {
			dbStatus = "error"
			dbError = err.Error()
		} else if err := sqlDB.Ping(); err != nil {
			dbStatus = "error"
			dbError = err.Error()
		}

		// 构建健康状态
		health := gin.H{
			"status":    "ok",
			"version":   "1.0.0",
			"timestamp": time.Now().Unix(),
			"services": gin.H{
				"database": gin.H{
					"status": dbStatus,
					"error":  dbError,
				},
			},
		}

		// 如果数据库有问题，返回 503
		if dbStatus != "ok" {
			c.JSON(503, health)
			return
		}

		c.JSON(200, health)
	})

	// 认证路由
	authHandler := handler.NewAuthHandler()
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.GET("/me", middleware.AuthMiddleware(), authHandler.GetCurrentUser)
		auth.POST("/refresh", middleware.AuthMiddleware(), authHandler.RefreshToken)
		auth.POST("/change-password", middleware.AuthMiddleware(), authHandler.ChangePassword)
	}

	// 需要认证的路由组
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware())

	// 代码源路由
	codeSourceHandler := handler.NewCodeSourceHandler()
	{
		api.POST("/code-sources/upload/zip", codeSourceHandler.UploadZip)
		api.POST("/code-sources/upload/jar", codeSourceHandler.UploadJar)
		api.POST("/code-sources/git", codeSourceHandler.AddGitRepo)
		api.GET("/code-sources", codeSourceHandler.List)
		api.GET("/code-sources/:id", codeSourceHandler.Get)
		api.GET("/code-sources/:id/file", codeSourceHandler.GetFile)
		api.DELETE("/code-sources/:id", codeSourceHandler.Delete)
	}

	// 模型配置路由
	modelHandler := handler.NewModelHandler()
	{
		api.GET("/models", modelHandler.List)
		api.POST("/models", modelHandler.Create)
		api.PUT("/models/:id", modelHandler.Update)
		api.DELETE("/models/:id", modelHandler.Delete)
		api.POST("/models/:id/test", modelHandler.TestModel)
	}

	// 任务路由
	taskHandler := handler.NewTaskHandler()
	{
		api.GET("/tasks", taskHandler.List)
		api.POST("/tasks", taskHandler.Create)
		api.GET("/tasks/:id", taskHandler.Get)
		api.GET("/tasks/:id/detail", taskHandler.GetDetail)
		api.PUT("/tasks/:id", taskHandler.Update)
		api.PUT("/tasks/:id/progress", taskHandler.UpdateProgress)
		api.DELETE("/tasks/:id", taskHandler.Delete)
		api.POST("/tasks/:id/start", taskHandler.Start)
		api.POST("/tasks/:id/stop", taskHandler.Stop)
		api.GET("/tasks/:id/export", taskHandler.ExportReport)
		// 下载完整报告（当报告保存到文件时使用）
		api.GET("/tasks/:id/download-report", taskHandler.DownloadReport)
		// 调用图路由
		api.GET("/tasks/:id/callgraph", taskHandler.GetCallGraph)
		// 节点关系路由 (Callees/Callers)
		api.GET("/tasks/:id/callgraph/relations", taskHandler.GetNodeRelations)
		// 代码片段路由
		api.GET("/tasks/:id/snippet", taskHandler.GetCodeSnippet)
		// 根据漏洞ID获取代码片段
		api.GET("/tasks/:id/vulnerabilities/:vulnId/snippet", taskHandler.GetCodeSnippetByVulnID)
		// 漏洞利用链图（简化版）
		api.GET("/tasks/:id/vuln-graph", taskHandler.GetVulnGraph)
	}

	// 分析路由 - 商业级功能
	analysisHandler := handler.NewAnalysisHandler()
	{
		api.GET("/analysis/:id/vulnerabilities", analysisHandler.GetVulnerabilities)
		api.GET("/analysis/:id/vulnerabilities/:vulnId", analysisHandler.GetVulnerability)
		api.GET("/analysis/:id/chains", analysisHandler.GetExploitChains)
		api.GET("/analysis/:id/stats", analysisHandler.GetProjectStats)
		api.GET("/analysis/:id/crossfile", analysisHandler.GetCrossFileAnalysis)
		api.GET("/analysis/:id/dependency", analysisHandler.GetDependencyAnalysis)
		api.GET("/analysis/:id/vuln-types", analysisHandler.GetVulnerabilityTypes)
		api.GET("/analysis/:id/files", analysisHandler.GetFilesBySeverity)
		api.GET("/analysis/:id/report", analysisHandler.ExportReport)
	}

	// 报告路由
	reportHandler := handler.NewReportHandler()
	{
		api.GET("/reports", reportHandler.ListReports)
		api.GET("/reports/:id", reportHandler.ExportReport)
		api.GET("/reports/:id/download", reportHandler.DownloadReport)
		api.DELETE("/reports/:id", reportHandler.DeleteReport)
	}

	// WebSocket路由
	r.GET("/ws/progress", func(c *gin.Context) {
		conn, err := websocket.Upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.JSON(500, gin.H{"error": "WebSocket升级失败"})
			return
		}

		progressManager.HandleProgressWebSocket(conn)
	})
	return r
}

// 全局进度管理器
var progressManager *websocket.ProgressManager

func init() {
	progressManager = websocket.NewProgressManager()
}

// GetProgressManager 获取全局进度管理器
func GetProgressManager() *websocket.ProgressManager {
	return progressManager
}

func StartServer() {
	r := SetupRouter()

	// 创建自定义 HTTP 服务器，设置更长的超时时间（用于大文件上传）
	// 绑定0.0.0.0供Docker容器访问（安全由前端nginx控制）
	srv := &http.Server{
		Addr:         "0.0.0.0:" + config.AppConfig.Port,
		Handler:      r,
		ReadTimeout:  5 * 60 * time.Second, // 读取超时 5 分钟（用于大文件上传）
		WriteTimeout: 5 * time.Minute,      // 写入超时 5 分钟（用于响应大型响应）
		IdleTimeout:  120 * time.Second,    // 空闲超时 2 分钟
	}

	// 启动服务器
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
