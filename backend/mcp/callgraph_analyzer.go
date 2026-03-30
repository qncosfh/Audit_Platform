package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// CallGraphNode 表示调用图中的节点（函数/方法）
type CallGraphNode struct {
	Name       string   // 方法名
	FilePath   string   // 所在文件
	Line       int      // 行号
	ClassName  string   // 类名（如果适用）
	Parameters []string // 参数列表
	IsPublic   bool     // 是否公开方法
}

// CallGraphEdge 表示调用图中的边（调用关系）
type CallGraphEdge struct {
	Caller   *CallGraphNode // 调用者
	Callee   *CallGraphNode // 被调用者
	CallType string         // 调用类型：direct, callback, interface
	Line     int            // 调用行号
	FilePath string         // 调用所在文件
}

// CallGraphAnalyzer 跨文件调用链分析器
type CallGraphAnalyzer struct {
	SourcePath  string
	Language    string
	Files       map[string]string         // 文件路径 -> 内容
	Nodes       map[string]*CallGraphNode // 方法名 -> 节点
	Edges       []*CallGraphEdge          // 调用边
	ClassMap    map[string]string         // 文件 -> 类名
	ImportMap   map[string][]string       // 文件 -> 导入的包/模块
	Mutex       sync.RWMutex
	FileIndex   map[string][]string // 文件 -> 调用的方法列表
	CalledByMap map[string][]string // 被调用的方法 -> 调用它的方法列表
}

// ExploitChain 漏洞利用链
type ExploitChain struct {
	ID          string             `json:"id"`          // 唯一ID
	VulnID      uint               `json:"vulnId"`      // 关联的漏洞ID
	Type        string             `json:"type"`        // 漏洞类型
	Severity    string             `json:"severity"`    // 严重程度
	Description string             `json:"description"` // 描述
	Steps       []ExploitChainStep `json:"steps"`       // 利用链步骤
	Source      *ChainNode         `json:"source"`      // 入口点
	Sink        *ChainNode         `json:"sink"`        // 危险点
	RiskLevel   string             `json:"riskLevel"`   // 风险等级
}

// ExploitChainStep 利用链步骤
type ExploitChainStep struct {
	StepNum     int      `json:"stepNum"`     // 步骤编号
	NodeType    string   `json:"nodeType"`    // 节点类型: source, controller, service, dao, sink, normal
	MethodName  string   `json:"methodName"`  // 方法名
	ClassName   string   `json:"className"`   // 类名
	FileName    string   `json:"fileName"`    // 文件名
	Line        int      `json:"line"`        // 行号
	Params      []string `json:"params"`      // 参数
	IsVulnPoint bool     `json:"isVulnPoint"` // 是否是漏洞点
	VulnType    string   `json:"vulnType"`    // 漏洞类型（如果是漏洞点）
}

// ChainNode 链节点（用于前端展示）
type ChainNode struct {
	ID         string `json:"id"`
	Label      string `json:"label"`      // 显示标签
	MethodName string `json:"methodName"` // 方法名
	ClassName  string `json:"className"`  // 类名
	FileName   string `json:"fileName"`   // 文件名
	Line       int    `json:"line"`       // 行号
	NodeType   string `json:"nodeType"`   // 节点类型
	Type       string `json:"type"`       // 漏洞类型
	Severity   string `json:"severity"`   // 严重程度
}

// CrossFileAnalysisResult 跨文件分析结果
type CrossFileAnalysisResult struct {
	CallChain      []string // 调用链
	SourceFile     string   // 源文件
	SinkFile       string   // Sink文件
	Vulnerability  string   // 漏洞类型
	Severity       string   // 严重程度
	Description    string   // 描述
	EntryPoints    []string // 入口点（污点源）
	SinkPoints     []string // Sink点
	DataFlow       []string // 数据流路径
	RiskAssessment string   // 风险评估
	Recommendation string   // 建议
}

// NewCallGraphAnalyzer 创建调用链分析器
func NewCallGraphAnalyzer(sourcePath string) (*CallGraphAnalyzer, error) {
	analyzer := &CallGraphAnalyzer{
		SourcePath:  sourcePath,
		Files:       make(map[string]string),
		Nodes:       make(map[string]*CallGraphNode),
		Edges:       []*CallGraphEdge{},
		ClassMap:    make(map[string]string),
		ImportMap:   make(map[string][]string),
		FileIndex:   make(map[string][]string),
		CalledByMap: make(map[string][]string),
	}

	// 检测语言
	analyzer.detectLanguage()

	// 加载所有代码文件
	if err := analyzer.loadFiles(); err != nil {
		return nil, fmt.Errorf("加载文件失败: %v", err)
	}

	return analyzer, nil
}

// detectLanguage 检测编程语言
func (c *CallGraphAnalyzer) detectLanguage() {
	extensions := make(map[string]int)

	filepath.Walk(c.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		extensions[ext]++
		return nil
	})

	// 根据扩展名判断语言（按支持的语言数量排序）
	extCounts := map[string]string{
		// 高级语言
		".java":   "java",
		".php":    "php",
		".py":     "python",
		".cs":     "csharp",
		".go":     "go",
		".js":     "javascript",
		".jsx":    "javascript",
		".ts":     "typescript",
		".tsx":    "typescript",
		".rb":     "ruby",
		".swift":  "swift",
		".kt":     "kotlin",
		".scala":  "scala",
		".groovy": "groovy",

		// 系统级语言
		".c":   "c",
		".h":   "c", // C/C++头文件
		".cpp": "cpp",
		".cc":  "cpp",
		".cxx": "cpp",
		".hpp": "cpp",
		".rs":  "rust",

		// Web前端
		".vue":    "vue",
		".svelte": "svelte",

		// Shell脚本
		".sh":   "bash",
		".bash": "bash",
		".zsh":  "bash",

		// 移动开发
		".m":  "objectivec",
		".mm": "objectivec",

		// 脚本语言
		".lua": "lua",
		".pl":  "perl",
		".pm":  "perl",
		".r":   "r",

		// 数据/配置文件
		".sql": "sql",

		// 编译型语言
		".dart": "dart",
		".ex":   "elixir",
		".exs":  "elixir",
		".erl":  "erlang",
		".fs":   "fsharp",
		".fsx":  "fsharp",
		".hs":   "haskell",
		".clj":  "clojure",
		".cljs": "clojure",

		// 配置文件
		".yaml": "yaml",
		".yml":  "yaml",
		".toml": "toml",
	}

	maxCount := 0
	for ext, count := range extensions {
		if count > maxCount {
			if lang, ok := extCounts[ext]; ok {
				c.Language = lang
				maxCount = count
			}
		}
	}

	if c.Language == "" {
		// 尝试通过文件名模式检测
		filepath.Walk(c.SourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			baseName := strings.ToLower(info.Name())
			// CMakeLists.txt 表示 C/C++ 项目
			if baseName == "cmakelists.txt" || baseName == "makefile" {
				c.Language = "cpp"
				return filepath.SkipAll
			}
			// Cargo.toml 表示 Rust 项目
			if baseName == "cargo.toml" {
				c.Language = "rust"
				return filepath.SkipAll
			}
			return nil
		})
	}

	if c.Language == "" {
		c.Language = "generic" // 默认
	}
}

// loadFiles 加载所有代码文件（自动解码 Unicode）
func (c *CallGraphAnalyzer) loadFiles() error {
	// 支持的所有代码文件扩展名
	codeExtensions := map[string]bool{
		// Java生态系统
		".java": true, ".kt": true, ".scala": true, ".groovy": true,
		// Web后端
		".php": true, ".py": true, ".rb": true, ".cs": true,
		// 前端
		".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".vue": true, ".svelte": true,
		// 系统级语言
		".c": true, ".h": true, ".cpp": true, ".cc": true, ".cxx": true, ".hpp": true, ".rs": true,
		// Go
		".go": true,
		// Shell脚本
		".sh": true, ".bash": true, ".zsh": true,
		// 移动开发
		".m": true, ".mm": true, ".swift": true,
		// 脚本语言
		".lua": true, ".pl": true, ".pm": true, ".r": true,
		// 函数式语言
		".erl": true, ".ex": true, ".exs": true, ".fs": true, ".fsx": true, ".hs": true, ".clj": true, ".cljs": true,
		// 其他
		".dart": true, ".sql": true,
	}

	return filepath.Walk(c.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过不需要的目录
		if info.IsDir() {
			skipDirs := []string{"node_modules", ".git", "target", "build", "dist", "vendor", "venv", "__pycache__", ".idea", ".vscode", ".gradle", ".maven", "bin", "obj"}
			for _, skip := range skipDirs {
				if strings.Contains(path, skip) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if codeExtensions[ext] {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			// 解码 Unicode 转义序列
			decoded := decodeUnicodeString(string(content))
			c.Files[path] = decoded
		}

		return nil
	})
}

// BuildCallGraph 构建调用图
func (c *CallGraphAnalyzer) BuildCallGraph() error {
	// 第一步：提取所有方法/函数节点
	if err := c.extractNodes(); err != nil {
		return fmt.Errorf("提取节点失败: %v", err)
	}

	// 第二步：提取导入/引用关系
	c.extractImports()

	// 第三步：构建调用边
	if err := c.extractEdges(); err != nil {
		return fmt.Errorf("提取调用边失败: %v", err)
	}

	// 第四步：建立反向索引（被调用 -> 调用者）
	c.buildReverseIndex()

	return nil
}

// extractNodes 从所有文件中提取方法/函数节点
func (c *CallGraphAnalyzer) extractNodes() error {
	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")

		switch c.Language {
		case "java", "csharp":
			c.extractJavaNodes(filePath, lines)
		case "python":
			c.extractPythonNodes(filePath, lines)
		case "php":
			c.extractPHPNodes(filePath, lines)
		case "go":
			c.extractGoNodes(filePath, lines)
		case "javascript", "typescript":
			c.extractJSNodes(filePath, lines)
		}
	}

	return nil
}

