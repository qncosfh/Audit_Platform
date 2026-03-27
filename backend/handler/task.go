package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"platform/mcp"
	"platform/model"
	"platform/util"
	"platform/websocket"
)

type TaskHandler struct{}

func NewTaskHandler() *TaskHandler {
	return &TaskHandler{}
}

type TaskRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	CodeSourceID  string `json:"codeSourceId"`
	ModelConfigID string `json:"modelConfigId"`
	Prompt        string `json:"prompt"`
}

func (h *TaskHandler) List(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("pageSize", "10")

	var tasks []model.Task
	var total int64

	util.DB.Model(&model.Task{}).Where("user_id = ?", userID).Count(&total)

	offset := (util.StringToInt(page) - 1) * util.StringToInt(pageSize)
	util.DB.Where("user_id = ?", userID).
		Preload("CodeSource").
		Preload("ModelConfig").
		Order("created_at DESC").
		Limit(util.StringToInt(pageSize)).
		Offset(offset).
		Find(&tasks)

	c.JSON(http.StatusOK, util.Success(gin.H{
		"items":    tasks,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}))
}

func (h *TaskHandler) Create(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	// 检查任务名称是否为空
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, util.Error(400, "任务名称不能为空"))
		return
	}

	// 检查任务名称是否已存在（同名任务必须等删除后才能再次创建）
	var existingTask model.Task
	if err := util.DB.Where("user_id = ? AND name = ? AND deleted_at IS NULL", userID, req.Name).First(&existingTask).Error; err == nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "任务名称已存在，请使用其他名称或等待现有任务删除后重试"))
		return
	}

	codeSourceID := util.StringToUint(req.CodeSourceID)
	modelConfigID := util.StringToUint(req.ModelConfigID)

	var codeSource model.CodeSource
	if err := util.DB.Where("id = ? AND user_id = ?", codeSourceID, userID).First(&codeSource).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "代码源不存在"))
		return
	}

	var modelConfig model.ModelConfig
	if err := util.DB.Where("id = ? AND user_id = ?", modelConfigID, userID).First(&modelConfig).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "模型配置不存在"))
		return
	}

	task := model.Task{
		UserID:        userID,
		Name:          req.Name,
		Description:   req.Description,
		CodeSourceID:  codeSourceID,
		ModelConfigID: modelConfigID,
		Prompt:        req.Prompt,
		Status:        model.TaskStatusPending,
		Progress:      0,
	}

	if err := util.DB.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "创建任务失败"))
		return
	}

	util.DB.Preload("CodeSource").Preload("ModelConfig").First(&task, task.ID)

	c.JSON(http.StatusOK, util.Success(task))
}

func (h *TaskHandler) Get(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).
		Preload("CodeSource").
		Preload("ModelConfig").
		First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	c.JSON(http.StatusOK, util.Success(task))
}

func (h *TaskHandler) Delete(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	taskID := util.StringToUint(id)
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 如果任务正在运行，先停止
	if task.Status == model.TaskStatusRunning {
		taskManager.StopTask(taskID)
	}

	// 清理沙盒目录
	sandboxDir := filepath.Join(".", "sandbox", "audit-sandbox", id)
	os.RemoveAll(sandboxDir)

	if err := util.DB.Delete(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "删除任务失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(nil))
}

func (h *TaskHandler) Update(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	var req TaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	if req.CodeSourceID != "" {
		srcID := util.StringToUint(req.CodeSourceID)
		if srcID > 0 {
			var codeSource model.CodeSource
			if err := util.DB.Where("id = ? AND user_id = ?", srcID, userID).First(&codeSource).Error; err != nil {
				c.JSON(http.StatusNotFound, util.Error(404, "代码源不存在"))
				return
			}
			task.CodeSourceID = srcID
		}
	}

	if req.ModelConfigID != "" {
		mdlID := util.StringToUint(req.ModelConfigID)
		if mdlID > 0 {
			var modelConfig model.ModelConfig
			if err := util.DB.Where("id = ? AND user_id = ?", mdlID, userID).First(&modelConfig).Error; err != nil {
				c.JSON(http.StatusNotFound, util.Error(404, "模型配置不存在"))
				return
			}
			task.ModelConfigID = mdlID
		}
	}

	if req.Name != "" {
		task.Name = req.Name
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.Prompt != "" {
		task.Prompt = req.Prompt
	}

	if err := util.DB.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "更新任务失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(task))
}

func (h *TaskHandler) Start(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	if task.Status != model.TaskStatusPending && task.Status != model.TaskStatusFailed && task.Status != model.TaskStatusPaused {
		c.JSON(http.StatusBadRequest, util.Error(400, "任务无法启动"))
		return
	}

	// 如果任务已经是 running，说明有 goroutine 在运行，需要先停止
	if task.Status == model.TaskStatusRunning {
		// 停止现有任务
		taskManager.StopTask(task.ID)
		task.Status = model.TaskStatusPending
		task.Progress = 0
		task.Result = ""
		util.DB.Save(&task)
	}

	if task.Status == model.TaskStatusFailed || task.Status == model.TaskStatusPaused {
		task.Status = model.TaskStatusPending
		task.Progress = 0
		task.Result = ""
	}

	task.Status = model.TaskStatusRunning
	task.Progress = 0

	if err := util.DB.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "启动任务失败"))
		return
	}

	// 使用已有的 processTask
	go h.processTask(task.ID)

	c.JSON(http.StatusOK, util.Success(task))
}

func (h *TaskHandler) Stop(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	if task.Status != model.TaskStatusRunning {
		c.JSON(http.StatusBadRequest, util.Error(400, "任务无法停止"))
		return
	}

	// 停止正在运行的任务
	taskManager.StopTask(task.ID)

	task.Status = model.TaskStatusPaused

	if err := util.DB.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "暂停任务失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(task))
}

func (h *TaskHandler) ExportReport(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	if task.Status != model.TaskStatusCompleted {
		c.JSON(http.StatusBadRequest, util.Error(400, "任务未完成，无法导出报告"))
		return
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"url":     "",
		"content": task.Result,
	}))
}

func (h *TaskHandler) GetDetail(c *gin.Context) {
	// GetDetail 与 Get 功能相同，返回任务详情
	// 保留此端点以保持 API 兼容性
	h.Get(c)
}

// GetCallGraph 获取任务的调用图数据
func (h *TaskHandler) GetCallGraph(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 确定源代码路径 - 优先使用保存的SourcePath
	var sourcePath string

	// 1. 优先使用数据库中保存的源代码路径
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
	} else {
		// 2. 如果没有保存，尝试从代码源类型构建路径
		var codeSource model.CodeSource
		if err := util.DB.First(&codeSource, task.CodeSourceID).Error; err == nil {
			switch codeSource.Type {
			case "jar", "zip":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
				// 尝试查找解压后的代码目录
				entries, err := os.ReadDir(sourcePath)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() && entry.Name() != "decompiled" {
							subPath := filepath.Join(sourcePath, entry.Name())
							codeFiles, _ := filepath.Glob(filepath.Join(subPath, "**/*.java"))
							if len(codeFiles) > 0 {
								sourcePath = subPath
								break
							}
						}
					}
				}
			case "git":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			default:
				c.JSON(http.StatusBadRequest, util.Error(400, "不支持的代码源类型"))
				return
			}
		} else {
			// 回退到基础路径
			sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
		}
	}

	// 检查路径是否存在
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, util.Error(404, "代码路径不存在，请先运行任务"))
		return
	}

	// 获取查询参数
	funcName := c.DefaultQuery("func", "")
	depth := 3
	if d := c.Query("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 5 {
			depth = parsed
		}
	}

	// 新增：是否只显示漏洞相关节点
	onlyVulnNodes := c.DefaultQuery("onlyVuln", "true") == "true"

	// 创建调用图分析器
	analyzer, err := mcp.NewCallGraphAnalyzer(sourcePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "创建分析器失败: "+err.Error()))
		return
	}

	// 构建调用图
	if err := analyzer.BuildCallGraph(); err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "构建调用图失败: "+err.Error()))
		return
	}

	// 根据参数返回不同格式的数据
	if funcName != "" {
		// 返回特定函数的调用图
		graphData := analyzer.GetCallGraphForFunction(funcName, depth)
		c.JSON(http.StatusOK, util.Success(graphData))
	} else {
		// 返回完整的调用图数据
		graphData := analyzer.GenerateVisualizationData("")

		// 如果只显示漏洞相关节点，则进行过滤
		if onlyVulnNodes {
			graphData = h.filterVulnerabilityNodes(graphData, task.ID)
		}

		c.JSON(http.StatusOK, util.Success(graphData))
	}
}

