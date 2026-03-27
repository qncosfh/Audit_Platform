package mcp

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"platform/model"
	"platform/util"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// 审计服务日志器 - 避免在高频循环中打印日志
var auditLogger = log.New(os.Stdout, "[审计] ", log.LstdFlags)

// AuditService 实现MCP服务接口
type AuditService struct {
	client      *openai.Client
	modelName   string
	temperature float32
	maxTokens   int
	// 优化：添加缓存和批量处理
	analysisCache *AnalysisCache
	batchSize     int
	// 优化：可配置的并发数
	maxConcurrentWorkers int
}

// AnalysisCache 分析结果缓存
type AnalysisCache struct {
	mu        sync.RWMutex
	cache     map[string]*CacheEntry
	maxSize   int
	hitCount  int64
	missCount int64
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Result    string
	Timestamp time.Time
	ExpiresAt time.Time
}

// getCacheKey 生成缓存键
func (c *AnalysisCache) getCacheKey(code, filePath, prompt, language string) string {
	// 使用代码哈希作为缓存键的一部分
	hash := fnv64a([]byte(code + filePath + prompt + language))
	return fmt.Sprintf("%x", hash)
}

// Get 获取缓存
func (c *AnalysisCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists {
		c.missCount++
		return "", false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.missCount++
		return "", false
	}

	c.hitCount++
	return entry.Result, true
}

// Set 设置缓存
func (c *AnalysisCache) Set(key, result string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果缓存已满，删除最老的条目
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}

	c.cache[key] = &CacheEntry{
		Result:    result,
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
}

// cleanup 清理过期缓存
func (c *AnalysisCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.After(entry.ExpiresAt) {
			delete(c.cache, key)
		}
	}
}

// evictOldest 删除最老的缓存条目
func (c *AnalysisCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.cache {
		if oldestTime.IsZero() || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// GetStats 获取缓存统计
func (c *AnalysisCache) GetStats() (hits, misses int64, ratio float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hits = c.hitCount
	misses = c.missCount
	total := hits + misses
	if total > 0 {
		ratio = float64(hits) / float64(total)
	}
	return
}

// fnv64a FNV-1a 64位哈希
func fnv64a(data []byte) uint64 {
	hash := uint64(2166136261)
	for _, b := range data {
		hash ^= uint64(b)
		hash *= 0x100000001b3
	}
	return hash
}

// NewAuditServiceWithModel 创建带有自定义模型参数的审计服务
func NewAuditServiceWithModel(apiKey, baseURL string, modelName string, temperature float32, maxTokens int) *AuditService {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	return &AuditService{
		client:      client,
		modelName:   modelName,
		temperature: temperature,
		maxTokens:   maxTokens,
	}
}

// ListDirectories 列出目录结构
func (s *AuditService) ListDirectories(path string) ([]string, error) {
	var directories []string

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			directories = append(directories, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("列出目录失败: %v", err)
	}

	return directories, nil
}

// ReadFile 读取文件内容
func (s *AuditService) ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	return string(content), nil
}

// SearchFiles 搜索文件中的特定字符串
func (s *AuditService) SearchFiles(path, pattern string) ([]string, error) {
	var results []string

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		content, err := s.ReadFile(filePath)
		if err != nil {
			return nil
		}

		if strings.Contains(content, pattern) {
			results = append(results, filePath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("搜索文件失败: %v", err)
	}

	return results, nil
}

// ExecuteCurl 执行curl命令验证漏洞
func (s *AuditService) ExecuteCurl(url, method, data string) (string, error) {
	cmd := exec.Command("curl", "-s", "-X", method)

	if data != "" {
		cmd.Args = append(cmd.Args, "-d", data)
	}

	cmd.Args = append(cmd.Args, url)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("执行curl失败: %v", err)
	}

	return string(output), nil
}

// checkChoicesValid 检查Choices是否有效
func checkChoicesValid(resp openai.ChatCompletionResponse) (string, error) {
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("模型返回为空")
	}

	content := resp.Choices[0].Message.Content
	// 检查内容是否为空或只包含空白字符
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("模型返回内容为空")
	}

	return content, nil
}

// GeneratePythonExp 生成Python验证脚本
func (s *AuditService) GeneratePythonExp(vulnerability, target string) (string, error) {
	prompt := fmt.Sprintf(`
请为以下漏洞生成Python验证脚本：

漏洞类型: %s
目标: %s

要求：
1. 生成完整的Python脚本
2. 包含必要的导入语句
3. 实现漏洞验证逻辑
4. 包含错误处理
5. 输出验证结果

请直接返回Python代码，不要包含任何解释或说明。
`, vulnerability, target)

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt}},
			MaxTokens:   s.maxTokens,
			Temperature: s.temperature,
		},
	)

	if err != nil {
		return "", fmt.Errorf("生成Python脚本失败: %v", err)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return "", fmt.Errorf("生成Python脚本失败: %v", err)
	}
	return content, nil
}

// GenerateExpScript 生成漏洞验证脚本
func (s *AuditService) GenerateExpScript(vulnerability, target string, language string) (string, error) {
	switch language {
	case "python":
		return s.GeneratePythonExp(vulnerability, target)
	case "bash":
		return s.GenerateBashExp(vulnerability, target)
	default:
		return "", fmt.Errorf("不支持的语言: %s", language)
	}
}

// GenerateBashExp 生成Bash验证脚本
func (s *AuditService) GenerateBashExp(vulnerability, target string) (string, error) {
	prompt := fmt.Sprintf(`
请为以下漏洞生成Bash验证脚本：

漏洞类型: %s
目标: %s

要求：
1. 生成完整的Bash脚本
2. 实现漏洞验证逻辑
3. 包含错误处理
4. 输出验证结果

请直接返回Bash代码，不要包含任何解释或说明。
`, vulnerability, target)

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt}},
			MaxTokens:   s.maxTokens,
			Temperature: s.temperature,
		},
	)

	if err != nil {
		return "", fmt.Errorf("生成Bash脚本失败: %v", err)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return "", fmt.Errorf("生成Bash脚本失败: %v", err)
	}
	return content, nil
}

// AnalyzeCode 使用AI分析代码
func (s *AuditService) AnalyzeCode(code, prompt string) (string, error) {
	analysisPrompt := fmt.Sprintf(`
请分析以下代码：

%s

分析提示: %s

请以Markdown格式输出分析结果，包括：
1. 漏洞描述
2. 风险等级
3. 修复建议
4. PoC验证代码（如适用）
`, code, prompt)

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: analysisPrompt}},
			MaxTokens:   s.maxTokens,
			Temperature: s.temperature,
		},
	)

	if err != nil {
		return "", fmt.Errorf("AI分析失败: %v", err)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return "", fmt.Errorf("AI分析失败: %v", err)
	}
	return content, nil
}

// GenerateReport 生成审计报告
func (s *AuditService) GenerateReport(results []AuditResult) (string, error) {
	reportPrompt := fmt.Sprintf(`
请根据以下审计结果生成详细的Markdown报告：

%s

报告要求：
1. 包含执行过程记录
2. 详细描述发现的漏洞
3. 提供修复建议
4. 包含验证脚本
5. 格式化为标准的Markdown文档

请直接返回Markdown内容。
`, formatAuditResults(results))

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: reportPrompt}},
			MaxTokens:   s.maxTokens,
			Temperature: s.temperature,
		},
	)

	if err != nil {
		return "", fmt.Errorf("生成报告失败: %v", err)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return "", fmt.Errorf("生成报告失败: %v", err)
	}
	return content, nil
}

// formatAuditResults 格式化审计结果
func formatAuditResults(results []AuditResult) string {
	var resultStr strings.Builder

	for i, result := range results {
		resultStr.WriteString(fmt.Sprintf("## 漏洞 %d\n", i+1))
		resultStr.WriteString(fmt.Sprintf("- **类型**: %s\n", result.Type))
		resultStr.WriteString(fmt.Sprintf("- **文件**: %s\n", result.File))
		resultStr.WriteString(fmt.Sprintf("- **行号**: %d\n", result.Line))
		resultStr.WriteString(fmt.Sprintf("- **描述**: %s\n", result.Description))
		resultStr.WriteString(fmt.Sprintf("- **风险等级**: %s\n", result.Severity))
		resultStr.WriteString(fmt.Sprintf("- **分析**: %s\n", result.Analysis))
		resultStr.WriteString("\n")
	}

	return resultStr.String()
}

// / AuditResult 审计结果
type AuditResult struct {
	Type          string    `json:"type"`
	File          string    `json:"file"`
	Line          int       `json:"line"`
	Description   string    `json:"description"`
	Severity      string    `json:"severity"`
	Analysis      string    `json:"analysis"`
	Timestamp     time.Time `json:"timestamp"`
	Language      string    `json:"language"`
	FixSuggestion string    `json:"fixSuggestion"` // 修复建议
	POC           string    `json:"poc"`           // PoC代码
	CWE           string    `json:"cwe"`           // CWE编号
	Confidence    string    `json:"confidence"`    // 置信度
	AttackVector  string    `json:"attackVector"`  // 攻击向量
	CodeSnippet   string    `json:"codeSnippet"`   // 漏洞代码片段
	IsVerified    bool      `json:"isVerified"`    // 是否通过复查验证
	VerifyResult  string    `json:"verifyResult"`  // 验证结果
}

// FileInfo 文件信息（用于项目结构分析）
type FileInfo struct {
	Path        string `json:"path"`        // 文件路径
	IsDir       bool   `json:"isDir"`       // 是否目录
	Size        int64  `json:"size"`        // 文件大小
	Extension   string `json:"extension"`   // 扩展名
	Priority    int    `json:"priority"`    // 优先级（1-10，数字越大优先级越高）
	Category    string `json:"category"`    // 分类（业务代码/测试代码/配置/依赖/构建产物）
	Language    string `json:"language"`    // 编程语言
	Hash        string `json:"hash"`        // 文件哈希（用于增量分析）
	ShouldAudit bool   `json:"shouldAudit"` // 是否应该审计
	SkipReason  string `json:"skipReason"`  // 跳过原因
}

// ProjectStructure 项目结构分析结果
type ProjectStructure struct {
	RootPath        string         `json:"rootPath"`
	TotalFiles      int            `json:"totalFiles"`
	TotalDirs       int            `json:"totalDirs"`
	TotalSize       int64          `json:"totalSize"`
	Language        string         `json:"language"`
	Files           []FileInfo     `json:"files"`           // 所有文件
	BusinessFiles   []FileInfo     `json:"businessFiles"`   // 业务代码文件（高优先级）
	TestFiles       []FileInfo     `json:"testFiles"`       // 测试代码文件
	ConfigFiles     []FileInfo     `json:"configFiles"`     // 配置文件
	DependencyFiles []FileInfo     `json:"dependencyFiles"` // 依赖文件（低优先级）
	BuildArtifacts  []FileInfo     `json:"buildArtifacts"`  // 构建产物（不审计）
	SkippedFiles    []FileInfo     `json:"skippedFiles"`    // 跳过的文件
	FileTypeStats   map[string]int `json:"fileTypeStats"`   // 文件类型统计
}

// FileHashStore 文件哈希存储（用于增量分析）
type FileHashStore struct {
	mu     sync.RWMutex
	hashes map[string]string // path -> hash
}

// AuditTask 审计任务
type AuditTask struct {
	ID                 string        `json:"id"`
	SourcePath         string        `json:"source_path"`
	Prompt             string        `json:"prompt"`
	Status             string        `json:"status"`
	Progress           int           `json:"progress"`
	Results            []AuditResult `json:"results"`
	Report             string        `json:"report"`
	StartTime          time.Time     `json:"start_time"`
	EndTime            time.Time     `json:"end_time"`
	ScannedFiles       int           `json:"scannedFiles"`
	Language           string        `json:"language"`
	CurrentFile        string        `json:"currentFile"`
	Log                string        `json:"log"`
	VulnerabilityCount int           `json:"vulnerabilityCount"`
	Duration           int           `json:"duration"`
	AILog              string        `json:"aiLog"`            // 大模型交互日志
	CrossFileContext   string        `json:"crossFileContext"` // 跨文件调用链上下文
}

// ExecuteAuditWithContext 执行完整的代码审计 - 支持context取消
func (s *AuditService) ExecuteAuditWithContext(ctx context.Context, task *AuditTask) error {
	return s.executeAuditInternal(ctx, task)
}

// ExecuteAudit 执行完整的代码审计 - 优化版，支持并发和实时更新
func (s *AuditService) ExecuteAudit(task *AuditTask) error {
	return s.executeAuditInternal(context.Background(), task)
}

func (s *AuditService) executeAuditInternal(ctx context.Context, task *AuditTask) error {
	task.Status = "running"
	task.Progress = 0
	task.StartTime = time.Now()
	task.ScannedFiles = 0
	task.Log = ""
	task.VulnerabilityCount = 0

	// 记录开始时间
	startTimestamp := time.Now().UnixMilli()

	// 1. 列出所有源代码文件
	s.appendLog(task, "正在扫描代码文件...")
	updateTaskProgress(task, "正在扫描代码文件...")

	files, err := s.ListCodeFiles(task.SourcePath)
	if err != nil {
		return fmt.Errorf("列出代码文件失败: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("未找到代码文件")
	}

	// 自动检测语言
	language := detectLanguageFromFiles(files)
	task.Language = language
	s.appendLog(task, fmt.Sprintf("检测到语言: %s, 共找到 %d 个代码文件", language, len(files)))
	updateTaskProgress(task, fmt.Sprintf("检测到语言: %s, 共找到 %d 个代码文件", language, len(files)))

	task.Progress = 5

	// 2. 并发分析文件 - 使用goroutine池（动态调整）
	totalFiles := len(files)
	// 根据CPU核心数和文件数量动态调整并发数
	numCPU := runtime.NumCPU()
	concurrentWorkers := numCPU
	if concurrentWorkers > 8 {
		concurrentWorkers = 8 // 最大8个并发
	}
	if totalFiles < 10 {
		concurrentWorkers = 1
	} else if totalFiles < 50 {
		concurrentWorkers = 2
	}

	s.appendLog(task, fmt.Sprintf("启动 %d 个并发 worker 进行分析...", concurrentWorkers))
	updateTaskProgress(task, fmt.Sprintf("启动 %d 个并发 worker 进行分析...", concurrentWorkers))

	// 使用channel控制并发
	fileChan := make(chan string, totalFiles)
	resultChan := make(chan *AuditResult, totalFiles)
	var wg sync.WaitGroup

	// 启动worker
	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for file := range fileChan {
				// 更新当前分析的文件
				task.CurrentFile = file
				updateCurrentFile(task, file)

				// 读取文件内容
				content, err := s.ReadFileContent(file)
				if err != nil {
					s.appendLog(task, fmt.Sprintf("Worker %d: 读取文件失败 %s: %v", workerID, file, err))
					continue
				}

				// 限制文件大小
				if len(content) > 50000 {
					content = content[:50000] + "\n... [文件过大，已截断]"
				}

				// 分析文件
				analysis, err := s.AnalyzeCodeWithFileNameAndLanguage(content, file, task.Prompt, language)
				if err != nil {
					s.appendLog(task, fmt.Sprintf("Worker %d: 分析文件失败 %s: %v", workerID, file, err))
					continue
				}

				// 记录 AI 交互日志
				s.appendAILog(task, fmt.Sprintf("分析文件: %s", file), analysis)

				// 解析结果
				result := s.ParseAnalysisResultWithLanguage(file, analysis, language)
				if result != nil {
					resultChan <- result
				}

				// 更新已扫描文件数
				task.ScannedFiles++
				updateScannedFiles(task, task.ScannedFiles)
			}
		}(i)
	}

	// 发送文件到channel
	go func() {
		for _, file := range files {
			fileChan <- file
		}
		close(fileChan)
	}()

	// 等待所有worker完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	var results []AuditResult
	for result := range resultChan {
		results = append(results, *result)
		task.VulnerabilityCount++
		updateVulnerabilityCount(task, task.VulnerabilityCount)

		// 更新进度 (5% - 80%)
		progress := 5 + (task.ScannedFiles*75)/totalFiles
		if progress > 80 {
			progress = 80
		}
		task.Progress = progress
		updateTaskProgress(task, fmt.Sprintf("已分析 %d/%d 个文件，发现 %d 个漏洞", task.ScannedFiles, totalFiles, task.VulnerabilityCount))
	}

	task.Results = results
	task.Progress = 80

	// 3. 执行深度代码分析（利用链分析）
	s.appendLog(task, "正在执行深度代码分析...")
	updateTaskProgress(task, "正在执行深度代码分析...")

	deepReport, err := s.ExecuteDeepAnalysis(task.SourcePath)
	if err != nil {
		s.appendLog(task, fmt.Sprintf("深度分析失败: %v", err))
	}

	// 4. 生成详细报告
	s.appendLog(task, "正在生成审计报告...")
	updateTaskProgress(task, "正在生成审计报告...")

	report, err := s.GenerateDetailedReport(results, task.SourcePath, task.Prompt, deepReport, language)
	if err != nil {
		return fmt.Errorf("生成报告失败: %v", err)
	}

	// 计算执行时长
	endTimestamp := time.Now().UnixMilli()
	duration := int((endTimestamp - startTimestamp) / 1000)

	task.Report = report
	task.Progress = 100
	task.Status = "completed"
	task.EndTime = time.Now()
	task.Duration = duration
	task.CurrentFile = ""

	s.appendLog(task, fmt.Sprintf("审计完成! 共扫描 %d 个文件，发现 %d 个漏洞，耗时 %d 秒", task.ScannedFiles, task.VulnerabilityCount, duration))
	updateTaskProgress(task, fmt.Sprintf("审计完成! 共扫描 %d 个文件，发现 %d 个漏洞，耗时 %d 秒", task.ScannedFiles, task.VulnerabilityCount, duration))

	return nil
}