// extractJavaNodes 提取Java方法节点
func (c *CallGraphAnalyzer) extractJavaNodes(filePath string, lines []string) {
	// 提取类名
	classPattern := regexp.MustCompile(`(?:public\s+)?class\s+(\w+)`)
	// 安全处理：确保切片不越界
	previewLines := len(lines)
	if previewLines > 50 {
		previewLines = 50
	}
	classMatch := classPattern.FindStringSubmatch(strings.Join(lines[:previewLines], "\n"))
	className := ""
	if len(classMatch) > 1 {
		className = classMatch[1]
		c.ClassMap[filePath] = className
	}

	// 提取方法
	methodPattern := regexp.MustCompile(`(?:public|private|protected)\s+(?:static\s+)?(?:final\s+)?(?:synchronized\s+)?(?:\w+\s+)+(\w+)\s*\(([^)]*)\)`)

	for i, line := range lines {
		matches := methodPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				methodName := match[1]
				params := match[2]

				// 跳过构造函数
				if methodName == className || methodName == "<init>" {
					continue
				}

				nodeKey := fmt.Sprintf("%s.%s", className, methodName)
				paramList := strings.Split(params, ",")
				for i := range paramList {
					paramList[i] = strings.TrimSpace(paramList[i])
				}

				c.Mutex.Lock()
				c.Nodes[nodeKey] = &CallGraphNode{
					Name:       methodName,
					FilePath:   filePath,
					Line:       i + 1,
					ClassName:  className,
					Parameters: paramList,
					IsPublic:   strings.Contains(line, "public"),
				}
				c.Mutex.Unlock()
			}
		}
	}
}

// extractPythonNodes 提取Python函数节点
func (c *CallGraphAnalyzer) extractPythonNodes(filePath string, lines []string) {
	// 提取类和函数
	classPattern := regexp.MustCompile(`^class\s+(\w+)(?:\([^)]*\))?:`)
	funcPattern := regexp.MustCompile(`^def\s+(\w+)\s*\(([^)]*)\):`)

	currentClass := ""

	for i, line := range lines {
		// 去除缩进
		trimmed := strings.TrimLeft(line, " \t")

		// 类定义
		classMatch := classPattern.FindStringSubmatch(trimmed)
		if len(classMatch) > 1 {
			currentClass = classMatch[1]
			continue
		}

		// 函数定义
		funcMatch := funcPattern.FindStringSubmatch(trimmed)
		if len(funcMatch) >= 2 {
			funcName := funcMatch[1]
			params := funcMatch[2]

			var nodeKey string
			if currentClass != "" {
				nodeKey = fmt.Sprintf("%s.%s", currentClass, funcName)
			} else {
				nodeKey = funcName
			}

			paramList := strings.Split(params, ",")
			for i := range paramList {
				paramList[i] = strings.TrimSpace(paramList[i])
			}

			c.Mutex.Lock()
			c.Nodes[nodeKey] = &CallGraphNode{
				Name:       funcName,
				FilePath:   filePath,
				Line:       i + 1,
				ClassName:  currentClass,
				Parameters: paramList,
				IsPublic:   true, // Python默认公开
			}
			c.Mutex.Unlock()
		}
	}
}

// extractPHPNodes 提取PHP函数/方法节点
func (c *CallGraphAnalyzer) extractPHPNodes(filePath string, lines []string) {
	// 提取类
	classPattern := regexp.MustCompile(`(?:abstract\s+)?class\s+(\w+)`)
	interfacePattern := regexp.MustCompile(`interface\s+(\w+)`)

	// 提取方法
	methodPattern := regexp.MustCompile(`(?:public|private|protected)\s+function\s+(\w+)\s*\(([^)]*)\)`)

	currentClass := ""

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")

		// 类定义
		if classMatch := classPattern.FindStringSubmatch(trimmed); len(classMatch) > 1 {
			currentClass = classMatch[1]
			c.ClassMap[filePath] = currentClass
			continue
		}

		// 接口定义
		if interfaceMatch := interfacePattern.FindStringSubmatch(trimmed); len(interfaceMatch) > 1 {
			currentClass = interfaceMatch[1]
			c.ClassMap[filePath] = currentClass
			continue
		}

		// 方法定义
		if methodMatch := methodPattern.FindStringSubmatch(trimmed); len(methodMatch) >= 2 {
			methodName := methodMatch[1]
			params := methodMatch[2]

			nodeKey := fmt.Sprintf("%s.%s", currentClass, methodName)
			paramList := strings.Split(params, ",")
			for i := range paramList {
				paramList[i] = strings.TrimSpace(paramList[i])
			}

			c.Mutex.Lock()
			c.Nodes[nodeKey] = &CallGraphNode{
				Name:       methodName,
				FilePath:   filePath,
				Line:       i + 1,
				ClassName:  currentClass,
				Parameters: paramList,
				IsPublic:   strings.Contains(trimmed, "public"),
			}
			c.Mutex.Unlock()
		}
	}
}

// extractGoNodes 提取Go函数节点
func (c *CallGraphAnalyzer) extractGoNodes(filePath string, lines []string) {
	// 提取包名
	packagePattern := regexp.MustCompile(`^package\s+(\w+)`)

	// 提取函数
	funcPattern := regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(([^)]*)\)`)

	// 提取方法
	methodPattern := regexp.MustCompile(`^func\s+\(([^)]+)\s+\*?(\w+)\)\s+(\w+)\s*\(([^)]*)\)`)

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")

		// 包名提取（暂时不使用，保留供将来扩展）
		if pkgMatch := packagePattern.FindStringSubmatch(trimmed); len(pkgMatch) > 1 {
			_ = pkgMatch[1] // 忽略未使用的包名
			continue
		}

		// 方法定义
		if methodMatch := methodPattern.FindStringSubmatch(trimmed); len(methodMatch) >= 4 {
			receiverType := methodMatch[2]
			methodName := methodMatch[3]
			params := methodMatch[4]

			nodeKey := fmt.Sprintf("%s.%s", receiverType, methodName)
			paramList := strings.Split(params, ",")
			for i := range paramList {
				paramList[i] = strings.TrimSpace(paramList[i])
			}

			// 安全处理：确保方法名不为空
			isPublic := false
			if len(methodName) > 0 {
				firstChar := string([]rune(methodName)[0])
				isPublic = strings.HasPrefix(methodName, strings.ToUpper(firstChar))
			}

			c.Mutex.Lock()
			c.Nodes[nodeKey] = &CallGraphNode{
				Name:       methodName,
				FilePath:   filePath,
				Line:       i + 1,
				ClassName:  receiverType,
				Parameters: paramList,
				IsPublic:   isPublic,
			}
			c.Mutex.Unlock()
			continue
		}

		// 函数定义
		if funcMatch := funcPattern.FindStringSubmatch(trimmed); len(funcMatch) >= 2 {
			funcName := funcMatch[1]
			params := funcMatch[2]

			// 跳过init
			if funcName == "init" {
				continue
			}

			nodeKey := funcName
			paramList := strings.Split(params, ",")
			for i := range paramList {
				paramList[i] = strings.TrimSpace(paramList[i])
			}

			// 安全处理：确保函数名不为空
			isPublicFunc := false
			if len(funcName) > 0 {
				firstChar := string([]rune(funcName)[0])
				isPublicFunc = strings.HasPrefix(funcName, strings.ToUpper(firstChar))
			}

			c.Mutex.Lock()
			c.Nodes[nodeKey] = &CallGraphNode{
				Name:       funcName,
				FilePath:   filePath,
				Line:       i + 1,
				Parameters: paramList,
				IsPublic:   isPublicFunc,
			}
			c.Mutex.Unlock()
		}
	}
}

// extractJSNodes 提取JavaScript/TypeScript函数节点
func (c *CallGraphAnalyzer) extractJSNodes(filePath string, lines []string) {
	// 提取类（ES6+）
	classPattern := regexp.MustCompile(`class\s+(\w+)(?:\s+extends\s+\w+)?\s*\{`)

	// 提取方法
	methodPattern := regexp.MustCompile(`(?:async\s+)?(\w+)\s*\(([^)]*)\)\s*\{`)

	currentClass := ""

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")

		// 类定义
		if classMatch := classPattern.FindStringSubmatch(trimmed); len(classMatch) > 1 {
			currentClass = classMatch[1]
			continue
		}

		// 方法定义
		if methodMatch := methodPattern.FindStringSubmatch(trimmed); len(methodMatch) >= 2 {
			methodName := methodMatch[1]
			params := methodMatch[2]

			// 跳过构造函数和关键字
			if methodName == "constructor" || methodName == "if" || methodName == "for" || methodName == "while" {
				continue
			}

			var nodeKey string
			if currentClass != "" {
				nodeKey = fmt.Sprintf("%s.%s", currentClass, methodName)
			} else {
				nodeKey = methodName
			}

			paramList := strings.Split(params, ",")
			for i := range paramList {
				paramList[i] = strings.TrimSpace(paramList[i])
			}

			c.Mutex.Lock()
			c.Nodes[nodeKey] = &CallGraphNode{
				Name:       methodName,
				FilePath:   filePath,
				Line:       i + 1,
				ClassName:  currentClass,
				Parameters: paramList,
				IsPublic:   true,
			}
			c.Mutex.Unlock()
		}
	}
}

// extractImports 提取导入/引用关系
func (c *CallGraphAnalyzer) extractImports() {
	switch c.Language {
	case "java":
		c.extractJavaImports()
	case "python":
		c.extractPythonImports()
	case "php":
		c.extractPHPImports()
	case "go":
		c.extractGoImports()
	}
}

func (c *CallGraphAnalyzer) extractJavaImports() {
	importPattern := regexp.MustCompile(`import\s+([\w.]+);`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")
		var imports []string

		for _, line := range lines {
			if match := importPattern.FindStringSubmatch(line); len(match) > 1 {
				imports = append(imports, match[1])
			}
		}

		c.ImportMap[filePath] = imports
	}
}

func (c *CallGraphAnalyzer) extractPythonImports() {
	importPattern := regexp.MustCompile(`(?:import\s+(\w+)|from\s+(\w+)\s+import)`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")
		var imports []string

		for _, line := range lines {
			if match := importPattern.FindStringSubmatch(line); len(match) > 1 {
				if match[1] != "" {
					imports = append(imports, match[1])
				}
				if match[2] != "" {
					imports = append(imports, match[2])
				}
			}
		}

		c.ImportMap[filePath] = imports
	}
}

func (c *CallGraphAnalyzer) extractPHPImports() {
	importPattern := regexp.MustCompile(`(?:require|require_once|include|include_once)\s+(?:_once\s+)?['"]([\w./]+)['"]`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")
		var imports []string

		for _, line := range lines {
			if match := importPattern.FindStringSubmatch(line); len(match) > 1 {
				imports = append(imports, match[1])
			}
		}

		c.ImportMap[filePath] = imports
	}
}