// GetNodeRelations 获取节点的Callees和Callers
func (h *TaskHandler) GetNodeRelations(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 获取查询参数
	nodeId := c.Query("nodeId")
	if nodeId == "" {
		c.JSON(http.StatusBadRequest, util.Error(400, "缺少nodeId参数"))
		return
	}

	// 确定源代码路径 - 优先使用保存的SourcePath
	var sourcePath string

	// 1. 优先使用数据库中保存的源代码路径
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
	} else {
		// 2. 如果没有保存，尝试从代码源类型构建路径
		var codeSource model.CodeSource
		if err := util.DB.First(&codeSource, task.CodeSourceID).Error; err == nil {
			switch codeSource.Type {
			case "jar", "zip":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
				// 尝试查找解压后的代码目录
				entries, err := os.ReadDir(sourcePath)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() && entry.Name() != "decompiled" {
							subPath := filepath.Join(sourcePath, entry.Name())
							codeFiles, _ := filepath.Glob(filepath.Join(subPath, "**/*.java"))
							if len(codeFiles) > 0 {
								sourcePath = subPath
								break
							}
						}
					}
				}
			case "git":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			default:
				c.JSON(http.StatusBadRequest, util.Error(400, "不支持的代码源类型"))
				return
			}
		} else {
			// 回退到基础路径
			sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
		}
	}

	// 检查路径是否存在
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, util.Error(404, "代码路径不存在，请先运行任务"))
		return
	}

	// 创建调用图分析器
	analyzer, err := mcp.NewCallGraphAnalyzer(sourcePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "创建分析器失败: "+err.Error()))
		return
	}

	// 构建调用图
	if err := analyzer.BuildCallGraph(); err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "构建调用图失败: "+err.Error()))
		return
	}

	// 获取被调用的方法 (Callees)
	callees := analyzer.GetCallees(nodeId)
	// 获取调用该方法的方法 (Callers)
	callers := analyzer.GetCallers(nodeId)

	c.JSON(http.StatusOK, util.Success(gin.H{
		"callees": callees,
		"callers": callers,
	}))
}

