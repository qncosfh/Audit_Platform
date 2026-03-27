package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"platform/model"
	"platform/util"
)

type ReportHandler struct{}

func NewReportHandler() *ReportHandler {
	return &ReportHandler{}
}

// ExportReport 导出审计报告
func (h *ReportHandler) ExportReport(c *gin.Context) {
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

	// 生成报告文件
	reportPath, err := h.generateReportFile(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "生成报告文件失败"))
		return
	}

	// 返回报告文件路径
	c.JSON(http.StatusOK, util.Success(gin.H{
		"url":      reportPath,
		"filename": filepath.Base(reportPath),
	}))
}

// DownloadReport 下载报告文件
func (h *ReportHandler) DownloadReport(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, util.Error(400, "任务未完成，无法下载报告"))
		return
	}

	// 生成报告文件
	reportPath, err := h.generateReportFile(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "生成报告文件失败"))
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, util.Error(404, "报告文件不存在"))
		return
	}

	// 设置下载头
	filename := fmt.Sprintf("audit_report_%d.md", task.ID)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/markdown")

	// 发送文件
	c.File(reportPath)
}

// generateReportFile 生成报告文件
func (h *ReportHandler) generateReportFile(task model.Task) (string, error) {
	// 确保reports目录存在
	reportsDir := "reports"
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return "", err
	}

	// 生成报告文件名
	filename := fmt.Sprintf("audit_report_%d.md", task.ID)
	filepath := filepath.Join(reportsDir, filename)

	// 创建报告内容
	reportContent := h.buildReportContent(task)

	// 写入文件
	err := os.WriteFile(filepath, []byte(reportContent), 0644)
	if err != nil {
		return "", err
	}

	return filepath, nil
}

// buildReportContent 构建报告内容
func (h *ReportHandler) buildReportContent(task model.Task) string {
	var content strings.Builder

	// 报告标题
	content.WriteString("# 代码安全审计报告\n\n")

	// 基本信息
	content.WriteString("## 基本信息\n\n")
	content.WriteString(fmt.Sprintf("- **任务名称**: %s\n", task.Name))
	content.WriteString(fmt.Sprintf("- **任务描述**: %s\n", task.Description))
	content.WriteString(fmt.Sprintf("- **创建时间**: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("- **完成时间**: %s\n", task.UpdatedAt.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("- **代码源**: %s\n", task.CodeSource.Name))
	content.WriteString(fmt.Sprintf("- **模型配置**: %s\n", task.ModelConfig.Name))
	content.WriteString("\n")

	// 审计结果
	content.WriteString("## 审计结果\n\n")
	if task.Result != "" {
		content.WriteString(task.Result)
	} else {
		content.WriteString("暂无审计结果\n")
	}

	// 执行统计
	content.WriteString("\n## 执行统计\n\n")
	content.WriteString(fmt.Sprintf("- **漏洞数量**: %d\n", h.countVulnerabilities(task.Result)))
	content.WriteString(fmt.Sprintf("- **扫描文件**: %d\n", h.countScannedFiles(task.CodeSource.Path)))
	content.WriteString(fmt.Sprintf("- **执行状态**: %s\n", task.Status))
	content.WriteString("\n")

	// 免责声明
	content.WriteString("## 免责声明\n\n")
	content.WriteString("本报告仅供参考，实际安全状况可能与报告内容存在差异。建议结合人工审计进行综合评估。\n")

	return content.String()
}

// countVulnerabilities 统计漏洞数量
func (h *ReportHandler) countVulnerabilities(result string) int {
	if result == "" {
		return 0
	}

	// 简单的漏洞统计，可以根据实际结果格式调整
	vulnerabilityKeywords := []string{
		"SQL注入",
		"XSS",
		"CSRF",
		"文件上传",
		"命令注入",
		"路径遍历",
		"认证绕过",
		"权限提升",
	}

	count := 0
	for _, keyword := range vulnerabilityKeywords {
		if strings.Contains(result, keyword) {
			count++
		}
	}

	return count
}

// countScannedFiles 统计扫描文件数量
func (h *ReportHandler) countScannedFiles(path string) int {
	if path == "" {
		return 0
	}

	count := 0
	filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			count++
		}

		return nil
	})

	return count
}

// ListReports 列出所有报告
func (h *ReportHandler) ListReports(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	// 获取用户的所有已完成任务
	var tasks []model.Task
	if err := util.DB.Where("user_id = ? AND status = ?", userID, model.TaskStatusCompleted).
		Preload("CodeSource").
		Preload("ModelConfig").
		Order("updated_at DESC").
		Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, util.Error(500, "获取报告列表失败"))
		return
	}

	// 构建报告列表
	var reports []gin.H
	for _, task := range tasks {
		reportPath := fmt.Sprintf("/reports/%d.md", task.ID)
		reports = append(reports, gin.H{
			"id":              task.ID,
			"name":            task.Name,
			"filename":        fmt.Sprintf("audit_report_%d.md", task.ID),
			"created_at":      task.CreatedAt,
			"updated_at":      task.UpdatedAt,
			"url":             reportPath,
			"vulnerabilities": h.countVulnerabilities(task.Result),
			"scanned_files":   h.countScannedFiles(task.CodeSource.Path),
		})
	}

	c.JSON(http.StatusOK, util.Success(reports))
}

// DeleteReport 删除报告文件
func (h *ReportHandler) DeleteReport(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, util.Error(400, "任务未完成，无法删除报告"))
		return
	}

	// 删除报告文件
	filename := fmt.Sprintf("audit_report_%d.md", task.ID)
	filepath := filepath.Join("reports", filename)

	if err := os.Remove(filepath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, util.Success("报告文件不存在，但任务记录已删除"))
		} else {
			c.JSON(http.StatusInternalServerError, util.Error(500, "删除报告文件失败"))
			return
		}
	}

	c.JSON(http.StatusOK, util.Success("报告删除成功"))
}