// appendLog 添加日志
func (s *AuditService) appendLog(task *AuditTask, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	// 清理无效的 UTF-8 字符，防止数据库写入错误
	validMessage := strings.ToValidUTF8(message, "")
	task.Log += fmt.Sprintf("[%s] %s\n", timestamp, validMessage)
	updateTaskLog(task, task.Log)
}

// appendAILog 添加大模型交互日志（详细版）
func (s *AuditService) appendAILog(task *AuditTask, prompt, response string) {
	dateStr := time.Now().Format("2006-01-02")
	timestamp := time.Now().Format("15:04:05")

	// 提取文件名
	fileName := filepath.Base(task.CurrentFile)

	// 清理 response 中的无效 UTF-8 字符，防止数据库写入错误
	validResponse := strings.ToValidUTF8(response, "")

	// 检查是否有漏洞
	hasVuln := !strings.Contains(validResponse, "未发现安全漏洞") && !strings.Contains(validResponse, "没有发现") && !strings.Contains(validResponse, "未发现")

	if !hasVuln {
		// 无漏洞的简化日志
		task.AILog += fmt.Sprintf("[%s %s] %s ✓ 无漏洞\n", dateStr, timestamp, fileName)
	} else {
		// 有漏洞的详细日志
		// 1. 提取漏洞类型
		vulnType := extractVulnType(validResponse)

		// 2. 提取函数/方法名
		funcName := extractFunctionName(task.CurrentFile, validResponse)

		// 3. 提取代码证据（关键代码片段）
		codeEvidence := extractCodeEvidence(validResponse)
		if codeEvidence == "" {
			codeEvidence = "（暂未提取到代码证据）"
		} else {
			// 限制证据长度
			if len(codeEvidence) > 100 {
				codeEvidence = codeEvidence[:100] + "..."
			}
		}

		// 详细日志格式：日期 文件 函数 存在xx漏洞 证据
		detailLog := fmt.Sprintf("[%s %s] %s → %s 存在 %s (暂未验证，等待后续深分析...) 证据(%s)\n",
			dateStr, timestamp, fileName, funcName, vulnType, codeEvidence)

		task.AILog += detailLog
	}

	// 实时更新数据库并推送WebSocket
	updateAILog(task, task.AILog)

	// 推送进度更新到 WebSocket
	vulnStatus := "✓ 无漏洞"
	if hasVuln {
		vulnStatus = "⚠ 存在潜在漏洞"
	}
	updateTaskProgress(task, fmt.Sprintf("AI分析中: %s (%s)", fileName, vulnStatus))
}

// extractVulnType 从AI响应中提取漏洞类型
func extractVulnType(response string) string {
	// 常见的漏洞类型关键词
	vulnTypes := []string{
		"SQL注入", "命令注入", "代码注入", "远程代码执行", "RCE",
		"跨站脚本", "XSS", "存储型XSS", "反射型XSS", "DOM型XSS",
		"路径遍历", "目录遍历", "文件包含", "任意文件读取", "任意文件写入",
		"不安全的反序列化", "反序列化漏洞", "XXE", "XML外部实体",
		"敏感信息泄露", "信息泄露", "密码泄露", "密钥泄露",
		"认证绕过", "授权问题", "越权访问", "权限绕过",
		"SSRF", "服务器端请求伪造",
		"CSRF", "跨站请求伪造",
		"模板注入", "SSTI", "SST",
		"文件上传", "任意文件上传",
		"日志注入", "格式化字符串",
		"正则表达式拒绝服务", "ReDoS",
		"竞态条件", "TOCTOU",
		"缓冲区溢出", "内存泄漏",
		"硬编码", "硬编码密码",
		"不安全的随机数",
		"危险函数", "不安全函数",
	}

	for _, vuln := range vulnTypes {
		if strings.Contains(response, vuln) {
			return vuln
		}
	}

	// 如果没找到具体类型，尝试提取"漏洞"前面的词
	patterns := []string{
		`(\w+)漏洞`,
		`(\w+)注入`,
		`(\w+)攻击`,
		`存在(\w+)问题`,
		`存在(\w+)风险`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)
		if len(matches) > 1 {
			return matches[1] + "漏洞"
		}
	}

	return "潜在安全漏洞"
}

// extractFunctionName 提取函数/方法名
func extractFunctionName(filePath, response string) string {
	// 1. 尝试从响应中提取函数名
	funcPatterns := []string{
		`函数[：:]\s*(\w+)`,
		`方法[：:]\s*(\w+)`,
		`位于(\w+)\s*\(`, // 位于函数名(
		`(?:调用|执行)\s+(\w+)\s*\(`,
		`(\w+)\s*函数`,
		`(\w+)\s*方法`,
	}

	for _, pattern := range funcPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// 2. 从文件路径提取类名/文件名作为参考
	baseName := filepath.Base(filePath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	// 如果是类文件（如 UserService.java），返回类名
	if ext == ".java" || ext == ".cs" || ext == ".kt" || ext == ".swift" {
		return nameWithoutExt
	}

	// 否则返回文件名
	return nameWithoutExt
}

// extractCodeEvidence 从AI响应中提取代码证据（改进版）
// 核心原则：只提取真正的代码片段，排除注释和误报
func extractCodeEvidence(response string) string {
	// 1. 尝试提取代码块中的内容
	codeBlockPattern := regexp.MustCompile("```[\\s\\S]*?```")
	codeBlocks := codeBlockPattern.FindAllString(response, -1)

	// 过滤掉注释和无用内容
	invalidPatterns := []string{
		"例如：", "比如：", "如：", "例如,",
		"移除", "过滤", "避免", "不要",
		"请确保", "建议", "应该",
		"该代码", "此代码", "这段代码",
	}

	for _, block := range codeBlocks {
		// 去除代码块标记
		block = strings.TrimPrefix(block, "```")
		block = strings.TrimSuffix(block, "```")
		// 去除语言标识
		lines := strings.Split(block, "\n")
		if len(lines) > 0 {
			block = strings.Join(lines[1:], "\n")
		}
		block = strings.TrimSpace(block)

		// 跳过太短的代码块（可能是标题或说明）
		if len(block) < 20 {
			continue
		}

		// 检查是否包含无效模式（注释、建议等）
		isInvalid := false
		for _, pattern := range invalidPatterns {
			if strings.Contains(block, pattern) {
				isInvalid = true
				break
			}
		}
		if isInvalid {
			continue
		}

		// 返回代码块内容（限制长度）
		if len(block) > 150 {
			block = block[:150] + "..."
		}
		return block
	}

	// 2. 尝试提取"问题代码"、"漏洞代码"等标记后的实际代码
	// 更严格的匹配：确保后面是代码而不是建议
	evidencePatterns := []string{
		`问题代码[：:]\s*(\{[\s\S]*?\})`,
		`问题代码[：:]\s*(//[^\n]+)`,
		`漏洞代码[：:]\s*(\{[\s\S]*?\})`,
		`漏洞代码[：:]\s*(//[^\n]+)`,
		`代码如下[：:]\s*(\{[\s\S]*?\})`,
		`示例代码[：:]\s*(\{[\s\S]*?\})`,
	}

	for _, pattern := range evidencePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(response)
		if len(matches) > 1 {
			evidence := strings.TrimSpace(matches[1])
			// 确保是实际代码（包含括号、分号等）
			if len(evidence) > 10 && (strings.Contains(evidence, "(") || strings.Contains(evidence, "{") || strings.Contains(evidence, ";")) {
				if len(evidence) > 150 {
					evidence = evidence[:150] + "..."
				}
				return evidence
			}
		}
	}

	// 3. 如果以上都失败，返回空字符串，让前端显示"（暂未提取到代码证据）"
	// 这样可以避免误报
	return ""
}

// detectLanguageFromFiles 从文件列表检测语言
func detectLanguageFromFiles(files []string) string {
	extensions := make(map[string]int)
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		extensions[ext]++
	}

	// 优先级映射 - 扩展更多语言支持
	priority := map[string]int{
		".java":   10,
		".php":    9,
		".py":     8,
		".cs":     7,
		".go":     6,
		".js":     5,
		".ts":     5,
		".rb":     4,
		".swift":  3,
		".kt":     2,
		".rs":     2, // Rust
		".c":      1,
		".cpp":    1,
		".scala":  1,
		".dart":   1,
		".groovy": 1,
	}

	maxScore := 0
	detected := "java"
	for ext, count := range extensions {
		if score, ok := priority[ext]; ok {
			if count*score > maxScore {
				maxScore = count * score
				switch ext {
				case ".java":
					detected = "java"
				case ".php":
					detected = "php"
				case ".py":
					detected = "python"
				case ".cs":
					detected = "csharp"
				case ".go":
					detected = "go"
				case ".js", ".jsx":
					detected = "javascript"
				case ".ts", ".tsx":
					detected = "typescript"
				case ".rb":
					detected = "ruby"
				case ".swift":
					detected = "swift"
				case ".kt", ".kts":
					detected = "kotlin"
				case ".rs":
					detected = "rust"
				case ".scala":
					detected = "scala"
				case ".dart":
					detected = "dart"
				case ".groovy":
					detected = "groovy"
				case ".c", ".cpp", ".h", ".hpp", ".cxx", ".cc":
					detected = "c_cpp"
				}
			}
		}
	}

	return detected
}

// ==================== 漏洞去重逻辑 ====================

// deduplicateVulnerabilities 去重漏洞列表（细粒度版）
// 去重条件：漏洞类型 + 文件路径 + 代码位置(行号/方法名) + 危险函数
func (s *AuditService) deduplicateVulnerabilities(results []AuditResult) []AuditResult {
	// 使用 map 来跟踪已见过的漏洞
	seen := make(map[string]bool)
	var deduplicated []AuditResult

	for _, vuln := range results {
		// 生成唯一的 key：漏洞类型 + 文件路径 + 位置 + 危险函数（细粒度）
		key := generateVulnKeyFineGrained(vuln)

		if !seen[key] {
			seen[key] = true
			deduplicated = append(deduplicated, vuln)
		}
	}

	duplicatedCount := len(results) - len(deduplicated)
	if duplicatedCount > 0 {
		auditLogger.Printf("去重完成: 去除 %d 个重复漏洞，原始 %d 个, 去重后 %d 个",
			duplicatedCount, len(results), len(deduplicated))
	}

	return deduplicated
}

// generateVulnKey 生成漏洞唯一标识键（简化版，用于兼容）
func generateVulnKey(vuln AuditResult) string {
	return generateVulnKeyFineGrained(vuln)
}

// generateVulnKeyFineGrained 生成漏洞唯一标识键（细粒度版）
// 组合：漏洞类型 + 文件路径 + 代码位置(行号/方法名) + 危险函数
func generateVulnKeyFineGrained(vuln AuditResult) string {
	// 提取漏洞类型
	vulnType := extractVulnerabilityName(vuln.Analysis)
	if vulnType == "" {
		vulnType = vuln.Type
	}

	// 清理文件路径
	filePath := cleanFilePath(vuln.File)

	// 提取代码位置（行号或方法名）
	location := extractVulnLocation(vuln.Analysis)

	// 提取危险函数
	dangerousFunc := extractDangerousFunction(vuln.Analysis)

	// 如果是跨文件分析漏洞，使用特殊标记
	if strings.Contains(filePath, "跨文件") || strings.Contains(vuln.File, "跨文件") {
		return fmt.Sprintf("%s|跨文件|%s|%s", vulnType, location, dangerousFunc)
	}

	// 细粒度key: 类型 + 文件 + 位置 + 危险函数
	return fmt.Sprintf("%s|%s|%s|%s", vulnType, filePath, location, dangerousFunc)
}

// extractVulnLocation 提取漏洞代码位置
func extractVulnLocation(analysis string) string {
	// 尝试提取行号
	linePatterns := []string{
		`(\d+)行`,
		`line[:\s]*(\d+)`,
		`位于.*?[:：](\d+)`,
		`:(\d+)`,
	}

	for _, pattern := range linePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(analysis)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	// 尝试提取方法名
	methodPatterns := []string{
		`函数[：:]\s*(\w+)`,
		`方法[：:]\s*(\w+)`,
		`在\s+(\w+)\s*\(`,
	}

	for _, pattern := range methodPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(analysis)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return "unknown"
}

// extractDangerousFunction 提取危险函数
func extractDangerousFunction(analysis string) string {
	functions := []string{
		"exec", "Runtime.exec", "ProcessBuilder",
		"execute", "Statement", "PreparedStatement",
		"eval", "assert",
		"ObjectInputStream", "readObject", "deserialize",
		"FileInputStream", "FileOutputStream", "readFile",
		"include", "require", "load",
		"system", "shell_exec", "popen",
		"query", "cursor.execute", "sql",
		"subprocess", "os.system",
		"pickle.load", "yaml.load",
	}

	for _, fn := range functions {
		if strings.Contains(analysis, fn) {
			return fn
		}
	}
	return "unknown"
}

// ==================== 漏洞复查验证逻辑 ====================