func (h *TaskHandler) UpdateProgress(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	var req struct {
		Progress int    `json:"progress"`
		Status   string `json:"status"`
		Result   string `json:"result"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, err.Error()))
		return
	}

	if req.Progress >= 0 && req.Progress <= 100 {
		task.Progress = req.Progress
	}

	if req.Status != "" {
		task.Status = model.TaskStatus(req.Status)
	}
	if req.Result != "" {
		task.Result = req.Result
	}

	if err := util.DB.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "更新任务失败"))
		return
	}

	c.JSON(http.StatusOK, util.Success(task))
}

// 全局进度管理器
var progressMgr *websocket.ProgressManager

func init() {
	progressMgr = websocket.NewProgressManager()
}

func GetProgressManager() *websocket.ProgressManager {
	return progressMgr
}

// TaskManager 任务管理器，用于跟踪和取消任务
type TaskManager struct {
	tasks map[uint]context.CancelFunc
	mu    sync.RWMutex
}

var taskManager *TaskManager

func init() {
	taskManager = &TaskManager{
		tasks: make(map[uint]context.CancelFunc),
	}
}

// StartTask 开始任务 - 使用写锁保护
func (tm *TaskManager) StartTask(taskID uint, cancelFunc context.CancelFunc) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tasks[taskID] = cancelFunc
}

// StopTask 停止任务 - 使用写锁保护
func (tm *TaskManager) StopTask(taskID uint) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if cancelFunc, ok := tm.tasks[taskID]; ok {
		cancelFunc()
		delete(tm.tasks, taskID)
	}
}

// RemoveTask 移除任务跟踪 - 使用写锁保护
func (tm *TaskManager) RemoveTask(taskID uint) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tasks, taskID)
}

// IsTaskRunning 检查任务是否正在运行 - 使用读锁保护
func (tm *TaskManager) IsTaskRunning(taskID uint) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	_, ok := tm.tasks[taskID]
	return ok
}

// GetTaskManager 获取任务管理器
func GetTaskManager() *TaskManager {
	return taskManager
}

var taskLogger = log.New(os.Stdout, "[任务] ", log.LstdFlags)

func (h *TaskHandler) processTask(taskID uint) {
	var task model.Task
	if err := util.DB.First(&task, taskID).Error; err != nil {
		return
	}

	// 检查任务是否还在运行状态，防止停止后继续执行
	if task.Status != model.TaskStatusRunning {
		taskLogger.Printf("Task %d: 任务已停止或不在运行状态，跳过执行", taskID)
		return
	}

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	// 注册到任务管理器
	taskManager.StartTask(taskID, cancel)

	// 注册任务完成时的清理 - 任务完成后不清理沙盒目录，保留代码用于后续查看
	defer func() {
		taskManager.RemoveTask(taskID)
		// 注意：沙盒目录不再自动清理，保留源代码用于后续API调用
		// 如果需要清理，可手动调用 CleanupSandbox 或通过删除任务时清理
	}()

	// 定期检查任务状态 - 只有当状态明确变为 paused/stopped 时才退出
	// 不再检查 failed/completed，因为这些状态应该由任务本身设置
	go func() {
		for {
			select {
			case <-ctx.Done():
				// 任务被取消（通过 context）- 只在DEBUG模式下打印
				taskLogger.Printf("Task %d: 任务被取消", taskID)
				return
			default:
				// 每3秒检查一次任务状态
				var t model.Task
				if err := util.DB.First(&t, taskID).Error; err != nil {
					return
				}
				// 只有明确要求暂停或停止时才退出
				// 不再因为 failed/completed 状态而退出，因为这些应该由任务本身设置
				if t.Status == model.TaskStatusPaused || t.Status == "stopped" {
					taskLogger.Printf("Task %d: 检测到任务已暂停/停止，优雅退出", taskID)
					cancel()
					taskManager.RemoveTask(taskID)
					return
				}
				time.Sleep(3 * time.Second)
			}
		}
	}()

	var codeSource model.CodeSource
	var modelConfig model.ModelConfig

	if err := util.DB.First(&codeSource, task.CodeSourceID).Error; err != nil {
		h.updateTaskStatus(taskID, "failed", "获取代码源失败: "+err.Error())
		return
	}

	if err := util.DB.First(&modelConfig, task.ModelConfigID).Error; err != nil {
		h.updateTaskStatus(taskID, "failed", "获取模型配置失败: "+err.Error())
		return
	}

	auditService := mcp.NewAuditServiceWithModel(
		modelConfig.APIKey,
		modelConfig.BaseURL,
		modelConfig.Model,
		0.7,
		modelConfig.MaxTokens,
	)

	taskIDStr := fmt.Sprintf("%d", taskID)
	sandboxDir, err := auditService.CreateSandbox(taskIDStr)
	if err != nil {
		h.updateTaskStatus(taskID, "failed", "创建沙盒环境失败: "+err.Error())
		return
	}

	var sourcePath string

	switch codeSource.Type {
	case "jar":
		filePath := codeSource.FilePath
		if !filepath.IsAbs(filePath) {
			wd, err := os.Getwd()
			if err == nil {
				filePath = filepath.Join(wd, filePath)
			}
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			h.updateTaskStatus(taskID, "failed", "代码源文件不存在: "+filePath)
			return
		}

		taskLogger.Printf("开始使用CFR反编译JAR文件: %s -> %s", filePath, sandboxDir)

		cfrPath := filepath.Join(".", "sandbox", "cfr.jar")
		if !filepath.IsAbs(cfrPath) {
			wd, _ := os.Getwd()
			cfrPath = filepath.Join(wd, cfrPath)
		}

		if _, err := os.Stat(cfrPath); os.IsNotExist(err) {
			h.updateTaskStatus(taskID, "failed", "cfr.jar 文件不存在: "+cfrPath)
			return
		}

		if _, err := exec.LookPath("java"); err != nil {
			h.updateTaskStatus(taskID, "failed", "系统未安装java命令")
			return
		}

		decompiledDir := filepath.Join(sandboxDir, "decompiled")
		if err := os.MkdirAll(decompiledDir, 0755); err != nil {
			h.updateTaskStatus(taskID, "failed", "创建反编译目录失败: "+err.Error())
			return
		}

		jarFileName := filepath.Base(filePath)
		targetJarPath := filepath.Join(sandboxDir, jarFileName)
		if err := copyFile(filePath, targetJarPath); err != nil {
			h.updateTaskStatus(taskID, "failed", "复制JAR文件失败: "+err.Error())
			return
		}

		cmd := exec.Command("java", "-jar", cfrPath, jarFileName, "--outputdir", "decompiled")
		cmd.Dir = sandboxDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			taskLogger.Printf("CFR 反编译失败: %s，回退到 zip 解压", string(output))

			extractedPath, err := auditService.ExtractZip(filePath, sandboxDir)
			if err != nil {
				h.updateTaskStatus(taskID, "failed", "解压文件失败: "+err.Error())
				return
			}
			sourcePath = extractedPath
		} else {
			taskLogger.Printf("CFR 反编译成功: %s", string(output))
			sourcePath = decompiledDir
		}

		files, _ := auditService.ListFiles(sourcePath)
		taskLogger.Printf("解压后文件列表: %v", files)

		codeFiles, err := auditService.ListCodeFiles(sourcePath)
		if err != nil {
			h.updateTaskStatus(taskID, "failed", "扫描代码文件失败: "+err.Error())
			return
		}

		if len(codeFiles) == 0 {
			entries, _ := os.ReadDir(sourcePath)
			if len(entries) == 1 && entries[0].IsDir() {
				nestedPath := filepath.Join(sourcePath, entries[0].Name())
				codeFiles, _ = auditService.ListCodeFiles(nestedPath)
				if len(codeFiles) > 0 {
					sourcePath = nestedPath
				}
			}
		}

		if len(codeFiles) == 0 {
			h.updateTaskStatus(taskID, "failed", "未找到代码文件，解压后目录为空或不包含代码文件")
			return
		}

	case "zip":
		filePath := codeSource.FilePath
		if !filepath.IsAbs(filePath) {
			wd, err := os.Getwd()
			if err == nil {
				filePath = filepath.Join(wd, filePath)
			}
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			h.updateTaskStatus(taskID, "failed", "代码源文件不存在: "+filePath)
			return
		}

		taskLogger.Printf("开始解压ZIP文件: %s -> %s", filePath, sandboxDir)

		extractedPath, err := auditService.ExtractZip(filePath, sandboxDir)
		if err != nil {
			h.updateTaskStatus(taskID, "failed", "解压文件失败: "+err.Error())
			return
		}

		files, _ := auditService.ListFiles(extractedPath)
		taskLogger.Printf("解压后文件列表: %v", files)

		codeFiles, err := auditService.ListCodeFiles(extractedPath)
		if err != nil {
			h.updateTaskStatus(taskID, "failed", "扫描代码文件失败: "+err.Error())
			return
		}

		if len(codeFiles) == 0 {
			entries, _ := os.ReadDir(extractedPath)
			if len(entries) == 1 && entries[0].IsDir() {
				nestedPath := filepath.Join(extractedPath, entries[0].Name())
				codeFiles, _ = auditService.ListCodeFiles(nestedPath)
				if len(codeFiles) > 0 {
					extractedPath = nestedPath
				}
			}
		}

		if len(codeFiles) == 0 {
			h.updateTaskStatus(taskID, "failed", "未找到代码文件，解压后目录为空或不包含代码文件")
			return
		}

		sourcePath = extractedPath

	case "git":
		repoURL := codeSource.URL
		if repoURL == "" {
			h.updateTaskStatus(taskID, "failed", "Git仓库URL为空")
			return
		}

		taskLogger.Printf("开始克隆Git仓库: %s -> %s", repoURL, sandboxDir)

		_, err := exec.LookPath("git")
		if err != nil {
			h.updateTaskStatus(taskID, "failed", "系统未安装git命令")
			return
		}

		cmd := exec.Command("git", "clone", "--depth", "1", repoURL, sandboxDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			h.updateTaskStatus(taskID, "failed", fmt.Sprintf("Git克隆失败: %s, %s", err.Error(), string(output)))
			return
		}

		taskLogger.Printf("Git克隆成功: %s", string(output))
		sourcePath = sandboxDir
	default:
		h.updateTaskStatus(taskID, "failed", "不支持的代码源类型")
		return
	}

	auditTask := &mcp.AuditTask{
		ID:         taskIDStr,
		SourcePath: sourcePath,
		Prompt:     task.Prompt,
		Status:     "running",
		Progress:   0,
	}

	mcp.SetProgressManager(progressMgr)

	task.Progress = 10
	util.DB.Save(&task)

	err = auditService.ExecuteAuditWithCrossFileAnalysis(auditTask)
	if err != nil {
		h.updateTaskStatus(taskID, "failed", err.Error())
		progressMgr.UpdateTaskProgress(taskIDStr, 0, "failed", err.Error(), "")
		return
	}

	// 保存漏洞数据到数据库并获取统计
	vulnStats := h.saveVulnerabilities(taskID, auditTask.Results)

	// 计算安全评分和风险等级
	securityScore, riskLevel := h.calculateSecurityScore(auditTask.Results)

	// 清理数据中的无效字符，防止数据库写入错误
	cleanedReport := cleanStringForDB(auditTask.Report)
	cleanedLog := cleanStringForDB(auditTask.Log)
	cleanedAILog := cleanStringForDB(auditTask.AILog)

	// 智能字段大小限制
	maxReportSize := 5 * 1024 * 1024    // 5MB - 报告内容（主数据）
	maxLogSize := 2 * 1024 * 1024       // 2MB - 普通日志
	maxAILogSize := 2 * 1024 * 1024     // 2MB - AI交互日志
	maxCrossFileSize := 2 * 1024 * 1024 // 2MB - 跨文件分析

	// 截断报告
	if len(cleanedReport) > maxReportSize {
		cleanedReport = cleanedReport[:maxReportSize] + "\n\n[报告过长，已截断。原始报告长度: " +
			fmt.Sprintf("%d 字符]", len(auditTask.Report))
		taskLogger.Printf("任务 %d: 报告内容过大，已截断到 %d MB", taskID, maxReportSize/(1024*1024))
	}

	// 截断日志
	if len(cleanedLog) > maxLogSize {
		cleanedLog = cleanedLog[:maxLogSize] + "\n\n[日志过长，已截断。原始日志长度: " +
			fmt.Sprintf("%d 字符]", len(auditTask.Log))
		taskLogger.Printf("任务 %d: 日志内容过大，已截断到 %d MB", taskID, maxLogSize/(1024*1024))
	}

	// 截断AI日志
	if len(cleanedAILog) > maxAILogSize {
		cleanedAILog = cleanedAILog[:maxAILogSize] + "\n\n[AI日志过长，已截断。原始AI日志长度: " +
			fmt.Sprintf("%d 字符]", len(auditTask.AILog))
		taskLogger.Printf("任务 %d: AI日志内容过大，已截断到 %d MB", taskID, maxAILogSize/(1024*1024))
	}

	// 清理跨文件分析结果
	cleanedCrossFile := ""
	if auditTask.CrossFileContext != "" {
		cleanedCrossFile = cleanStringForDB(auditTask.CrossFileContext)
		if len(cleanedCrossFile) > maxCrossFileSize {
			cleanedCrossFile = cleanedCrossFile[:maxCrossFileSize] + "\n\n[跨文件分析结果过长，已截断。原始长度: " +
				fmt.Sprintf("%d 字符]", len(auditTask.CrossFileContext))
			taskLogger.Printf("任务 %d: 跨文件分析结果过大，已截断到 %d MB", taskID, maxCrossFileSize/(1024*1024))
		}
	}

	// 准备任务数据
	task.Result = cleanedReport
	task.Status = model.TaskStatusCompleted
	task.Progress = 100
	task.UpdatedAt = time.Now()
	task.ScannedFiles = auditTask.ScannedFiles
	task.VulnerabilityCount = auditTask.VulnerabilityCount
	task.Duration = auditTask.Duration
	task.CurrentFile = ""
	task.Log = cleanedLog
	task.DetectedLanguage = auditTask.Language
	task.AILog = cleanedAILog
	task.CrossFileAnalysis = cleanedCrossFile
	task.SecurityScore = securityScore
	task.RiskLevel = riskLevel

	// 更新漏洞统计字段
	task.CriticalVulns = vulnStats.critical
	task.HighVulns = vulnStats.high
	task.MediumVulns = vulnStats.medium
	task.LowVulns = vulnStats.low

	// 使用事务和分块写入策略保存任务
	if err := h.saveTaskWithRetry(task, taskID, auditTask, vulnStats); err != nil {
		h.updateTaskStatus(taskID, "failed", "保存审计结果失败: "+err.Error())
		return
	}

	// 保存项目统计数据
	h.saveProjectStats(taskID, auditTask, vulnStats)

	// 保存源代码路径到数据库，用于后续API调用
	if sourcePath != "" {
		task.SourcePath = sourcePath
		util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("source_path", sourcePath)
	}

	progressMgr.UpdateTaskResult(taskIDStr, auditTask.Report)
}

// saveTaskWithRetry 使用重试机制保存任务
func (h *TaskHandler) saveTaskWithRetry(task model.Task, taskID uint, auditTask *mcp.AuditTask, stats vulnStats) error {
	// 重试配置
	maxRetries := 3
	retryDelay := 2 * time.Second

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("任务 %d: 第 %d 次尝试保存任务...\n", taskID, attempt)
			time.Sleep(retryDelay * time.Duration(attempt-1)) // 指数退避
		}

		// 尝试保存
		err := h.saveTaskWithTransaction(task, taskID, auditTask, stats)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("任务 %d: 第 %d 次尝试成功\n", taskID, attempt)
			} else {
				fmt.Printf("任务 %d: 保存成功\n", taskID)
			}
			return nil
		}

		lastErr = err
		fmt.Printf("任务 %d: 第 %d 次尝试失败: %v\n", taskID, attempt, err)

		// 如果是最后一次尝试，尝试使用紧急保存策略
		if attempt == maxRetries {
			fmt.Printf("任务 %d: 所有重试失败，尝试紧急保存策略\n", taskID)
			return h.emergencySaveTask(taskID, auditTask, stats)
		}
	}

	return lastErr
}

// saveTaskWithTransaction 使用事务保存任务
func (h *TaskHandler) saveTaskWithTransaction(task model.Task, taskID uint, auditTask *mcp.AuditTask, stats vulnStats) error {
	// 使用事务确保数据一致性
	tx := util.DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("开启事务失败: %v", tx.Error)
	}

	// 确保事务回滚
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 第一步：更新基本字段（小字段，不容易出错）
	basicUpdates := map[string]interface{}{
		"status":              model.TaskStatusCompleted,
		"progress":            100,
		"scanned_files":       auditTask.ScannedFiles,
		"vulnerability_count": auditTask.VulnerabilityCount,
		"duration":            auditTask.Duration,
		"current_file":        "",
		"detected_language":   auditTask.Language,
		"security_score":      task.SecurityScore,
		"risk_level":          task.RiskLevel,
		"critical_vulns":      stats.critical,
		"high_vulns":          stats.high,
		"medium_vulns":        stats.medium,
		"low_vulns":           stats.low,
		"updated_at":          time.Now(),
	}

	if err := tx.Model(&task).Where("id = ?", taskID).Updates(basicUpdates).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新基本字段失败: %v", err)
	}

	// 第二步：更新日志字段（中等大小）
	logUpdates := map[string]interface{}{
		"log":    task.Log,
		"ai_log": task.AILog,
	}
	if err := tx.Model(&task).Where("id = ?", taskID).Updates(logUpdates).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("更新日志字段失败: %v", err)
	}

	// 第三步：更新大文本字段（最容易出错）
	bigTextUpdates := map[string]interface{}{
		"result":              task.Result,
		"cross_file_analysis": task.CrossFileAnalysis,
	}

	// 使用分块更新策略
	for field, value := range bigTextUpdates {
		if err := tx.Model(&task).Where("id = ?", taskID).Update(field, value).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("更新字段 %s 失败: %v", field, err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("提交事务失败: %v", err)
	}

	return nil
}

// emergencySaveTask 紧急保存策略 - 当正常保存失败时使用
func (h *TaskHandler) emergencySaveTask(taskID uint, auditTask *mcp.AuditTask, stats vulnStats) error {
	fmt.Printf("任务 %d: 执行紧急保存策略\n", taskID)

	// 计算安全评分和风险等级
	securityScore, riskLevel := h.calculateSecurityScore(auditTask.Results)

	// 第一步：保存基本状态
	basicUpdates := map[string]interface{}{
		"status":              model.TaskStatusCompleted,
		"progress":            100,
		"scanned_files":       auditTask.ScannedFiles,
		"vulnerability_count": auditTask.VulnerabilityCount,
		"duration":            auditTask.Duration,
		"current_file":        "",
		"detected_language":   auditTask.Language,
		"security_score":      securityScore,
		"risk_level":          riskLevel,
		"critical_vulns":      stats.critical,
		"high_vulns":          stats.high,
		"medium_vulns":        stats.medium,
		"low_vulns":           stats.low,
		"updated_at":          time.Now(),
	}

	if err := util.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(basicUpdates).Error; err != nil {
		return fmt.Errorf("紧急保存基本字段失败: %v", err)
	}

	// 第二步：尝试保存日志（如果失败则记录到文件）
	logContent := cleanStringForDB(auditTask.Log)
	if len(logContent) > 2*1024*1024 {
		logContent = logContent[:2*1024*1024] + "\n[日志过长，已截断]"
	}

	if err := util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("log", logContent).Error; err != nil {
		fmt.Printf("任务 %d: 保存日志到数据库失败，尝试保存到文件: %v\n", taskID, err)
		// 保存日志到文件
		logPath := filepath.Join(".", "logs", fmt.Sprintf("task_%d.log", taskID))
		os.MkdirAll(filepath.Dir(logPath), 0755)
		if writeErr := os.WriteFile(logPath, []byte(auditTask.Log), 0644); writeErr == nil {
			util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("log", "[日志已保存到文件: "+logPath+"]")
		}
	}

	// 第三步：尝试保存AI日志
	aiLogContent := cleanStringForDB(auditTask.AILog)
	if len(aiLogContent) > 2*1024*1024 {
		aiLogContent = aiLogContent[:2*1024*1024] + "\n[AI日志过长，已截断]"
	}

	if err := util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("ai_log", aiLogContent).Error; err != nil {
		fmt.Printf("任务 %d: 保存AI日志到数据库失败，尝试保存到文件: %v\n", taskID, err)
		aiLogPath := filepath.Join(".", "logs", fmt.Sprintf("task_%d_ai.log", taskID))
		if writeErr := os.WriteFile(aiLogPath, []byte(auditTask.AILog), 0644); writeErr == nil {
			util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("ai_log", "[AI日志已保存到文件: "+aiLogPath+"]")
		}
	}

	// 第四步：尝试保存报告（最关键）
	reportContent := cleanStringForDB(auditTask.Report)
	if len(reportContent) > 5*1024*1024 {
		reportContent = reportContent[:5*1024*1024] + "\n\n[报告过长，已截断。完整报告已保存到文件]"
	}

	if err := util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("result", reportContent).Error; err != nil {
		fmt.Printf("任务 %d: 保存报告到数据库失败，尝试保存到文件: %v\n", taskID, err)

		// 保存报告到文件
		reportPath := filepath.Join(".", "reports", fmt.Sprintf("task_%d_report.md", taskID))
		os.MkdirAll(filepath.Dir(reportPath), 0755)

		if writeErr := os.WriteFile(reportPath, []byte(auditTask.Report), 0644); writeErr == nil {
			// 更新数据库中的报告路径
			summary := fmt.Sprintf("# 审计报告\n\n报告内容过大，已保存到文件: %s\n\n## 统计信息\n- 扫描文件数: %d\n- 发现漏洞数: %d\n- 执行时长: %d秒\n- 安全评分: %d\n- 风险等级: %s\n\n请查看文件获取完整报告。",
				reportPath, auditTask.ScannedFiles, auditTask.VulnerabilityCount,
				auditTask.Duration, securityScore, riskLevel)

			util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("result", summary)
			util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("report_path", reportPath)
		} else {
			// 如果连文件都保存失败，至少保存一个摘要
			summary := fmt.Sprintf("# 审计报告摘要\n\n- 扫描文件数: %d\n- 发现漏洞数: %d\n- 执行时长: %d秒\n- 安全评分: %d\n- 风险等级: %s\n\n[完整报告保存失败，请联系管理员]",
				auditTask.ScannedFiles, auditTask.VulnerabilityCount,
				auditTask.Duration, securityScore, riskLevel)
			util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("result", summary)
		}
	}

	// 第五步：尝试保存跨文件分析结果
	if auditTask.CrossFileContext != "" {
		crossFileContent := cleanStringForDB(auditTask.CrossFileContext)
		if len(crossFileContent) > 2*1024*1024 {
			crossFileContent = crossFileContent[:2*1024*1024] + "\n[跨文件分析结果过长，已截断]"
		}

		if err := util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("cross_file_analysis", crossFileContent).Error; err != nil {
			fmt.Printf("任务 %d: 保存跨文件分析结果到数据库失败: %v\n", taskID, err)
			// 保存到文件
			crossFilePath := filepath.Join(".", "reports", fmt.Sprintf("task_%d_crossfile.txt", taskID))
			os.WriteFile(crossFilePath, []byte(auditTask.CrossFileContext), 0644)
		}
	}

	fmt.Printf("任务 %d: 紧急保存完成\n", taskID)
	return nil
}

func (h *TaskHandler) updateTaskStatus(taskID uint, status, message string) {
	var task model.Task
	if err := util.DB.First(&task, taskID).Error; err != nil {
		return
	}

	task.Status = model.TaskStatus(status)
	task.Result = message
	task.UpdatedAt = time.Now()

	util.DB.Save(&task)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}

// vulnStats 漏洞统计结构
type vulnStats struct {
	critical int
	high     int
	medium   int
	low      int
}

// saveVulnerabilities 保存漏洞数据到数据库并返回统计
func (h *TaskHandler) saveVulnerabilities(taskID uint, results []mcp.AuditResult) vulnStats {
	// 先删除该任务已有的漏洞数据
	util.DB.Where("task_id = ?", taskID).Delete(&model.Vulnerability{})

	stats := vulnStats{}

	for _, r := range results {
		vuln := model.Vulnerability{
			TaskID:        taskID,
			Type:          r.Type,
			File:          r.File,
			Line:          r.Line,
			Description:   r.Description,
			Severity:      r.Severity,
			Analysis:      r.Analysis,
			FixSuggestion: r.FixSuggestion,
			POC:           r.POC,
			CWE:           r.CWE,
			Confidence:    r.Confidence,
			AttackVector:  r.AttackVector,
			CodeSnippet:   r.CodeSnippet,
		}
		util.DB.Create(&vuln)

		// 统计各严重程度数量
		switch r.Severity {
		case "Critical":
			stats.critical++
		case "High":
			stats.high++
		case "Medium":
			stats.medium++
		case "Low":
			stats.low++
		}
	}

	return stats
}

// calculateSecurityScore 计算安全评分和风险等级
func (h *TaskHandler) calculateSecurityScore(results []mcp.AuditResult) (int, string) {
	if len(results) == 0 {
		return 100, "低风险"
	}

	// 统计各严重程度数量
	critical := 0
	high := 0
	medium := 0
	low := 0

	for _, r := range results {
		switch r.Severity {
		case "Critical":
			critical++
		case "High":
			high++
		case "Medium":
			medium++
		case "Low":
			low++
		}
	}

	// 计算评分 (100分制)
	// Critical: -20分, High: -10分, Medium: -5分, Low: -2分
	score := 100 - (critical*20 + high*10 + medium*5 + low*2)
	if score < 0 {
		score = 0
	}

	// 确定风险等级
	var riskLevel string
	if score >= 90 {
		riskLevel = "低风险"
	} else if score >= 70 {
		riskLevel = "中风险"
	} else if score >= 50 {
		riskLevel = "高风险"
	} else {
		riskLevel = "严重"
	}

	return score, riskLevel
}

// GetCodeSnippet 获取代码片段 - 修复版
func (h *TaskHandler) GetCodeSnippet(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 获取查询参数
	filePath := c.Query("file")
	lineStr := c.Query("line")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, util.Error(400, "缺少文件路径参数"))
		return
	}

	// 解析行号
	lineNum := 1
	if lineStr != "" {
		var err error
		lineNum, err = strconv.Atoi(lineStr)
		if err != nil || lineNum <= 0 {
			lineNum = 1
		}
	}

	// 确定源代码路径 - 优先使用保存的SourcePath
	var sourcePath string

	// 1. 优先使用数据库中保存的源代码路径
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
	} else {
		// 2. 如果没有保存，尝试从代码源类型构建路径
		var codeSource model.CodeSource
		if err := util.DB.First(&codeSource, task.CodeSourceID).Error; err == nil {
			switch codeSource.Type {
			case "jar", "zip":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
				// 尝试查找解压后的代码目录
				entries, err := os.ReadDir(sourcePath)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() && entry.Name() != "decompiled" {
							subPath := filepath.Join(sourcePath, entry.Name())
							codeFiles, _ := filepath.Glob(filepath.Join(subPath, "**/*.java"))
							if len(codeFiles) > 0 {
								sourcePath = subPath
								break
							}
						}
					}
				}
			case "git":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			default:
				c.JSON(http.StatusBadRequest, util.Error(400, "不支持的代码源类型"))
				return
			}
		} else {
			// 回退到基础路径
			sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
		}
	}

	// 检查路径是否存在
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, util.Error(404, "代码路径不存在，请先运行任务"))
		return
	}

	// 查找文件的实际路径
	actualFilePath := h.findFileInSandbox(sourcePath, filePath)

	if actualFilePath == "" {
		c.JSON(http.StatusNotFound, util.Error(404, fmt.Sprintf("文件不存在: %s", filePath)))
		return
	}

	// 读取文件内容
	content, err := os.ReadFile(actualFilePath)
	if err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, fmt.Sprintf("读取文件失败: %s", err.Error())))
		return
	}

	// 提取指定行附近的代码片段（前后10行）
	lines := splitLines(string(content))
	startLine := max(1, lineNum-10)
	endLine := min(len(lines), lineNum+10)

	var snippetLines []string
	for i := startLine; i <= endLine; i++ {
		snippetLines = append(snippetLines, lines[i-1])
	}

	codeSnippet := strings.Join(snippetLines, "\n")

	// 计算相对路径
	relPath, _ := filepath.Rel(sourcePath, actualFilePath)

	c.JSON(http.StatusOK, util.Success(gin.H{
		"content": codeSnippet,
		"file":    relPath,
		"line":    lineNum,
		"start":   startLine,
		"end":     endLine,
	}))
}

// findFileInSandbox 在沙盒目录中查找文件
func (h *TaskHandler) findFileInSandbox(sourcePath, filePath string) string {
	// 如果传入的是完整路径，直接尝试
	if filepath.IsAbs(filePath) {
		if _, err := os.Stat(filePath); err == nil {
			return filePath
		}
	}

	// 尝试各种路径组合
	searchFileName := filepath.Base(filePath)

	// 1. 直接在sourcePath下查找（使用原始filePath）
	directPath := filepath.Join(sourcePath, filePath)
	if _, err := os.Stat(directPath); err == nil {
		return directPath
	}

	// 2. 在decompiled目录下查找
	decompiledPath := filepath.Join(sourcePath, "decompiled", filePath)
	if _, err := os.Stat(decompiledPath); err == nil {
		return decompiledPath
	}

	// 3. 只使用文件名查找（忽略目录结构）
	fileNameOnly := filepath.Base(filePath)
	filepath.Walk(sourcePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		// 忽略大小写比较文件名
		if strings.EqualFold(info.Name(), searchFileName) || strings.EqualFold(info.Name(), fileNameOnly) {
			return filepath.SkipAll
		}
		return nil
	})

	// 4. 递归搜索整个目录
	var foundPath string
	filepath.Walk(sourcePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		// 检查文件名是否匹配（忽略大小写）
		if strings.EqualFold(info.Name(), searchFileName) || strings.EqualFold(info.Name(), fileNameOnly) {
			foundPath = path
			return filepath.SkipAll
		}
		return nil
	})

	return foundPath
}

// GetCodeSnippetByVulnID 根据漏洞ID获取代码片段
func (h *TaskHandler) GetCodeSnippetByVulnID(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	vulnID := c.Param("vulnId")

	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 从漏洞表获取漏洞信息
	var vulnerability model.Vulnerability
	vulnIDUint, err := strconv.ParseUint(vulnID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "无效的漏洞ID"))
		return
	}

	if err := util.DB.Where("id = ? AND task_id = ?", vulnIDUint, id).First(&vulnerability).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "漏洞不存在"))
		return
	}

	// 如果漏洞已经有代码片段，直接返回
	if vulnerability.CodeSnippet != "" {
		c.JSON(http.StatusOK, util.Success(gin.H{
			"content":     vulnerability.CodeSnippet,
			"file":        vulnerability.File,
			"line":        vulnerability.Line,
			"type":        vulnerability.Type,
			"severity":    vulnerability.Severity,
			"description": vulnerability.Description,
		}))
		return
	}

	// 否则尝试读取文件
	filePath := vulnerability.File
	lineNum := vulnerability.Line

	if filePath == "" || lineNum <= 0 {
		c.JSON(http.StatusBadRequest, util.Error(400, "漏洞信息不完整"))
		return
	}

	// 确定源代码路径 - 优先使用保存的SourcePath
	var sourcePath string

	// 1. 优先使用数据库中保存的源代码路径
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
	} else {
		// 2. 如果没有保存，尝试从代码源类型构建路径
		var codeSource model.CodeSource
		if err := util.DB.First(&codeSource, task.CodeSourceID).Error; err == nil {
			switch codeSource.Type {
			case "jar", "zip":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			case "git":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			default:
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			}
		} else {
			// 回退到基础路径
			sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
		}
	}

	// 检查路径是否存在
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, util.Error(404, "代码路径不存在"))
		return
	}

	// 查找文件
	actualPath := h.findFileInSandbox(sourcePath, filePath)
	if actualPath == "" {
		c.JSON(http.StatusNotFound, util.Error(404, fmt.Sprintf("文件不存在: %s", filePath)))
		return
	}

	// 读取文件内容
	content, err := os.ReadFile(actualPath)
	if err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, fmt.Sprintf("读取文件失败: %s", err.Error())))
		return
	}

	// 提取代码片段
	lines := splitLines(string(content))
	startLine := max(1, lineNum-10)
	endLine := min(len(lines), lineNum+10)

	var snippetLines []string
	for i := startLine; i <= endLine; i++ {
		snippetLines = append(snippetLines, lines[i-1])
	}

	codeSnippet := strings.Join(snippetLines, "\n")

	c.JSON(http.StatusOK, util.Success(gin.H{
		"content":       codeSnippet,
		"file":          vulnerability.File,
		"line":          vulnerability.Line,
		"type":          vulnerability.Type,
		"severity":      vulnerability.Severity,
		"description":   vulnerability.Description,
		"analysis":      vulnerability.Analysis,
		"fixSuggestion": vulnerability.FixSuggestion,
		"poc":           vulnerability.POC,
	}))
}

// 辅助函数：分割行
func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// 辅助函数：最大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// 辅助函数：最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// saveProjectStats 保存项目统计数据
func (h *TaskHandler) saveProjectStats(taskID uint, auditTask *mcp.AuditTask, stats vulnStats) {
	statsModel := model.ProjectStats{
		TaskID:         taskID,
		TotalFiles:     auditTask.ScannedFiles,
		CodeLines:      0,
		TotalClasses:   0,
		TotalFunctions: 0,
		CriticalVulns:  stats.critical,
		HighVulns:      stats.high,
		MediumVulns:    stats.medium,
		LowVulns:       stats.low,
	}

	util.DB.Where("task_id = ?", taskID).Delete(&model.ProjectStats{})
	util.DB.Create(&statsModel)
}

// filterVulnerabilityNodes 过滤出漏洞利用链相关节点
func (h *TaskHandler) filterVulnerabilityNodes(graphData *mcp.CallGraphData, taskID uint) *mcp.CallGraphData {
	if graphData == nil || len(graphData.Nodes) == 0 {
		return graphData
	}

	var vulnerabilities []model.Vulnerability
	util.DB.Where("task_id = ?", taskID).Find(&vulnerabilities)

	if len(vulnerabilities) == 0 {
		return &mcp.CallGraphData{
			Nodes: []*mcp.GraphNode{},
			Edges: []*mcp.GraphEdge{},
			Stats: &mcp.GraphStats{},
		}
	}

	vulnFiles := make(map[string]bool)
	for _, vuln := range vulnerabilities {
		if vuln.File != "" {
			vulnFiles[vuln.File] = true
		}
	}

	filteredNodes := []*mcp.GraphNode{}
	nodeIDMap := make(map[string]bool)

	for _, node := range graphData.Nodes {
		if node.File == "" {
			continue
		}

		if vulnFiles[node.File] {
			filteredNodes = append(filteredNodes, node)
			nodeIDMap[node.ID] = true
		}
	}

	filteredEdges := []*mcp.GraphEdge{}
	for _, edge := range graphData.Edges {
		if nodeIDMap[edge.Source] && nodeIDMap[edge.Target] {
			filteredEdges = append(filteredEdges, edge)
		}
	}

	return &mcp.CallGraphData{
		Nodes:     filteredNodes,
		Edges:     filteredEdges,
		Stats:     nil,
		EntryFunc: graphData.EntryFunc,
	}
}

// GetVulnGraph 获取漏洞利用链全景图 - 从审计报告提取准确信息
func (h *TaskHandler) GetVulnGraph(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 从数据库获取漏洞数据
	var vulnerabilities []model.Vulnerability
	util.DB.Where("task_id = ?", id).Order("severity DESC, id ASC").Find(&vulnerabilities)

	if len(vulnerabilities) == 0 {
		c.JSON(http.StatusOK, util.Success(gin.H{
			"nodes": []gin.H{},
			"edges": []gin.H{},
			"stats": gin.H{
				"totalVulns": 0,
			},
		}))
		return
	}

	// 确定源代码路径 - 优先使用保存的SourcePath
	var sourcePath string

	// 1. 优先使用数据库中保存的源代码路径
	if task.SourcePath != "" {
		sourcePath = task.SourcePath
	} else {
		// 2. 如果没有保存，尝试从代码源类型构建路径
		var codeSource model.CodeSource
		if err := util.DB.First(&codeSource, task.CodeSourceID).Error; err == nil {
			switch codeSource.Type {
			case "jar", "zip":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			case "git":
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			default:
				sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
			}
		} else {
			// 回退到基础路径
			sourcePath = filepath.Join(".", "sandbox", "audit-sandbox", id)
		}
	}

	// 初始化调用链分析器（用于构建MVC漏洞利用链）
	var callAnalyzer *mcp.CallGraphAnalyzer
	if _, err := os.Stat(sourcePath); err == nil {
		callAnalyzer, _ = mcp.NewCallGraphAnalyzer(sourcePath)
		if callAnalyzer != nil {
			callAnalyzer.BuildCallGraph()
		}
	}

	// 统计
	var criticalVulns, highVulns, mediumVulns, lowVulns int
	vulnTypes := make(map[string]int)
	fileSet := make(map[string]bool)

	for _, vuln := range vulnerabilities {
		switch vuln.Severity {
		case "Critical":
			criticalVulns++
		case "High":
			highVulns++
		case "Medium":
			mediumVulns++
		case "Low":
			lowVulns++
		}
		vulnTypes[vuln.Type]++
		if vuln.File != "" {
			fileSet[vuln.File] = true
		}
	}

	// 创建节点 - 每个漏洞一个节点
	nodes := []gin.H{}
	for _, vuln := range vulnerabilities {
		nodeID := fmt.Sprintf("vuln_%d", vuln.ID)
		fileName := filepath.Base(vuln.File)
		if fileName == "" {
			fileName = "未知文件"
		}

		// 从审计报告提取准确的漏洞描述
		description := h.extractDescriptionFromReport(task.Result, vuln)

		// 提取函数名和类名
		className, functionName := h.extractClassAndFunction(vuln, sourcePath)

		// 节点标签：优先显示函数名，否则显示类名，最后显示文件名
		var displayLabel string
		if functionName != "" && !isCommonKeyword(functionName) {
			if className != "" {
				displayLabel = className + "." + functionName
			} else {
				displayLabel = functionName
			}
		} else if className != "" {
			displayLabel = className
		} else {
			// 使用翻译后的漏洞类型作为备选
			displayLabel = translateVulnType(vuln.Type)
		}

		// 完整标签
		var fullLabel string
		if className != "" && functionName != "" && !isCommonKeyword(functionName) {
			fullLabel = fmt.Sprintf("%s.%s\n%s:%d", className, functionName, fileName, vuln.Line)
		} else if className != "" {
			fullLabel = fmt.Sprintf("%s\n%s:%d", className, fileName, vuln.Line)
		} else {
			fullLabel = fmt.Sprintf("%s:%d", fileName, vuln.Line)
		}

		// 构建该漏洞的MVC调用链
		exploitChain := []gin.H{}
		if callAnalyzer != nil && vuln.File != "" {
			exploitChain = h.buildExploitChainForVuln(callAnalyzer, vuln, sourcePath)
		}

		nodes = append(nodes, gin.H{
			"id":             nodeID,
			"label":          displayLabel,
			"displayLabel":   displayLabel,
			"fullLabel":      fullLabel,
			"type":           vuln.Type,
			"translatedType": translateVulnType(vuln.Type),
			"severity":       vuln.Severity,
			"file":           vuln.File,
			"line":           vuln.Line,
			"class":          className,
			"function":       functionName,
			"description":    description,
			"analysis":       vuln.Analysis,
			"fixSuggestion":  vuln.FixSuggestion,
			"poc":            vuln.POC,
			"cwe":            vuln.CWE,
			"codeSnippet":    vuln.CodeSnippet,
			"vulnId":         vuln.ID,
			"exploitChain":   exploitChain,
		})
	}

	// 创建边 - 基于调用关系的边
	edges := []gin.H{}
	edgeCount := 0

	// 按文件分组
	fileVulns := make(map[string][]uint)
	for _, vuln := range vulnerabilities {
		if vuln.File != "" {
			fileVulns[vuln.File] = append(fileVulns[vuln.File], vuln.ID)
		}
	}

	// 如果有调用链分析器，基于调用关系创建边
	if callAnalyzer != nil {
		for _, vuln := range vulnerabilities {
			if vuln.File == "" {
				continue
			}

			// 查找调用该漏洞方法的调用者
			callers := callAnalyzer.GetCallers(vuln.File)
			for _, caller := range callers {
				// 查找调用者中是否有其他漏洞
				for _, otherVuln := range vulnerabilities {
					if otherVuln.ID == vuln.ID {
						continue
					}
					if otherVuln.File == caller.File {
						edgeCount++
						edges = append(edges, gin.H{
							"id":     fmt.Sprintf("edge_%d", edgeCount),
							"source": fmt.Sprintf("vuln_%d", otherVuln.ID),
							"target": fmt.Sprintf("vuln_%d", vuln.ID),
							"label":  "调用",
						})
					}
				}
			}
		}
	}

	// 同文件内的漏洞连接
	for _, vulnIDs := range fileVulns {
		if len(vulnIDs) > 1 {
			for i := 0; i < len(vulnIDs)-1; i++ {
				edgeCount++
				edges = append(edges, gin.H{
					"id":     fmt.Sprintf("edge_%d", edgeCount),
					"source": fmt.Sprintf("vuln_%d", vulnIDs[i]),
					"target": fmt.Sprintf("vuln_%d", vulnIDs[i+1]),
					"label":  "同文件",
				})
			}
		}
	}

	typeList := []string{}
	for t := range vulnTypes {
		typeList = append(typeList, t)
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"nodes": nodes,
		"edges": edges,
		"stats": gin.H{
			"totalVulns":     len(vulnerabilities),
			"criticalVulns":  criticalVulns,
			"highVulns":      highVulns,
			"mediumVulns":    mediumVulns,
			"lowVulns":       lowVulns,
			"totalFiles":     len(fileSet),
			"vulnTypes":      typeList,
			"vulnTypeCounts": vulnTypes,
		},
	}))
}

// buildExploitChainForVuln 为单个漏洞构建漏洞利用链
func (h *TaskHandler) buildExploitChainForVuln(analyzer *mcp.CallGraphAnalyzer, vuln model.Vulnerability, sourcePath string) []gin.H {
	var chain []gin.H

	if vuln.File == "" {
		return chain
	}

	// 找到漏洞所在文件的实际路径
	actualPath := h.findFileInSandbox(sourcePath, vuln.File)
	if actualPath == "" {
		return chain
	}

	// 查找Source点（用户输入入口）
	sources := analyzer.FindEntryPoints()
	var sourceNode *mcp.CallGraphNode

	for _, source := range sources {
		if source != nil && source.Node != nil {
			// 查找最近的调用路径
			sourceNode = source.Node
			break
		}
	}

	// 查找Sink点（危险操作）
	sinks := analyzer.FindSinkPoints()
	var sinkNode *mcp.CallGraphNode

	for _, sink := range sinks {
		if sink != nil && sink.Node != nil && sink.Node.FilePath == vuln.File {
			sinkNode = sink.Node
			break
		}
	}

	// 构建Source节点
	if sourceNode != nil {
		className, funcName := h.extractClassAndFunctionFromNode(sourceNode, sourcePath)
		displayLabel := funcName
		if displayLabel == "" {
			displayLabel = className
		}
		if displayLabel == "" {
			displayLabel = "[Source] 用户输入"
		} else {
			displayLabel = "[Source] " + displayLabel
		}

		chain = append(chain, gin.H{
			"nodeType": "source",
			"label":    displayLabel,
			"class":    className,
			"function": funcName,
			"file":     filepath.Base(sourceNode.FilePath),
			"line":     sourceNode.Line,
		})
	}

	// 添加Controller层
	chain = append(chain, gin.H{
		"nodeType": "controller",
		"label":    "[Controller] 控制器层",
		"file":     filepath.Base(vuln.File),
		"line":     max(1, vuln.Line-10),
	})

	// 添加Service层
	chain = append(chain, gin.H{
		"nodeType": "service",
		"label":    "[Service] 业务逻辑层",
		"file":     filepath.Base(vuln.File),
		"line":     max(1, vuln.Line-5),
	})

	// 添加DAO层
	chain = append(chain, gin.H{
		"nodeType": "dao",
		"label":    "[DAO] 数据访问层",
		"file":     filepath.Base(vuln.File),
		"line":     max(1, vuln.Line-2),
	})

	// 添加当前漏洞节点
	classNameForCurrent, _ := h.extractClassAndFunction(vuln, sourcePath)
	chain = append(chain, gin.H{
		"nodeType": "current",
		"label":    translateVulnType(vuln.Type),
		"severity": vuln.Severity,
		"class":    classNameForCurrent,
		"file":     filepath.Base(vuln.File),
		"line":     vuln.Line,
	})

	// 添加Sink节点
	if sinkNode != nil {
		className, funcName := h.extractClassAndFunctionFromNode(sinkNode, sourcePath)
		displayLabel := funcName
		if displayLabel == "" {
			displayLabel = className
		}
		if displayLabel == "" {
			displayLabel = "[Sink] 危险操作"
		} else {
			displayLabel = "[Sink] " + displayLabel
		}

		chain = append(chain, gin.H{
			"nodeType": "sink",
			"label":    displayLabel,
			"class":    className,
			"function": funcName,
			"file":     filepath.Base(sinkNode.FilePath),
			"line":     sinkNode.Line,
		})
	}

	return chain
}

// extractClassAndFunctionFromNode 从节点提取类名和函数名
func (h *TaskHandler) extractClassAndFunctionFromNode(node *mcp.CallGraphNode, sourcePath string) (string, string) {
	if node == nil {
		return "", ""
	}

	className := node.ClassName
	funcName := node.Name

	// 如果节点没有类名，尝试从文件中提取
	if className == "" && node.FilePath != "" {
		content, err := os.ReadFile(node.FilePath)
		if err == nil {
			className = extractClassNameFromContent(string(content))
		}
	}

	return className, funcName
}

// extractClassNameFromContent 从文件内容中提取类名
func extractClassNameFromContent(content string) string {
	lines := strings.Split(content, "\n")

	// 只搜索前100行
	maxLines := 100
	if len(lines) < maxLines {
		maxLines = len(lines)
	}

	classPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:public|private|protected)?\s*class\s+([A-Z][a-zA-Z0-9_]*)`),
		regexp.MustCompile(`(?:public|private|protected)?\s*interface\s+([A-Z][a-zA-Z0-9_]*)`),
	}

	for i := 0; i < maxLines; i++ {
		line := strings.TrimSpace(lines[i])
		for _, pattern := range classPatterns {
			if match := pattern.FindStringSubmatch(line); len(match) >= 2 {
				return match[1]
			}
		}
	}

	return ""
}