func (c *CallGraphAnalyzer) extractGoImports() {
	importPattern := regexp.MustCompile(`"([^"]+)"`)
	inImportBlock := false

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")
		var imports []string

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			if trimmed == "import (" {
				inImportBlock = true
				continue
			}
			if inImportBlock && trimmed == ")" {
				inImportBlock = false
				continue
			}

			if inImportBlock || strings.HasPrefix(trimmed, "import ") {
				if match := importPattern.FindStringSubmatch(trimmed); len(match) > 1 {
					imports = append(imports, match[1])
				}
			}
		}

		c.ImportMap[filePath] = imports
	}
}

// extractEdges 提取调用边
func (c *CallGraphAnalyzer) extractEdges() error {
	switch c.Language {
	case "java", "csharp":
		return c.extractJavaEdges()
	case "python":
		return c.extractPythonEdges()
	case "php":
		return c.extractPHPEdges()
	case "go":
		return c.extractGoEdges()
	case "javascript", "typescript":
		return c.extractJSEdges()
	}
	return nil
}

func (c *CallGraphAnalyzer) extractJavaEdges() error {
	callPattern := regexp.MustCompile(`(\w+)\.(\w+)\s*\(`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")

		for lineNum, line := range lines {
			matches := callPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					_ = match[1]
					methodName := match[2]

					// 跳过常见的方法调用
					if isCommonMethod(methodName) {
						continue
					}

					// 查找被调用的方法
					calleeKey := fmt.Sprintf("%s.%s", match[1], methodName)

					// 如果找不到类方法匹配，尝试纯方法名
					if _, exists := c.Nodes[calleeKey]; !exists {
						calleeKey = methodName
					}

					if callerNode := c.findNodeInFile(filePath); callerNode != nil {
						if calleeNode, exists := c.Nodes[calleeKey]; exists {
							edge := &CallGraphEdge{
								Caller:   callerNode,
								Callee:   calleeNode,
								CallType: "direct",
								Line:     lineNum + 1,
								FilePath: filePath,
							}
							c.Edges = append(c.Edges, edge)
						}
					}
				}
			}
		}
	}

	return nil
}

func (c *CallGraphAnalyzer) extractPythonEdges() error {
	callPattern := regexp.MustCompile(`(\w+)\.(\w+)\s*\(`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")

		for lineNum, line := range lines {
			matches := callPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					_ = match[1]
					methodName := match[2]

					if isCommonMethod(methodName) {
						continue
					}

					calleeKey := methodName
					if c.ClassMap[filePath] != "" {
						calleeKey = fmt.Sprintf("%s.%s", c.ClassMap[filePath], methodName)
					}

					if _, exists := c.Nodes[calleeKey]; exists {
						if callerNode := c.findNodeInFile(filePath); callerNode != nil {
							edge := &CallGraphEdge{
								Caller:   callerNode,
								Callee:   c.Nodes[calleeKey],
								CallType: "direct",
								Line:     lineNum + 1,
								FilePath: filePath,
							}
							c.Edges = append(c.Edges, edge)
						}
					}
				}
			}
		}
	}

	return nil
}