// verifyVulnerability 复查验证单个漏洞
func (s *AuditService) verifyVulnerability(vuln AuditResult, sourcePath string) (bool, string) {
	// 提取漏洞类型
	vulnType := extractVulnerabilityName(vuln.Analysis)
	if vulnType == "" {
		vulnType = vuln.Type
	}

	// 构造验证提示词
	verifyPrompt := fmt.Sprintf(`你是资深代码安全审计专家，请复查以下漏洞是否真实存在。

## 漏洞信息
- **漏洞类型**: %s
- **文件位置**: %s
- **分析详情**: %s

## 复查要求
请从以下角度进行严格复查：

1. **输入验证**: 代码中是否有明确的用户输入点（请求参数、请求体、URL路径、文件上传等）
2. **危险函数**: 用户输入是否真的进入了危险函数/危险操作
3. **安全措施**: 是否存在有效的输入验证、过滤、转义或参数化处理
4. **利用条件**: 该漏洞是否真的可以被实际利用

## 判断标准
- 如果以上4个条件**全部满足**，返回"验证通过"
- 如果任何一个条件**不满足**，返回"误报"并说明原因

请直接返回以下格式：
- 验证通过：该漏洞真实存在，可以利用
- 误报：[具体原因]

注意：请严格审查，宁可误报也不放过任何可能的真实漏洞。`, vulnType, vuln.File, vuln.Analysis)

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: verifyPrompt}},
			MaxTokens:   s.maxTokens,
			Temperature: s.temperature,
		},
	)

	if err != nil {
		return false, fmt.Sprintf("复查失败: %v", err)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return false, fmt.Sprintf("复查失败: %v", err)
	}

	// 判断验证结果
	content = strings.TrimSpace(content)
	if strings.Contains(content, "验证通过") {
		return true, content
	} else if strings.Contains(content, "误报") {
		return false, content
	}

	// 无法确定，保守处理
	return false, fmt.Sprintf("无法确定验证结果: %s", content[:min(100, len(content))])
}

// verifyAllVulnerabilities 复查所有漏洞（第三阶段）
func (s *AuditService) verifyAllVulnerabilities(task *AuditTask, sourcePath string) {
	s.appendLog(task, "【第三阶段】正在复查验证漏洞...")
	updateTaskProgress(task, "正在复查验证漏洞...")

	totalVulns := len(task.Results)
	if totalVulns == 0 {
		s.appendLog(task, "无漏洞需要复查")
		return
	}

	verifiedCount := 0
	falsePositiveCount := 0
	var verifiedResults []AuditResult

	for i, vuln := range task.Results {
		// 更新进度
		progress := 80 + (i+1)*18/totalVulns // 80% - 98%
		task.Progress = progress
		updateTaskProgress(task, fmt.Sprintf("复查进度: %d/%d (%d%%)", i+1, totalVulns, progress))

		// 复查单个漏洞
		isVerified, verifyResult := s.verifyVulnerability(vuln, sourcePath)

		// 更新漏洞状态
		vuln.IsVerified = isVerified
		vuln.VerifyResult = verifyResult

		if isVerified {
			verifiedCount++
			verifiedResults = append(verifiedResults, vuln)
			s.appendLog(task, fmt.Sprintf("✓ 漏洞验证通过: %s (%s)", extractVulnerabilityName(vuln.Analysis), vuln.File))
		} else {
			falsePositiveCount++
			s.appendLog(task, fmt.Sprintf("✗ 漏洞判定为误报: %s (%s) - %s",
				extractVulnerabilityName(vuln.Analysis), vuln.File, verifyResult))
		}

		// 短暂休息，避免 API 限流
		time.Sleep(100 * time.Millisecond)
	}

	// 更新最终结果
	task.Results = verifiedResults
	task.VulnerabilityCount = verifiedCount

	s.appendLog(task, fmt.Sprintf("复查完成! 验证通过: %d 个, 误报: %d 个", verifiedCount, falsePositiveCount))
}

// ExecuteDeepAnalysis 执行深度代码分析（包含跨文件调用链分析）
func (s *AuditService) ExecuteDeepAnalysis(sourcePath string) (string, error) {
	// 1. 传统的静态分析
	analyzer, err := NewCodeAnalyzer(sourcePath)
	if err != nil {
		return "", err
	}

	basicReport := analyzer.GenerateDeepAnalysisReport()

	// 2. 新增：跨文件调用链分析
	cgAnalyzer, err := NewCallGraphAnalyzer(sourcePath)
	if err != nil {
		// 如果调用链分析失败，回退到基础分析
		return basicReport, nil
	}

	// 构建调用图
	if err := cgAnalyzer.BuildCallGraph(); err != nil {
		return basicReport, nil
	}

	// 生成跨文件分析报告
	crossFileReport := cgAnalyzer.GenerateCrossFileReport()

	// 合并报告
	var mergedReport strings.Builder
	mergedReport.WriteString(basicReport)
	mergedReport.WriteString("\n\n")
	mergedReport.WriteString("## 跨文件调用链分析\n\n")
	mergedReport.WriteString(crossFileReport)

	return mergedReport.String(), nil
}

// ExecuteAuditWithCrossFileAnalysis 执行带跨文件分析的审计（单文件扫描 + 跨文件分析）
// 组合方式：先进行单文件扫描，再进行跨文件深度分析
func (s *AuditService) ExecuteAuditWithCrossFileAnalysis(task *AuditTask) error {
	task.Status = "running"
	task.Progress = 0
	task.StartTime = time.Now()
	task.ScannedFiles = 0
	task.Log = ""
	task.VulnerabilityCount = 0

	// 记录开始时间
	startTimestamp := time.Now().UnixMilli()

	// ==================== 第一阶段：单文件扫描 ====================
	s.appendLog(task, "【第一阶段】正在扫描代码文件...")
	updateTaskProgress(task, "正在扫描代码文件...")

	files, err := s.ListCodeFiles(task.SourcePath)
	if err != nil {
		return fmt.Errorf("列出代码文件失败: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("未找到代码文件")
	}

	// 自动检测语言
	language := detectLanguageFromFiles(files)
	task.Language = language
	s.appendLog(task, fmt.Sprintf("检测到语言: %s, 共找到 %d 个代码文件", language, len(files)))
	updateTaskProgress(task, fmt.Sprintf("检测到语言: %s, 共找到 %d 个代码文件", language, len(files)))

	task.Progress = 5

	// 并发分析文件 - 使用goroutine池（动态调整）
	totalFiles := len(files)
	concurrentWorkers := calculateOptimalWorkers(totalFiles)

	s.appendLog(task, fmt.Sprintf("启动 %d 个并发 worker 进行单文件扫描...", concurrentWorkers))
	updateTaskProgress(task, fmt.Sprintf("启动 %d 个并发 worker 进行单文件扫描...", concurrentWorkers))

	// 使用channel控制并发
	fileChan := make(chan string, totalFiles)
	resultChan := make(chan *AuditResult, totalFiles)
	var wg sync.WaitGroup

	// 启动worker进行单文件分析
	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for file := range fileChan {
				// 更新当前分析的文件
				task.CurrentFile = file
				updateCurrentFile(task, file)

				// 读取文件内容
				content, err := s.ReadFileContent(file)
				if err != nil {
					s.appendLog(task, fmt.Sprintf("Worker %d: 读取文件失败 %s: %v", workerID, file, err))
					continue
				}

				// 限制文件大小
				if len(content) > 50000 {
					content = content[:50000] + "\n... [文件过大，已截断]"
				}

				// 单文件分析（不带跨文件上下文）
				analysis, err := s.AnalyzeCodeWithFileNameAndLanguage(content, file, task.Prompt, language)
				if err != nil {
					s.appendLog(task, fmt.Sprintf("Worker %d: 分析文件失败 %s: %v", workerID, file, err))
					continue
				}

				// 记录 AI 交互日志
				s.appendAILog(task, fmt.Sprintf("单文件扫描: %s", file), analysis)

				// 解析结果
				result := s.ParseAnalysisResultWithLanguage(file, analysis, language)
				if result != nil {
					resultChan <- result
				}

				// 更新已扫描文件数
				task.ScannedFiles++
				updateScannedFiles(task, task.ScannedFiles)
			}
		}(i)
	}

	// 发送文件到channel
	go func() {
		for _, file := range files {
			fileChan <- file
		}
		close(fileChan)
	}()

	// 等待所有worker完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集单文件扫描结果
	var singleFileResults []AuditResult
	for result := range resultChan {
		singleFileResults = append(singleFileResults, *result)
		task.VulnerabilityCount++
		updateVulnerabilityCount(task, task.VulnerabilityCount)

		// 更新进度 (5% - 50%)
		progress := 5 + (task.ScannedFiles*45)/totalFiles
		if progress > 50 {
			progress = 50
		}
		task.Progress = progress
		updateTaskProgress(task, fmt.Sprintf("单文件扫描: 已分析 %d/%d 个文件，发现 %d 个漏洞", task.ScannedFiles, totalFiles, task.VulnerabilityCount))
	}

	task.Results = singleFileResults

	// ==================== 第二阶段：跨文件分析（先打印函数调用图）====================
	task.Progress = 50
	updateTaskProgress(task, "【第二阶段】正在构建跨文件调用链...")

	// 立即刷新日志到数据库，确保前端能实时看到
	updateTaskLog(task, task.Log)
	updateAILog(task, task.AILog)

	// 构建跨文件调用链
	cgAnalyzer, err := NewCallGraphAnalyzer(task.SourcePath)
	if err == nil {
		// 分步骤构建调用图，提供更细粒度的进度更新
		task.Progress = 55
		updateTaskProgress(task, "正在解析代码结构和依赖关系...")

		if err := cgAnalyzer.BuildCallGraph(); err == nil {
			task.Progress = 62
			updateTaskProgress(task, fmt.Sprintf("调用链构建完成: %d 个方法, %d 个调用关系",
				len(cgAnalyzer.Nodes), len(cgAnalyzer.Edges)))
			s.appendLog(task, fmt.Sprintf("调用链构建完成: %d 个方法, %d 个调用关系",
				len(cgAnalyzer.Nodes), len(cgAnalyzer.Edges)))

			// 保存跨文件调用上下文（用于前端图形展示）
			task.CrossFileContext = cgAnalyzer.GenerateCrossFileReport()
		} else {
			s.appendLog(task, fmt.Sprintf("调用链构建失败: %v", err))
		}
	} else {
		s.appendLog(task, fmt.Sprintf("创建调用链分析器失败: %v", err))
	}

	// 执行跨文件深度分析
	task.Progress = 68
	updateTaskProgress(task, "正在执行跨文件深度分析（利用链分析）...")
	s.appendLog(task, "正在执行跨文件深度分析（利用链分析）...")

	deepReport, err := s.ExecuteDeepAnalysis(task.SourcePath)
	if err != nil {
		s.appendLog(task, fmt.Sprintf("深度分析失败: %v", err))
		task.Progress = 72
		updateTaskProgress(task, fmt.Sprintf("深度分析完成（部分失败）"))
	} else {
		task.Progress = 72
		updateTaskProgress(task, "深度分析完成，正在提取漏洞...")
	}

	// 合并跨文件分析结果
	if deepReport != "" {
		// 从深度报告中提取额外的漏洞
		crossFileVulns := s.ExtractVulnerabilitiesFromDeepReport(deepReport, language)
		for _, vuln := range crossFileVulns {
			task.Results = append(task.Results, vuln)
			task.VulnerabilityCount++
		}
		s.appendLog(task, fmt.Sprintf("跨文件分析完成，发现 %d 个额外漏洞", len(crossFileVulns)))
	}

	task.Progress = 76
	updateTaskProgress(task, "正在合并去重漏洞...")

	// ==================== 合并后去重 ====================
	s.appendLog(task, "正在合并去重漏洞...")
	originalCount := len(task.Results)
	task.Results = s.deduplicateVulnerabilities(task.Results)
	dedupCount := originalCount - len(task.Results)
	if dedupCount > 0 {
		s.appendLog(task, fmt.Sprintf("去重完成: 原始 %d 个, 去重后 %d 个, 去除 %d 个重复",
			originalCount, len(task.Results), dedupCount))
	}

	task.Progress = 80
	updateTaskProgress(task, fmt.Sprintf("去重完成，发现 %d 个唯一漏洞，准备复查验证", len(task.Results)))

	// ==================== 第三阶段：漏洞复查验证 ====================
	if len(task.Results) > 0 {
		s.verifyAllVulnerabilities(task, task.SourcePath)
	}

	// ==================== 生成报告 ====================
	s.appendLog(task, "正在生成审计报告...")
	updateTaskProgress(task, "正在生成审计报告...")

	report, err := s.GenerateDetailedReport(task.Results, task.SourcePath, task.Prompt, deepReport, language)
	if err != nil {
		return fmt.Errorf("生成报告失败: %v", err)
	}

	// 计算执行时长
	endTimestamp := time.Now().UnixMilli()
	duration := int((endTimestamp - startTimestamp) / 1000)

	task.Report = report
	task.Progress = 100
	task.Status = "completed"
	task.EndTime = time.Now()
	task.Duration = duration
	task.CurrentFile = ""

	s.appendLog(task, fmt.Sprintf("审计完成! 共扫描 %d 个文件，发现 %d 个漏洞，耗时 %d 秒", task.ScannedFiles, task.VulnerabilityCount, duration))
	updateTaskProgress(task, fmt.Sprintf("审计完成! 共扫描 %d 个文件，发现 %d 个漏洞，耗时 %d 秒", task.ScannedFiles, task.VulnerabilityCount, duration))

	return nil
}

// ExtractVulnerabilitiesFromDeepReport 从深度分析报告中提取漏洞
func (s *AuditService) ExtractVulnerabilitiesFromDeepReport(deepReport, language string) []AuditResult {
	var vulns []AuditResult

	// 尝试从深度报告中提取漏洞信息
	// 这里可以根据实际报告格式进行解析

	// 示例：查找报告中的漏洞模式
	vulnPatterns := []string{"SQL注入", "命令注入", "跨站脚本", "XSS", "路径遍历", "不安全的反序列化"}

	for _, pattern := range vulnPatterns {
		if strings.Contains(deepReport, pattern) {
			vuln := AuditResult{
				Type:        "Security Vulnerability (Cross-File)",
				File:        "跨文件分析",
				Description: fmt.Sprintf("通过跨文件分析发现潜在%s漏洞", pattern),
				Severity:    "High",
				Analysis:    "该漏洞通过跨文件调用链分析发现，需要结合多个文件进行验证",
				Timestamp:   time.Now(),
				Language:    language,
			}
			vulns = append(vulns, vuln)
		}
	}

	return vulns
}

// AnalyzeCodeWithCrossFileContext 带跨文件上下文的AI分析
func (s *AuditService) AnalyzeCodeWithCrossFileContext(code, filePath, prompt, language, crossContext string) (string, error) {
	fileName := filepath.Base(filePath)

	var analysisPrompt string
	if prompt != "" && prompt != "给我分析代码的安全漏洞" {
		// 用户自定义提示词
		analysisPrompt = fmt.Sprintf(`%s

请分析以下%s (%s) 文件的源代码。

文件路径: %s

%s

跨文件调用上下文（供参考）:
%s

请根据用户的需求进行分析，并提供详细的结果。`, prompt, fileName, language, filePath, code, crossContext)
	} else {
		// 默认漏洞分析提示词
		vulnChecks := getVulnerabilityChecksForLanguage(language)

		var contextInfo string
		if crossContext != "" {
			contextInfo = fmt.Sprintf(`
## 跨文件上下文分析
该方法/函数在整个项目中的调用关系：
%s

请结合上述调用关系，分析是否存在跨文件的安全漏洞。`, crossContext)
		}

		analysisPrompt = fmt.Sprintf(`请分析以下%s (%s) 文件的源代码，找出其中的安全漏洞和代码质量问题。

文件路径: %s

%s

请仔细分析代码中的以下漏洞类型：
%s
%s

对于发现的每个漏洞，请提供：
- 漏洞类型
- 严重程度（Critical/High/Medium/Low）
- 详细描述
- 具体代码位置（行号）
- 修复建议

如果该文件没有发现漏洞，请明确说明"该文件未发现安全漏洞"。

请直接返回分析结果，不要包含任何格式前缀。`, fileName, language, filePath, code, vulnChecks, contextInfo)
	}

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: analysisPrompt}},
			MaxTokens:   s.maxTokens,
			Temperature: s.temperature,
		},
	)

	if err != nil {
		return "", fmt.Errorf("AI分析失败: %v", err)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return "", fmt.Errorf("AI分析失败: %v", err)
	}
	return content, nil
}