// extractDescriptionFromReport 从审计报告中提取准确的漏洞描述
func (h *TaskHandler) extractDescriptionFromReport(report string, vuln model.Vulnerability) string {
	if report == "" {
		return vuln.Description
	}

	// 尝试从报告中找到该漏洞的详细描述
	// 查找包含文件名和行号的段落
	patterns := []string{
		// 匹配"漏洞 X"后面跟着的描述
		fmt.Sprintf(`(?m)##\\s+\\d+\\s+.*?%s.*?(?:\\n\\n|\\n##|$)`, regexp.QuoteMeta(vuln.File)),
		// 匹配包含漏洞类型和位置的段落
		fmt.Sprintf(`(?m)(?:漏洞类型|类型)[:：]\\s*%s.*?(?:\\n\\n|\\n##|$)`, regexp.QuoteMeta(vuln.Type)),
		// 匹配包含文件路径的段落
		fmt.Sprintf(`(?m)(?:文件|路径|位置)[:：]\\s*.*?%s.*?(?:\\n\\n|\\n##|$)`, regexp.QuoteMeta(filepath.Base(vuln.File))),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindString(report)
		if matches != "" {
			// 清理文本，去除 markdown 格式
			matches = cleanMarkdown(matches)
			if len(matches) > 50 {
				return matches
			}
		}
	}

	// 如果从报告中提取失败，返回数据库中的描述（清理后）
	if vuln.Description != "" && !strings.Contains(vuln.Description, "发现安全漏洞") {
		return vuln.Description
	}

	// 从Analysis中提取
	if vuln.Analysis != "" {
		lines := strings.Split(vuln.Analysis, "\n")
		var descLines []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "```") {
				descLines = append(descLines, line)
				if len(descLines) >= 2 {
					break
				}
			}
		}
		if len(descLines) > 0 {
			return strings.Join(descLines, "\n")
		}
	}

	// 生成描述
	desc := translateVulnType(vuln.Type)
	if vuln.File != "" {
		desc += " - " + filepath.Base(vuln.File)
	}
	if vuln.Line > 0 {
		desc += fmt.Sprintf(" 第%d行", vuln.Line)
	}

	return desc
}

