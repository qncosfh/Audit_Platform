package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"platform/utils"
)

// MethodCall 方法调用关系
type MethodCall struct {
	CallerFile string
	Caller     string
	CalleeFile string
	Callee     string
	Line       int
}

// DataFlow 数据流信息
type DataFlow struct {
	Source     string
	Sink       string
	Path       []string
	SourceFile string
	SinkFile   string
}

// VulnerabilityChain 利用链
type VulnerabilityChain struct {
	Type        string
	Severity    string
	Source      string
	Sink        string
	Description string
	Chain       []string
	RiskLevel   string
	Language    string // 编程语言
}

// CodeAnalyzer 代码分析器 - 支持多语言
type CodeAnalyzer struct {
	Files        map[string]string   // 文件路径 -> 内容
	Language     string              // 主要语言
	FilePatterns map[string][]string // 语言 -> 扩展名
}

// LanguageConfig 语言配置（支持审计的语言）
var LanguageConfig = map[string][]string{
	"java":         {".java"},
	"php":          {".php"},
	"python":       {".py"},
	"csharp":       {".cs"},
	"go":           {".go"},
	"js":           {".js", ".jsx"},
	"ts":           {".ts", ".tsx"},
	"vue":          {".vue"},
	"svelte":       {".svelte"},
	"ruby":         {".rb"},
	"swift":        {".swift"},
	"kotlin":       {".kt", ".kts"},
	"scala":        {".scala"},
	"c":            {".c", ".h"},
	"cpp":          {".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".c++", ".h++"},
	"rust":         {".rs"},
	"dart":         {".dart"},
	"groovy":       {".groovy"},
	"lua":          {".lua"},
	"perl":         {".pl", ".pm"},
	"haskell":      {".hs", ".lhs"},
	"erlang":       {".erl", ".hrl"},
	"elixir":       {".ex", ".exs"},
	"clojure":      {".clj", ".cljs", ".cljc"},
	"fsharp":       {".fs", ".fsi", ".fsx"},
	"ocaml":        {".ml", ".mli"},
	"r":            {".r", ".R"},
	"julia":        {".jl"},
	"matlab":       {".m"},
	"fortran":      {".f", ".f90", ".f95", ".f03"},
	"zig":          {".zig"},
	"nim":          {".nim"},
	"d":            {".d"},
	"delphi":       {".pas", ".dpk"},
	"vb":           {".vb"},
	"coffeescript": {".coffee"},
	"shell":        {".sh", ".bash", ".zsh", ".fish"},
	"powershell":   {".ps1", ".psm1"},
	"sql":          {".sql"},
	"yaml":         {".yaml", ".yml"},
	"toml":         {".toml"},
	"ini":          {".ini", ".conf", ".cfg"},
}

// NewCodeAnalyzer 创建代码分析器（自动解码 Unicode）
func NewCodeAnalyzer(sourcePath string) (*CodeAnalyzer, error) {
	analyzer := &CodeAnalyzer{
		Files:        make(map[string]string),
		FilePatterns: LanguageConfig,
	}

	// 自动检测语言
	analyzer.detectLanguage(sourcePath)

	// 读取所有代码文件
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// 跳过不需要的目录
			skipDirs := []string{"node_modules", ".git", "target", "build", "dist", "vendor", "venv", ".idea", ".vscode", "__pycache__", "vendor/bundle"}
			for _, skip := range skipDirs {
				if strings.Contains(path, skip) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// 根据语言过滤文件
		if analyzer.shouldIncludeFile(path) {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			// 解码 Unicode 转义序列
			decoded := utils.DecodeUnicodeString(string(content))
			analyzer.Files[path] = decoded
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return analyzer, nil
}

// detectLanguage 自动检测源代码语言
func (a *CodeAnalyzer) detectLanguage(sourcePath string) {
	extensions := make(map[string]int)

	filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))
		extensions[ext]++
		return nil
	})

	// 找出最常见的扩展名
	maxCount := 0
	for ext, count := range extensions {
		if count > maxCount {
			maxCount = count
			for lang, patterns := range LanguageConfig {
				for _, pattern := range patterns {
					if pattern == ext {
						a.Language = lang
						break
					}
				}
			}
		}
	}

	// 如果未检测到，默认使用Java风格
	if a.Language == "" {
		a.Language = "java"
	}
}

// shouldIncludeFile 检查是否应该包含该文件
func (a *CodeAnalyzer) shouldIncludeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, patterns := range LanguageConfig {
		for _, pattern := range patterns {
			if ext == pattern {
				return true
			}
		}
	}
	return false
}

// AnalyzeCallGraph 分析调用图
func (a *CodeAnalyzer) AnalyzeCallGraph() []MethodCall {
	var calls []MethodCall

	// 根据语言选择合适的正则
	methodCallPattern := a.getMethodCallPattern()

	for file, content := range a.Files {
		lines := strings.Split(content, "\n")
		for lineNum, line := range lines {
			matches := methodCallPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					calls = append(calls, MethodCall{
						CallerFile: file,
						Caller:     extractMethodName(content, lineNum, a.Language),
						CalleeFile: file,
						Callee:     match[2],
						Line:       lineNum + 1,
					})
				}
			}
		}
	}

	return calls
}

