package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"platform/model"
	"platform/util"
)

// AnalysisHandler 代码分析处理器
type AnalysisHandler struct{}

// NewAnalysisHandler 创建分析处理器
func NewAnalysisHandler() *AnalysisHandler {
	return &AnalysisHandler{}
}

// GetVulnerabilities 获取漏洞列表
func (h *AnalysisHandler) GetVulnerabilities(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")
	if _, err := strconv.ParseUint(taskID, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "无效的任务ID"))
		return
	}

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 获取漏洞列表
	var vulnerabilities []model.Vulnerability
	severity := c.Query("severity")
	query := util.DB.Where("task_id = ?", taskID)

	if severity != "" {
		query = query.Where("severity = ?", severity)
	}

	query.Order("CASE severity WHEN 'Critical' THEN 1 WHEN 'High' THEN 2 WHEN 'Medium' THEN 3 ELSE 4 END, id ASC")
	query.Find(&vulnerabilities)

	// 统计各严重程度数量 - 合并为一次查询
	var stats struct {
		Critical int64 `json:"critical"`
		High     int64 `json:"high"`
		Medium   int64 `json:"medium"`
		Low      int64 `json:"low"`
		Info     int64 `json:"info"`
	}

	// 使用单次聚合查询获取所有统计
	util.DB.Model(&model.Vulnerability{}).
		Where("task_id = ?", taskID).
		Select("COALESCE(SUM(CASE WHEN severity = 'Critical' THEN 1 ELSE 0 END), 0) as critical, " +
			"COALESCE(SUM(CASE WHEN severity = 'High' THEN 1 ELSE 0 END), 0) as high, " +
			"COALESCE(SUM(CASE WHEN severity = 'Medium' THEN 1 ELSE 0 END), 0) as medium, " +
			"COALESCE(SUM(CASE WHEN severity = 'Low' THEN 1 ELSE 0 END), 0) as low, " +
			"COALESCE(SUM(CASE WHEN severity = 'Info' THEN 1 ELSE 0 END), 0) as info").
		Scan(&stats)

	c.JSON(http.StatusOK, util.Success(gin.H{
		"items": vulnerabilities,
		"stats": stats,
		"total": len(vulnerabilities),
	}))
}

// GetVulnerability 获取单个漏洞详情
func (h *AnalysisHandler) GetVulnerability(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	vulnID := c.Param("vulnId")
	vulnIDUint, err := strconv.ParseUint(vulnID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "无效的漏洞ID"))
		return
	}

	var vulnerability model.Vulnerability
	if err := util.DB.First(&vulnerability, vulnIDUint).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "漏洞不存在"))
		return
	}

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", vulnerability.TaskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	c.JSON(http.StatusOK, util.Success(vulnerability))
}

// GetExploitChains 获取漏洞利用链
func (h *AnalysisHandler) GetExploitChains(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")
	if _, err := strconv.ParseUint(taskID, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "无效的任务ID"))
		return
	}

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	var chains []model.VulnerabilityChain
	util.DB.Where("task_id = ?", taskID).Order("total_score DESC").Find(&chains)

	c.JSON(http.StatusOK, util.Success(gin.H{
		"items": chains,
		"total": len(chains),
	}))
}

// GetProjectStats 获取项目统计信息
func (h *AnalysisHandler) GetProjectStats(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")
	if _, err := strconv.ParseUint(taskID, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, util.Error(400, "无效的任务ID"))
		return
	}

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	var stats model.ProjectStats
	if err := util.DB.Where("task_id = ?", taskID).First(&stats).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "统计信息不存在"))
		return
	}

	c.JSON(http.StatusOK, util.Success(stats))
}

// GetCrossFileAnalysis 获取跨文件分析结果
func (h *AnalysisHandler) GetCrossFileAnalysis(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"crossFileAnalysis": task.CrossFileAnalysis,
		"dataFlowAnalysis":  task.DataFlowAnalysis,
		"callChainAnalysis": task.CallChainAnalysis,
	}))
}

// GetDependencyAnalysis 获取依赖分析结果
func (h *AnalysisHandler) GetDependencyAnalysis(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"dependencyAnalysis": task.DependencyAnalysis,
	}))
}

// GetVulnerabilityTypes 获取漏洞类型分布
func (h *AnalysisHandler) GetVulnerabilityTypes(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 统计漏洞类型分布
	var typeStats []struct {
		Type  string `json:"type"`
		Count int    `json:"count"`
	}

	util.DB.Model(&model.Vulnerability{}).
		Select("type, COUNT(*) as count").
		Where("task_id = ?", taskID).
		Group("type").
		Order("count DESC").
		Scan(&typeStats)

	c.JSON(http.StatusOK, util.Success(typeStats))
}