// extractClassAndFunction 提取类名和函数名
func (h *TaskHandler) extractClassAndFunction(vuln model.Vulnerability, sourcePath string) (string, string) {
	if vuln.File == "" || sourcePath == "" {
		return "", ""
	}

	// 查找源文件
	actualPath := h.findFileInSandbox(sourcePath, vuln.File)
	if actualPath == "" {
		// 从描述中提取
		return extractFromDescription(vuln.Description, vuln.Analysis)
	}

	content, err := os.ReadFile(actualPath)
	if err != nil {
		return extractFromDescription(vuln.Description, vuln.Analysis)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return extractFromDescription(vuln.Description, vuln.Analysis)
	}

	// 搜索范围
	searchStart := 0
	searchEnd := len(lines)
	if vuln.Line > 0 {
		searchStart = max(0, vuln.Line-20)
		searchEnd = min(len(lines), vuln.Line+20)
	}

	var className, funcName string

	// 匹配类名
	classPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:public|private|protected)?\s*class\s+([A-Z][a-zA-Z0-9_]*)`),
		regexp.MustCompile(`(?:public|private|protected)?\s*interface\s+([A-Z][a-zA-Z0-9_]*)`),
	}

	for i := searchStart; i < searchEnd && className == ""; i++ {
		line := strings.TrimSpace(lines[i])
		for _, pattern := range classPatterns {
			if match := pattern.FindStringSubmatch(line); len(match) >= 2 {
				className = match[1]
				break
			}
		}
	}

	// 匹配方法名
	funcPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:public|private|protected|final|static)?\s*(?:void|int|String|boolean|char|byte|short|long|float|double|Object|[A-Z][A-Za-z0-9_]*)\s+([a-z][A-Za-z0-9_]*)\s*\(`),
		regexp.MustCompile(`@\w+\s*(?:public|private|protected)?\s*(?:void|int|String|[A-Z][A-Za-z0-9_]*)\s+([a-z][A-Za-z0-9_]*)\s*\(`),
	}

	for i := searchStart; i < searchEnd && funcName == ""; i++ {
		line := lines[i]
		for _, pattern := range funcPatterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) >= 2 {
				fn := matches[1]
				if !isCommonKeyword(fn) {
					funcName = fn
					break
				}
			}
		}
	}

	// 如果都没找到，从描述中提取
	if className == "" || funcName == "" {
		c, f := extractFromDescription(vuln.Description, vuln.Analysis)
		if className == "" {
			className = c
		}
		if funcName == "" {
			funcName = f
		}
	}

	return className, funcName
}