// getMethodCallPattern 根据语言获取方法调用正则（扩展支持多语言）
func (a *CodeAnalyzer) getMethodCallPattern() *regexp.Regexp {
	patterns := map[string]string{
		"java":       `(\w+)\.(\w+)\s*\(`,
		"php":        `\$(\w+)->(\w+)\s*\(|(\w+)\((\w+)\s*\)|(\w+)::(\w+)\s*\(`,
		"python":     `(\w+)\.(\w+)\s*\(|(\w+)\s+=\s+(\w+)\(`,
		"csharp":     `(\w+)\.(\w+)\s*\(`,
		"go":         `(\w+)\.(\w+)\s*\(`,
		"js":         `(\w+)\.(\w+)\s*\(`,
		"typescript": `(\w+)\.(\w+)\s*\(`,
		"rust":       `(\w+)::(\w+)\s*\(|(\w+)\.(\w+)\s*\(`,
		"kotlin":     `(\w+)\.(\w+)\s*\(`,
		"scala":      `(\w+)\.(\w+)\s*\(|(\w+)\s+(\w+)\s*\(`,
		"ruby":       `(\w+)\.(\w+)\s*\(|(\w+)\s+(\w+)\s*\(`,
		"swift":      `(\w+)\.(\w+)\s*\(`,
		"dart":       `(\w+)\.(\w+)\s*\(`,
		"groovy":     `(\w+)\.(\w+)\s*\(`,
	}

	pattern := patterns[a.Language]
	if pattern == "" {
		pattern = patterns["java"]
	}
	return regexp.MustCompile(pattern)
}

// extractMethodName 提取方法名
func extractMethodName(content string, lineNum int, language string) string {
	lines := strings.Split(content, "\n")
	if lineNum < 0 || lineNum >= len(lines) {
		return ""
	}

	line := lines[lineNum]

	// 根据语言提取方法名
	switch language {
	case "java", "csharp", "go":
		if strings.Contains(line, "public") || strings.Contains(line, "private") || strings.Contains(line, "protected") {
			methodPattern := regexp.MustCompile(`(?:public|private|protected)\s+(?:static\s+)?(?:\w+\s+)+(\w+)\s*\(`)
			match := methodPattern.FindStringSubmatch(line)
			if len(match) >= 2 {
				return match[1]
			}
		}
	case "python":
		if strings.Contains(line, "def ") {
			methodPattern := regexp.MustCompile(`def\s+(\w+)\s*\(`)
			match := methodPattern.FindStringSubmatch(line)
			if len(match) >= 2 {
				return match[1]
			}
		}
	case "php":
		if strings.Contains(line, "function ") {
			methodPattern := regexp.MustCompile(`function\s+(\w+)\s*\(`)
			match := methodPattern.FindStringSubmatch(line)
			if len(match) >= 2 {
				return match[1]
			}
		}
	}
	return ""
}

// FindTaintSources 查找污点源（用户输入入口点）- 多语言支持
func (a *CodeAnalyzer) FindTaintSources() []string {
	var sources []string

	// 根据语言定义污点源模式
	taintPatterns := a.getTaintPatterns()

	for file, content := range a.Files {
		for _, pattern := range taintPatterns {
			if strings.Contains(content, pattern) {
				sources = append(sources, fmt.Sprintf("%s: 包含潜在污点源 [%s]", filepath.Base(file), pattern))
			}
		}
	}

	return sources
}

// getTaintPatterns 根据语言获取污点源模式（扩展支持更多语言）
func (a *CodeAnalyzer) getTaintPatterns() []string {
	patterns := map[string][]string{
		"java": {
			"request.getParameter", "request.getHeader", "HttpServletRequest",
			"@RequestParam", "@RequestBody", "@PathVariable",
			"BufferedReader", "Scanner", "DataInputStream",
			"getCookies", "HttpSession.getAttribute", "ServletInputStream",
			"@RequestHeader", "@CookieValue", "HttpServletRequestWrapper",
		},
		"php": {
			"$_GET", "$_POST", "$_REQUEST", "$_COOKIE", "$_SERVER",
			"file_get_contents", "fopen", "curl_exec",
			"mysqli_query", "PDO::query", "apache_request_headers",
		},
		"python": {
			"request.", "input(", "sys.argv", "os.environ",
			"flask.request", "django.request", "fastapi.Request",
			"os.getenv", "sys.stdin", "argparse.ArgumentParser",
		},
		"csharp": {
			"Request.QueryString", "Request.Form", "Request.Params",
			"HttpContext.Request", "RouteData.Values", "Request.Browser",
			"Request.Cookies", "Request.UserHostAddress",
		},
		"js": {
			"req.body", "req.query", "req.params", "req.headers",
			"process.argv", "document.cookie", "window.location",
			"localStorage", "sessionStorage",
		},
		"typescript": {
			"req.body", "req.query", "req.params", "req.headers",
			"process.argv", "express.Request",
		},
		"go": {
			"r.FormValue", "r.Form", "r.PostForm", "r.URL.Query",
			"http.Request", "r.FormFile", "r.Header",
		},
		"rust": {
			"std::io::stdin", "std::env::args", "std::fs::read",
			"std::net::TcpStream", "std::env::var",
		},
		"kotlin": {
			"request.getParameter", "@RequestParam", "@RequestBody",
			"readLine", "Scanner", "BufferedReader",
		},
		"scala": {
			"request.getParameter", "scala.io.StdIn", "Source.stdin",
			"play.api.mvc.Request",
		},
		"ruby": {
			"params", "request", "env", "gets", "ARGF",
			"Rails.application.config",
		},
		"swift": {
			"readLine", "CommandLine.arguments", "FileHandle.standardInput",
			"ProcessInfo.processInfo.environment",
		},
		"dart": {
			"stdin.readLineSync", "Platform.environment",
			"html.HttpRequest", "dart:io stdin",
		},
	}

	if patterns, ok := patterns[a.Language]; ok {
		return patterns
	}
	return patterns["java"]
}