// ListCodeFiles 列出代码文件
func (s *AuditService) ListCodeFiles(dirPath string) ([]string, error) {
	// 支持的代码文件扩展名 - 扩展更多语言
	codeExtensions := []string{
		// 主流编程语言
		".java", ".php", ".py", ".js", ".ts", ".go", ".cs", ".rb", ".swift", ".kt",
		".scala", ".c", ".cpp", ".h", ".hpp", ".class",
		// 更多语言支持
		".rs",         // Rust
		".dart",       // Dart
		".groovy",     // Groovy
		".scala",      // Scala
		".r",          // R
		".lua",        // Lua
		".pl",         // Perl
		".hs",         // Haskell
		".erl",        // Erlang
		".ex", ".exs", // Elixir
		".clj", ".cljs", // Clojure
		".ml", ".mli", // OCaml
		".fs", ".fsi", // F#
		".d",   // D语言
		".pas", // Delphi
		".nim", // Nim
		".zig", // Zig
		// 前端
		".jsx", ".vue", ".svelte", ".scss", ".sass", ".less",
		// 脚本
		".sh", ".bash", ".zsh", ".fish", ".ps1",
	}

	var codeFiles []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			skipDirs := []string{"node_modules", ".git", "target", "build", "dist", "vendor", "venv", ".idea", ".vscode", "__pycache__", "node_modules", "bower_components", "vendor", ".npm"}
			for _, skip := range skipDirs {
				if strings.Contains(path, skip) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, codeExt := range codeExtensions {
			if ext == codeExt {
				codeFiles = append(codeFiles, path)
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return codeFiles, nil
}

// AnalyzeCodeWithFileNameAndLanguage 带语言信息的AI分析（记录交互日志）
func (s *AuditService) AnalyzeCodeWithFileNameAndLanguage(code, filePath, prompt, language string) (string, error) {
	fileName := filepath.Base(filePath)

	// 如果用户提供了自定义提示词，使用用户的提示词；否则使用默认的漏洞分析提示词
	var analysisPrompt string
	if prompt != "" && prompt != "给我分析代码的安全漏洞" {
		// 用户自定义提示词
		analysisPrompt = fmt.Sprintf(`%s

请分析以下%s (%s) 文件的源代码。

文件路径: %s

代码内容:
%s

请根据用户的需求进行分析，并提供详细的结果。`, prompt, fileName, language, filePath, code)
	} else {
		// 默认漏洞分析提示词
		vulnChecks := getVulnerabilityChecksForLanguage(language)
		analysisPrompt = fmt.Sprintf(`请分析以下%s (%s) 文件的源代码，找出其中的安全漏洞和代码质量问题。

文件路径: %s

%s

请仔细分析代码中的以下漏洞类型：
%s

对于发现的每个漏洞，请提供：
- 漏洞类型
- 严重程度（Critical/High/Medium/Low）
- 详细描述
- 具体代码位置（行号）
- 修复建议

如果该文件没有发现漏洞，请明确说明"该文件未发现安全漏洞"。

请直接返回分析结果，不要包含任何格式前缀。`, fileName, language, filePath, code, vulnChecks)
	}

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: analysisPrompt}},
			MaxTokens:   s.maxTokens,
			Temperature: s.temperature,
		},
	)

	if err != nil {
		return "", fmt.Errorf("AI分析失败: %v", err)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return "", fmt.Errorf("AI分析失败: %v", err)
	}
	return content, nil
}

func getVulnerabilityChecksForLanguage(language string) string {
	// 优化版审计提示词 - 强调实战化、可利用、高价值
	auditPrompt := "You are a senior security expert with 10 years of experience. " +
		"Your audit style is:宁可漏报,不可误报.\n\n" +
		"## Core Principle: Practical Vulnerability Criteria\n\n" +
		"A vulnerability must meet ALL 5 conditions:\n\n" +
		"1. Clear Attack Entry: HTTP/REST endpoints, file upload, CLI args only\n" +
		"2. Dangerous Function: SQL exec, command exec, deserialize, file ops\n" +
		"3. No Protection: No parameterized queries, whitelist, path limits\n" +
		"4. Constructible Payload: Can construct actual exploit payloads\n" +
		"5. Practical Value: RCE, database access, credential theft, auth bypass\n\n" +
		"## Strictly Forbidden (False Positive Zones)\n" +
		"1. XSS: Only report with complete stored XSS chain proof\n" +
		"2. CSRF: Frontend issue\n" +
		"3. Info Disclosure: Version info, paths (unless leads to password leak)\n" +
		"4. Weak Crypto: Only if protecting critical credentials\n" +
		"5. URL Redirect: Only if used for phishing\n\n" +
		"## Output Format for Each Vulnerability\n" +
		"```\n" +
		"===VULN_START===\n" +
		"Threat: [Type-Description]\n" +
		"Severity: [Critical/High/Medium/Low]\n" +
		"Location: File=[path] Line=[num] Function=[name]\n" +
		"DataFlow: Entry->Propagation->DangerPoint\n" +
		"DangerCode: ```[lang]...\n```\n" +
		"ExploitCond: Auth required? Privilege level? Complexity?\n" +
		"Payload: [Specific executable payload]\n" +
		"Fix: ```[lang]...safe code...\n```\n" +
		"===VULN_END===\n" +
		"```\n\n" +
		"## No Vulnerability Case\n" +
		"```\n" +
		"===NO_VULN===\n" +
		"Code passed strict audit. No directly exploitable high-risk vulnerabilities found.\n" +
		"Audit criteria: Attack Entry + Dangerous Function + No Protection + Constructible Payload + Practical Value\n" +
		"===NO_VULN===\n" +
		"```\n\n" +
		"## Example: Should Report\n" +
		"```java\n" +
		"@PostMapping(\"/login\")\n" +
		"public User login(@RequestParam String username, @RequestParam String password) {\n" +
		"    String sql = \"SELECT * FROM users WHERE name='\" + username + \"'\";\n" +
		"    return jdbcTemplate.queryForObject(sql);\n" +
		"}\n" +
		"```\n" +
		"判定: POST参数 + 字符串拼接SQL + 无参数化 + ' OR '1'='1 -> 报告SQL注入\n\n" +
		"## Example: Should NOT Report\n" +
		"```java\n" +
		"@GetMapping(\"/user/{id}\")\n" +
		"public User getUser(@PathVariable Long id) {\n" +
		"    return userRepository.findById(id);\n" +
		"}\n" +
		"```\n" +
		"判定: 使用参数化查询(findById) -> 不报告\n"

	// 语言特定的危险函数映射
	languageSpecific := map[string]string{
		"java": "\n### Java Danger Functions\n" +
			"| Function | Vuln Type | Condition |\n" +
			"|:---|:---|:---|\n" +
			"| Statement.executeQuery(sql) | SQL Injection | String concat |\n" +
			"| Runtime.exec(cmd) | Command Injection | User input in cmd |\n" +
			"| ProcessBuilder | Command Injection | Array from user |\n" +
			"| ObjectInputStream.readObject() | Deserialization | No whitelist |\n" +
			"| FileInputStream/FileReader | Path Traversal | Path from user |\n" +
			"| InitialContext().lookup() | JNDI Injection | Param from user |\n" +
			"| SpEL Expression | SpEL Injection | No safe context |",
		"python": "\n### Python Danger Functions\n" +
			"| Function | Vuln Type | Condition |\n" +
			"|:---|:---|:---|\n" +
			"| cursor.execute(sql) | SQL Injection | f-string concat |\n" +
			"| os.system/subprocess.Popen | Command Injection | From user |\n" +
			"| pickle.loads/yaml.load | Deserialization | Not safe_load |\n" +
			"| eval/exec | Code Injection | From user |\n" +
			"| open(filename) | Path Traversal | Filename from user |\n" +
			"| render_template_string | SSTI | String template |\n" +
			"| requests.get(url) | SSRF | URL from user |",
		"go": "\n### Go Danger Functions\n" +
			"| Function | Vuln Type | Condition |\n" +
			"|:---|:---|:---|\n" +
			"| db.Query(sql) | SQL Injection | String concat |\n" +
			"| exec.Command | Command Injection | Param from user |\n" +
			"| os.Open/ioutil.ReadFile | Path Traversal | Path from user |\n" +
			"| regexp.Compile(user) | ReDoS | Regex from user |",
		"javascript": "\n### JavaScript/Node.js Danger Functions\n" +
			"| Function | Vuln Type | Condition |\n" +
			"|:---|:---|:---|\n" +
			"| child_process.exec | Command Injection | From user |\n" +
			"| eval/new Function | Code Injection | From user |\n" +
			"| require(user) | Module Loading | Dynamic require |\n" +
			"| JSON.parse(user) | Prototype Pollution | No __proto__ filter |\n" +
			"| fs.readFile | Path Traversal | Path from user |\n" +
			"| fetch(url)/axios.get(url) | SSRF | URL from user |",
		"php": "\n### PHP Danger Functions\n" +
			"| Function | Vuln Type | Condition |\n" +
			"|:---|:---|:---|\n" +
			"| mysqli_query/execute | SQL Injection | String concat |\n" +
			"| system/exec/shell_exec | Command Injection | From user |\n" +
			"| include/require | File Inclusion | Path from user |\n" +
			"| unserialize | Deserialization | No signature |\n" +
			"| file_get_contents/open | Path Traversal | Path from user |\n" +
			"| move_uploaded_file | File Upload | No type check |",
	}

	if extra, ok := languageSpecific[language]; ok {
		return auditPrompt + extra
	}

	return auditPrompt
}

// ParseAnalysisResultWithLanguage 解析AI返回的分析结果 - 带语言
func (s *AuditService) ParseAnalysisResultWithLanguage(filePath, analysis, language string) *AuditResult {
	if strings.Contains(analysis, "未发现安全漏洞") || strings.Contains(analysis, "没有发现") {
		return nil
	}

	severity := "Medium"
	if strings.Contains(analysis, "Critical") || strings.Contains(analysis, "严重") {
		severity = "Critical"
	} else if strings.Contains(analysis, "High") || strings.Contains(analysis, "高") {
		severity = "High"
	} else if strings.Contains(analysis, "Low") || strings.Contains(analysis, "低") {
		severity = "Low"
	}

	return &AuditResult{
		Type:        "Security Vulnerability",
		File:        filePath,
		Description: "发现安全漏洞",
		Severity:    severity,
		Analysis:    analysis,
		Timestamp:   time.Now(),
		Language:    language,
	}
}

// GenerateDetailedReport 生成详细报告 - 支持用户自定义提示词（优化版）
func (s *AuditService) GenerateDetailedReport(results []AuditResult, sourcePath, prompt, deepReport, language string) (string, error) {
	// 如果用户提供了自定义提示词，使用 AI 生成个性化报告
	if prompt != "" && prompt != "给我分析代码的安全漏洞" {
		// 使用 AI 根据用户提示词生成报告
		return s.generateCustomReport(results, sourcePath, prompt, deepReport, language)
	}

	// 默认安全漏洞审计报告 - 优化版
	var report strings.Builder

	// 按严重程度分组统计
	var critical, high, medium, low []AuditResult
	for _, r := range results {
		switch r.Severity {
		case "Critical":
			critical = append(critical, r)
		case "High":
			high = append(high, r)
		case "Medium":
			medium = append(medium, r)
		case "Low":
			low = append(low, r)
		}
	}

	// 报告生成时间
	reportTime := time.Now().Format("2006-01-02 15:04:05")

	// 1. 报告头部 - 美化版
	report.WriteString("# 🔒 安全审计报告\n\n")
	report.WriteString("> **生成时间**: " + reportTime + "  \n")
	report.WriteString("> **审计目标**: " + filepath.Base(sourcePath) + "  \n")
	report.WriteString("> **检测语言**: " + language + "\n\n")

	// 2. 漏洞统计 - 使用徽章样式
	report.WriteString("## 📊 漏洞统计\n\n")
	report.WriteString("| 严重程度 | 数量 | 状态 |\n")
	report.WriteString("|:-------|:----:|:----:|\n")
	report.WriteString(fmt.Sprintf("| 🔴 Critical | %d | ⚠️ 需立即修复 | \n", len(critical)))
	report.WriteString(fmt.Sprintf("| 🟠 High | %d | ⚠️ 优先修复 |\n", len(high)))
	report.WriteString(fmt.Sprintf("| 🟡 Medium | %d | 📅 计划修复 |\n", len(medium)))
	report.WriteString(fmt.Sprintf("| 🟢 Low | %d | 📋 后续关注 |\n", len(low)))
	report.WriteString(fmt.Sprintf("| **总计** | **%d** | - |\n\n", len(results)))

	// 3. 风险评估摘要
	if len(critical) > 0 || len(high) > 0 {
		report.WriteString("> ⚠️ **风险评估**: 发现 " + strconv.Itoa(len(critical)+len(high)) + " 个高危漏洞，建议优先处理\n\n")
	} else if len(medium) > 0 {
		report.WriteString("> 📅 **风险评估**: 发现 " + strconv.Itoa(len(medium)) + " 个中危漏洞，建议按计划修复\n\n")
	} else if len(low) > 0 {
		report.WriteString("> ✅ **风险评估**: 发现 " + strconv.Itoa(len(low)) + " 个低危漏洞，影响较小\n\n")
	} else {
		report.WriteString("> ✅ **风险评估**: 未发现明显安全漏洞\n\n")
	}

	// 4. 漏洞详情
	if len(results) > 0 {
		report.WriteString("---\n\n")
		report.WriteString("## 🐛 漏洞详情\n\n")

		// 按严重程度输出
		for i, r := range critical {
			report.WriteString(s.formatVulnerabilityEnhanced(i+1, r, "Critical", len(critical)))
		}
		for i, r := range high {
			report.WriteString(s.formatVulnerabilityEnhanced(len(critical)+i+1, r, "High", len(high)))
		}
		for i, r := range medium {
			report.WriteString(s.formatVulnerabilityEnhanced(len(critical)+len(high)+i+1, r, "Medium", len(medium)))
		}
		for i, r := range low {
			report.WriteString(s.formatVulnerabilityEnhanced(len(critical)+len(high)+len(medium)+i+1, r, "Low", len(low)))
		}
	} else {
		report.WriteString("---\n\n")
		report.WriteString("## ✅ 审计结论\n\n")
		report.WriteString("未发现安全漏洞。代码审计通过，建议继续保持安全编码习惯。\n")
	}

	// 5. 安全修复建议
	report.WriteString("---\n\n")
	report.WriteString("## 🛡️ 安全修复建议\n\n")
	report.WriteString("### 通用安全建议\n\n")
	report.WriteString("| 序号 | 建议类别 | 具体措施 | 优先级 |\n")
	report.WriteString("|:---:|:-------|:--------|:----:|\n")
	report.WriteString("| 1 | 输入验证 | 对所有用户输入进行严格的验证和过滤 | 🔴 高 |\n")
	report.WriteString("| 2 | 参数化查询 | 使用预编译语句防止SQL注入 | 🔴 高 |\n")
	report.WriteString("| 3 | 输出编码 | 对所有输出进行适当编码，防止XSS | 🟠 中 |\n")
	report.WriteString("| 4 | 最小权限 | 运行服务时使用最小必要权限 | 🟠 中 |\n")
	report.WriteString("| 5 | 密钥管理 | 使用环境变量或密钥管理服务存储敏感信息 | 🟠 中 |\n")
	report.WriteString("| 6 | 安全更新 | 定期更新依赖库和框架到最新安全版本 | 🟡 低 |\n\n")

	// 语言特定建议
	switch language {
	case "java":
		report.WriteString("### ☕ Java特定安全建议\n\n")
		report.WriteString("```java\n")
		report.WriteString("// ✅ 推荐: 使用 PreparedStatement\n")
		report.WriteString("String sql = \"SELECT * FROM users WHERE name=?\";\n")
		report.WriteString("PreparedStatement pstmt = conn.prepareStatement(sql);\n")
		report.WriteString("pstmt.setString(1, username);\n\n")
		report.WriteString("// ❌ 避免: 使用 Statement + 字符串拼接\n")
		report.WriteString("// String sql = \"SELECT * FROM users WHERE name='\" + username + \"'\";\n")
		report.WriteString("```\n\n")
		report.WriteString("- 使用 Spring Security 进行认证授权\n")
		report.WriteString("- 使用 OWASP ESAPI 进行输入验证\n")
		report.WriteString("- 避免使用 ObjectInputStream 进行反序列化\n\n")
	case "php":
		report.WriteString("### 🐘 PHP特定安全建议\n\n")
		report.WriteString("```php\n")
		report.WriteString("// ✅ 推荐: 使用 PDO 预处理\n")
		report.WriteString("$stmt = $pdo->prepare('SELECT * FROM users WHERE name=?');\n")
		report.WriteString("$stmt->execute([$username]);\n\n")
		report.WriteString("// ❌ 避免: 直接拼接 SQL\n")
		report.WriteString("// $sql = \"SELECT * FROM users WHERE name='\" . $username . \"'\";\n")
		report.WriteString("```\n\n")
		report.WriteString("- 避免使用 eval() 和 assert()\n")
		report.WriteString("- 使用 htmlspecialchars() 防止XSS\n")
		report.WriteString("- 验证所有文件包含路径\n\n")
	case "python":
		report.WriteString("### 🐍 Python特定安全建议\n\n")
		report.WriteString("```python\n")
		report.WriteString("# ✅ 推荐: 使用参数化查询\n")
		report.WriteString("cursor.execute('SELECT * FROM users WHERE name=?', (username,))\n\n")
		report.WriteString("# ❌ 避免: 使用字符串拼接 SQL\n")
		report.WriteString("# cursor.execute(f'SELECT * FROM users WHERE name=\"{username}\"')\n\n")
		report.WriteString("# ✅ 推荐: 使用 safe_load\n")
		report.WriteString("data = yaml.safe_load(user_input)\n")
		report.WriteString("```\n\n")
		report.WriteString("- 避免使用 pickle 进行反序列化\n")
		report.WriteString("- 使用 Jinja2 模板而非 render_template_string\n\n")
	}

	// 6. 附录
	report.WriteString("---\n\n")
	report.WriteString("## 📎 附录\n\n")
	report.WriteString("### 漏洞等级说明\n\n")
	report.WriteString("| 等级 | 说明 | 修复时间 |\n")
	report.WriteString("|:----|:----|:-------|\n")
	report.WriteString("| Critical | 远程代码执行、严重数据泄露等 | 24小时内 |\n")
	report.WriteString("| High | SQL注入、命令注入等 | 1周内 |\n")
	report.WriteString("| Medium | XSS、CSRF等 | 1个月内 |\n")
	report.WriteString("| Low | 信息泄露、配置问题等 | 计划内 |\n\n")
	report.WriteString("### 审计方法\n\n")
	report.WriteString("- 静态代码分析 (SAST)\n")
	report.WriteString("- 污点追踪分析\n")
	report.WriteString("- 跨文件调用链分析\n")
	report.WriteString("- AI 辅助漏洞识别\n\n")
	report.WriteString("---\n\n")
	report.WriteString("> 📝 **报告说明**: 本报告由自动化代码审计系统生成，仅供参考。实际漏洞验证和修复建议请结合代码上下文进行评估。\n")

	return report.String(), nil
}