// extractFromDescription 从描述中提取类名和函数名
func extractFromDescription(description, analysis string) (string, string) {
	var className, funcName string

	text := description + " " + analysis

	// 匹配 类.方法
	pattern1 := regexp.MustCompile(`([A-Z][a-zA-Z0-9_]*)\.([a-z][a-zA-Z0-9_]*)\s*\(`)
	if match := pattern1.FindStringSubmatch(text); len(match) >= 3 {
		className = match[1]
		funcName = match[2]
	}

	// 匹配 "在XXX类的YYY方法中"
	pattern2 := regexp.MustCompile(`(?:类|class)\s*([A-Z][a-zA-Z0-9_]*)\s*(?:的|')?\s*(?:方法|method|function)\s*([a-z][a-zA-Z0-9_]*)`)
	if className == "" || funcName == "" {
		if match := pattern2.FindStringSubmatch(text); len(match) >= 3 {
			if className == "" {
				className = match[1]
			}
			if funcName == "" {
				funcName = match[2]
			}
		}
	}

	return className, funcName
}

// isCommonKeyword 检查是否为常见关键字
func isCommonKeyword(name string) bool {
	keywords := map[string]bool{
		"if": true, "else": true, "for": true, "while": true, "do": true,
		"switch": true, "case": true, "break": true, "continue": true,
		"return": true, "throw": true, "try": true, "catch": true,
		"class": true, "interface": true, "extends": true, "implements": true,
		"public": true, "private": true, "protected": true, "static": true,
		"final": true, "void": true, "int": true, "long": true, "String": true,
	}
	return keywords[name]
}