// FindSinks 查找危险Sink点 - 多语言支持
func (a *CodeAnalyzer) FindSinks() []string {
	var sinks []string

	// 根据语言定义Sink点模式
	sinkPatterns := a.getSinkPatterns()

	for file, content := range a.Files {
		for _, pattern := range sinkPatterns {
			if strings.Contains(content, pattern) {
				sinks = append(sinks, fmt.Sprintf("%s: 包含潜在危险Sink [%s]", filepath.Base(file), pattern))
			}
		}
	}

	return sinks
}

// getSinkPatterns 根据语言获取Sink点模式（扩展支持更多语言）
func (a *CodeAnalyzer) getSinkPatterns() []string {
	patterns := map[string][]string{
		"java": {
			"execute(", "exec(", "Runtime.getRuntime()",
			"Statement.execute", "PreparedStatement",
			"createQuery", "createSQLQuery",
			"ProcessBuilder", "eval(", "executeScript(",
			"FileWriter", "FileOutputStream",
			"response.getWriter", "ObjectInputStream",
			"new InitialContext().lookup",
		},
		"php": {
			"eval(", "exec(", "system(", "shell_exec(",
			"passthru(", "popen(", "mysqli_query",
			"mysql_query", "PDO::exec",
			"file_put_contents", "file_get_contents",
			"unserialize(", "preg_replace", "assert(",
			"move_uploaded_file",
		},
		"python": {
			"eval(", "exec(", "subprocess.",
			"os.system(", "os.popen(",
			"pickle.load", "yaml.load",
			"sqlalchemy.text", "cursor.execute",
			"render_template_string", "Template(",
		},
		"csharp": {
			"Process.Start", "Response.Write",
			"Server.Execute", "Eval(",
			"DataAdapter.SelectCommand", "SqlCommand.Execute",
			"BinaryFormatter", "JavaScriptSerializer",
		},
		"js": {
			"eval(", "Function(", "execScript",
			"child_process.", "require('child_process')",
			"innerHTML", ".sql",
		},
		"typescript": {
			"eval(", "new Function(",
			"child_process.", "innerHTML",
			"dangerouslySetInnerHTML",
		},
		"go": {
			"exec.Command", "os/exec", "syscall.Exec",
			"template.HTML", "html/template",
			"ioutil.ReadFile", "ioutil.WriteFile",
		},
		"rust": {
			"std::process::Command", "std::fs::write",
			"std::fs::read", "serde_json::from_str",
			"bincode::deserialize",
		},
		"kotlin": {
			"Runtime.getRuntime().exec",
			"ProcessBuilder", "ObjectInputStream",
			"readObject", "eval(",
		},
		"scala": {
			"scala.util.evaluator", "System.exec",
			"pickle.loads", "eval(",
		},
		"ruby": {
			"eval(", "system(", "`", "Marshal.load",
			"YAML.load", "File.write",
		},
		"swift": {
			"Process(", "system(", "NSTask",
			"Runtime.getInstance().exec",
		},
		"dart": {
			"Process.run", "eval(", "dart:js",
			"innerHtml", "compile()",
		},
		"groovy": {
			"Eval.me(", "GroovyShell.evaluate",
			"Runtime.getRuntime().exec",
			"ProcessBuilder",
		},
	}

	if patterns, ok := patterns[a.Language]; ok {
		return patterns
	}
	return patterns["java"]
}