// GetFilesBySeverity 按严重程度获取文件列表
func (h *AnalysisHandler) GetFilesBySeverity(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")
	severity := c.Query("severity")

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	// 获取包含该严重程度漏洞的文件列表
	var files []struct {
		File  string `json:"file"`
		Count int    `json:"count"`
		Types string `json:"types"`
	}

	// PostgreSQL 使用 string_agg 而不是 GROUP_CONCAT
	util.DB.Model(&model.Vulnerability{}).
		Select("file, COUNT(*) as count, string_agg(DISTINCT type, ',') as types").
		Where("task_id = ? AND severity = ?", taskID, severity).
		Group("file").
		Order("count DESC").
		Scan(&files)

	c.JSON(http.StatusOK, util.Success(files))
}

// ExportReport 导出报告
func (h *AnalysisHandler) ExportReport(c *gin.Context) {
	userID := util.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, util.Error(401, "未授权"))
		return
	}

	taskID := c.Param("id")
	format := c.DefaultQuery("format", "markdown")

	// 验证任务属于当前用户
	var task model.Task
	if err := util.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, util.Error(404, "任务不存在"))
		return
	}

	if task.Status != model.TaskStatusCompleted {
		c.JSON(http.StatusBadRequest, util.Error(400, "任务未完成，无法导出报告"))
		return
	}

	// 获取漏洞列表
	var vulnerabilities []model.Vulnerability
	util.DB.Where("task_id = ?", taskID).Order("CASE severity WHEN 'Critical' THEN 1 WHEN 'High' THEN 2 WHEN 'Medium' THEN 3 ELSE 4 END").Find(&vulnerabilities)

	// 获取统计信息
	var stats model.ProjectStats
	util.DB.Where("task_id = ?", taskID).First(&stats)

	// 根据格式生成报告
	var reportContent string
	switch format {
	case "json":
		reportContent = generateJSONReport(task, vulnerabilities, stats)
	case "html":
		reportContent = generateHTMLReport(task, vulnerabilities, stats)
	default: // markdown
		reportContent = task.Result
	}

	c.JSON(http.StatusOK, util.Success(gin.H{
		"content":  reportContent,
		"format":   format,
		"filename": task.Name + "_report." + format,
	}))
}

// generateJSONReport 生成JSON格式报告
func generateJSONReport(task model.Task, vulnerabilities []model.Vulnerability, stats model.ProjectStats) string {
	// 返回Markdown格式作为后备
	_ = stats
	_ = vulnerabilities
	return task.Result
}

// generateHTMLReport 生成HTML格式报告
func generateHTMLReport(task model.Task, vulnerabilities []model.Vulnerability, stats model.ProjectStats) string {
	var html strings.Builder
	html.WriteString("<!DOCTYPE html>\n<html>\n<head>\n    <meta charset=\"UTF-8\">\n    <title>安全审计报告 - " + task.Name + "</title>\n    <style>\n        body { font-family: Arial, sans-serif; margin: 40px; }\n        h1 { color: #333; }\n        .summary { background: #f5f5f5; padding: 20px; border-radius: 5px; }\n        .vuln { border-left: 4px solid #ff0000; padding: 10px; margin: 10px 0; background: #fff5f5; }\n        .critical { border-color: #d32f2f; }\n        .high { border-color: #f57c00; }\n        .medium { border-color: #fbc02d; }\n        .low { border-color: #689f38; }\n        .code { background: #272822; color: #f8f8f2; padding: 10px; border-radius: 3px; overflow-x: auto; }\n    </style>\n</head>\n<body>\n    <h1>安全审计报告</h1>\n    <div class=\"summary\">\n        <h2>项目概述</h2>\n        <p><strong>项目名称:</strong> " + task.Name + "</p>\n        <p><strong>检测语言:</strong> " + task.DetectedLanguage + "</p>\n        <p><strong>扫描文件:</strong> " + strconv.Itoa(task.ScannedFiles) + "</p>\n        <p><strong>安全评分:</strong> " + strconv.Itoa(task.SecurityScore) + "/100</p>\n        <p><strong>风险等级:</strong> " + task.RiskLevel + "</p>\n    </div>\n    <h2>漏洞统计</h2>\n    <ul>\n        <li>严重: " + strconv.Itoa(stats.CriticalVulns) + "</li>\n        <li>高危: " + strconv.Itoa(stats.HighVulns) + "</li>\n        <li>中危: " + strconv.Itoa(stats.MediumVulns) + "</li>\n        <li>低危: " + strconv.Itoa(stats.LowVulns) + "</li>\n    </ul>\n    <h2>漏洞详情</h2>\n")

	for _, v := range vulnerabilities {
		html.WriteString("    <div class=\"vuln " + v.Severity + "\">\n")
		html.WriteString("        <h3>" + v.Type + " [" + v.Severity + "]</h3>\n")
		html.WriteString("        <p><strong>文件:</strong> " + v.File + "</p>\n")
		html.WriteString("        <p><strong>行号:</strong> " + strconv.Itoa(v.Line) + "</p>\n")
		html.WriteString("        <p>" + v.Description + "</p>\n")
		html.WriteString("        <h4>修复建议</h4>\n")
		html.WriteString("        <p>" + v.FixSuggestion + "</p>\n")
		html.WriteString("    </div>\n")
	}

	html.WriteString("\n</body>\n</html>")

	return html.String()
}