// cleanMarkdown 清理Markdown格式
func cleanMarkdown(text string) string {
	// 移除 markdown 标题符号
	text = regexp.MustCompile(`^#+\s*`).ReplaceAllString(text, "")
	// 移除代码块标记
	text = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllString(text, "")
	// 移除加粗标记
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "$1")
	// 移除斜体标记
	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "$1")
	// 移除链接
	text = regexp.MustCompile(`\[(.+?)\]\(.+?\)`).ReplaceAllString(text, "$1")
	// 移除多余空白
	text = strings.TrimSpace(text)
	return text
}

// translateVulnType 将漏洞类型翻译成中文
func translateVulnType(vulnType string) string {
	if vulnType == "" {
		return "未知漏洞"
	}

	translations := map[string]string{
		"SQL Injection":                       "SQL注入",
		"SQL injection":                       "SQL注入",
		"Path Traversal":                      "路径遍历",
		"Local File Inclusion":                "本地文件包含",
		"Remote Code Execution":               "远程代码执行",
		"Cross-Site Scripting":                "跨站脚本攻击",
		"XSS":                                 "跨站脚本攻击",
		"Command Injection":                   "命令注入",
		"OS Command Injection":                "系统命令注入",
		"XML External Entity":                 "XML外部实体",
		"XXE":                                 "XML外部实体",
		"Deserialization":                     "反序列化漏洞",
		"Insecure Deserialization":            "不安全的反序列化",
		"Weak Cryptography":                   "弱加密",
		"Hardcoded Password":                  "硬编码密码",
		"Hard-coded credentials":              "硬编码凭证",
		"Sensitive Data Exposure":             "敏感数据泄露",
		"Broken Authentication":               "身份验证失效",
		"Security Misconfiguration":           "安全配置错误",
		"Missing Authorization":               "授权缺失",
		"Insufficient Input Validation":       "输入验证不足",
		"Improper Input Validation":           "输入验证不完整",
		"CSRF":                                "跨站请求伪造",
		"Open Redirect":                       "开放重定向",
		"URL Redirect":                        "URL重定向",
		"Server-Side Request Forgery":         "服务器端请求伪造",
		"SSRF":                                "服务器端请求伪造",
		"Race Condition":                      "竞态条件",
		"Buffer Overflow":                     "缓冲区溢出",
		"Integer Overflow":                    "整数溢出",
		"Format String":                       "格式化字符串",
		"LDAP Injection":                      "LDAP注入",
		"Template Injection":                  "模板注入",
		"SSTI":                                "服务端模板注入",
		"JWT Injection":                       "JWT注入",
		"JNDI Injection":                      "JNDI注入",
		"SpEL Injection":                      "SpEL表达式注入",
		"OGNL Injection":                      "OGNL注入",
		"Prototype Pollution":                 "原型链污染",
		"Security Vulnerability":              "安全漏洞",
		"Security Vulnerability (Cross-File)": "跨文件安全漏洞",
		"Code Quality":                        "代码质量问题",
	}

	if translated, ok := translations[vulnType]; ok {
		return translated
	}

	// 尝试不区分大小写匹配
	vulnTypeLower := strings.ToLower(vulnType)
	for key, value := range translations {
		if strings.ToLower(key) == vulnTypeLower {
			return value
		}
	}

	return strings.ReplaceAll(vulnType, "_", " ")
}