// generateCustomReport 使用 AI 根据用户提示词生成个性化报告
func (s *AuditService) generateCustomReport(results []AuditResult, sourcePath, userPrompt, deepReport, language string) (string, error) {
	// 准备分析结果摘要
	var resultSummary strings.Builder
	resultSummary.WriteString(fmt.Sprintf("项目路径: %s\n", sourcePath))
	resultSummary.WriteString(fmt.Sprintf("检测语言: %s\n", language))
	resultSummary.WriteString(fmt.Sprintf("发现项目: %d 个\n\n", len(results)))

	for i, r := range results {
		resultSummary.WriteString(fmt.Sprintf("## 项目 %d\n", i+1))
		resultSummary.WriteString(fmt.Sprintf("- 类型: %s\n", r.Type))
		resultSummary.WriteString(fmt.Sprintf("- 文件: %s\n", r.File))
		resultSummary.WriteString(fmt.Sprintf("- 严重程度: %s\n", r.Severity))
		resultSummary.WriteString(fmt.Sprintf("- 分析: %s\n\n", r.Analysis))
	}

	// 调用 AI 生成报告
	reportPrompt := fmt.Sprintf(`请根据用户的需求和代码分析结果生成报告。

用户需求: %s

分析结果:
%s

深度分析:
%s

请根据用户的需求生成对应的报告格式，不要只关注安全漏洞，要全面分析代码。如果用户要求分析算法，请分析算法逻辑；如果要求逆向，请提供逆向分析结果。`, userPrompt, resultSummary.String(), deepReport)

	resp, err := s.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       s.modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: reportPrompt}},
			MaxTokens:   s.maxTokens * 2, // 增加输出长度
			Temperature: s.temperature,
		},
	)

	if err != nil {
		// 如果 AI 调用失败，回退到默认报告
		return s.generateDefaultReport(results, sourcePath, language)
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		return s.generateDefaultReport(results, sourcePath, language)
	}

	// 为每个漏洞生成 POC 并追加到报告中
	report := content
	if len(results) > 0 && s.client != nil {
		var pocSection strings.Builder
		pocSection.WriteString("\n\n---\n\n## 漏洞验证POC\n\n")

		for i, r := range results {
			fmt.Printf("[POC生成] 正在为漏洞 %d 生成POC: %s\n", i+1, r.File)

			poc := generatePOC(r, s.client, s.modelName, s.maxTokens, s.temperature)
			if poc != "" {
				// POC已经是完整格式（包含代码块），直接输出
				pocSection.WriteString(fmt.Sprintf("### 漏洞 %d: %s [%s]\n\n", i+1, r.Type, r.Severity))
				pocSection.WriteString(fmt.Sprintf("文件: %s\n\n", r.File))
				pocSection.WriteString("**漏洞验证POC**:\n\n")
				pocSection.WriteString(poc)
				pocSection.WriteString("\n\n")
			} else {
				fmt.Printf("[POC生成] 漏洞 %d POC生成失败或返回空\n", i+1)
			}
		}

		// 只有当有 POC 内容时才追加
		if pocSection.Len() > len("\n\n---\n\n## 漏洞验证POC\n\n") {
			report = content + pocSection.String()
		}
	}

	return report, nil
}

// generateDefaultReport 生成默认报告（当 AI 调用失败时）
func (s *AuditService) generateDefaultReport(results []AuditResult, sourcePath, language string) (string, error) {
	var report strings.Builder
	report.WriteString(fmt.Sprintf("# 代码分析报告 - %s\n\n", filepath.Base(sourcePath)))
	report.WriteString(fmt.Sprintf("共发现 %d 个问题项\n\n", len(results)))

	for i, r := range results {
		report.WriteString(fmt.Sprintf("## %d. %s [%s]\n", i+1, r.Type, r.Severity))
		report.WriteString(fmt.Sprintf("文件: %s\n\n", cleanFilePath(r.File)))
		report.WriteString(fmt.Sprintf("%s\n\n", r.Analysis))
	}

	return report.String(), nil
}

// formatVulnerabilityEnhanced 增强版漏洞格式化 - 专业排版
func (s *AuditService) formatVulnerabilityEnhanced(index int, r AuditResult, severityLabel string, totalInLevel int) string {
	var b strings.Builder

	// 提取漏洞名称
	vulnName := extractVulnerabilityName(r.Analysis)

	// 严重程度样式
	var severityEmoji, severityIcon string
	if severityLabel == "Critical" {
		severityEmoji = "🔴"
		severityIcon = "严重"
	} else if severityLabel == "High" {
		severityEmoji = "🟠"
		severityIcon = "高危"
	} else if severityLabel == "Medium" {
		severityEmoji = "🟡"
		severityIcon = "中危"
	} else {
		severityEmoji = "🟢"
		severityIcon = "低危"
	}

	// ========== 漏洞标题区块 ==========
	b.WriteString(fmt.Sprintf("### %s [%d] %s\n\n", severityEmoji, index, vulnName))

	// ========== 核心信息卡片 ==========
	b.WriteString("> **⚡ 风险评级**: ")
	b.WriteString(severityIcon)
	b.WriteString(" | ")
	b.WriteString("**📍 位置**: ")
	b.WriteString(fmt.Sprintf("`%s`", cleanFilePath(r.File)))
	b.WriteString(" | ")
	b.WriteString("**🏷️ 类型**: ")
	b.WriteString(vulnName)
	b.WriteString("\n\n")

	// ========== 漏洞详情区块 ==========
	b.WriteString("<details>\n")
	b.WriteString("<summary><b>📋 点击展开漏洞详情</b></summary>\n\n")

	// 1. 危险代码
	b.WriteString("#### 🔴 危险代码\n\n")
	b.WriteString("```" + getCodeBlockLang(r.Language) + "\n")
	codeSnippet := extractCodeSnippetFromAnalysis(r.Analysis)
	if codeSnippet != "" {
		b.WriteString(codeSnippet)
	} else {
		b.WriteString("// 未能提取代码片段，请查看原始分析结果")
	}
	b.WriteString("\n```\n\n")

	// 2. 数据流分析
	b.WriteString("#### 📊 数据流分析\n\n")
	dataFlow := extractDataFlowAnalysis(r.Analysis)
	b.WriteString(dataFlow)
	b.WriteString("\n\n")

	// 3. 利用条件
	b.WriteString("#### 🎯 利用条件\n\n")
	exploitCond := extractExploitCondition(r.Analysis)
	b.WriteString(exploitCond)
	b.WriteString("\n\n")

	// 4. POC/EXP
	b.WriteString("#### 💀 验证POC\n\n")
	if r.POC != "" {
		b.WriteString("```bash\n")
		b.WriteString(r.POC)
		b.WriteString("\n```\n\n")
	} else {
		b.WriteString("*POC待生成...*\n\n")
	}

	// 5. 修复建议
	b.WriteString("#### 🛡️ 修复建议\n\n")
	fixSuggestion := extractFixSuggestionFromAnalysis(r.Analysis)
	if fixSuggestion != "" {
		b.WriteString("```" + getCodeBlockLang(r.Language) + "\n")
		b.WriteString(fixSuggestion)
		b.WriteString("\n```\n\n")
	} else {
		b.WriteString("```" + getCodeBlockLang(r.Language) + "\n")
		b.WriteString(getDefaultFixSuggestion(vulnName))
		b.WriteString("\n```\n\n")
	}

	// 6. CWE/CVE参考
	cwe := extractCWEFromAnalysis(r.Analysis)
	if cwe != "" {
		b.WriteString("#### 📚 参考资料\n\n")
		b.WriteString(fmt.Sprintf("- **CWE**: [%s](https://cwe.mitre.org/data/definitions/%s.html)\n", cwe, strings.TrimPrefix(cwe, "CWE-")))
	}

	b.WriteString("</details>\n\n")
	b.WriteString("---\n\n")

	return b.String()
}

// getCodeBlockLang 根据语言返回代码块标识
func getCodeBlockLang(language string) string {
	langMap := map[string]string{
		"java":       "java",
		"python":     "python",
		"php":        "php",
		"javascript": "javascript",
		"typescript": "typescript",
		"go":         "go",
		"csharp":     "csharp",
		"ruby":       "ruby",
		"swift":      "swift",
		"kotlin":     "kotlin",
	}
	if lang, ok := langMap[language]; ok {
		return lang
	}
	return "java" // 默认
}

// getDefaultFixSuggestion 根据漏洞类型返回默认修复建议
func getDefaultFixSuggestion(vulnType string) string {
	suggestions := map[string]string{
		"SQL注入": `// ✅ 修复方式：使用参数化查询
// ❌ 错误示例
String sql = "SELECT * FROM users WHERE name='" + username + "'";

// ✅ 正确示例
String sql = "SELECT * FROM users WHERE name=?";
PreparedStatement pstmt = conn.prepareStatement(sql);
pstmt.setString(1, username);`,
		"命令注入": `// ✅ 修复方式：避免使用用户输入执行命令
// ❌ 错误示例
Runtime.getRuntime().exec("ls " + userInput);

// ✅ 正确示例：使用白名单验证输入
if (!userInput.matches("^[a-zA-Z0-9]+$")) {
    throw new IllegalArgumentException("Invalid input");
}
// 或使用数组方式执行命令
String[] cmd = {"ls", userInput};
Runtime.getRuntime().exec(cmd);`,
		"路径遍历": `// ✅ 修复方式：验证路径并使用标准化路径
// ❌ 错误示例
File file = new File(baseDir, userInput);

// ✅ 正确示例
File file = new File(baseDir, userInput);
String canonicalPath = file.getCanonicalPath();
if (!canonicalPath.startsWith(baseDir.getCanonicalPath())) {
    throw new SecurityException("Path traversal detected");
}`,
		"不安全的反序列化": `// ✅ 修复方式：避免反序列化不可信数据
// ❌ 错误示例
ObjectInputStream ois = new ObjectInputStream(input);
Object obj = ois.readObject();

// ✅ 正确示例：使用白名单或JSON等安全替代方案
ObjectMapper mapper = new ObjectMapper();
MyObject obj = mapper.readValue(jsonString, MyObject.class);`,
		"硬编码凭据": `// ✅ 修复方式：使用环境变量或密钥管理服务
// ❌ 错误示例
private static final String API_KEY = "sk-xxxx";

// ✅ 正确示例
private static final String API_KEY = System.getenv("API_KEY");
// 或使用AWS Secrets Manager、HashiCorp Vault等`,
	}
	if suggestion, ok := suggestions[vulnType]; ok {
		return suggestion
	}
	return "// 请根据具体漏洞类型进行修复\n// 1. 输入验证\n// 2. 参数化查询\n// 3. 最小权限原则"
}

