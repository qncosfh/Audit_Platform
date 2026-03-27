package model

import (
	"time"

	"gorm.io/gorm"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusPaused    TaskStatus = "paused"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Vulnerability 漏洞结构
type Vulnerability struct {
	gorm.Model
	TaskID        uint   `gorm:"not null;index:idx_task_severity;index:idx_task_type" json:"taskId"`
	Type          string `gorm:"type:varchar(100);index:idx_task_type" json:"type"`        // 漏洞类型
	File          string `gorm:"type:varchar(500)" json:"file"`                            // 文件路径
	Line          int    `json:"line"`                                                     // 行号
	Severity      string `gorm:"type:varchar(20);index:idx_task_severity" json:"severity"` // Critical/High/Medium/Low
	Description   string `gorm:"type:text" json:"description"`                             // 描述
	Analysis      string `gorm:"type:text" json:"analysis"`                                // 分析过程
	FixSuggestion string `gorm:"type:text" json:"fixSuggestion"`                           // 修复建议
	POC           string `gorm:"type:text" json:"poc"`                                     // PoC代码
	CWE           string `gorm:"type:varchar(50)" json:"cwe"`                              // CWE编号
	CVE           string `gorm:"type:varchar(50)" json:"cve"`                              // CVE编号（如适用）
	Confidence    string `gorm:"type:varchar(20)" json:"confidence"`                       // 置信度
	AttackVector  string `gorm:"type:varchar(50)" json:"attackVector"`                     // 攻击向量
	Impact        string `gorm:"type:text" json:"impact"`                                  // 影响描述
	Refs          string `gorm:"type:text" json:"refs"`                                    // 参考链接
	CodeSnippet   string `gorm:"type:text" json:"codeSnippet"`                             // 漏洞代码片段
}

// VulnerabilityChain 漏洞利用链
type VulnerabilityChain struct {
	gorm.Model
	TaskID             uint    `gorm:"not null;index" json:"taskId"`
	Name               string  `gorm:"type:varchar(200)" json:"name"`              // 利用链名称
	Description        string  `gorm:"type:text" json:"description"`               // 描述
	Severity           string  `gorm:"type:varchar(20)" json:"severity"`           // 严重程度
	TotalScore         float64 `json:"totalScore"`                                 // 总评分(CVSS)
	AttackComplexity   string  `gorm:"type:varchar(50)" json:"attackComplexity"`   // 攻击复杂度
	PrivilegesRequired string  `gorm:"type:varchar(50)" json:"privilegesRequired"` // 需要权限
	UserInteraction    string  `gorm:"type:varchar(50)" json:"userInteraction"`    // 是否需要用户交互
	Scope              string  `gorm:"type:varchar(50)" json:"scope"`              // 影响范围
	Confidentiality    string  `gorm:"type:varchar(20)" json:"confidentiality"`    // 机密性影响
	Integrity          string  `gorm:"type:varchar(20)" json:"integrity"`          // 完整性影响
	Availability       string  `gorm:"type:varchar(20)" json:"availability"`       // 可用性影响
	Steps              string  `gorm:"type:text" json:"steps"`                     // 利用步骤
	Chain              string  `gorm:"type:text" json:"chain"`                     // 利用链JSON
}

// ProjectStats 项目统计
type ProjectStats struct {
	gorm.Model
	TaskID               uint   `gorm:"not null;index" json:"taskId"`
	TotalFiles           int    `json:"totalFiles"`                            // 总文件数
	CodeLines            int    `json:"codeLines"`                             // 代码行数
	TotalClasses         int    `json:"totalClasses"`                          // 类/模块数
	TotalFunctions       int    `json:"totalFunctions"`                        // 函数/方法数
	CriticalVulns        int    `json:"criticalVulns"`                         // 严重漏洞数
	HighVulns            int    `json:"highVulns"`                             // 高危漏洞数
	MediumVulns          int    `json:"mediumVulns"`                           // 中危漏洞数
	LowVulns             int    `json:"lowVulns"`                              // 低危漏洞数
	InfoVulns            int    `json:"infoVulns"`                             // 信息性问题
	SecurityScore        int    `json:"securityScore"`                         // 安全评分(0-100)
	Language             string `gorm:"type:varchar(50)" json:"language"`      // 编程语言
	Framework            string `gorm:"type:varchar(100)" json:"framework"`    // 框架
	Dependencies         string `gorm:"type:text" json:"dependencies"`         // 依赖项
	FileTypeDistribution string `gorm:"type:text" json:"fileTypeDistribution"` // 文件类型分布
	VulnTypeDistribution string `gorm:"type:text" json:"vulnTypeDistribution"` // 漏洞类型分布
}

// Task 任务结构
type Task struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	UserID        uint           `gorm:"not null;index" json:"userId"`
	Name          string         `gorm:"not null" json:"name"`
	Description   string         `json:"description"`
	CodeSourceID  uint           `gorm:"not null" json:"codeSourceId"`
	ModelConfigID uint           `gorm:"not null" json:"modelConfigId"`
	Prompt        string         `gorm:"type:text" json:"prompt"`
	Status        TaskStatus     `gorm:"type:varchar(20);default:'pending';index" json:"status"`
	Progress      int            `gorm:"default:0" json:"progress"`
	Result        string         `gorm:"type:text" json:"result"`
	ReportPath    string         `json:"reportPath"`
	CodeSource    CodeSource     `gorm:"foreignKey:CodeSourceID" json:"codeSource"`
	ModelConfig   ModelConfig    `gorm:"foreignKey:ModelConfigID" json:"modelConfig"`

	// 基础统计字段
	ScannedFiles       int    `gorm:"default:0" json:"scannedFiles"`            // 扫描文件数
	VulnerabilityCount int    `gorm:"default:0" json:"vulnerabilityCount"`      // 漏洞数量
	Duration           int    `gorm:"default:0" json:"duration"`                // 执行时长(秒)
	StartTime          *int64 `json:"startTime"`                                // 开始时间戳
	EndTime            *int64 `json:"endTime"`                                  // 结束时间戳
	CurrentFile        string `gorm:"type:varchar(500)" json:"currentFile"`     // 当前分析的文件
	Log                string `gorm:"type:text" json:"log"`                     // 审计日志
	DetectedLanguage   string `gorm:"type:varchar(50)" json:"detectedLanguage"` // 检测到的语言
	AILog              string `gorm:"type:text" json:"aiLog"`                   // 大模型交互日志

	// 新增：高级分析字段
	CrossFileAnalysis  string `gorm:"type:text" json:"crossFileAnalysis"`  // 跨文件分析结果
	DataFlowAnalysis   string `gorm:"type:text" json:"dataFlowAnalysis"`   // 数据流分析结果
	CallChainAnalysis  string `gorm:"type:text" json:"callChainAnalysis"`  // 调用链分析结果
	DependencyAnalysis string `gorm:"type:text" json:"dependencyAnalysis"` // 依赖分析结果
	ExploitChain       string `gorm:"type:text" json:"exploitChain"`       // 利用链分析结果

	// 安全评分
	SecurityScore int    `gorm:"default:100" json:"securityScore"`  // 安全评分(0-100)
	RiskLevel     string `gorm:"type:varchar(20)" json:"riskLevel"` // 风险等级

	// 代码统计
	CodeLines      int `gorm:"default:0" json:"codeLines"`      // 代码行数
	TotalClasses   int `gorm:"default:0" json:"totalClasses"`   // 类/模块数
	TotalFunctions int `gorm:"default:0" json:"totalFunctions"` // 函数/方法数

	// 漏洞统计（冗余字段，用于快速展示）
	CriticalVulns int `gorm:"default:0" json:"criticalVulns"`
	HighVulns     int `gorm:"default:0" json:"highVulns"`
	MediumVulns   int `gorm:"default:0" json:"mediumVulns"`
	LowVulns      int `gorm:"default:0" json:"lowVulns"`

	// 源代码路径 - 用于后续API调用（如调用图、代码片段）
	SourcePath string `gorm:"type:varchar(500)" json:"sourcePath"`

	// 关联关系
	Vulnerabilities []Vulnerability      `gorm:"foreignKey:TaskID" json:"vulnerabilities"`
	Chains          []VulnerabilityChain `gorm:"foreignKey:TaskID" json:"chains"`
	Stats           *ProjectStats        `gorm:"foreignKey:TaskID" json:"stats"`
}

// BeforeCreate 创建任务前设置默认值
func (t *Task) BeforeCreate(tx *gorm.DB) error {
	// 可以在这里添加额外的创建前逻辑
	return nil
}

func (Task) TableName() string {
	return "tasks"
}

func (Vulnerability) TableName() string {
	return "vulnerabilities"
}

func (VulnerabilityChain) TableName() string {
	return "vulnerability_chains"
}

func (ProjectStats) TableName() string {
	return "project_stats"
}