// AnalyzeVulnerabilityChains 分析漏洞利用链 - 多语言深度分析
func (a *CodeAnalyzer) AnalyzeVulnerabilityChains() []VulnerabilityChain {
	var chains []VulnerabilityChain

	// 基础分析
	sources := a.FindTaintSources()
	sinks := a.FindSinks()

	// 简单的链分析
	for _, source := range sources {
		for _, sink := range sinks {
			if strings.Split(source, ":")[0] == strings.Split(sink, ":")[0] {
				chains = append(chains, VulnerabilityChain{
					Type:        "潜在代码执行/注入",
					Severity:    "High",
					Source:      source,
					Sink:        sink,
					Description: "检测到从用户输入到危险操作的路径",
					Chain:       []string{source, sink},
					RiskLevel:   "需要人工确认",
					Language:    a.Language,
				})
			}
		}
	}

	// 语言特定的深度分析
	for file, content := range a.Files {
		fileChains := a.analyzeFileVulnerabilities(file, content)
		chains = append(chains, fileChains...)
	}

	return chains
}

// analyzeFileVulnerabilities 分析单个文件的漏洞
func (a *CodeAnalyzer) analyzeFileVulnerabilities(file, content string) []VulnerabilityChain {
	var chains []VulnerabilityChain

	switch a.Language {
	case "java":
		chains = append(chains, a.analyzeJavaVulnerabilities(file, content)...)
	case "php":
		chains = append(chains, a.analyzePHPVulnerabilities(file, content)...)
	case "python":
		chains = append(chains, a.analyzePythonVulnerabilities(file, content)...)
	case "csharp":
		chains = append(chains, a.analyzeCSharpVulnerabilities(file, content)...)
	default:
		// 通用分析
		chains = append(chains, a.analyzeCommonVulnerabilities(file, content)...)
	}

	return chains
}

// analyzeJavaVulnerabilities Java特定漏洞分析
func (a *CodeAnalyzer) analyzeJavaVulnerabilities(file, content string) []VulnerabilityChain {
	var chains []VulnerabilityChain

	// SQL注入
	if (strings.Contains(content, "Statement") || strings.Contains(content, "PreparedStatement")) &&
		(strings.Contains(content, "+") && (strings.Contains(content, "select") || strings.Contains(content, "insert") || strings.Contains(content, "update") || strings.Contains(content, "delete"))) {
		chains = append(chains, VulnerabilityChain{
			Type:        "SQL注入",
			Severity:    "High",
			Source:      "数据库操作（可能包含用户输入）",
			Sink:        fmt.Sprintf("%s: SQL执行", filepath.Base(file)),
			Description: "检测到可能的SQL注入风险：使用字符串拼接构建SQL查询",
			Chain:       []string{"用户输入", "字符串拼接", "SQL执行"},
			RiskLevel:   "高风险",
			Language:    "java",
		})
	}

	// 命令注入
	if strings.Contains(content, "Runtime") || strings.Contains(content, "exec(") || strings.Contains(content, "ProcessBuilder") {
		chains = append(chains, VulnerabilityChain{
			Type:        "命令注入",
			Severity:    "Critical",
			Source:      "Process execution",
			Sink:        fmt.Sprintf("%s: 系统命令执行", filepath.Base(file)),
			Description: "检测到可能的命令注入风险",
			Chain:       []string{"用户输入", "命令拼接", "系统执行"},
			RiskLevel:   "极高风险",
			Language:    "java",
		})
	}

	// XSS
	if strings.Contains(content, "response") && strings.Contains(content, "getWriter") {
		chains = append(chains, VulnerabilityChain{
			Type:        "跨站脚本(XSS)",
			Severity:    "Medium",
			Source:      "用户输入",
			Sink:        fmt.Sprintf("%s: HTTP响应输出", filepath.Base(file)),
			Description: "检测到可能的XSS风险：未经过滤的用户输入直接输出到响应",
			Chain:       []string{"用户输入", "未过滤", "响应输出"},
			RiskLevel:   "中风险",
			Language:    "java",
		})
	}

	// 路径遍历
	if strings.Contains(content, "File") && strings.Contains(content, "new File(") {
		if strings.Contains(content, "request") || strings.Contains(content, "parameter") {
			chains = append(chains, VulnerabilityChain{
				Type:        "路径遍历",
				Severity:    "High",
				Source:      "用户输入（文件名）",
				Sink:        fmt.Sprintf("%s: 文件操作", filepath.Base(file)),
				Description: "检测到可能的路径遍历风险：用户输入用于文件路径",
				Chain:       []string{"用户输入", "文件路径", "文件操作"},
				RiskLevel:   "高风险",
				Language:    "java",
			})
		}
	}

	// 不安全的反序列化
	if strings.Contains(content, "ObjectInputStream") || strings.Contains(content, "readObject") {
		chains = append(chains, VulnerabilityChain{
			Type:        "不安全的反序列化",
			Severity:    "Critical",
			Source:      "反序列化操作",
			Sink:        fmt.Sprintf("%s: 对象反序列化", filepath.Base(file)),
			Description: "检测到不安全的反序列化风险，可能导致远程代码执行",
			Chain:       []string{"用户输入", "反序列化", "代码执行"},
			RiskLevel:   "极高风险",
			Language:    "java",
		})
	}

	return chains
}