// extractDataFlowAnalysis 提取数据流分析
func extractDataFlowAnalysis(analysis string) string {
	// 尝试提取数据流信息
	patterns := []string{
		`数据流[：:]\s*([\s\S]{50,500})`,
		`入口点[：:]\s*([\s\S]{30,300})`,
		`传播路径[：:]\s*([\s\S]{30,300})`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(analysis)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	// 默认返回
	return "**输入点**: 用户输入 → **处理过程**: 未做校验 → **危险点**: 进入危险函数"
}

// extractExploitCondition 提取利用条件
func extractExploitCondition(analysis string) string {
	// 尝试提取利用条件
	pattern := regexp.MustCompile(`利用条件[：:]\s*([\s\S]{30,300})`)
	matches := pattern.FindStringSubmatch(analysis)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	// 返回通用条件
	return `| 条件 | 说明 |
|:---|:---|
| 认证要求 | 部分漏洞需要登录权限 |
| 利用复杂度 | 中（需要构造特定Payload）|
| 影响范围 | 可导致数据泄露/服务器权限丢失 |
| 利用方式 | 通过HTTP请求发送恶意Payload |`
}

// formatVulnerabilitySimple 简化版漏洞格式化 - 优化排版
func (s *AuditService) formatVulnerabilitySimple(index int, r AuditResult, severityLabel string) string {
	var b strings.Builder

	// 提取漏洞名称
	vulnName := extractVulnerabilityName(r.Analysis)

	// 严重程度emoji
	severityEmoji := "🟡"
	severityIcon := "中危"
	if strings.Contains(r.Severity, "Critical") || strings.Contains(severityLabel, "严重") {
		severityEmoji = "🔴"
		severityIcon = "严重"
	} else if strings.Contains(r.Severity, "High") || strings.Contains(severityLabel, "高危") {
		severityEmoji = "🟠"
		severityIcon = "高危"
	} else if strings.Contains(r.Severity, "Low") || strings.Contains(severityLabel, "低危") {
		severityEmoji = "🟢"
		severityIcon = "低危"
	}

	b.WriteString(fmt.Sprintf("### 漏洞 %d: %s [%s]\n\n", index, vulnName, severityIcon))

	// 属性表格
	b.WriteString("| 属性 | 内容 |\n")
	b.WriteString("|:----|:----|\n")
	b.WriteString(fmt.Sprintf("| **漏洞类型** | %s |\n", vulnName))
	b.WriteString(fmt.Sprintf("| **严重程度** | %s %s |\n", severityEmoji, severityIcon))
	b.WriteString(fmt.Sprintf("| **位置** | `%s` |\n", cleanFilePath(r.File)))

	// 如果有CWE编号，添加
	cwe := extractCWEFromAnalysis(r.Analysis)
	if cwe != "" {
		b.WriteString(fmt.Sprintf("| **CWE** | %s |\n", cwe))
	}

	b.WriteString("\n")

	// 危险代码
	b.WriteString("**危险代码**:\n")
	b.WriteString("```java\n")
	codeSnippet := extractCodeSnippetFromAnalysis(r.Analysis)
	if codeSnippet != "" {
		b.WriteString(codeSnippet)
	} else {
		b.WriteString("// 未能提取代码片段，请查看原始分析结果")
	}
	b.WriteString("\n```\n\n")

	// 漏洞描述
	b.WriteString("**漏洞描述**: \n")
	b.WriteString(extractDescriptionFromAnalysis(r.Analysis))
	b.WriteString("\n\n")

	// 漏洞利用链
	b.WriteString("**漏洞利用链**: \n")
	b.WriteString("```\n")
	b.WriteString(generateExploitChain(r))
	b.WriteString("\n```\n\n")

	// 修复建议
	b.WriteString("**修复建议**:\n")
	fixSuggestion := extractFixSuggestionFromAnalysis(r.Analysis)
	if fixSuggestion != "" {
		b.WriteString("```java\n")
		b.WriteString(fixSuggestion)
		b.WriteString("\n```\n\n")
	} else {
		b.WriteString("暂无修复建议，请参考通用安全编码规范。\n\n")
	}

	b.WriteString("---\n\n")

	return b.String()
}

// extractVulnerabilityName 从分析结果中提取漏洞名称
func extractVulnerabilityName(analysis string) string {
	types := []string{"SQL注入", "命令注入", "跨站脚本", "XSS", "路径遍历", "不安全的反序列化",
		"敏感信息泄露", "文件包含", "SSRF", "模板注入", "认证绕过", "授权问题"}

	for _, t := range types {
		if strings.Contains(analysis, t) {
			return t
		}
	}

	return "安全漏洞"
}

// getVulnerabilityPrerequisite 获取漏洞利用前提
func getVulnerabilityPrerequisite(vulnType string) string {
	prereqs := map[string]string{
		"SQL注入":    "需要后台数据库操作权限，部分SQL注入需要高权限",
		"命令注入":     "需要后台代码执行权限，通常需要认证",
		"XSS":      "无需后台权限，主要针对其他用户或管理员",
		"路径遍历":     "需要文件读取权限，部分需要认证",
		"不安全的反序列化": "需要反序列化功能入口，通常需要认证",
		"文件包含":     "需要文件上传或包含功能",
		"SSRF":     "需要发起网络请求的功能点",
		"认证绕过":     "无需认证或可绕过认证机制",
		"授权问题":     "需要普通用户权限，可尝试越权操作",
	}

	if prereq, ok := prereqs[vulnType]; ok {
		return prereq
	}
	return "需要根据实际漏洞情况分析"
}

var pocGenLogger = log.New(os.Stdout, "[POC生成] ", log.LstdFlags)

// generatePOC 生成漏洞验证POC（增强版）
func generatePOC(r AuditResult, client *openai.Client, modelName string, maxTokens int, temperature float32) string {
	if client == nil {
		pocGenLogger.Print("跳过: client为nil")
		return ""
	}

	// 提取漏洞代码片段 - 改进提取逻辑
	codeSnippet := extractCodeSnippetEnhanced(r.Analysis, r.File)

	// 提取漏洞类型和严重程度
	vulnType := extractVulnTypeFromAnalysis(r.Analysis)
	severity := r.Severity

	// 提取行号信息
	lineNum := extractLineNumber(r.Analysis)

	// 调用AI生成POC - 优化提示词，生成标准格式POC
	prompt := "请为以下漏洞生成可执行的验证POC。\n\n" +
		"## 漏洞基本信息\n" +
		"- **漏洞类型**: " + vulnType + "\n" +
		"- **严重等级**: " + severity + "\n" +
		"- **文件位置**: " + r.File + "\n" +
		"- **代码行号**: " + lineNum + "\n\n" +
		"## 漏洞代码片段\n" +
		"```\n" + codeSnippet + "\n```\n\n" +
		"## AI分析详情\n" +
		r.Analysis + "\n\n" +
		"## POC生成要求（必须严格遵守）\n\n" +
		"### 输出格式要求\n" +
		"必须使用标准格式输出POC，包含以下部分：\n\n" +
		"1. curl命令POC（使用bash代码块）\n" +
		"2. Python脚本POC（使用python代码块）\n" +
		"3. 预期响应结果\n" +
		"4. 注意事项\n\n" +
		"### 特殊漏洞处理\n" +
		"- SQL注入: 提供具体注入payload（如 ' OR '1'='1, UNION SELECT等）\n" +
		"- 硬编码Token: 提供具体Token值，说明如何利用\n" +
		"- 命令注入: 提供具体命令payload（如 ; whoami, $(whoami)等）\n" +
		"- 路径遍历: 提供穿越路径payload（如 ../../../etc/passwd）\n\n" +
		"### 关键要求\n" +
		"- POC必须可直接执行\n" +
		"- 必须包含具体的攻击Payload\n" +
		"- 保持输出简洁，不要添加额外解释\n\n" +
		"请严格按照上述格式生成POC。"

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       modelName,
			Messages:    []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: prompt}},
			MaxTokens:   maxTokens * 2, // 增加输出长度以容纳详细POC
			Temperature: temperature,
		},
	)

	if err != nil {
		pocGenLogger.Printf("AI调用失败: %v", err)
		return ""
	}

	content, err := checkChoicesValid(resp)
	if err != nil {
		pocGenLogger.Printf("检查响应失败: %v", err)
		return ""
	}

	// 清理返回内容，移除可能的解释性文字
	content = strings.TrimSpace(content)
	if strings.Contains(content, "无法生成POC") {
		pocGenLogger.Printf("AI返回'无法生成POC'，漏洞类型: %s, 文件: %s", vulnType, r.File)
		return ""
	}

	// 检查内容是否为空或太短
	if len(content) < 50 {
		pocGenLogger.Printf("返回内容过短，长度: %d，漏洞类型: %s, 文件: %s", len(content), vulnType, r.File)
		return ""
	}

	pocGenLogger.Printf("成功生成POC，漏洞类型: %s, 文件: %s", vulnType, r.File)
	return content
}

// extractCodeSnippetEnhanced 增强版代码片段提取（用于POC生成）
func extractCodeSnippetEnhanced(analysis, filePath string) string {
	// 1. 首先尝试提取代码块
	codeBlockPattern := regexp.MustCompile("```[\\s\\S]*?```")
	codeBlocks := codeBlockPattern.FindAllString(analysis, -1)

	// 过滤并返回最长的代码块
	var validBlocks []string
	for _, block := range codeBlocks {
		block = strings.TrimPrefix(block, "```")
		block = strings.TrimSuffix(block, "```")
		lines := strings.Split(block, "\n")
		if len(lines) > 0 {
			block = strings.Join(lines[1:], "\n")
		}
		block = strings.TrimSpace(block)

		// 跳过太短的或包含建议性文字的代码块
		if len(block) > 30 && !strings.Contains(block, "建议") && !strings.Contains(block, "修复") {
			validBlocks = append(validBlocks, block)
		}
	}

	if len(validBlocks) > 0 {
		// 返回最长的代码块
		maxLen := 0
		var bestBlock string
		for _, block := range validBlocks {
			if len(block) > maxLen {
				maxLen = len(block)
				bestBlock = block
			}
		}
		if len(bestBlock) > 200 {
			return bestBlock[:200] + "..."
		}
		return bestBlock
	}

	// 2. 尝试提取"威胁代码"、"漏洞代码"等标记后的内容
	evidencePatterns := []string{
		`威胁代码[：:]\s*([\s\S]{50,500})`,
		`问题代码[：:]\s*([\s\S]{50,500})`,
		`漏洞代码[：:]\s*([\s\S]{50,500})`,
		`代码如下[：:]\s*([\s\S]{50,500})`,
		`示例代码[：:]\s*([\s\S]{50,500})`,
		`关键代码[：:]\s*([\s\S]{50,500})`,
	}

	for _, pattern := range evidencePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(analysis)
		if len(matches) > 1 {
			snippet := strings.TrimSpace(matches[1])
			if len(snippet) > 30 {
				if len(snippet) > 200 {
					return snippet[:200] + "..."
				}
				return snippet
			}
		}
	}

	// 3. 尝试提取包含危险函数调用的行
	dangerousPatterns := []string{
		`ObjectInputStream.*?readObject`,
		`Runtime\.getRuntime\(\)\.exec`,
		`ProcessBuilder`,
		`exec\s*\(\s*`,
		`system\s*\(\s*`,
		`eval\s*\(\s*`,
		`assert\s*\(\s*`,
		`executeQuery\s*\(\s*`,
		`execute\s*\(\s*`,
		`Statement\s*`,
		`FileInputStream\s*\(\s*`,
		`FileReader\s*\(\s*`,
		`readFile\s*\(\s*`,
		`include\s*\(\s*`,
		`require\s*\(\s*`,
		`pickle\.loads\s*\(\s*`,
		`yaml\.load\s*\(\s*`,
		`JSON\.parse\s*\(\s*`,
	}

	for _, pattern := range dangerousPatterns {
		re := regexp.MustCompile(`(?m)^.*` + pattern + `.*$`)
		matches := re.FindAllString(analysis, 3)
		if len(matches) > 0 {
			result := strings.Join(matches, "\n")
			if len(result) > 200 {
				return result[:200] + "..."
			}
			return result
		}
	}

	// 4. 尝试提取包含变量定义或函数调用的行
	codePatterns := []string{
		`(?m)^.*=\s*.*$`,   // 变量定义
		`(?m)^.*\(.*\).*$`, // 函数调用
		`(?m)^.*\{.*\}.*$`, // 代码块
		`(?m)^.*\;.*$`,     // 以分号结尾的行
	}

	for _, pattern := range codePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(analysis, 5)
		if len(matches) > 0 {
			result := strings.Join(matches, "\n")
			if len(result) > 200 {
				return result[:200] + "..."
			}
			return result
		}
	}

	// 5. 如果以上都失败，返回文件路径信息
	return fmt.Sprintf("（未能提取代码片段，文件路径: %s）", filePath)
}

// extractVulnTypeFromAnalysis 从分析结果中提取漏洞类型
func extractVulnTypeFromAnalysis(analysis string) string {
	// 常见的漏洞类型
	vulnTypes := []string{
		"SQL注入", "命令注入", "代码注入", "远程代码执行", "RCE",
		"跨站脚本", "XSS", "存储型XSS", "反射型XSS", "DOM型XSS",
		"路径遍历", "目录遍历", "文件包含", "任意文件读取", "任意文件写入",
		"不安全的反序列化", "反序列化漏洞", "XXE", "XML外部实体",
		"敏感信息泄露", "信息泄露", "密码泄露", "密钥泄露",
		"认证绕过", "授权问题", "越权访问", "权限绕过",
		"SSRF", "服务器端请求伪造",
		"CSRF", "跨站请求伪造",
		"模板注入", "SSTI", "SST",
		"文件上传", "任意文件上传",
		"日志注入", "格式化字符串",
		"正则表达式拒绝服务", "ReDoS",
		"硬编码", "硬编码密码",
		"JNDI注入", "SpEL注入", "OGNL注入",
	}

	for _, vuln := range vulnTypes {
		if strings.Contains(analysis, vuln) {
			return vuln
		}
	}

	// 尝试从标题中提取
	titlePattern := regexp.MustCompile(`(?:#{1,6}\s+)?(?:漏洞|问题)\s*\d*\s*[:：]\s*(\S+)`)
	matches := titlePattern.FindStringSubmatch(analysis)
	if len(matches) > 1 {
		return matches[1]
	}

	return "未知漏洞类型"
}

// extractLineNumber 从分析结果中提取行号
func extractLineNumber(analysis string) string {
	// 尝试匹配常见的行号格式
	linePatterns := []string{
		`行[号]?\s*[:：]\s*(\d+)`,
		`line\s*[:：]\s*(\d+)`,
		`第\s*(\d+)\s*行`,
		`位于\s+.*?[:：](\d+)`,
		`:(\d+)[)\s]`, // 如 "file.java:38)"
	}

	for _, pattern := range linePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(analysis)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return "未知"
}

func (s *AuditService) formatVulnerability(r AuditResult) string {
	var b strings.Builder
	// 清理文件路径
	b.WriteString(fmt.Sprintf("### %s\n", cleanFilePath(r.File)))
	b.WriteString(fmt.Sprintf("- **严重程度**: %s\n", r.Severity))
	b.WriteString(fmt.Sprintf("- **类型**: %s\n\n", r.Type))
	b.WriteString("**分析结果**:\n\n")
	b.WriteString(r.Analysis)
	b.WriteString("\n\n---\n\n")
	return b.String()
}

// extractCWEFromAnalysis 从分析结果中提取CWE编号
func extractCWEFromAnalysis(analysis string) string {
	cwePattern := regexp.MustCompile(`(?i)CWE[-\s:]?(\d+)`)
	matches := cwePattern.FindStringSubmatch(analysis)
	if len(matches) > 1 {
		return "CWE-" + matches[1]
	}
	return ""
}

// extractCodeSnippetFromAnalysis 从分析结果中提取代码片段
func extractCodeSnippetFromAnalysis(analysis string) string {
	// 尝试从Markdown代码块中提取
	codeBlockPattern := regexp.MustCompile("```[\\s\\S]*?```")
	codeBlocks := codeBlockPattern.FindAllString(analysis, -1)

	for _, block := range codeBlocks {
		block = strings.TrimPrefix(block, "```")
		block = strings.TrimSuffix(block, "```")
		lines := strings.Split(block, "\n")
		if len(lines) > 0 {
			block = strings.Join(lines[1:], "\n")
		}
		block = strings.TrimSpace(block)

		// 跳过太短的代码块
		if len(block) > 20 && !strings.Contains(block, "建议") && !strings.Contains(block, "修复") {
			return block
		}
	}

	// 尝试提取"危险代码"、"漏洞代码"后的内容
	evidencePatterns := []string{
		`危险代码[：:]\s*([\s\S]{20,300})`,
		`漏洞代码[：:]\s*([\s\S]{20,300})`,
		`问题代码[：:]\s*([\s\S]{20,300})`,
	}

	for _, pattern := range evidencePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(analysis)
		if len(matches) > 1 {
			snippet := strings.TrimSpace(matches[1])
			// 限制长度
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			return snippet
		}
	}

	return ""
}