func (c *CallGraphAnalyzer) extractPHPEdges() error {
	callPattern := regexp.MustCompile(`(\w+)->(\w+)\s*\(|(\w+)\s*\(([^)]*)\)`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")

		for lineNum, line := range lines {
			matches := callPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					_ = match[1] // objName - 暂时不使用
					methodName := match[2]

					if methodName == "" {
						methodName = match[3]
					}

					if isCommonMethod(methodName) {
						continue
					}

					// 查找方法
					for nodeKey, node := range c.Nodes {
						if strings.HasSuffix(nodeKey, "."+methodName) {
							if callerNode := c.findNodeInFile(filePath); callerNode != nil {
								edge := &CallGraphEdge{
									Caller:   callerNode,
									Callee:   node,
									CallType: "direct",
									Line:     lineNum + 1,
									FilePath: filePath,
								}
								c.Edges = append(c.Edges, edge)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (c *CallGraphAnalyzer) extractGoEdges() error {
	callPattern := regexp.MustCompile(`(\w+)\.(\w+)\s*\(`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")

		for lineNum, line := range lines {
			matches := callPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					_ = match[1]
					methodName := match[2]

					if isCommonMethod(methodName) {
						continue
					}

					// 查找方法
					for nodeKey, node := range c.Nodes {
						if strings.HasSuffix(nodeKey, "."+methodName) || nodeKey == methodName {
							if callerNode := c.findNodeInFile(filePath); callerNode != nil {
								edge := &CallGraphEdge{
									Caller:   callerNode,
									Callee:   node,
									CallType: "direct",
									Line:     lineNum + 1,
									FilePath: filePath,
								}
								c.Edges = append(c.Edges, edge)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (c *CallGraphAnalyzer) extractJSEdges() error {
	callPattern := regexp.MustCompile(`(\w+)\.(\w+)\s*\(`)

	for filePath, content := range c.Files {
		lines := strings.Split(content, "\n")

		for lineNum, line := range lines {
			matches := callPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) >= 3 {
					_ = match[1]
					methodName := match[2]

					if isCommonMethod(methodName) {
						continue
					}

					// 查找方法
					for nodeKey, node := range c.Nodes {
						if strings.HasSuffix(nodeKey, "."+methodName) || nodeKey == methodName {
							if callerNode := c.findNodeInFile(filePath); callerNode != nil {
								edge := &CallGraphEdge{
									Caller:   callerNode,
									Callee:   node,
									CallType: "direct",
									Line:     lineNum + 1,
									FilePath: filePath,
								}
								c.Edges = append(c.Edges, edge)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// findNodeInFile 查找文件中定义的节点
func (c *CallGraphAnalyzer) findNodeInFile(filePath string) *CallGraphNode {
	for _, node := range c.Nodes {
		if node.FilePath == filePath {
			return node
		}
	}
	return nil
}

// buildReverseIndex 建立反向索引
func (c *CallGraphAnalyzer) buildReverseIndex() {
	for _, edge := range c.Edges {
		calleeKey := fmt.Sprintf("%s.%s", edge.Callee.ClassName, edge.Callee.Name)
		if edge.Callee.ClassName == "" {
			calleeKey = edge.Callee.Name
		}

		c.Mutex.Lock()
		c.CalledByMap[calleeKey] = append(c.CalledByMap[calleeKey],
			fmt.Sprintf("%s:%d", edge.Caller.FilePath, edge.Line))

		// 同时更新文件索引
		c.FileIndex[edge.FilePath] = append(c.FileIndex[edge.FilePath], calleeKey)
		c.Mutex.Unlock()
	}
}

// ============ 漏洞利用链分析核心功能 ============

// SourcePoint 入口点（用户输入源）
type SourcePoint struct {
	Node     *CallGraphNode
	Patterns []string // 匹配的模式
	Type     string   // source类型
}

// SinkPoint 危险点
type SinkPoint struct {
	Node     *CallGraphNode
	Patterns []string // 匹配的模式
	Type     string   // sink类型
	VulnType string   // 对应的漏洞类型
}

// FindEntryPoints 查找入口点（用户输入相关的方法）
func (c *CallGraphAnalyzer) FindEntryPoints() []*SourcePoint {
	entryPointPatterns := map[string][]struct {
		Pattern string
		Type    string
	}{
		"java": {
			{"getParameter", "query_param"},
			{"getHeader", "header"},
			{"getQueryString", "query_string"},
			{"@RequestParam", "annotation"},
			{"@RequestBody", "body"},
			{"@PathVariable", "path_variable"},
			{"@RequestMapping", "annotation"},
			{"HttpServletRequest", "servlet"},
			{"ServletInputStream", "input"},
			{"BufferedReader", "input"},
			{"getCookies", "cookie"},
			{"getSession", "session"},
			{"getAttribute", "attribute"},
			{"@GetMapping", "rest_annotation"},
			{"@PostMapping", "rest_annotation"},
			{"@PutMapping", "rest_annotation"},
			{"@DeleteMapping", "rest_annotation"},
		},
		"python": {
			{"request.", "framework"},
			{"input(", "builtin"},
			{"sys.argv", "cli"},
			{"os.environ", "env"},
			{"flask.request", "flask"},
			{"django.request", "django"},
			{"fastapi.Request", "fastapi"},
			{"@app.route", "decorator"},
			{"@router.get", "router"},
		},
		"php": {
			{"$_GET", "superglobal"},
			{"$_POST", "superglobal"},
			{"$_REQUEST", "superglobal"},
			{"$_COOKIE", "superglobal"},
			{"$_SERVER", "superglobal"},
			{"$GLOBALS", "superglobal"},
			{"file_get_contents", "file_input"},
			{"fopen", "file_input"},
		},
		"go": {
			{"r.FormValue", "form"},
			{"r.Form", "form"},
			{"r.PostForm", "form"},
			{"r.URL.Query", "query"},
			{"c.Query", "gin"},
			{"c.PostForm", "gin"},
			{"c.Bind", "gin"},
		},
		"javascript": {
			{"req.body", "express"},
			{"req.query", "express"},
			{"req.params", "express"},
			{"process.argv", "node"},
			{"process.env", "env"},
			{"document.cookie", "dom"},
		},
	}

	patterns := entryPointPatterns[c.Language]
	if patterns == nil {
		patterns = entryPointPatterns["java"]
	}

	var sources []*SourcePoint
	seenFiles := make(map[string]bool)

	for filePath, content := range c.Files {
		if seenFiles[filePath] {
			continue
		}

		for _, p := range patterns {
			if strings.Contains(content, p.Pattern) {
				seenFiles[filePath] = true
				sources = append(sources, &SourcePoint{
					Node: &CallGraphNode{
						FilePath: filePath,
						Name:     filepath.Base(filePath),
					},
					Patterns: []string{p.Pattern},
					Type:     p.Type,
				})
				break
			}
		}
	}

	return sources
}

// FindSinkPoints 查找危险Sink点
func (c *CallGraphAnalyzer) FindSinkPoints() []*SinkPoint {
	sinkPatterns := map[string][]struct {
		Pattern  string
		Type     string
		VulnType string
	}{
		"java": {
			{"Statement.execute", "sql", "SQL注入"},
			{"PreparedStatement", "sql", "SQL注入"},
			{"createQuery", "sql", "SQL注入"},
			{"createNativeQuery", "sql", "SQL注入"},
			{"Runtime.getRuntime().exec", "command", "命令注入"},
			{"ProcessBuilder", "command", "命令注入"},
			{"eval(", "eval", "代码注入"},
			{"executeScript(", "script", "代码注入"},
			{"FileWriter", "file", "文件操作"},
			{"FileOutputStream", "file", "文件操作"},
			{"ObjectInputStream", "deserialize", "反序列化漏洞"},
			{"readObject", "deserialize", "反序列化漏洞"},
			{"XMLStreamReader", "xml", "XXE"},
			{"DocumentBuilder", "xml", "XXE"},
		},
		"python": {
			{"eval(", "eval", "代码注入"},
			{"exec(", "exec", "代码注入"},
			{"subprocess.", "command", "命令注入"},
			{"os.system", "command", "命令注入"},
			{"os.popen", "command", "命令注入"},
			{"pickle.load", "deserialize", "反序列化漏洞"},
			{"yaml.load", "deserialize", "YAML注入"},
			{"cursor.execute", "sql", "SQL注入"},
			{"text(", "sql", "SQL注入"},
			{"open(", "file", "文件操作"},
		},
		"php": {
			{"eval(", "eval", "代码注入"},
			{"exec(", "command", "命令注入"},
			{"system(", "command", "命令注入"},
			{"shell_exec(", "command", "命令注入"},
			{"mysqli_query", "sql", "SQL注入"},
			{"mysql_query", "sql", "SQL注入"},
			{"PDO::exec", "sql", "SQL注入"},
			{"file_put_contents", "file", "文件操作"},
			{"unserialize(", "deserialize", "反序列化漏洞"},
		},
		"go": {
			{"exec.Command", "command", "命令注入"},
			{"template.HTML", "xss", "XSS"},
			{"sql.DB.Exec", "sql", "SQL注入"},
			{"sql.DB.Query", "sql", "SQL注入"},
		},
		"javascript": {
			{"eval(", "eval", "代码注入"},
			{"Function(", "eval", "代码注入"},
			{"execScript", "script", "代码注入"},
			{"child_process", "command", "命令注入"},
			{".sql", "sql", "SQL注入"},
		},
	}

	patterns := sinkPatterns[c.Language]
	if patterns == nil {
		patterns = sinkPatterns["java"]
	}

	var sinks []*SinkPoint
	seenFiles := make(map[string]bool)

	for filePath, content := range c.Files {
		if seenFiles[filePath] {
			continue
		}

		for _, p := range patterns {
			if strings.Contains(content, p.Pattern) {
				seenFiles[filePath] = true
				sinks = append(sinks, &SinkPoint{
					Node: &CallGraphNode{
						FilePath: filePath,
						Name:     filepath.Base(filePath),
					},
					Patterns: []string{p.Pattern},
					Type:     p.Type,
					VulnType: p.VulnType,
				})
				break
			}
		}
	}

	return sinks
}

// AnalyzeExploitChains 分析漏洞利用链（从Source到Sink的完整路径）
func (c *CallGraphAnalyzer) AnalyzeExploitChains(vulnFile string, vulnLine int, vulnType string) []*ExploitChain {
	var chains []*ExploitChain

	// 查找Source点和Sink点
	sources := c.FindEntryPoints()
	sinks := c.FindSinkPoints()

	// 对于每个Sink点，尝试找到从Source到它的路径
	for _, sink := range sinks {
		// 查找从Source到Sink的调用链
		path := c.findPathFromSourceToSink(sink.Node, sources)
		if len(path) > 0 {
			chain := c.buildExploitChain(path, sink, vulnFile, vulnLine, vulnType)
			chains = append(chains, chain)
		}
	}

	// 如果没有找到路径，生成基于当前漏洞的简化链
	if len(chains) == 0 && vulnFile != "" {
		chain := c.buildSimpleExploitChain(vulnFile, vulnLine, vulnType)
		chains = append(chains, chain)
	}

	return chains
}

// findPathFromSourceToSink 查找从Source到Sink的路径
func (c *CallGraphAnalyzer) findPathFromSourceToSink(sinkNode *CallGraphNode, sources []*SourcePoint) []*CallGraphNode {
	var path []*CallGraphNode

	// 使用BFS从Sink反向追溯到Source
	visited := make(map[string]bool)
	queue := []*CallGraphNode{sinkNode}

	for len(queue) > 0 && len(path) < 10 { // 最大路径长度10
		current := queue[0]
		queue = queue[1:]

		key := fmt.Sprintf("%s:%d", current.FilePath, current.Line)
		if visited[key] {
			continue
		}
		visited[key] = true

		// 查找调用当前方法的调用者
		callers := c.GetCallersByNode(current)
		for _, caller := range callers {
			callerKey := fmt.Sprintf("%s:%d", caller.FilePath, caller.Line)
			if !visited[callerKey] {
				queue = append(queue, caller)
				path = append([]*CallGraphNode{caller}, path...)
			}
		}
	}

	return path
}

// GetCallersByNode 获取调用指定节点的调用者
func (c *CallGraphAnalyzer) GetCallersByNode(node *CallGraphNode) []*CallGraphNode {
	var callers []*CallGraphNode

	for _, edge := range c.Edges {
		if edge.Callee.FilePath == node.FilePath && edge.Callee.Line == node.Line {
			callers = append(callers, edge.Caller)
		}
	}

	return callers
}

// GetCalleesByNode 获取指定节点调用的方法
func (c *CallGraphAnalyzer) GetCalleesByNode(node *CallGraphNode) []*CallGraphNode {
	var callees []*CallGraphNode

	for _, edge := range c.Edges {
		if edge.Caller.FilePath == node.FilePath && edge.Caller.Line == node.Line {
			callees = append(callees, edge.Callee)
		}
	}

	return callees
}

// buildExploitChain 构建漏洞利用链
func (c *CallGraphAnalyzer) buildExploitChain(path []*CallGraphNode, sink *SinkPoint, vulnFile string, vulnLine int, vulnType string) *ExploitChain {
	chain := &ExploitChain{
		ID:          fmt.Sprintf("chain_%s_%d", filepath.Base(sink.Node.FilePath), sink.Node.Line),
		Type:        vulnType,
		Description: fmt.Sprintf("从用户输入到危险操作 [%s] 的利用链", sink.VulnType),
		RiskLevel:   "High",
		Steps:       make([]ExploitChainStep, 0),
	}

	// 添加Source节点
	if len(path) > 0 {
		sourceNode := path[0]
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:    1,
			NodeType:   "source",
			MethodName: sourceNode.Name,
			ClassName:  sourceNode.ClassName,
			FileName:   filepath.Base(sourceNode.FilePath),
			Line:       sourceNode.Line,
			Params:     sourceNode.Parameters,
		})
		chain.Source = &ChainNode{
			ID:         "source",
			Label:      "用户输入",
			MethodName: sourceNode.Name,
			ClassName:  sourceNode.ClassName,
			FileName:   filepath.Base(sourceNode.FilePath),
			Line:       sourceNode.Line,
			NodeType:   "source",
		}
	}

	// 添加中间节点
	for i, node := range path[1:] {
		nodeType := c.classifyNodeType(node)
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:     i + 2,
			NodeType:    nodeType,
			MethodName:  node.Name,
			ClassName:   node.ClassName,
			FileName:    filepath.Base(node.FilePath),
			Line:        node.Line,
			Params:      node.Parameters,
			IsVulnPoint: node.FilePath == vulnFile && node.Line == vulnLine,
			VulnType:    vulnType,
		})
	}

	// 添加Sink节点
	chain.Steps = append(chain.Steps, ExploitChainStep{
		StepNum:     len(path) + 1,
		NodeType:    "sink",
		MethodName:  sink.Node.Name,
		ClassName:   sink.Node.ClassName,
		FileName:    filepath.Base(sink.Node.FilePath),
		Line:        sink.Node.Line,
		Params:      sink.Node.Parameters,
		IsVulnPoint: sink.Node.FilePath == vulnFile && sink.Node.Line == vulnLine,
		VulnType:    sink.VulnType,
	})
	chain.Sink = &ChainNode{
		ID:         "sink",
		Label:      sink.VulnType,
		MethodName: sink.Node.Name,
		ClassName:  sink.Node.ClassName,
		FileName:   filepath.Base(sink.Node.FilePath),
		Line:       sink.Node.Line,
		NodeType:   "sink",
		Type:       sink.VulnType,
		Severity:   "High",
	}

	return chain
}

// buildSimpleExploitChain 构建简化的漏洞利用链
func (c *CallGraphAnalyzer) buildSimpleExploitChain(vulnFile string, vulnLine int, vulnType string) *ExploitChain {
	chain := &ExploitChain{
		ID:          fmt.Sprintf("chain_%s_%d", filepath.Base(vulnFile), vulnLine),
		Type:        vulnType,
		Description: fmt.Sprintf("漏洞位于 %s 第%d行", filepath.Base(vulnFile), vulnLine),
		RiskLevel:   "High",
		Steps:       make([]ExploitChainStep, 0),
	}

	// 查找包含漏洞的文件
	var vulnNode *CallGraphNode
	for _, node := range c.Nodes {
		if node.FilePath == vulnFile {
			// 找到最接近的行号
			if vulnNode == nil || abs(node.Line-vulnLine) < abs(vulnNode.Line-vulnLine) {
				vulnNode = node
			}
		}
	}

	// 尝试找到调用链
	var path []*CallGraphNode
	if vulnNode != nil {
		path = c.findPathFromSourceToSink(vulnNode, c.FindEntryPoints())
	}

	// 如果没有找到路径，查找Controller/Service/DAO模式
	if len(path) == 0 {
		path = c.findMVCPath(vulnFile, vulnLine)
	}

	// 添加Source节点
	if len(path) > 0 {
		sourceNode := path[0]
		chain.Source = &ChainNode{
			ID:         "source",
			Label:      "用户输入",
			MethodName: sourceNode.Name,
			ClassName:  sourceNode.ClassName,
			FileName:   filepath.Base(sourceNode.FilePath),
			Line:       sourceNode.Line,
			NodeType:   "source",
		}
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:    1,
			NodeType:   "source",
			MethodName: sourceNode.Name,
			ClassName:  sourceNode.ClassName,
			FileName:   filepath.Base(sourceNode.FilePath),
			Line:       sourceNode.Line,
		})
	} else {
		// 没有找到Source，添加一个默认的
		chain.Source = &ChainNode{
			ID:       "source",
			Label:    "用户输入",
			NodeType: "source",
		}
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:    1,
			NodeType:   "source",
			MethodName: "用户入口",
		})
	}

	// 添加中间路径节点
	for i, node := range path[1:] {
		nodeType := c.classifyNodeType(node)
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:    i + 2,
			NodeType:   nodeType,
			MethodName: node.Name,
			ClassName:  node.ClassName,
			FileName:   filepath.Base(node.FilePath),
			Line:       node.Line,
		})
	}

	// 添加漏洞点
	if vulnNode != nil {
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:     len(path) + 1,
			NodeType:    "current",
			MethodName:  vulnNode.Name,
			ClassName:   vulnNode.ClassName,
			FileName:    filepath.Base(vulnFile),
			Line:        vulnLine,
			IsVulnPoint: true,
			VulnType:    vulnType,
		})
	}

	// 添加Sink节点
	sinks := c.FindSinkPoints()
	var matchingSink *SinkPoint
	for _, sink := range sinks {
		if sink.Node.FilePath == vulnFile {
			matchingSink = sink
			break
		}
	}

	if matchingSink != nil {
		chain.Sink = &ChainNode{
			ID:         "sink",
			Label:      matchingSink.VulnType,
			MethodName: matchingSink.Node.Name,
			ClassName:  matchingSink.Node.ClassName,
			FileName:   filepath.Base(matchingSink.Node.FilePath),
			Line:       matchingSink.Node.Line,
			NodeType:   "sink",
			Type:       matchingSink.VulnType,
		}
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:    len(path) + 2,
			NodeType:   "sink",
			MethodName: matchingSink.Node.Name,
			ClassName:  matchingSink.Node.ClassName,
			FileName:   filepath.Base(matchingSink.Node.FilePath),
			Line:       matchingSink.Node.Line,
			VulnType:   matchingSink.VulnType,
		})
	} else {
		chain.Sink = &ChainNode{
			ID:       "sink",
			Label:    "危险操作",
			NodeType: "sink",
		}
		chain.Steps = append(chain.Steps, ExploitChainStep{
			StepNum:    len(path) + 2,
			NodeType:   "sink",
			MethodName: "危险操作",
		})
	}

	return chain
}

// findMVCPath 查找MVC调用路径
func (c *CallGraphAnalyzer) findMVCPath(vulnFile string, vulnLine int) []*CallGraphNode {
	var path []*CallGraphNode

	// 识别Controller/Service/DAO模式
	for _, node := range c.Nodes {
		className := strings.ToLower(node.ClassName)
		fileName := strings.ToLower(filepath.Base(node.FilePath))

		// Controller层
		if strings.Contains(className, "controller") || strings.Contains(fileName, "controller") {
			path = append(path, node)
		}
	}

	// Service层
	for _, node := range c.Nodes {
		className := strings.ToLower(node.ClassName)
		fileName := strings.ToLower(filepath.Base(node.FilePath))

		if strings.Contains(className, "service") || strings.Contains(fileName, "service") {
			// 避免重复
			exists := false
			for _, n := range path {
				if n.FilePath == node.FilePath && n.Line == node.Line {
					exists = true
					break
				}
			}
			if !exists {
				path = append(path, node)
			}
		}
	}

	// DAO层
	for _, node := range c.Nodes {
		className := strings.ToLower(node.ClassName)
		fileName := strings.ToLower(filepath.Base(node.FilePath))

		if strings.Contains(className, "dao") || strings.Contains(className, "repository") ||
			strings.Contains(fileName, "dao") || strings.Contains(fileName, "repository") {
			exists := false
			for _, n := range path {
				if n.FilePath == node.FilePath && n.Line == node.Line {
					exists = true
					break
				}
			}
			if !exists {
				path = append(path, node)
			}
		}
	}

	return path
}

// classifyNodeType 分类节点类型
func (c *CallGraphAnalyzer) classifyNodeType(node *CallGraphNode) string {
	className := strings.ToLower(node.ClassName)
	fileName := strings.ToLower(filepath.Base(node.FilePath))
	methodName := strings.ToLower(node.Name)

	// Controller层
	if strings.Contains(className, "controller") || strings.Contains(fileName, "controller") {
		return "controller"
	}

	// Service层
	if strings.Contains(className, "service") || strings.Contains(fileName, "service") {
		return "service"
	}

	// DAO层
	if strings.Contains(className, "dao") || strings.Contains(className, "repository") ||
		strings.Contains(fileName, "dao") || strings.Contains(fileName, "repository") {
		return "dao"
	}

	// Model层
	if strings.Contains(className, "model") || strings.Contains(className, "entity") ||
		strings.Contains(fileName, "model") || strings.Contains(fileName, "entity") {
		return "model"
	}

	// Config层
	if strings.Contains(className, "config") || strings.Contains(fileName, "config") {
		return "config"
	}

	// Filter层
	if strings.Contains(className, "filter") || strings.Contains(fileName, "filter") {
		return "filter"
	}

	// 根据方法名判断
	if strings.Contains(methodName, "get") || strings.Contains(methodName, "set") ||
		strings.Contains(methodName, "add") || strings.Contains(methodName, "create") {
		return "normal"
	}

	return "normal"
}

// GetExploitChainData 获取漏洞利用链数据（用于前端展示）
func (c *CallGraphAnalyzer) GetExploitChainData(vulnFile string, vulnLine int, vulnType string) map[string]interface{} {
	chains := c.AnalyzeExploitChains(vulnFile, vulnLine, vulnType)

	// 转换为前端需要的格式
	nodes := make([]map[string]interface{}, 0)
	edges := make([]map[string]interface{}, 0)

	for _, chain := range chains {
		for i, step := range chain.Steps {
			nodeID := fmt.Sprintf("step_%s_%s", chain.ID, step.NodeType)

			// 添加节点
			label := step.MethodName
			if step.ClassName != "" {
				label = step.ClassName + "." + step.MethodName
			}
			if label == "." {
				label = step.MethodName
			}

			nodes = append(nodes, map[string]interface{}{
				"id":           nodeID,
				"label":        label,
				"displayLabel": label,
				"methodName":   step.MethodName,
				"className":    step.ClassName,
				"fileName":     step.FileName,
				"line":         step.Line,
				"nodeType":     step.NodeType,
				"type":         step.VulnType,
				"isVulnPoint":  step.IsVulnPoint,
			})

			// 添加边
			if i > 0 {
				prevNodeID := fmt.Sprintf("step_%s_%s", chain.ID, chain.Steps[i-1].NodeType)
				edges = append(edges, map[string]interface{}{
					"id":     fmt.Sprintf("edge_%s_%d", chain.ID, i),
					"source": prevNodeID,
					"target": nodeID,
					"label":  "→",
				})
			}
		}
	}

	return map[string]interface{}{
		"nodes":  nodes,
		"edges":  edges,
		"chains": chains,
	}
}

// 辅助函数
func isCommonMethod(methodName string) bool {
	commonMethods := map[string]bool{
		"toString": true, "equals": true, "hashCode": true,
		"length": true, "size": true, "getClass": true,
		"println": true, "printf": true, "print": true,
		"String": true, "Integer": true, "Long": true,
		"Double": true, "Float": true, "Boolean": true,
		"List": true, "Map": true, "Set": true,
		"Array": true, "Object": true,
		"append": true, "split": true, "trim": true,
		"substring": true, "replace": true, "replaceAll": true,
		"indexOf": true, "lastIndexOf": true, "contains": true,
		"startsWith": true, "endsWith": true,
		"get": true, "set": true, "put": true,
		"add": true, "remove": true, "clear": true,
		"isEmpty": true, "iterator": true,
		"keySet": true, "values": true, "entrySet": true,
	}
	return commonMethods[methodName]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// FindCallChainFromFile 查找从源文件到目标文件的调用链
func (c *CallGraphAnalyzer) FindCallChainFromFile(sourceFile, targetFile string) []string {
	var chain []string

	// 查找从源文件出发的调用
	var findPath func(currentFile string, visited map[string]bool) bool
	findPath = func(currentFile string, visited map[string]bool) bool {
		if visited[currentFile] {
			return false
		}

		visited[currentFile] = true

		if currentFile == targetFile {
			return true
		}

		// 查找当前文件调用的下一个文件
		for _, edge := range c.Edges {
			if edge.Caller.FilePath == currentFile {
				chain = append(chain, fmt.Sprintf("%s -> %s (line %d)",
					filepath.Base(edge.Caller.FilePath),
					filepath.Base(edge.Callee.FilePath),
					edge.Line))

				if findPath(edge.Callee.FilePath, visited) {
					return true
				}
				chain = chain[:len(chain)-1]
			}
		}

		return false
	}

	visited := make(map[string]bool)
	if findPath(sourceFile, visited) {
		return chain
	}

	return []string{}
}

// GetCrossFileContext 获取跨文件上下文用于AI分析
func (c *CallGraphAnalyzer) GetCrossFileContext(filePath string, methodName string) string {
	var ctx strings.Builder

	ctx.WriteString(fmt.Sprintf("## 跨文件调用分析\n\n"))
	ctx.WriteString(fmt.Sprintf("当前文件: %s\n", filePath))
	ctx.WriteString(fmt.Sprintf("当前方法: %s\n\n", methodName))

	// 查找定义的方法
	nodeKey := methodName
	if !strings.Contains(methodName, ".") {
		// 尝试找到完整的方法名
		for key := range c.Nodes {
			if strings.HasSuffix(key, "."+methodName) && c.Nodes[key].FilePath == filePath {
				nodeKey = key
				break
			}
		}
	}

	// 查找调用该方法的方法（调用者）
	ctx.WriteString("### 调用该方法的地方 (Callers)\n")
	calledBy := c.CalledByMap[nodeKey]
	if len(calledBy) > 0 {
		for _, caller := range calledBy {
			ctx.WriteString(fmt.Sprintf("- %s\n", caller))
		}
	} else {
		ctx.WriteString("无\n")
	}
	ctx.WriteString("\n")

	// 查找该方法调用的其他方法（被调用者）
	ctx.WriteString("### 该方法调用的方法 (Callees)\n")
	var callees []string
	for _, edge := range c.Edges {
		if edge.Caller.FilePath == filePath &&
			(strings.HasSuffix(edge.Caller.Name, "."+methodName) || edge.Caller.Name == methodName) {
			callees = append(callees, fmt.Sprintf("%s.%s (line %d)",
				filepath.Base(edge.Callee.FilePath), edge.Callee.Name, edge.Line))
		}
	}
	if len(callees) > 0 {
		for _, callee := range callees {
			ctx.WriteString(fmt.Sprintf("- %s\n", callee))
		}
	} else {
		ctx.WriteString("无\n")
	}
	ctx.WriteString("\n")

	// 入口点分析
	ctx.WriteString("### 入口点分析\n")
	entryPoints := c.FindEntryPoints()
	if len(entryPoints) > 0 {
		displayPoints := entryPoints
		if len(displayPoints) > 5 {
			displayPoints = displayPoints[:5]
		}
		for _, ep := range displayPoints {
			ctx.WriteString(fmt.Sprintf("- 文件: %s, 类型: %s\n", filepath.Base(ep.Node.FilePath), ep.Type))
		}
	} else {
		ctx.WriteString("无\n")
	}
	ctx.WriteString("\n")

	// Sink点分析
	ctx.WriteString("### 危险Sink点分析\n")
	sinks := c.FindSinkPoints()
	if len(sinks) > 0 {
		displaySinks := sinks
		if len(displaySinks) > 5 {
			displaySinks = displaySinks[:5]
		}
		for _, sink := range displaySinks {
			ctx.WriteString(fmt.Sprintf("- 文件: %s, 漏洞类型: %s\n", filepath.Base(sink.Node.FilePath), sink.VulnType))
		}
	} else {
		ctx.WriteString("无\n")
	}

	return ctx.String()
}

// AnalyzeCrossFileVulnerabilities 分析跨文件漏洞
func (c *CallGraphAnalyzer) AnalyzeCrossFileVulnerabilities() []*CrossFileAnalysisResult {
	var results []*CrossFileAnalysisResult

	sources := c.FindEntryPoints()
	sinks := c.FindSinkPoints()

	for _, source := range sources {
		for _, sink := range sinks {
			// 查找从source到sink的路径
			chain := c.FindCallChainFromFile(source.Node.FilePath, sink.Node.FilePath)
			if len(chain) > 0 {
				result := &CrossFileAnalysisResult{
					Vulnerability:  sink.VulnType,
					Severity:       "High",
					SourceFile:     source.Node.FilePath,
					SinkFile:       sink.Node.FilePath,
					Description:    fmt.Sprintf("发现从用户输入到危险操作的路径 [%s]", sink.VulnType),
					CallChain:      chain,
					RiskAssessment: "存在从用户输入到危险操作的路径，需要人工确认",
				}
				results = append(results, result)
			}
		}
	}

	return results
}

// GenerateCrossFileReport 生成跨文件分析报告
func (c *CallGraphAnalyzer) GenerateCrossFileReport() string {
	var report strings.Builder

	report.WriteString("# 跨文件调用链分析报告\n\n")

	// 基本信息
	report.WriteString("## 基本信息\n\n")
	report.WriteString(fmt.Sprintf("- 源代码路径: %s\n", c.SourcePath))
	report.WriteString(fmt.Sprintf("- 检测语言: %s\n", c.Language))
	report.WriteString(fmt.Sprintf("- 代码文件数: %d\n", len(c.Files)))
	report.WriteString(fmt.Sprintf("- 方法/函数数: %d\n", len(c.Nodes)))
	report.WriteString(fmt.Sprintf("- 调用关系数: %d\n\n", len(c.Edges)))

	// 入口点
	report.WriteString("## 入口点分析（用户输入入口）\n\n")
	entryPoints := c.FindEntryPoints()
	if len(entryPoints) > 0 {
		for _, ep := range entryPoints {
			if ep != nil && ep.Node != nil {
				report.WriteString(fmt.Sprintf("- 文件: %s, 类型: %s\n", filepath.Base(ep.Node.FilePath), ep.Type))
			}
		}
	} else {
		report.WriteString("未发现明显的入口点\n")
	}
	report.WriteString("\n")

	// Sink点
	report.WriteString("## 危险Sink点分析\n\n")
	sinks := c.FindSinkPoints()
	if len(sinks) > 0 {
		for _, sink := range sinks {
			if sink != nil && sink.Node != nil {
				report.WriteString(fmt.Sprintf("- 文件: %s, 漏洞类型: %s\n", filepath.Base(sink.Node.FilePath), sink.VulnType))
			}
		}
	} else {
		report.WriteString("未发现明显的危险Sink点\n")
	}
	report.WriteString("\n")

	// 跨文件漏洞分析
	report.WriteString("## 跨文件漏洞分析\n\n")
	results := c.AnalyzeCrossFileVulnerabilities()
	if len(results) > 0 {
		for i, result := range results {
			report.WriteString(fmt.Sprintf("### 潜在漏洞 %d\n\n", i+1))
			report.WriteString(fmt.Sprintf("- **类型**: %s\n", result.Vulnerability))
			report.WriteString(fmt.Sprintf("- **严重程度**: %s\n", result.Severity))
			report.WriteString(fmt.Sprintf("- **源文件**: %s\n", result.SourceFile))
			report.WriteString(fmt.Sprintf("- **目标文件**: %s\n", result.SinkFile))
			report.WriteString(fmt.Sprintf("- **描述**: %s\n", result.Description))
			report.WriteString("- **调用链**:\n")
			for _, step := range result.CallChain {
				report.WriteString(fmt.Sprintf("  - %s\n", step))
			}
			report.WriteString("\n")
		}
	} else {
		report.WriteString("未发现明显的跨文件漏洞路径\n")
	}
	report.WriteString("\n")

	// 重要调用关系
	report.WriteString("## 关键调用关系\n\n")
	if len(c.Edges) > 0 {
		// 按文件分组显示
		fileCalls := make(map[string][]string)
		for _, edge := range c.Edges {
			key := fmt.Sprintf("%s -> %s",
				filepath.Base(edge.Caller.FilePath),
				filepath.Base(edge.Callee.FilePath))
			fileCalls[key] = append(fileCalls[key],
				fmt.Sprintf("%s.%s -> %s.%s (line %d)",
					edge.Caller.ClassName, edge.Caller.Name,
					edge.Callee.ClassName, edge.Callee.Name,
					edge.Line))
		}

		for key, calls := range fileCalls {
			report.WriteString(fmt.Sprintf("### %s\n", key))
			for _, call := range calls {
				report.WriteString(fmt.Sprintf("- %s\n", call))
			}
		}
	} else {
		report.WriteString("未发现调用关系\n")
	}

	return report.String()
}

// GraphNode 可视化节点
type GraphNode struct {
	ID       string  `json:"id"`       // 节点ID
	Label    string  `json:"label"`    // 显示标签
	File     string  `json:"file"`     // 文件名
	Line     int     `json:"line"`     // 行号
	Class    string  `json:"class"`    // 类名
	Type     string  `json:"type"`     // 节点类型: method, class, function
	IsPublic bool    `json:"isPublic"` // 是否公开
	NodeType string  `json:"nodeType"` // 节点特殊类型: entry(入口点), sink(危险点), normal(普通)
	X        float64 `json:"x"`        // X坐标（用于布局）
	Y        float64 `json:"y"`        // Y坐标（用于布局）
}

// GraphEdge 可视化边
type GraphEdge struct {
	ID       string `json:"id"`       // 边ID
	Source   string `json:"source"`   // 源节点ID
	Target   string `json:"target"`   // 目标节点ID
	CallType string `json:"callType"` // 调用类型: direct, callback, interface
	Line     int    `json:"line"`     // 调用行号
	Label    string `json:"label"`    // 显示标签
}

// CallGraphData 调用图数据（用于前端可视化）
type CallGraphData struct {
	Nodes     []*GraphNode `json:"nodes"`
	Edges     []*GraphEdge `json:"edges"`
	Stats     *GraphStats  `json:"stats"`
	EntryFunc string       `json:"entryFunc,omitempty"` // 入口函数
}

// GraphStats 统计信息
type GraphStats struct {
	TotalNodes   int            `json:"totalNodes"`   // 总节点数
	TotalEdges   int            `json:"totalEdges"`   // 总边数
	TotalFiles   int            `json:"totalFiles"`   // 文件数
	TotalClasses int            `json:"totalClasses"` // 类/模块数
	MaxDepth     int            `json:"maxDepth"`     // 最大调用深度
	EntryPoints  []*SourcePoint `json:"entryPoints"`  // 入口点
	SinkPoints   []*SinkPoint   `json:"sinkPoints"`   // 危险点
}

// GenerateVisualizationData 生成可视化数据（带节点类型区分）
func (c *CallGraphAnalyzer) GenerateVisualizationData(entryFunc string) *CallGraphData {
	data := &CallGraphData{
		Nodes: make([]*GraphNode, 0),
		Edges: make([]*GraphEdge, 0),
		Stats: &GraphStats{
			TotalNodes:  len(c.Nodes),
			TotalEdges:  len(c.Edges),
			TotalFiles:  len(c.Files),
			EntryPoints: c.FindEntryPoints(),
			SinkPoints:  c.FindSinkPoints(),
		},
		EntryFunc: entryFunc,
	}

	// 收集所有唯一的类
	classes := make(map[string]bool)
	for _, node := range c.Nodes {
		if node.ClassName != "" {
			classes[node.ClassName] = true
		}
	}
	data.Stats.TotalClasses = len(classes)

	// 预计算入口点和Sink点所在的文件
	entryPointFiles := make(map[string]bool)
	sinkPointFiles := make(map[string]bool)

	for _, ep := range c.FindEntryPoints() {
		if ep != nil && ep.Node != nil {
			entryPointFiles[filepath.Base(ep.Node.FilePath)] = true
		}
	}
	for _, sink := range c.FindSinkPoints() {
		if sink != nil && sink.Node != nil {
			sinkPointFiles[filepath.Base(sink.Node.FilePath)] = true
		}
	}

	// 生成节点（带类型区分）
	nodeIndex := make(map[string]*GraphNode)
	for key, node := range c.Nodes {
		fileName := filepath.Base(node.FilePath)

		// 判断节点类型
		nodeType := "normal"
		if entryPointFiles[fileName] {
			nodeType = "entry" // 入口点 - 用户输入相关的方法
		} else if sinkPointFiles[fileName] {
			nodeType = "sink" // Sink点 - 危险操作
		}

		graphNode := &GraphNode{
			ID:       key,
			Label:    key,
			File:     fileName,
			Line:     node.Line,
			Class:    node.ClassName,
			Type:     "method",
			IsPublic: node.IsPublic,
			NodeType: nodeType,
		}
		data.Nodes = append(data.Nodes, graphNode)
		nodeIndex[key] = graphNode
	}

	// 添加文件节点（作为类的容器）
	fileNodes := make(map[string]*GraphNode)
	for filePath := range c.Files {
		fileName := filepath.Base(filePath)
		fileNode := &GraphNode{
			ID:    "file_" + fileName,
			Label: fileName,
			File:  fileName,
			Type:  "file",
		}
		data.Nodes = append(data.Nodes, fileNode)
		fileNodes[filePath] = fileNode
	}

	// 构建节点ID集合，用于验证边的source和target是否存在
	validNodeIDs := make(map[string]bool)
	for key := range c.Nodes {
		validNodeIDs[key] = true
	}

	// 生成边 - 只添加source和target都存在于节点集合中的边
	edgeCount := 0
	for _, edge := range c.Edges {
		callerKey := fmt.Sprintf("%s.%s", edge.Caller.ClassName, edge.Caller.Name)
		if edge.Caller.ClassName == "" {
			callerKey = edge.Caller.Name
		}
		calleeKey := fmt.Sprintf("%s.%s", edge.Callee.ClassName, edge.Callee.Name)
		if edge.Callee.ClassName == "" {
			calleeKey = edge.Callee.Name
		}

		// 验证source和target节点是否存在于节点集合中
		// 首先检查完整键是否存在
		sourceExists := validNodeIDs[callerKey]
		targetExists := validNodeIDs[calleeKey]

		// 如果不存在，尝试使用纯方法名匹配
		if !sourceExists {
			sourceExists = validNodeIDs[edge.Caller.Name]
		}
		if !targetExists {
			targetExists = validNodeIDs[edge.Callee.Name]
		}

		// 只有当source和target至少有一个存在于节点集合中时才添加边
		if sourceExists && targetExists {
			// 确定最终的source和target键
			sourceKey := callerKey
			targetKey := calleeKey
			if !validNodeIDs[callerKey] {
				sourceKey = edge.Caller.Name
			}
			if !validNodeIDs[calleeKey] {
				targetKey = edge.Callee.Name
			}

			graphEdge := &GraphEdge{
				ID:       fmt.Sprintf("edge_%d", edgeCount),
				Source:   sourceKey,
				Target:   targetKey,
				CallType: edge.CallType,
				Line:     edge.Line,
				Label:    edge.Callee.Name,
			}
			data.Edges = append(data.Edges, graphEdge)
			edgeCount++
		}
	}

	// 计算最大调用深度
	data.Stats.MaxDepth = c.calculateMaxDepth(entryFunc)

	// 生成简单的树形布局
	c.generateLayout(data, entryFunc)

	return data
}

// calculateMaxDepth 计算最大调用深度
func (c *CallGraphAnalyzer) calculateMaxDepth(entryFunc string) int {
	if entryFunc == "" {
		return 0
	}

	maxDepth := 0
	visited := make(map[string]bool)

	var dfs func(funcName string, depth int)
	dfs = func(funcName string, depth int) {
		if visited[funcName] || depth > 20 { // 防止无限递归
			return
		}
		visited[funcName] = true
		if depth > maxDepth {
			maxDepth = depth
		}

		// 查找该函数调用的其他函数
		for _, edge := range c.Edges {
			callerKey := fmt.Sprintf("%s.%s", edge.Caller.ClassName, edge.Caller.Name)
			if edge.Caller.ClassName == "" {
				callerKey = edge.Caller.Name
			}
			if callerKey == funcName {
				calleeKey := fmt.Sprintf("%s.%s", edge.Callee.ClassName, edge.Callee.Name)
				if edge.Callee.ClassName == "" {
					calleeKey = edge.Callee.Name
				}
				dfs(calleeKey, depth+1)
			}
		}
	}

	dfs(entryFunc, 0)
	return maxDepth
}

// generateLayout 生成简单的树形布局
func (c *CallGraphAnalyzer) generateLayout(data *CallGraphData, entryFunc string) {
	if len(data.Nodes) == 0 {
		return
	}

	// 按文件分组
	fileGroups := make(map[string][]*GraphNode)
	for _, node := range data.Nodes {
		if node.Type == "file" {
			continue
		}
		fileGroups[node.File] = append(fileGroups[node.File], node)
	}

	// 简单的层级布局
	level := make(map[string]int)
	visited := make(map[string]bool)

	// 从入口函数开始计算层级
	var assignLevel func(funcName string, l int)
	assignLevel = func(funcName string, l int) {
		if visited[funcName] || l > 20 {
			return
		}
		visited[funcName] = true
		level[funcName] = l

		// 查找该函数调用的函数（正向）
		for _, edge := range data.Edges {
			if edge.Source == funcName {
				assignLevel(edge.Target, l+1)
			}
		}
		// 查找调用该函数的函数（反向）
		for _, edge := range data.Edges {
			if edge.Target == funcName {
				assignLevel(edge.Source, l-1)
			}
		}
	}

	// 尝试从入口函数开始
	if entryFunc != "" {
		assignLevel(entryFunc, 0)
	}

	// 为所有节点分配层级（使用BFS）
	queue := make([]string, 0)
	for _, node := range data.Nodes {
		if !visited[node.ID] {
			queue = append(queue, node.ID)
		}
	}

	// 为没有层级的节点分配默认层级
	currentLevel := 1
	for len(queue) > 0 && currentLevel <= 10 {
		nextQueue := make([]string, 0)
		for _, funcName := range queue {
			if visited[funcName] {
				continue
			}
			level[funcName] = currentLevel
			visited[funcName] = true

			// 查找该函数调用的函数
			for _, edge := range data.Edges {
				if edge.Source == funcName && !visited[edge.Target] {
					nextQueue = append(nextQueue, edge.Target)
				}
			}
		}
		queue = nextQueue
		currentLevel++
	}

	// 为仍然没有层级的节点分配默认层级
	for _, node := range data.Nodes {
		if _, ok := level[node.ID]; !ok {
			level[node.ID] = 0
		}
	}

	// 根据层级和文件分组计算坐标
	xSpacing := 250.0
	ySpacing := 80.0

	// 按文件分组计算x坐标
	fileX := make(map[string]float64)
	x := 100.0
	for file := range fileGroups {
		fileX[file] = x
		x += xSpacing * 2
	}

	// 计算y坐标 - 使用更好的布局算法
	levelNodes := make(map[int][]*GraphNode)
	for _, node := range data.Nodes {
		l := level[node.ID]
		levelNodes[l] = append(levelNodes[l], node)
	}

	// 从中心向两边展开布局
	levels := make([]int, 0)
	for l := range levelNodes {
		levels = append(levels, l)
	}
	sort.Ints(levels)

	centerX := 500.0 // 中心X坐标

	for idx, l := range levels {
		nodes := levelNodes[l]
		// 计算这一层的X坐标（从中心向两侧展开）
		levelX := centerX + float64(idx-len(levels)/2)*xSpacing

		for i, node := range nodes {
			// Y坐标：从上到下排列
			node.X = levelX
			node.Y = float64(i)*ySpacing + 50

			// 如果有文件信息，使用文件的X坐标
			if node.File != "" && fileX[node.File] > 0 {
				node.X = fileX[node.File]
			}
		}
	}

	// 为文件节点设置位置
	fileIndex := 0
	for _, fileNodes := range fileGroups {
		if len(fileGroups) > 1 && len(fileNodes) > 0 {
			fileNodes[0].X = float64(fileIndex) * xSpacing * 3
			fileNodes[0].Y = -50
		}
		fileIndex++
	}
}

// GetCallGraphForFunction 获取特定函数的调用图
func (c *CallGraphAnalyzer) GetCallGraphForFunction(funcName string, depth int) *CallGraphData {
	if depth <= 0 {
		depth = 3 // 默认深度
	}
	if depth > 5 {
		depth = 5 // 最大深度限制
	}

	data := &CallGraphData{
		Nodes:     make([]*GraphNode, 0),
		Edges:     make([]*GraphEdge, 0),
		Stats:     &GraphStats{},
		EntryFunc: funcName,
	}

	visited := make(map[string]bool)
	edgeSet := make(map[string]bool)

	// BFS 获取指定深度内的调用关系
	queue := []string{funcName}
	currentLevel := 0

	for currentLevel < depth && len(queue) > 0 {
		nextQueue := []string{}

		for _, currentFunc := range queue {
			if visited[currentFunc] {
				continue
			}
			visited[currentFunc] = true

			// 添加节点
			if node, exists := c.Nodes[currentFunc]; exists {
				graphNode := &GraphNode{
					ID:       currentFunc,
					Label:    currentFunc,
					File:     filepath.Base(node.FilePath),
					Line:     node.Line,
					Class:    node.ClassName,
					Type:     "method",
					IsPublic: node.IsPublic,
				}
				data.Nodes = append(data.Nodes, graphNode)
			}

			// 查找调用关系
			for _, edge := range c.Edges {
				callerKey := fmt.Sprintf("%s.%s", edge.Caller.ClassName, edge.Caller.Name)
				if edge.Caller.ClassName == "" {
					callerKey = edge.Caller.Name
				}
				calleeKey := fmt.Sprintf("%s.%s", edge.Callee.ClassName, edge.Callee.Name)
				if edge.Callee.ClassName == "" {
					calleeKey = edge.Callee.Name
				}

				if callerKey == currentFunc {
					edgeKey := callerKey + "->" + calleeKey
					if !edgeSet[edgeKey] {
						edgeSet[edgeKey] = true

						// 添加被调用节点
						if calleeNode, exists := c.Nodes[calleeKey]; exists {
							if !visited[calleeKey] {
								graphNode := &GraphNode{
									ID:       calleeKey,
									Label:    calleeKey,
									File:     filepath.Base(calleeNode.FilePath),
									Line:     calleeNode.Line,
									Class:    calleeNode.ClassName,
									Type:     "method",
									IsPublic: calleeNode.IsPublic,
								}
								data.Nodes = append(data.Nodes, graphNode)
							}
						}

						// 添加边
						graphEdge := &GraphEdge{
							ID:       edgeKey,
							Source:   callerKey,
							Target:   calleeKey,
							CallType: edge.CallType,
							Line:     edge.Line,
							Label:    filepath.Base(edge.Callee.FilePath),
						}
						data.Edges = append(data.Edges, graphEdge)

						nextQueue = append(nextQueue, calleeKey)
					}
				}
			}
		}

		queue = nextQueue
		currentLevel++
	}

	// 统计信息
	data.Stats.TotalNodes = len(data.Nodes)
	data.Stats.TotalEdges = len(data.Edges)
	data.Stats.MaxDepth = depth

	// 生成布局
	c.generateLayout(data, funcName)

	return data
}

// GetCallers 获取调用指定函数的所有函数（反向调用链）
func (c *CallGraphAnalyzer) GetCallers(funcName string) []*GraphNode {
	var callers []*GraphNode
	visited := make(map[string]bool)

	// 尝试多种匹配方式
	for _, edge := range c.Edges {
		// 被调用的方法key
		calleeKey := fmt.Sprintf("%s.%s", edge.Callee.ClassName, edge.Callee.Name)
		if edge.Callee.ClassName == "" {
			calleeKey = edge.Callee.Name
		}

		// 纯方法名匹配（不带类名）
		calleeSimpleName := edge.Callee.Name

		// 检查是否匹配目标函数
		matched := false
		if calleeKey == funcName {
			matched = true
		} else if calleeSimpleName == funcName {
			matched = true
		} else if strings.HasSuffix(funcName, "."+calleeSimpleName) &&
			(edge.Callee.ClassName == "" || strings.HasSuffix(funcName, "."+edge.Callee.ClassName+"."+calleeSimpleName)) {
			matched = true
		}

		if matched {
			// 使用完整的callerKey作为唯一标识
			callerKey := fmt.Sprintf("%s.%s", edge.Caller.ClassName, edge.Caller.Name)
			if edge.Caller.ClassName == "" {
				callerKey = edge.Caller.Name
			}

			if !visited[callerKey] {
				visited[callerKey] = true
				callers = append(callers, &GraphNode{
					ID:       callerKey,
					Label:    callerKey,
					File:     filepath.Base(edge.Caller.FilePath),
					Line:     edge.Caller.Line,
					Class:    edge.Caller.ClassName,
					Type:     "method",
					IsPublic: edge.Caller.IsPublic,
				})
			}
		}
	}

	return callers
}

// GetCallees 获取指定函数调用的所有函数（正向调用链）
func (c *CallGraphAnalyzer) GetCallees(funcName string) []*GraphNode {
	var callees []*GraphNode
	visited := make(map[string]bool)

	// 尝试多种匹配方式
	for _, edge := range c.Edges {
		// 调用者的方法key
		callerKey := fmt.Sprintf("%s.%s", edge.Caller.ClassName, edge.Caller.Name)
		if edge.Caller.ClassName == "" {
			callerKey = edge.Caller.Name
		}

		// 纯方法名匹配（不带类名）
		callerSimpleName := edge.Caller.Name

		// 检查是否匹配目标函数
		matched := false
		if callerKey == funcName {
			matched = true
		} else if callerSimpleName == funcName {
			matched = true
		} else if strings.HasSuffix(funcName, "."+callerSimpleName) &&
			(edge.Caller.ClassName == "" || strings.HasSuffix(funcName, "."+edge.Caller.ClassName+"."+callerSimpleName)) {
			matched = true
		}

		if matched {
			// 使用完整的calleeKey作为唯一标识
			calleeKey := fmt.Sprintf("%s.%s", edge.Callee.ClassName, edge.Callee.Name)
			if edge.Callee.ClassName == "" {
				calleeKey = edge.Callee.Name
			}

			if !visited[calleeKey] {
				visited[calleeKey] = true
				callees = append(callees, &GraphNode{
					ID:       calleeKey,
					Label:    calleeKey,
					File:     filepath.Base(edge.Callee.FilePath),
					Line:     edge.Callee.Line,
					Class:    edge.Callee.ClassName,
					Type:     "method",
					IsPublic: edge.Callee.IsPublic,
				})
			}
		}
	}

	return callees
}

// GenerateTextCallGraph 生成文本格式的调用图（类似IDA）
func (c *CallGraphAnalyzer) GenerateTextCallGraph(entryFunc string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("调用图分析: %s\n", filepath.Base(c.SourcePath)))
	result.WriteString(fmt.Sprintf("语言: %s\n", c.Language))
	result.WriteString(fmt.Sprintf("方法数: %d, 调用关系: %d\n\n", len(c.Nodes), len(c.Edges)))

	// 按文件分组显示
	fileCalls := make(map[string][]string)
	for _, edge := range c.Edges {
		callerKey := fmt.Sprintf("%s.%s", edge.Caller.ClassName, edge.Caller.Name)
		if edge.Caller.ClassName == "" {
			callerKey = edge.Caller.Name
		}
		calleeKey := fmt.Sprintf("%s.%s", edge.Callee.ClassName, edge.Callee.Name)
		if edge.Callee.ClassName == "" {
			calleeKey = edge.Callee.Name
		}

		callerFile := filepath.Base(edge.Caller.FilePath)
		callStr := fmt.Sprintf("  --> %s (line %d)", calleeKey, edge.Line)
		fileCalls[callerFile] = append(fileCalls[callerFile], callerKey+callStr)
	}

	// 输出
	for file, calls := range fileCalls {
		result.WriteString(fmt.Sprintf("\n%s\n", file))
		result.WriteString(strings.Repeat("-", len(file)) + "\n")

		// 按调用者分组
		callerGroups := make(map[string][]string)
		for _, call := range calls {
			parts := strings.SplitN(call, " --> ", 2)
			if len(parts) == 2 {
				callerGroups[parts[0]] = append(callerGroups[parts[0]], parts[1])
			}
		}

		for caller, callees := range callerGroups {
			result.WriteString(fmt.Sprintf("  %s\n", caller))
			for _, callee := range callees {
				result.WriteString(fmt.Sprintf("    |\n    +-- %s\n", callee))
			}
		}
	}

	// 显示入口点和Sink点
	result.WriteString("\n\n入口点:\n")
	entryPoints := c.FindEntryPoints()
	if len(entryPoints) > 10 {
		entryPoints = entryPoints[:10]
	}
	for _, ep := range entryPoints {
		if ep != nil && ep.Node != nil {
			result.WriteString(fmt.Sprintf("  - 文件: %s, 类型: %s\n", filepath.Base(ep.Node.FilePath), ep.Type))
		}
	}

	result.WriteString("\n危险Sink点:\n")
	sinkPoints := c.FindSinkPoints()
	if len(sinkPoints) > 10 {
		sinkPoints = sinkPoints[:10]
	}
	for _, sink := range sinkPoints {
		if sink != nil && sink.Node != nil {
			result.WriteString(fmt.Sprintf("  - 文件: %s, 漏洞类型: %s\n", filepath.Base(sink.Node.FilePath), sink.VulnType))
		}
	}

	return result.String()
}