// analyzePHPVulnerabilities PHP特定漏洞分析
func (a *CodeAnalyzer) analyzePHPVulnerabilities(file, content string) []VulnerabilityChain {
	var chains []VulnerabilityChain

	// SQL注入
	if (strings.Contains(content, "mysqli_query") || strings.Contains(content, "mysql_query") || strings.Contains(content, "PDO::query")) &&
		strings.Contains(content, "\"") || strings.Contains(content, "'") {
		chains = append(chains, VulnerabilityChain{
			Type:        "SQL注入",
			Severity:    "High",
			Source:      "数据库操作",
			Sink:        fmt.Sprintf("%s: SQL执行", filepath.Base(file)),
			Description: "检测到可能的SQL注入风险",
			Chain:       []string{"用户输入", "字符串拼接", "SQL执行"},
			RiskLevel:   "高风险",
			Language:    "php",
		})
	}

	// 命令注入
	if strings.Contains(content, "eval(") || strings.Contains(content, "exec(") || strings.Contains(content, "system(") ||
		strings.Contains(content, "shell_exec(") || strings.Contains(content, "passthru(") {
		chains = append(chains, VulnerabilityChain{
			Type:        "命令注入/代码执行",
			Severity:    "Critical",
			Source:      "命令执行函数",
			Sink:        fmt.Sprintf("%s: 系统命令执行", filepath.Base(file)),
			Description: "检测到命令注入或代码执行风险",
			Chain:       []string{"用户输入", "命令拼接", "系统执行"},
			RiskLevel:   "极高风险",
			Language:    "php",
		})
	}

	// 文件包含
	if strings.Contains(content, "include") || strings.Contains(content, "require") {
		if strings.Contains(content, "$_") || strings.Contains(content, "${") {
			chains = append(chains, VulnerabilityChain{
				Type:        "文件包含",
				Severity:    "High",
				Source:      "动态文件包含",
				Sink:        fmt.Sprintf("%s: 文件包含", filepath.Base(file)),
				Description: "检测到远程/本地文件包含风险",
				Chain:       []string{"用户输入", "文件包含", "代码执行"},
				RiskLevel:   "高风险",
				Language:    "php",
			})
		}
	}

	// XSS
	if strings.Contains(content, "echo") || strings.Contains(content, "print") || strings.Contains(content, "print_r") {
		if strings.Contains(content, "$_") && !strings.Contains(content, "htmlspecialchars") && !strings.Contains(content, "htmlentities") {
			chains = append(chains, VulnerabilityChain{
				Type:        "跨站脚本(XSS)",
				Severity:    "Medium",
				Source:      "用户输入输出",
				Sink:        fmt.Sprintf("%s: HTTP响应输出", filepath.Base(file)),
				Description: "检测到XSS风险：用户输入未经过滤直接输出",
				Chain:       []string{"用户输入", "未过滤", "响应输出"},
				RiskLevel:   "中风险",
				Language:    "php",
			})
		}
	}

	// 不安全的反序列化
	if strings.Contains(content, "unserialize(") {
		chains = append(chains, VulnerabilityChain{
			Type:        "不安全的反序列化",
			Severity:    "Critical",
			Source:      "反序列化操作",
			Sink:        fmt.Sprintf("%s: 反序列化", filepath.Base(file)),
			Description: "检测到不安全的反序列化风险",
			Chain:       []string{"用户输入", "反序列化", "代码执行"},
			RiskLevel:   "极高风险",
			Language:    "php",
		})
	}

	// SSRF
	if strings.Contains(content, "curl_exec") || strings.Contains(content, "file_get_contents") || strings.Contains(content, "fopen") {
		if strings.Contains(content, "$_") {
			chains = append(chains, VulnerabilityChain{
				Type:        "服务端请求伪造(SSRF)",
				Severity:    "High",
				Source:      "用户控制的URL",
				Sink:        fmt.Sprintf("%s: 网络请求", filepath.Base(file)),
				Description: "检测到SSRF风险：用户输入被用于发起网络请求",
				Chain:       []string{"用户输入", "网络请求", "内网访问"},
				RiskLevel:   "高风险",
				Language:    "php",
			})
		}
	}

	return chains
}