// extractDescriptionFromAnalysis 从分析结果中提取漏洞描述
func extractDescriptionFromAnalysis(analysis string) string {
	// 尝试提取描述段落
	descPatterns := []string{
		`(?:漏洞描述|描述)[:：]\s*([\s\S]{50,500}?)(?:\n\n|\n##|\n---\n)`,
		`该漏洞[是会导致]?([\s\S]{50,300}?)(?:\n\n|\n##|\n---\n)`,
		`(?m)^(?!.*(?:修复|建议|代码|POC|利用)).+$`, // 排除包含特定关键词的行
	}

	for _, pattern := range descPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(analysis)
		if len(matches) > 1 {
			desc := strings.TrimSpace(matches[1])
			// 清理格式
			desc = strings.ReplaceAll(desc, "**", "")
			desc = strings.ReplaceAll(desc, "```", "")
			if len(desc) > 300 {
				desc = desc[:300] + "..."
			}
			return desc
		}
	}

	// 如果无法提取，返回清理后的分析结果前500字符
	cleaned := analysis
	cleaned = strings.ReplaceAll(cleaned, "```", "")
	cleaned = strings.ReplaceAll(cleaned, "**", "")
	if len(cleaned) > 300 {
		cleaned = cleaned[:300] + "..."
	}
	return cleaned
}

// generateExploitChain 生成漏洞利用链
func generateExploitChain(r AuditResult) string {
	// 根据漏洞类型生成典型的利用链
	vulnType := extractVulnerabilityName(r.Analysis)

	chains := map[string]string{
		"SQL注入":    "用户输入 → 拼接到SQL语句 → 执行恶意SQL → 数据库信息泄露/篡改",
		"命令注入":     "用户输入 → 拼接到系统命令 → 执行任意系统命令 → 服务器接管",
		"路径遍历":     "用户输入 → 拼接到文件路径 → 读取/写入任意文件 → 配置泄露/代码执行",
		"不安全的反序列化": "恶意序列化数据 → 反序列化 → 执行任意代码 → 服务器接管",
		"模板注入":     "用户输入 → 拼接到模板 → 模板引擎执行 → 远程代码执行",
		"敏感信息泄露":   "代码中硬编码 → 配置错误 → 信息被获取",
		"文件包含":     "用户输入 → 拼接到文件路径 → 包含恶意文件 → 代码执行",
		"SSRF":     "用户输入URL → 服务器发起请求 → 访问内部资源/端口探测",
		"认证绕过":     "未验证身份 → 直接访问受保护资源 → 越权操作",
	}

	if chain, ok := chains[vulnType]; ok {
		return chain
	}

	// 默认利用链
	return fmt.Sprintf("用户输入 → 漏洞函数 → %s → 造成安全风险", vulnType)
}

// extractFixSuggestionFromAnalysis 从分析结果中提取修复建议
func extractFixSuggestionFromAnalysis(analysis string) string {
	// 尝试提取修复代码
	codeBlockPattern := regexp.MustCompile("```(?:java|python|go|javascript|php)?[\\s\\S]*?```")
	codeBlocks := codeBlockPattern.FindAllString(analysis, -1)

	var fixBlocks []string
	for _, block := range codeBlocks {
		if strings.Contains(block, "修复") || strings.Contains(block, "建议") ||
			strings.Contains(block, "使用") || strings.Contains(block, "应该") {
			block = strings.TrimPrefix(block, "```")
			block = strings.TrimSuffix(block, "```")
			lines := strings.Split(block, "\n")
			if len(lines) > 1 {
				block = strings.Join(lines[1:], "\n")
			}
			block = strings.TrimSpace(block)
			if len(block) > 10 {
				fixBlocks = append(fixBlocks, block)
			}
		}
	}

	if len(fixBlocks) > 0 {
		return strings.Join(fixBlocks, "\n\n")
	}

	// 尝试提取"修复建议"后的内容
	fixPattern := regexp.MustCompile(`修复建议[:：]\s*([\s\S]{20,500})`)
	matches := fixPattern.FindStringSubmatch(analysis)
	if len(matches) > 1 {
		fix := strings.TrimSpace(matches[1])
		if len(fix) > 300 {
			fix = fix[:300] + "..."
		}
		return fix
	}

	return ""
}

// cleanFilePath 清理文件路径，将任务ID替换为任务名称显示
func cleanFilePath(filePath string) string {
	// 替换 sandbox/audit-sandbox/{ID}/ 为相对路径
	re := regexp.MustCompile(`sandbox/audit-sandbox/\d+/`)
	cleanPath := re.ReplaceAllString(filePath, "sandbox/[任务]/")

	// 简化路径
	if strings.Contains(cleanPath, "/") {
		parts := strings.Split(cleanPath, "/")
		if len(parts) > 3 {
			cleanPath = strings.Join(parts[len(parts)-3:], "/")
		}
	}
	return cleanPath
}

// updateTaskProgress 更新任务进度到数据库
func updateTaskProgress(task *AuditTask, message string) {
	fmt.Printf("Task %s: %d%% - %s\n", task.ID, task.Progress, message)

	if globalProgressManager != nil {
		globalProgressManager.UpdateTaskProgress(task.ID, task.Progress, "running", message, task.AILog)
	}

	if taskID, err := strconv.ParseUint(task.ID, 10, 32); err == nil {
		// 更新数据库中的进度和AI日志
		util.DB.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
			"progress":            task.Progress,
			"scanned_files":       task.ScannedFiles,
			"vulnerability_count": task.VulnerabilityCount,
			"current_file":        task.CurrentFile,
			"log":                 task.Log,
			"ai_log":              task.AILog,
			"detected_language":   task.Language,
			"status":              "running",
		})
	}
}

// updateCurrentFile 更新当前分析的文件
func updateCurrentFile(task *AuditTask, file string) {
	if taskID, err := strconv.ParseUint(task.ID, 10, 32); err == nil {
		util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("current_file", file)
	}
}

// updateScannedFiles 更新已扫描文件数
func updateScannedFiles(task *AuditTask, count int) {
	if taskID, err := strconv.ParseUint(task.ID, 10, 32); err == nil {
		util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("scanned_files", count)
	}
}

// updateVulnerabilityCount 更新漏洞数量
func updateVulnerabilityCount(task *AuditTask, count int) {
	if taskID, err := strconv.ParseUint(task.ID, 10, 32); err == nil {
		util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("vulnerability_count", count)
	}
}

// updateTaskLog 更新任务日志
func updateTaskLog(task *AuditTask, log string) {
	if taskID, err := strconv.ParseUint(task.ID, 10, 32); err == nil {
		util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("log", log)
	}
}

// updateAILog 更新大模型交互日志
func updateAILog(task *AuditTask, aiLog string) {
	if taskID, err := strconv.ParseUint(task.ID, 10, 32); err == nil {
		util.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("ai_log", aiLog)
	}
}

// ProgressManagerInterface 进度管理器接口
type ProgressManagerInterface interface {
	UpdateTaskProgress(taskID string, progress int, status, message, aiLog string)
	UpdateTaskResult(taskID string, result string)
}

// 全局进度管理器
var globalProgressManager ProgressManagerInterface

// SetProgressManager 设置全局进度管理器
func SetProgressManager(pm ProgressManagerInterface) {
	globalProgressManager = pm
}

// SaveTask 保存任务状态
func (s *AuditService) SaveTask(task *AuditTask) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("tasks/%s.json", task.ID)

	if err := os.MkdirAll("tasks", 0755); err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadTask 加载任务状态
func (s *AuditService) LoadTask(id string) (*AuditTask, error) {
	filename := fmt.Sprintf("tasks/%s.json", id)

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var task AuditTask
	err = json.Unmarshal(data, &task)
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// ListTasks 列出所有任务
func (s *AuditService) ListTasks() ([]*AuditTask, error) {
	var tasks []*AuditTask

	files, err := filepath.Glob("tasks/*.json")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var task AuditTask
		err = json.Unmarshal(data, &task)
		if err != nil {
			continue
		}

		tasks = append(tasks, &task)
	}

	return tasks, nil
}

// ======================== MCP核心功能 ========================
// ExtractZip 解压ZIP文件到沙盒目录
func (s *AuditService) ExtractZip(zipPath, destDir string) (string, error) {
	if !isPathSafe(zipPath) || !isPathSafe(destDir) {
		return "", fmt.Errorf("不安全的路径")
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("打开ZIP文件失败: %v", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		filePath := filepath.Join(destDir, file.Name)

		if !isPathSafe(filePath) {
			continue
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(filePath, 0755)
		} else {
			os.MkdirAll(filepath.Dir(filePath), 0755)
			src, err := file.Open()
			if err != nil {
				continue
			}
			dst, err := os.Create(filePath)
			if err != nil {
				src.Close()
				continue
			}
			io.Copy(dst, src)
			src.Close()
			dst.Close()
		}
	}

	return destDir, nil
}

// ExtractJar 解压JAR文件到沙盒目录（JAR就是ZIP格式）
func (s *AuditService) ExtractJar(jarPath, destDir string) (string, error) {
	return s.ExtractZip(jarPath, destDir)
}

// ListFiles 列出目录中的文件
func (s *AuditService) ListFiles(dirPath string) ([]string, error) {
	if !isPathSafe(dirPath) {
		return nil, fmt.Errorf("不安全的路径")
	}

	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %v", err)
	}

	return files, nil
}

// ReadFileContent 读取文件内容（支持 Unicode/GBK/GB2312 自动解码）
func (s *AuditService) ReadFileContent(filePath string) (string, error) {
	if !isPathSafe(filePath) {
		return "", fmt.Errorf("不安全的路径")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	// 尝试自动检测并转换编码
	decodedContent := decodeUnicodeString(string(content))
	return decodedContent, nil
}

// WriteFile 写入文件内容到沙盒
func (s *AuditService) WriteFile(filePath, content string) error {
	if !isPathSafe(filePath) {
		return fmt.Errorf("不安全的路径")
	}

	if len(content) > 10*1024*1024 {
		return fmt.Errorf("文件内容过大")
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

// ExecuteCurlInSandbox 在沙盒环境中执行curl
func (s *AuditService) ExecuteCurlInSandbox(url, method, data string) (string, error) {
	if !isURLSafe(url) {
		return "", fmt.Errorf("不安全的URL")
	}

	args := []string{"-s", "-X", method, "--max-time", "30", "--connect-timeout", "10"}

	if data != "" {
		args = append(args, "-d", data)
	}

	args = append(args, url)

	cmd := exec.Command("curl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("执行curl失败: %v", err)
	}

	return string(output), nil
}

// isPathSafe 检查路径是否安全（防止路径遍历攻击）
func isPathSafe(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	dangerousPatterns := []string{"../", "..\\", "~", "$"}
	for _, pattern := range dangerousPatterns {
		if strings.Contains(absPath, pattern) {
			return false
		}
	}

	return true
}

// isURLSafe 检查URL是否安全
func isURLSafe(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// CreateSandbox 创建沙盒目录
func (s *AuditService) CreateSandbox(taskID string) (string, error) {
	sandboxDir := filepath.Join(".", "sandbox", "audit-sandbox", taskID)
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		return "", fmt.Errorf("创建沙盒失败: %v", err)
	}
	return sandboxDir, nil
}

// CleanupSandbox 清理沙盒目录
func (s *AuditService) CleanupSandbox(taskID string) error {
	sandboxDir := filepath.Join(".", "sandbox", "audit-sandbox", taskID)
	return os.RemoveAll(sandboxDir)
}

// ==================== Worker动态调整功能 ====================

// calculateOptimalWorkers 根据服务器性能和文件数量动态计算最佳worker数量
func calculateOptimalWorkers(totalFiles int) int {
	// 获取CPU核心数
	numCPU := runtime.NumCPU()

	// 获取内存信息（单位：字节）
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	totalMemoryGB := memStats.Sys / (1024 * 1024 * 1024) // 转换为GB

	// 根据文件数量确定基础worker数
	baseWorkers := numCPU
	if baseWorkers > 16 {
		baseWorkers = 16 // 最多16个worker
	}
	if baseWorkers < 2 {
		baseWorkers = 2 // 最少2个worker
	}

	// 根据文件数量调整
	if totalFiles < 10 {
		baseWorkers = 1
	} else if totalFiles < 50 {
		baseWorkers = min(2, baseWorkers)
	} else if totalFiles < 100 {
		baseWorkers = min(4, baseWorkers)
	} else if totalFiles < 500 {
		baseWorkers = min(6, baseWorkers)
	} else {
		baseWorkers = min(8, baseWorkers)
	}

	// 根据可用内存调整（每GB内存支持约2个worker）
	maxWorkersByMem := int(totalMemoryGB * 2)
	if maxWorkersByMem < 1 {
		maxWorkersByMem = 1
	}

	// 取两者的最小值，避免内存不足
	optimalWorkers := min(baseWorkers, maxWorkersByMem)

	// 确保至少1个worker
	if optimalWorkers < 1 {
		optimalWorkers = 1
	}

	auditLogger.Printf("动态worker计算: CPU=%d核, 内存=%.1fGB, 文件数=%d, 最佳worker数=%d",
		numCPU, totalMemoryGB, totalFiles, optimalWorkers)

	return optimalWorkers
}

// ==================== 新增功能 ====================

// AnalyzeProjectStructure 分析项目结构，返回详细的文件分类和优先级
func (s *AuditService) AnalyzeProjectStructure(rootPath string) (*ProjectStructure, error) {
	structure := &ProjectStructure{
		RootPath:        rootPath,
		Files:           []FileInfo{},
		BusinessFiles:   []FileInfo{},
		TestFiles:       []FileInfo{},
		ConfigFiles:     []FileInfo{},
		DependencyFiles: []FileInfo{},
		BuildArtifacts:  []FileInfo{},
		SkippedFiles:    []FileInfo{},
		FileTypeStats:   make(map[string]int),
	}

	var totalSize int64

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(rootPath, path)
		ext := strings.ToLower(filepath.Ext(path))

		// 判断是否为目录
		if info.IsDir() {
			structure.TotalDirs++
			return nil
		}

		structure.TotalFiles++
		totalSize += info.Size()

		// 创建文件信息
		fileInfo := FileInfo{
			Path:        relPath,
			IsDir:       false,
			Size:        info.Size(),
			Extension:   ext,
			Priority:    5, // 默认优先级
			Category:    "unknown",
			ShouldAudit: true,
		}

		// 分类和设置优先级
		category, priority, shouldAudit, skipReason := classifyFile(relPath, ext, info)
		fileInfo.Category = category
		fileInfo.Priority = priority
		fileInfo.ShouldAudit = shouldAudit
		fileInfo.SkipReason = skipReason

		// 设置编程语言
		fileInfo.Language = getLanguageFromExt(ext)

		// 统计文件类型
		structure.FileTypeStats[ext]++

		// 根据分类添加到对应列表
		switch category {
		case "business":
			structure.BusinessFiles = append(structure.BusinessFiles, fileInfo)
		case "test":
			structure.TestFiles = append(structure.TestFiles, fileInfo)
		case "config":
			structure.ConfigFiles = append(structure.ConfigFiles, fileInfo)
		case "dependency":
			structure.DependencyFiles = append(structure.DependencyFiles, fileInfo)
		case "build":
			structure.BuildArtifacts = append(structure.BuildArtifacts, fileInfo)
		case "skip":
			structure.SkippedFiles = append(structure.SkippedFiles, fileInfo)
		}

		if shouldAudit {
			structure.Files = append(structure.Files, fileInfo)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("分析项目结构失败: %v", err)
	}

	structure.TotalSize = totalSize

	// 按优先级排序（高优先级在前）
	sortFilesByPriority(structure.BusinessFiles)
	sortFilesByPriority(structure.TestFiles)

	return structure, nil
}

// classifyFile 分类文件并确定优先级
func classifyFile(relPath, ext string, info os.FileInfo) (category string, priority int, shouldAudit bool, skipReason string) {
	// 构建产物 - 不审计
	buildPatterns := []string{
		"target/", "build/", "dist/", ".next/", ".nuxt/",
		"out/", "bin/", "obj/", ".gradle/", ".mvn/",
		"node_modules/", "bower_components/", "vendor/",
		"__pycache__/", ".pytest_cache/", ".tox/",
	}
	for _, pattern := range buildPatterns {
		if strings.Contains(relPath, pattern) {
			return "build", 0, false, "构建产物"
		}
	}

	// 依赖文件 - 低优先级
	depPatterns := []string{
		".min.js", ".min.css", ".bundle.js", ".bundle.css",
		"package-lock.json", "yarn.lock", "package.json",
		"pom.xml", "build.gradle", "requirements.txt",
		"go.mod", "go.sum", "Cargo.lock",
	}
	for _, pattern := range depPatterns {
		if strings.Contains(relPath, pattern) {
			return "dependency", 1, false, "依赖配置文件"
		}
	}

	// 二进制文件 - 不审计
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".a", ".o",
		".class", ".pyc", ".pyo", ".jar", ".war",
		".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico",
		".mp3", ".mp4", ".avi", ".mov", ".wav",
		".pdf", ".doc", ".docx", ".xls", ".xlsx",
		".zip", ".tar", ".gz", ".rar", ".7z",
		".ttf", ".otf", ".woff", ".woff2", ".eot",
	}
	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return "skip", 0, false, "二进制/资源文件"
		}
	}

	// 测试文件 - 中等优先级
	testPatterns := []string{
		"/test/", "/tests/", "/spec/", "/specs/",
		"_test.go", "_test.py", ".test.js", ".test.ts",
		".spec.js", ".spec.ts", "Test.java", "Tests.java",
	}
	for _, pattern := range testPatterns {
		if strings.Contains(relPath, pattern) {
			return "test", 5, true, ""
		}
	}

	// 配置文件 - 低优先级
	configPatterns := []string{
		".yml", ".yaml", ".json", ".xml", ".toml",
		".ini", ".conf", ".config", ".properties",
		".env", ".gitignore", ".dockerignore",
	}
	for _, pattern := range configPatterns {
		if ext == pattern || strings.HasSuffix(relPath, pattern) {
			return "config", 2, true, ""
		}
	}

	// 脚本文件
	scriptExts := []string{".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd"}
	for _, scriptExt := range scriptExts {
		if ext == scriptExt {
			return "business", 6, true, "" // 脚本通常很重要
		}
	}

	// 业务代码 - 高优先级
	businessExts := []string{
		".java", ".php", ".py", ".js", ".ts", ".jsx", ".tsx",
		".go", ".cs", ".rb", ".swift", ".kt", ".scala",
		".rs", ".dart", ".groovy", ".c", ".cpp", ".h", ".hpp",
		".vue", ".svelte", ".jsx",
	}
	for _, businessExt := range businessExts {
		if ext == businessExt {
			// 根据文件大小调整优先级
			priority := 8
			if info.Size() > 100*1024 { // > 100KB
				priority = 7 // 太大可能不是核心逻辑
			} else if info.Size() < 1024 { // < 1KB
				priority = 6 // 太小的文件可能不重要
			}
			return "business", priority, true, ""
		}
	}

	// 默认
	return "skip", 0, false, "非代码文件"
}