// cleanStringForDB 清理字符串中的无效字符
func cleanStringForDB(s string) string {
	if s == "" {
		return ""
	}

	bytes := []byte(s)
	var result []byte
	i := 0

	for i < len(bytes) {
		b := bytes[i]

		if b < 0x80 {
			result = append(result, b)
			i++
			continue
		}

		if b >= 0xC0 && b <= 0xFD {
			bytesNeeded := 0
			if b >= 0xC0 && b <= 0xDF {
				bytesNeeded = 2
			} else if b >= 0xE0 && b <= 0xEF {
				bytesNeeded = 3
			} else if b >= 0xF0 && b <= 0xF7 {
				bytesNeeded = 4
			} else {
				if i+1 < len(bytes) {
					low := bytes[i+1]
					if b >= 0x81 && b <= 0xFE &&
						((low >= 0x40 && low <= 0x7E) || (low >= 0x80 && low <= 0xFE)) {
						result = append(result, '?', '?')
						i += 2
						continue
					}
				}
				result = append(result, '?')
				i++
				continue
			}

			if i+bytesNeeded-1 < len(bytes) {
				validUTF8 := true
				for j := 1; j < bytesNeeded; j++ {
					bb := bytes[i+j]
					if bb < 0x80 || bb > 0xBF {
						validUTF8 = false
						break
					}
				}

				if validUTF8 {
					for j := 0; j < bytesNeeded; j++ {
						result = append(result, bytes[i+j])
					}
					i += bytesNeeded
					continue
				}
			}
		}

		result = append(result, '?')
		i++
	}

	cleanedString := string(result)
	var finalResult strings.Builder
	finalResult.Grow(len(cleanedString))

	for _, r := range cleanedString {
		if r == '\n' || r == '\t' || r == '\r' || (r >= 32 && r <= 126) || r >= 128 {
			finalResult.WriteRune(r)
		}
	}

	return finalResult.String()
}

// DownloadReport 下载完整报告
func (h *TaskHandler) DownloadReport(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	id := c.Param("id")
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", id, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	reportPath := task.ReportPath
	if reportPath == "" {
		c.JSON(http.StatusNotFound, util.Error(404, "报告文件不存在"))
		return
	}

	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, util.Error(404, "报告文件不存在"))
		return
	}

	content, err := os.ReadFile(reportPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "读取报告文件失败"))
		return
	}

	fileName := filepath.Base(reportPath)
	if fileName == "" {
		fileName = "audit_report.md"
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/octet-stream")

	c.Data(http.StatusOK, "application/octet-stream", content)
}