// analyzePythonVulnerabilities Python特定漏洞分析
func (a *CodeAnalyzer) analyzePythonVulnerabilities(file, content string) []VulnerabilityChain {
	var chains []VulnerabilityChain

	// 命令注入
	if strings.Contains(content, "subprocess.") || strings.Contains(content, "os.system") ||
		strings.Contains(content, "os.popen") || strings.Contains(content, "eval(") || strings.Contains(content, "exec(") {
		chains = append(chains, VulnerabilityChain{
			Type:        "命令注入/代码执行",
			Severity:    "Critical",
			Source:      "命令执行",
			Sink:        fmt.Sprintf("%s: 系统命令执行", filepath.Base(file)),
			Description: "检测到命令注入或代码执行风险",
			Chain:       []string{"用户输入", "命令拼接", "系统执行"},
			RiskLevel:   "极高风险",
			Language:    "python",
		})
	}

	// 不安全的反序列化
	if strings.Contains(content, "pickle.load") || strings.Contains(content, "pickle.loads") ||
		strings.Contains(content, "yaml.load") || strings.Contains(content, "yaml.unsafe_load") {
		chains = append(chains, VulnerabilityChain{
			Type:        "不安全的反序列化",
			Severity:    "Critical",
			Source:      "反序列化操作",
			Sink:        fmt.Sprintf("%s: 反序列化", filepath.Base(file)),
			Description: "检测到不安全的反序列化风险",
			Chain:       []string{"用户输入", "反序列化", "代码执行"},
			RiskLevel:   "极高风险",
			Language:    "python",
		})
	}

	// SQL注入
	if strings.Contains(content, "cursor.execute") || strings.Contains(content, "text(") ||
		strings.Contains(content, "sqlalchemy.text") {
		if strings.Contains(content, "%") || strings.Contains(content, "f\"") || strings.Contains(content, "format(") {
			chains = append(chains, VulnerabilityChain{
				Type:        "SQL注入",
				Severity:    "High",
				Source:      "数据库操作",
				Sink:        fmt.Sprintf("%s: SQL执行", filepath.Base(file)),
				Description: "检测到SQL注入风险",
				Chain:       []string{"用户输入", "字符串拼接", "SQL执行"},
				RiskLevel:   "高风险",
				Language:    "python",
			})
		}
	}

	// 路径遍历
	if strings.Contains(content, "open(") || strings.Contains(content, "os.path.join") {
		if strings.Contains(content, "request") || strings.Contains(content, "args") || strings.Contains(content, "kwargs") {
			chains = append(chains, VulnerabilityChain{
				Type:        "路径遍历",
				Severity:    "High",
				Source:      "用户输入（文件名）",
				Sink:        fmt.Sprintf("%s: 文件操作", filepath.Base(file)),
				Description: "检测到路径遍历风险",
				Chain:       []string{"用户输入", "文件路径", "文件操作"},
				RiskLevel:   "高风险",
				Language:    "python",
			})
		}
	}

	// 模板注入
	if strings.Contains(content, "render_template_string") || strings.Contains(content, "Template(") {
		chains = append(chains, VulnerabilityChain{
			Type:        "模板注入(SSTI)",
			Severity:    "Critical",
			Source:      "模板渲染",
			Sink:        fmt.Sprintf("%s: 模板渲染", filepath.Base(file)),
			Description: "检测到服务器端模板注入风险",
			Chain:       []string{"用户输入", "模板渲染", "代码执行"},
			RiskLevel:   "极高风险",
			Language:    "python",
		})
	}

	return chains
}

// analyzeCSharpVulnerabilities C#特定漏洞分析
func (a *CodeAnalyzer) analyzeCSharpVulnerabilities(file, content string) []VulnerabilityChain {
	var chains []VulnerabilityChain

	// SQL注入
	if strings.Contains(content, "SqlCommand") || strings.Contains(content, "SqlDataAdapter") {
		if strings.Contains(content, "+") || strings.Contains(content, "string.Format") {
			chains = append(chains, VulnerabilityChain{
				Type:        "SQL注入",
				Severity:    "High",
				Source:      "数据库操作",
				Sink:        fmt.Sprintf("%s: SQL执行", filepath.Base(file)),
				Description: "检测到SQL注入风险",
				Chain:       []string{"用户输入", "字符串拼接", "SQL执行"},
				RiskLevel:   "高风险",
				Language:    "csharp",
			})
		}
	}

	// 命令注入
	if strings.Contains(content, "Process.Start") || strings.Contains(content, "System.Diagnostics.Process") {
		chains = append(chains, VulnerabilityChain{
			Type:        "命令注入",
			Severity:    "Critical",
			Source:      "进程执行",
			Sink:        fmt.Sprintf("%s: 系统命令执行", filepath.Base(file)),
			Description: "检测到命令注入风险",
			Chain:       []string{"用户输入", "命令拼接", "系统执行"},
			RiskLevel:   "极高风险",
			Language:    "csharp",
		})
	}

	// XSS
	if strings.Contains(content, "Response.Write") || strings.Contains(content, "Html.Raw") {
		chains = append(chains, VulnerabilityChain{
			Type:        "跨站脚本(XSS)",
			Severity:    "Medium",
			Source:      "用户输入输出",
			Sink:        fmt.Sprintf("%s: HTTP响应输出", filepath.Base(file)),
			Description: "检测到XSS风险",
			Chain:       []string{"用户输入", "未过滤", "响应输出"},
			RiskLevel:   "中风险",
			Language:    "csharp",
		})
	}

	// 不安全的反序列化
	if strings.Contains(content, "BinaryFormatter") || strings.Contains(content, "ObjectBinder") {
		chains = append(chains, VulnerabilityChain{
			Type:        "不安全的反序列化",
			Severity:    "Critical",
			Source:      "反序列化操作",
			Sink:        fmt.Sprintf("%s: 反序列化", filepath.Base(file)),
			Description: "检测到不安全的反序列化风险",
			Chain:       []string{"用户输入", "反序列化", "代码执行"},
			RiskLevel:   "极高风险",
			Language:    "csharp",
		})
	}

	return chains
}