// sortFilesByPriority 按优先级降序排序
func sortFilesByPriority(files []FileInfo) {
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].Priority > files[i].Priority {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}

// getLanguageFromExt 根据扩展名获取编程语言
func getLanguageFromExt(ext string) string {
	langMap := map[string]string{
		".java": "Java", ".kt": "Kotlin", ".scala": "Scala",
		".py": "Python", ".js": "JavaScript", ".ts": "TypeScript",
		".jsx": "React", ".tsx": "React", ".vue": "Vue",
		".go": "Go", ".cs": "C#", ".rb": "Ruby",
		".swift": "Swift", ".rs": "Rust", ".php": "PHP",
		".c": "C", ".cpp": "C++", ".h": "C/C++",
		".dart": "Dart", ".groovy": "Groovy",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "Unknown"
}

// GetAuditFiles 获取应该审计的文件列表（按优先级排序）
func (s *AuditService) GetAuditFiles(rootPath string, selectedPaths []string) ([]string, error) {
	structure, err := s.AnalyzeProjectStructure(rootPath)
	if err != nil {
		return nil, err
	}

	// 如果用户选择了特定路径，只审计选中的
	if len(selectedPaths) > 0 {
		var selectedFiles []string
		for _, selected := range selectedPaths {
			fullPath := filepath.Join(rootPath, selected)
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				selectedFiles = append(selectedFiles, fullPath)
			}
		}
		return selectedFiles, nil
	}

	// 按优先级返回所有应审计的文件
	var auditFiles []string
	for _, f := range structure.BusinessFiles {
		if f.ShouldAudit {
			auditFiles = append(auditFiles, filepath.Join(rootPath, f.Path))
		}
	}
	for _, f := range structure.TestFiles {
		if f.ShouldAudit {
			auditFiles = append(auditFiles, filepath.Join(rootPath, f.Path))
		}
	}
	for _, f := range structure.ConfigFiles {
		if f.ShouldAudit {
			auditFiles = append(auditFiles, filepath.Join(rootPath, f.Path))
		}
	}

	return auditFiles, nil
}

// ==================== 增量分析功能 ====================

// globalHashStore 全局文件哈希存储
var globalHashStore = &FileHashStore{
	hashes: make(map[string]string),
}

// CalculateFileHash 计算文件哈希
func (s *AuditService) CalculateFileHash(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := fnv64a(content)
	return fmt.Sprintf("%x", hash), nil
}

// GetChangedFiles 获取变更的文件列表
func (s *AuditService) GetChangedFiles(rootPath string, previousHashes map[string]string) ([]string, []string, error) {
	structure, err := s.AnalyzeProjectStructure(rootPath)
	if err != nil {
		return nil, nil, err
	}

	var changedFiles []string
	var newFiles []string

	for _, f := range structure.Files {
		fullPath := filepath.Join(rootPath, f.Path)
		currentHash, err := s.CalculateFileHash(fullPath)
		if err != nil {
			continue
		}

		prevHash, existed := previousHashes[f.Path]
		if !existed {
			// 新文件
			newFiles = append(newFiles, fullPath)
		} else if prevHash != currentHash {
			// 变更的文件
			changedFiles = append(changedFiles, fullPath)
		}
	}

	return changedFiles, newFiles, nil
}

// GetUnchangedFiles 获取未变更的文件列表
func (s *AuditService) GetUnchangedFiles(rootPath string, previousHashes map[string]string) ([]string, error) {
	structure, err := s.AnalyzeProjectStructure(rootPath)
	if err != nil {
		return nil, err
	}

	var unchangedFiles []string

	for _, f := range structure.Files {
		fullPath := filepath.Join(rootPath, f.Path)
		currentHash, err := s.CalculateFileHash(fullPath)
		if err != nil {
			continue
		}

		prevHash, existed := previousHashes[f.Path]
		if existed && prevHash == currentHash {
			unchangedFiles = append(unchangedFiles, fullPath)
		}
	}

	return unchangedFiles, nil
}

// StoreFileHashes 存储文件哈希
func (s *AuditService) StoreFileHashes(rootPath string) (map[string]string, error) {
	structure, err := s.AnalyzeProjectStructure(rootPath)
	if err != nil {
		return nil, err
	}

	hashes := make(map[string]string)
	for _, f := range structure.Files {
		fullPath := filepath.Join(rootPath, f.Path)
		hash, err := s.CalculateFileHash(fullPath)
		if err != nil {
			continue
		}
		hashes[f.Path] = hash
	}

	return hashes, nil
}

// ==================== POC批量生成功能 ====================

// POCBatchRequest POC批量生成请求
type POCBatchRequest struct {
	Vulnerabilities []AuditResult `json:"vulnerabilities"`
	BatchSize       int           `json:"batchSize"`  // 每批处理的漏洞数
	MaxRetries      int           `json:"maxRetries"` // 最大重试次数
}

// POCBatchResult POC批量生成结果
type POCBatchResult struct {
	TotalCount    int                   `json:"totalCount"`
	SuccessCount  int                   `json:"successCount"`
	FailedCount   int                   `json:"failedCount"`
	Results       []POCGenerationResult `json:"results"`
	TotalDuration time.Duration         `json:"totalDuration"`
}

// POCGenerationResult 单个POC生成结果
type POCGenerationResult struct {
	Index        int    `json:"index"`
	VulnType     string `json:"vulnType"`
	FilePath     string `json:"filePath"`
	POC          string `json:"poc"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// BatchGeneratePOC 批量生成POC（并发优化版）
func (s *AuditService) BatchGeneratePOC(req POCBatchRequest) POCBatchResult {
	startTime := time.Now()
	result := POCBatchResult{
		TotalCount:    len(req.Vulnerabilities),
		Results:       make([]POCGenerationResult, len(req.Vulnerabilities)),
		TotalDuration: 0,
	}

	if req.BatchSize <= 0 {
		req.BatchSize = 3 // 默认每批3个
	}
	if req.MaxRetries <= 0 {
		req.MaxRetries = 2
	}

	// 使用信号量控制并发数
	sem := make(chan struct{}, req.BatchSize)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, vuln := range req.Vulnerabilities {
		wg.Add(1)
		go func(index int, v AuditResult) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			pocResult := POCGenerationResult{
				Index:    index,
				VulnType: extractVulnTypeFromAnalysis(v.Analysis),
				FilePath: v.File,
			}

			// 带重试的POC生成
			for retry := 0; retry <= req.MaxRetries; retry++ {
				poc := generatePOC(v, s.client, s.modelName, s.maxTokens, s.temperature)
				if poc != "" {
					pocResult.POC = poc
					pocResult.Success = true
					break
				}

				// 失败后等待重试
				if retry < req.MaxRetries {
					time.Sleep(time.Duration(retry+1) * time.Second)
				}
			}

			if !pocResult.Success {
				pocResult.ErrorMessage = "POC生成失败"
			}

			mu.Lock()
			result.Results[index] = pocResult
			if pocResult.Success {
				result.SuccessCount++
			} else {
				result.FailedCount++
			}
			mu.Unlock()

		}(i, vuln)
	}

	wg.Wait()
	result.TotalDuration = time.Since(startTime)

	return result
}

// ==================== POC异步生成功能 ====================

// POCAsyncRequest POC异步生成请求
type POCAsyncRequest struct {
	Vulnerabilities []AuditResult            `json:"vulnerabilities"`
	TaskID          string                   `json:"taskId"`
	OnProgress      func(current, total int) `json:"-"` // 进度回调
}

// POCAsyncJob POC异步任务
type POCAsyncJob struct {
	ID          string                `json:"id"`
	Request     POCAsyncRequest       `json:"request"`
	Status      string                `json:"status"` // pending, running, completed, failed
	Progress    int                   `json:"progress"`
	Current     int                   `json:"current"`
	Total       int                   `json:"total"`
	Results     []POCGenerationResult `json:"results"`
	Error       string                `json:"error,omitempty"`
	CreatedAt   time.Time             `json:"createdAt"`
	CompletedAt *time.Time            `json:"completedAt,omitempty"`
}

// pocJobs 全局POC异步任务存储
var pocJobs = make(map[string]*POCAsyncJob)
var pocJobsMu sync.RWMutex

// GeneratePOCAsync 异步生成POC（后台运行）
func (s *AuditService) GeneratePOCAsync(req POCAsyncRequest) string {
	jobID := fmt.Sprintf("poc_%d", time.Now().UnixNano())

	job := &POCAsyncJob{
		ID:        jobID,
		Request:   req,
		Status:    "pending",
		Progress:  0,
		Current:   0,
		Total:     len(req.Vulnerabilities),
		Results:   make([]POCGenerationResult, len(req.Vulnerabilities)),
		CreatedAt: time.Now(),
	}

	// 保存任务
	pocJobsMu.Lock()
	pocJobs[jobID] = job
	pocJobsMu.Unlock()

	// 后台执行
	go func() {
		s.runPOCGenerationJob(job)
	}()

	return jobID
}

// runPOCGenerationJob 执行POC生成任务
func (s *AuditService) runPOCGenerationJob(job *POCAsyncJob) {
	job.Status = "running"

	for i, vuln := range job.Request.Vulnerabilities {
		// 检查任务是否被取消
		pocJobsMu.RLock()
		currentJob := pocJobs[job.ID]
		pocJobsMu.RUnlock()

		if currentJob.Status == "failed" {
			return
		}

		pocResult := POCGenerationResult{
			Index:    i,
			VulnType: extractVulnTypeFromAnalysis(vuln.Analysis),
			FilePath: vuln.File,
		}

		// 生成POC
		poc := generatePOC(vuln, s.client, s.modelName, s.maxTokens, s.temperature)
		if poc != "" {
			pocResult.POC = poc
			pocResult.Success = true
		} else {
			pocResult.ErrorMessage = "POC生成失败"
		}

		// 更新结果
		job.Results[i] = pocResult
		job.Current = i + 1
		job.Progress = (i + 1) * 100 / job.Total

		// 调用进度回调
		if job.Request.OnProgress != nil {
			job.Request.OnProgress(job.Current, job.Total)
		}

		// 短暂休息，避免API限流
		time.Sleep(200 * time.Millisecond)
	}

	// 标记完成
	job.Status = "completed"
	now := time.Now()
	job.CompletedAt = &now
}

// GetPOCJobStatus 获取POC异步任务状态
func (s *AuditService) GetPOCJobStatus(jobID string) (*POCAsyncJob, bool) {
	pocJobsMu.RLock()
	defer pocJobsMu.RUnlock()

	job, exists := pocJobs[jobID]
	if !exists {
		return nil, false
	}

	// 返回副本，避免竞争
	jobCopy := *job
	jobCopy.Results = make([]POCGenerationResult, len(job.Results))
	copy(jobCopy.Results, job.Results)

	return &jobCopy, true
}

// CancelPOCJob 取消POC异步任务
func (s *AuditService) CancelPOCJob(jobID string) bool {
	pocJobsMu.Lock()
	defer pocJobsMu.Unlock()

	job, exists := pocJobs[jobID]
	if !exists {
		return false
	}

	if job.Status == "running" {
		job.Status = "failed"
		job.Error = "任务被取消"
		return true
	}

	return false
}

// ListPOCJobs 列出所有POC异步任务
func (s *AuditService) ListPOCJobs() []*POCAsyncJob {
	pocJobsMu.RLock()
	defer pocJobsMu.RUnlock()

	jobs := make([]*POCAsyncJob, 0, len(pocJobs))
	for _, job := range pocJobs {
		jobs = append(jobs, job)
	}

	return jobs
}