// analyzeCommonVulnerabilities 通用漏洞分析
func (a *CodeAnalyzer) analyzeCommonVulnerabilities(file, content string) []VulnerabilityChain {
	var chains []VulnerabilityChain

	// 硬编码密码/密钥
	passwordPatterns := []string{
		"password",
		"pwd",
		"secret",
		"api_key",
		"apikey",
		"access_token",
	}

	for _, pattern := range passwordPatterns {
		if regexp.MustCompile(fmt.Sprintf(`(?i)%s\s*=\s*["'][^"']+["']`, pattern)).MatchString(content) {
			chains = append(chains, VulnerabilityChain{
				Type:        "硬编码凭证",
				Severity:    "High",
				Source:      "代码中的硬编码",
				Sink:        fmt.Sprintf("%s: 敏感信息", filepath.Base(file)),
				Description: fmt.Sprintf("检测到硬编码的敏感信息: %s", pattern),
				Chain:       []string{"硬编码", "代码仓库", "信息泄露"},
				RiskLevel:   "高风险",
				Language:    a.Language,
			})
			break
		}
	}

	return chains
}

// GetCodeStructure 获取代码结构
func (a *CodeAnalyzer) GetCodeStructure() string {
	var result strings.Builder

	result.WriteString("# 代码结构分析\n\n")
	result.WriteString(fmt.Sprintf("## 检测到的编程语言: %s\n\n", a.Language))

	// 按目录分组
	packages := make(map[string][]string)
	for file := range a.Files {
		dir := filepath.Dir(file)
		packages[dir] = append(packages[dir], filepath.Base(file))
	}

	result.WriteString("## 文件统计\n")
	result.WriteString(fmt.Sprintf("- 总文件数: %d\n", len(a.Files)))
	result.WriteString(fmt.Sprintf("- 总代码行数: %d\n", a.countLines()))
	result.WriteString("\n")

	result.WriteString("## 目录结构\n")
	for pkg, files := range packages {
		result.WriteString(fmt.Sprintf("### %s\n", pkg))
		for _, file := range files {
			result.WriteString(fmt.Sprintf("- %s\n", file))
		}
	}

	return result.String()
}

// countLines 统计代码行数
func (a *CodeAnalyzer) countLines() int {
	total := 0
	for _, content := range a.Files {
		total += len(strings.Split(content, "\n"))
	}
	return total
}

// GenerateDeepAnalysisReport 生成深度分析报告
func (a *CodeAnalyzer) GenerateDeepAnalysisReport() string {
	var report strings.Builder

	report.WriteString("# 深度代码安全审计报告\n\n")
	report.WriteString("## 审计范围\n\n")
	report.WriteString(fmt.Sprintf("- 检测到的语言: %s\n", a.Language))
	report.WriteString(fmt.Sprintf("- 分析文件数: %d\n", len(a.Files)))
	report.WriteString(fmt.Sprintf("- 代码行数: %d\n\n", a.countLines()))

	// 代码结构
	report.WriteString(a.GetCodeStructure())
	report.WriteString("\n")

	// 污点源分析
	report.WriteString("## 污点源分析（用户输入入口点）\n\n")
	sources := a.FindTaintSources()
	if len(sources) > 0 {
		for _, s := range sources {
			report.WriteString(fmt.Sprintf("- %s\n", s))
		}
	} else {
		report.WriteString("未发现明显的污点源\n")
	}
	report.WriteString("\n")

	// Sink点分析
	report.WriteString("## 危险Sink点分析\n\n")
	sinks := a.FindSinks()
	if len(sinks) > 0 {
		for _, s := range sinks {
			report.WriteString(fmt.Sprintf("- %s\n", s))
		}
	} else {
		report.WriteString("未发现明显的危险Sink点\n")
	}
	report.WriteString("\n")

	// 利用链分析
	report.WriteString("## 漏洞利用链分析\n\n")
	chains := a.AnalyzeVulnerabilityChains()
	if len(chains) > 0 {
		// 按严重程度分组
		var critical, high, medium, low []VulnerabilityChain
		for _, chain := range chains {
			switch chain.Severity {
			case "Critical":
				critical = append(critical, chain)
			case "High":
				high = append(high, chain)
			case "Medium":
				medium = append(medium, chain)
			default:
				low = append(low, chain)
			}
		}

		report.WriteString("### 严重漏洞 (Critical)\n\n")
		for i, chain := range critical {
			report.WriteString(fmt.Sprintf("#### %d. %s\n", i+1, chain.Type))
			report.WriteString(fmt.Sprintf("- **严重程度**: %s\n", chain.Severity))
			report.WriteString(fmt.Sprintf("- **语言**: %s\n", chain.Language))
			report.WriteString(fmt.Sprintf("- **Source**: %s\n", chain.Source))
			report.WriteString(fmt.Sprintf("- **Sink**: %s\n", chain.Sink))
			report.WriteString(fmt.Sprintf("- **描述**: %s\n", chain.Description))
			report.WriteString(fmt.Sprintf("- **风险等级**: %s\n", chain.RiskLevel))
			report.WriteString("- **利用链路径**:\n")
			for j, step := range chain.Chain {
				report.WriteString(fmt.Sprintf("  %d. %s\n", j+1, step))
			}
			report.WriteString("\n")
		}

		report.WriteString("### 高危漏洞 (High)\n\n")
		for i, chain := range high {
			report.WriteString(fmt.Sprintf("#### %d. %s\n", i+1, chain.Type))
			report.WriteString(fmt.Sprintf("- **严重程度**: %s\n", chain.Severity))
			report.WriteString(fmt.Sprintf("- **语言**: %s\n", chain.Language))
			report.WriteString(fmt.Sprintf("- **Source**: %s\n", chain.Source))
			report.WriteString(fmt.Sprintf("- **Sink**: %s\n", chain.Sink))
			report.WriteString(fmt.Sprintf("- **描述**: %s\n", chain.Description))
			report.WriteString(fmt.Sprintf("- **风险等级**: %s\n", chain.RiskLevel))
			report.WriteString("- **利用链路径**:\n")
			for j, step := range chain.Chain {
				report.WriteString(fmt.Sprintf("  %d. %s\n", j+1, step))
			}
			report.WriteString("\n")
		}

		report.WriteString("### 中危漏洞 (Medium)\n\n")
		for i, chain := range medium {
			report.WriteString(fmt.Sprintf("#### %d. %s\n", i+1, chain.Type))
			report.WriteString(fmt.Sprintf("- **严重程度**: %s\n", chain.Severity))
			report.WriteString(fmt.Sprintf("- **语言**: %s\n", chain.Language))
			report.WriteString(fmt.Sprintf("- **Source**: %s\n", chain.Source))
			report.WriteString(fmt.Sprintf("- **Sink**: %s\n", chain.Sink))
			report.WriteString(fmt.Sprintf("- **描述**: %s\n", chain.Description))
			report.WriteString("\n")
		}

		report.WriteString(fmt.Sprintf("### 低危漏洞: %d个\n\n", len(low)))
	} else {
		report.WriteString("未发现明显的利用链\n")
	}

	// 添加安全建议
	report.WriteString("## 安全建议\n\n")
	report.WriteString(a.generateSecurityRecommendations(chains))

	return report.String()
}

// generateSecurityRecommendations 生成安全建议
func (a *CodeAnalyzer) generateSecurityRecommendations(chains []VulnerabilityChain) string {
	var recommendations strings.Builder

	recommendations.WriteString("### 通用安全建议\n\n")
	recommendations.WriteString("1. **输入验证**: 对所有用户输入进行严格的验证和过滤\n")
	recommendations.WriteString("2. **参数化查询**: 使用预编译语句或参数化查询防止SQL注入\n")
	recommendations.WriteString("3. **输出编码**: 对所有输出进行适当的编码，防止XSS\n")
	recommendations.WriteString("4. **最小权限原则**: 运行服务时使用最小必要权限\n")
	recommendations.WriteString("5. **敏感信息管理**: 使用环境变量或密钥管理服务存储敏感信息\n")
	recommendations.WriteString("6. **安全更新**: 定期更新依赖库和框架到最新安全版本\n\n")

	// 语言特定建议
	recommendations.WriteString("### 语言特定建议\n\n")

	switch a.Language {
	case "java":
		recommendations.WriteString("**Java安全建议**:\n")
		recommendations.WriteString("- 使用 PreparedStatement 而非 Statement\n")
		recommendations.WriteString("- 避免使用 ObjectInputStream 进行反序列化\n")
		recommendations.WriteString("- 使用 Spring Security 进行认证授权\n")
		recommendations.WriteString("- 使用 OWASP ESAPI 进行输入验证\n\n")
	case "php":
		recommendations.WriteString("**PHP安全建议**:\n")
		recommendations.WriteString("- 使用 PDO 预处理语句\n")
		recommendations.WriteString("- 避免使用 eval() 和 assert()\n")
		recommendations.WriteString("- 使用 htmlspecialchars() 防止XSS\n")
		recommendations.WriteString("- 验证所有文件包含路径\n\n")
	case "python":
		recommendations.WriteString("**Python安全建议**:\n")
		recommendations.WriteString("- 使用参数化查询 (SQLAlchemy, Django ORM)\n")
		recommendations.WriteString("- 避免使用 pickle 进行反序列化\n")
		recommendations.WriteString("- 使用 yaml.safe_load() 而非 yaml.load()\n")
		recommendations.WriteString("- 使用 Jinja2 模板而非 render_template_string\n\n")
	case "csharp":
		recommendations.WriteString("**C#安全建议**:\n")
		recommendations.WriteString("- 使用参数化查询 (SqlParameter)\n")
		recommendations.WriteString("- 避免使用 BinaryFormatter\n")
		recommendations.WriteString("- 使用 AntiXss 库防止XSS\n\n")
	}

	return recommendations.String()
}
