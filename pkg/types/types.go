package types

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Content string `json:"content"`
	Delta   string `json:"delta,omitempty"`
	Done    bool   `json:"done,omitempty"`
}

// TodoItem represents a single todo task
type TodoItem struct {
	ID          string     `json:"id"`
	Content     string     `json:"content"`
	Status      string     `json:"status"` // pending, in_progress, completed
	Order       int        `json:"order"`  // execution order (1, 2, 3...)
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// Config represents unified application configuration
type Config struct {
	// Core Application Settings
	DefaultLanguage  string   `yaml:"defaultLanguage" json:"defaultLanguage" mapstructure:"defaultLanguage"`
	OutputFormat     string   `yaml:"outputFormat" json:"outputFormat" mapstructure:"outputFormat"`
	AnalysisDepth    int      `yaml:"analysisDepth" json:"analysisDepth" mapstructure:"analysisDepth"`
	BackupOnRefactor bool     `yaml:"backupOnRefactor" json:"backupOnRefactor" mapstructure:"backupOnRefactor"`
	ExcludePatterns  []string `yaml:"excludePatterns" json:"excludePatterns" mapstructure:"excludePatterns"`

	// API Configuration
	APIKey  string `yaml:"api_key" json:"api_key" mapstructure:"api_key"`
	BaseURL string `yaml:"base_url" json:"base_url" mapstructure:"base_url"`
	Model   string `yaml:"model" json:"model" mapstructure:"model"`

	// Tavily API Configuration
	TavilyAPIKey string `yaml:"tavily_api_key" json:"tavily_api_key" mapstructure:"tavily_api_key"`

	// Agent Configuration (previously AgentConfig)
	AllowedTools   []string `yaml:"allowedTools" json:"allowedTools" mapstructure:"allowedTools"`
	MaxTokens      int      `yaml:"maxTokens" json:"maxTokens" mapstructure:"maxTokens"`
	Temperature    float64  `yaml:"temperature" json:"temperature" mapstructure:"temperature"`
	StreamResponse bool     `yaml:"streamResponse" json:"streamResponse" mapstructure:"streamResponse"`
	SessionTimeout int      `yaml:"sessionTimeout" json:"sessionTimeout" mapstructure:"sessionTimeout"` // minutes

	// CLI Configuration (previously CLIConfig)
	Interactive bool   `yaml:"interactive" json:"interactive" mapstructure:"interactive"`
	SessionID   string `yaml:"sessionId" json:"sessionId" mapstructure:"sessionId"`
	ConfigFile  string `yaml:"configFile" json:"configFile" mapstructure:"configFile"`

	// Session Management
	SessionCleanupInterval int `yaml:"sessionCleanupInterval" json:"sessionCleanupInterval" mapstructure:"sessionCleanupInterval"` // hours
	MaxSessionAge          int `yaml:"maxSessionAge" json:"maxSessionAge" mapstructure:"maxSessionAge"`                            // days
	MaxMessagesPerSession  int `yaml:"maxMessagesPerSession" json:"maxMessagesPerSession" mapstructure:"maxMessagesPerSession"`

	// Security Settings
	EnableSandbox        bool     `yaml:"enableSandbox" json:"enableSandbox" mapstructure:"enableSandbox"`
	RestrictedTools      []string `yaml:"restrictedTools" json:"restrictedTools" mapstructure:"restrictedTools"`
	MaxConcurrentTools   int      `yaml:"maxConcurrentTools" json:"maxConcurrentTools" mapstructure:"maxConcurrentTools"`
	ToolExecutionTimeout int      `yaml:"toolExecutionTimeout" json:"toolExecutionTimeout" mapstructure:"toolExecutionTimeout"` // seconds

	// MCP Configuration
	MCPEnabled           bool     `yaml:"mcpEnabled" json:"mcpEnabled" mapstructure:"mcpEnabled"`
	MCPServers           []string `yaml:"mcpServers" json:"mcpServers" mapstructure:"mcpServers"`
	MCPConnectionTimeout int      `yaml:"mcpConnectionTimeout" json:"mcpConnectionTimeout" mapstructure:"mcpConnectionTimeout"` // seconds
	MCPMaxConnections    int      `yaml:"mcpMaxConnections" json:"mcpMaxConnections" mapstructure:"mcpMaxConnections"`

	// ReAct Agent Configuration (ReAct is the core execution mode)
	ReActMaxIterations   int  `yaml:"reactMaxIterations" json:"reactMaxIterations" mapstructure:"reactMaxIterations"`
	ReActThinkingEnabled bool `yaml:"reactThinkingEnabled" json:"reactThinkingEnabled" mapstructure:"reactThinkingEnabled"`

	// Todo Management
	Todos []TodoItem `yaml:"todos" json:"todos" mapstructure:"todos"`

	CustomSettings map[string]string `yaml:"customSettings" json:"customSettings" mapstructure:"customSettings"`
	LastUpdated    time.Time         `yaml:"lastUpdated" json:"lastUpdated" mapstructure:"lastUpdated"`
}

// SupportedLanguages contains the list of supported programming languages
var SupportedLanguages = map[string]string{
	".go":    "go",
	".js":    "javascript",
	".ts":    "typescript",
	".jsx":   "jsx",
	".tsx":   "tsx",
	".py":    "python",
	".java":  "java",
	".cpp":   "cpp",
	".c":     "c",
	".cs":    "csharp",
	".php":   "php",
	".rb":    "ruby",
	".rs":    "rust",
	".kt":    "kotlin",
	".swift": "swift",
}

// FunctionCall represents a standard OpenAI-style function call
type FunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCall represents a tool call with standard format
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // Always "function"
	Function FunctionCall `json:"function"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ID      string      `json:"id"`
	Success bool        `json:"success"`
	Content string      `json:"content,omitempty"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ChatMessage represents a chat message in the conversation
type ChatMessage struct {
	Role       string     `json:"role"` // system, user, assistant, tool
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolDefinition represents a tool's schema definition
type ToolDefinition struct {
	Type     string             `json:"type"` // Always "function"
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition represents a function's schema
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ProjectSummary - 项目信息汇总
type ProjectSummary struct {
	Info    string `json:"info"`    // 项目信息汇总 (类型、主要文件、构建工具等)
	Context string `json:"context"` // 系统环境汇总 (OS、工具版本、用户等)
}

// DirectoryContextInfo - 目录上下文信息
type DirectoryContextInfo struct {
	Path         string     `json:"path"`          // 完整路径
	FileCount    int        `json:"file_count"`    // 文件数量
	DirCount     int        `json:"dir_count"`     // 目录数量
	TotalSize    int64      `json:"total_size"`    // 总大小
	LastModified time.Time  `json:"last_modified"` // 最后修改时间
	TopFiles     []FileInfo `json:"top_files"`     // 主要文件列表
	ProjectType  string     `json:"project_type"`  // 项目类型（Go、Python等）
	Description  string     `json:"description"`   // 目录简要描述
}

// FileInfo - 文件信息
type FileInfo struct {
	Name     string    `json:"name"`     // 文件名
	Path     string    `json:"path"`     // 相对路径
	Size     int64     `json:"size"`     // 文件大小
	Modified time.Time `json:"modified"` // 修改时间
	Type     string    `json:"type"`     // 文件类型
	IsDir    bool      `json:"is_dir"`   // 是否为目录
}

// ReactTaskContext - ReAct任务上下文
type ReactTaskContext struct {
	TaskID           string                 `json:"task_id"`           // 任务ID
	Goal             string                 `json:"goal"`              // 任务目标
	History          []ReactExecutionStep   `json:"history"`           // 执行历史
	Memory           map[string]interface{} `json:"memory"`            // 任务内存
	StartTime        time.Time              `json:"start_time"`        // 开始时间
	LastUpdate       time.Time              `json:"last_update"`       // 最后更新时间
	TokensUsed       int                    `json:"tokens_used"`       // 已使用token数
	PromptTokens     int                    `json:"prompt_tokens"`     // 累计输入token数
	CompletionTokens int                    `json:"completion_tokens"` // 累计输出token数
	Metadata         map[string]interface{} `json:"metadata"`          // 元数据
	// Directory context information
	WorkingDir    string                `json:"working_dir"`              // 对话发起时的工作目录
	DirectoryInfo *DirectoryContextInfo `json:"directory_info,omitempty"` // 目录信息

	// Project and environment information
	ProjectSummary *ProjectSummary `json:"project_summary,omitempty"` // 项目和系统环境汇总
}

// ReactExecutionStep - ReAct执行步骤
type ReactExecutionStep struct {
	Number      int                `json:"number"`              // 步骤编号
	Thought     string             `json:"thought"`             // 思考内容
	Analysis    string             `json:"analysis"`            // 分析结果
	Action      string             `json:"action"`              // 执行动作
	ToolCall    []*ReactToolCall   `json:"tool_call,omitempty"` // 工具调用
	Result      []*ReactToolResult `json:"result,omitempty"`    // 执行结果
	Observation string             `json:"observation"`         // 观察结果
	Confidence  float64            `json:"confidence"`          // 置信度 0.0-1.0
	Duration    time.Duration      `json:"duration"`            // 执行时长
	Timestamp   time.Time          `json:"timestamp"`           // 时间戳
	Error       string             `json:"error,omitempty"`     // 错误信息
	TokensUsed  int                `json:"tokens_used"`         // 本步骤使用的token数
}

// ReactTaskResult - ReAct任务执行结果
type ReactTaskResult struct {
	Success          bool                   `json:"success"`            // 是否成功
	Answer           string                 `json:"answer"`             // 答案内容
	Confidence       float64                `json:"confidence"`         // 整体置信度
	Steps            []ReactExecutionStep   `json:"steps"`              // 执行步骤
	Duration         time.Duration          `json:"duration"`           // 总耗时
	TokensUsed       int                    `json:"tokens_used"`        // 总token使用量
	PromptTokens     int                    `json:"prompt_tokens"`      // 输入token数
	CompletionTokens int                    `json:"completion_tokens"`  // 输出token数
	Metadata         map[string]interface{} `json:"metadata,omitempty"` // 额外元数据
	Error            string                 `json:"error,omitempty"`    // 错误信息
}

// ReactToolCall - ReAct工具调用
type ReactToolCall struct {
	Name      string                 `json:"name"`      // 工具名称
	Arguments map[string]interface{} `json:"arguments"` // 调用参数
	CallID    string                 `json:"id"`        // 调用ID
}

// ReactToolResult - ReAct工具执行结果
type ReactToolResult struct {
	Success   bool                   `json:"success"`              // 是否成功
	Content   string                 `json:"content"`              // 结果内容
	Data      map[string]interface{} `json:"data,omitempty"`       // 结构化数据
	Error     string                 `json:"error,omitempty"`      // 错误信息
	Duration  time.Duration          `json:"duration"`             // 执行时长
	Metadata  map[string]interface{} `json:"metadata,omitempty"`   // 元数据
	ToolName  string                 `json:"tool_name,omitempty"`  // 工具名称
	ToolArgs  map[string]interface{} `json:"tool_args,omitempty"`  // 工具参数
	ToolCalls []*ReactToolCall       `json:"tool_calls,omitempty"` // 多个工具调用（并行执行时）
	CallID    string                 `json:"call_id,omitempty"`    // 调用ID
}

// ReactConfig - ReAct代理配置
type ReactConfig struct {
	MaxIterations       int           `json:"max_iterations"`       // 最大迭代次数，默认5
	ConfidenceThreshold float64       `json:"confidence_threshold"` // 置信度阈值，默认0.7
	TaskTimeout         time.Duration `json:"task_timeout"`         // 任务超时时间
	EnableAsync         bool          `json:"enable_async"`         // 启用异步执行
	ContextCompression  bool          `json:"context_compression"`  // 启用上下文压缩
	StreamingMode       bool          `json:"streaming_mode"`       // 流式模式
	LogLevel            string        `json:"log_level"`            // 日志级别
	Temperature         float64       `json:"temperature"`          // LLM温度参数
	MaxTokens           int           `json:"max_tokens"`           // 最大token数
}

// ReactConfig默认配置常量
const (
	ReactDefaultMaxIterations       = 5
	ReactDefaultConfidenceThreshold = 0.7
	ReactDefaultTaskTimeout         = 5 * time.Minute
	ReactDefaultMaxTokens           = 2000
	ReactDefaultTemperature         = 0.7
	ReactDefaultLogLevel            = "info"
	ReactDefaultMaxContextSize      = 10
	ReactDefaultCompressionRatio    = 0.6
	ReactDefaultMemorySlots         = 5
)

// NewReactConfig 创建默认的ReAct配置
func NewReactConfig() *ReactConfig {
	return &ReactConfig{
		MaxIterations:       ReactDefaultMaxIterations,
		ConfidenceThreshold: ReactDefaultConfidenceThreshold,
		TaskTimeout:         ReactDefaultTaskTimeout,
		EnableAsync:         true,
		ContextCompression:  true,
		StreamingMode:       true,
		LogLevel:            ReactDefaultLogLevel,
		Temperature:         ReactDefaultTemperature,
		MaxTokens:           ReactDefaultMaxTokens,
	}
}

// NewReactTaskContext 创建新的ReAct任务上下文
func NewReactTaskContext(taskID, goal string) *ReactTaskContext {
	workingDir, _ := getCurrentWorkingDir()
	directoryInfo := gatherDirectoryInfo(workingDir)
	projectSummary := gatherProjectSummary(workingDir, directoryInfo)

	return &ReactTaskContext{
		TaskID:         taskID,
		Goal:           goal,
		History:        make([]ReactExecutionStep, 0),
		Memory:         make(map[string]interface{}),
		StartTime:      time.Now(),
		LastUpdate:     time.Now(),
		TokensUsed:     0,
		Metadata:       make(map[string]interface{}),
		WorkingDir:     workingDir,
		DirectoryInfo:  directoryInfo,
		ProjectSummary: projectSummary,
	}
}

// getCurrentWorkingDir 获取当前工作目录
func getCurrentWorkingDir() (string, error) {
	return os.Getwd()
}

// gatherDirectoryInfo 收集目录信息
func gatherDirectoryInfo(dirPath string) *DirectoryContextInfo {
	if dirPath == "" {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return &DirectoryContextInfo{
			Path:        dirPath,
			Description: "Unable to read directory",
		}
	}

	var fileCount, dirCount int
	var totalSize int64
	var lastModified time.Time
	var topFiles []FileInfo
	projectType := "Unknown"

	// 分析文件
	for _, entry := range entries {
		// 跳过隐藏文件
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if entry.IsDir() {
			dirCount++
		} else {
			fileCount++
			totalSize += info.Size()

			// 检测项目类型
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			switch ext {
			case ".go":
				if projectType == "Unknown" {
					projectType = "Go"
				}
			case ".py":
				if projectType == "Unknown" || projectType == "Go" {
					projectType = "Python"
				}
			case ".js", ".ts", ".jsx", ".tsx":
				if projectType == "Unknown" {
					projectType = "JavaScript/TypeScript"
				}
			case ".java":
				if projectType == "Unknown" {
					projectType = "Java"
				}
			case ".rs":
				if projectType == "Unknown" {
					projectType = "Rust"
				}
			}
		}

		// 更新最后修改时间
		if info.ModTime().After(lastModified) {
			lastModified = info.ModTime()
		}

		// 收集主要文件（优先重要文件，限制数量）
		fileType := "file"
		if entry.IsDir() {
			fileType = "directory"
		} else {
			ext := filepath.Ext(entry.Name())
			if ext != "" {
				fileType = ext[1:] // 去掉点号
			}
		}

		fileInfo := FileInfo{
			Name:     entry.Name(),
			Path:     entry.Name(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			Type:     fileType,
			IsDir:    entry.IsDir(),
		}

		// 重要文件优先收集，不受10个文件限制
		isImportantFile := false
		if !entry.IsDir() {
			switch entry.Name() {
			case "go.mod", "go.sum", "package.json", "package-lock.json", "Cargo.toml", "Cargo.lock",
				"requirements.txt", "setup.py", "pyproject.toml", "pom.xml", "build.gradle",
				"tsconfig.json", "CMakeLists.txt", "Makefile", "makefile", "Dockerfile",
				"docker-compose.yml", "README.md", "CLAUDE.md":
				isImportantFile = true
			}
		}

		if isImportantFile || len(topFiles) < 10 {
			topFiles = append(topFiles, fileInfo)
		}
	}

	// 生成描述
	description := generateDirectoryDescription(dirPath, fileCount, dirCount, projectType, topFiles)

	return &DirectoryContextInfo{
		Path:         dirPath,
		FileCount:    fileCount,
		DirCount:     dirCount,
		TotalSize:    totalSize,
		LastModified: lastModified,
		TopFiles:     topFiles,
		ProjectType:  projectType,
		Description:  description,
	}
}

// generateDirectoryDescription 生成目录描述
func generateDirectoryDescription(dirPath string, fileCount, dirCount int, projectType string, topFiles []FileInfo) string {
	baseName := filepath.Base(dirPath)
	if baseName == "." || baseName == "/" {
		baseName = "current directory"
	}

	var desc strings.Builder
	desc.WriteString("Working in ")
	desc.WriteString(baseName)

	if projectType != "Unknown" {
		desc.WriteString(" (")
		desc.WriteString(projectType)
		desc.WriteString(" project)")
	}

	desc.WriteString(" containing ")
	if fileCount > 0 {
		desc.WriteString(formatCount(fileCount, "file"))
	}
	if dirCount > 0 {
		if fileCount > 0 {
			desc.WriteString(" and ")
		}
		desc.WriteString(formatCount(dirCount, "directory", "directories"))
	}

	// 添加主要文件和文件夹信息
	if len(topFiles) > 0 {
		desc.WriteString(". Key items: ")

		var directories []string
		var files []string

		// 分类重要文件和目录
		for _, file := range topFiles {
			if file.IsDir {
				// 识别重要目录
				switch file.Name {
				case "cmd", "internal", "pkg", "src", "lib", "docs", "tests", "scripts", "config", "assets", "build", "dist", "node_modules", ".git", ".github":
					directories = append(directories, file.Name+"/")
				default:
					directories = append(directories, file.Name+"/")
				}
			} else {
				// 识别重要文件
				switch file.Name {
				case "main.go", "README.md", "Makefile", "go.mod", "package.json", "Dockerfile", "docker-compose.yml", "CLAUDE.md", "config.json", "config.yaml", "config.yml":
					files = append(files, file.Name)
				default:
					// 只显示前几个非重要文件
					if len(files) < 3 {
						files = append(files, file.Name)
					}
				}
			}
		}

		// 按优先级组合显示
		var allItems []string

		// 首先显示重要目录
		for _, dir := range directories {
			if len(allItems) < 8 { // 限制显示数量
				allItems = append(allItems, dir)
			}
		}

		// 然后显示重要文件
		for _, file := range files {
			if len(allItems) < 8 { // 限制显示数量
				allItems = append(allItems, file)
			}
		}

		// 格式化输出
		if len(allItems) > 0 {
			desc.WriteString(strings.Join(allItems, ", "))
		}
	}

	return desc.String()
}

// formatCount 格式化计数文本
func formatCount(count int, singular string, plural ...string) string {
	pluralForm := singular + "s"
	if len(plural) > 0 {
		pluralForm = plural[0]
	}

	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, pluralForm)
}

// gatherProjectSummary 收集项目信息汇总
func gatherProjectSummary(dirPath string, directoryInfo *DirectoryContextInfo) *ProjectSummary {
	if dirPath == "" || directoryInfo == nil {
		return nil
	}

	projectName := filepath.Base(dirPath)
	var buildTools []string
	var mainFiles []string
	var versions []string

	// 分析主要文件和构建工具
	for _, file := range directoryInfo.TopFiles {
		if !file.IsDir {
			// 检测构建工具和版本
			switch file.Name {
			case "Makefile", "makefile":
				buildTools = appendUnique(buildTools, "Make")
			case "package.json":
				buildTools = appendUnique(buildTools, "npm")
				if version := getNodeVersion(dirPath, file.Name); version != "" {
					versions = appendUnique(versions, fmt.Sprintf("Node.js %s", version))
				}
				// 检测Node.js虚拟环境
				if venv := detectNodeVirtualEnv(); venv != "" {
					versions = appendUnique(versions, venv)
				}
			case "go.mod":
				buildTools = appendUnique(buildTools, "Go")
				if version := getGoModVersion(dirPath, file.Name); version != "" {
					versions = appendUnique(versions, fmt.Sprintf("Go %s", version))
				}
			case "Cargo.toml":
				buildTools = appendUnique(buildTools, "Cargo")
				if version := getRustVersion(); version != "" {
					versions = appendUnique(versions, fmt.Sprintf("Rust %s", version))
				}
				// 检测Rust虚拟环境
				if venv := detectRustVirtualEnv(); venv != "" {
					versions = appendUnique(versions, venv)
				}
			case "requirements.txt", "setup.py", "pyproject.toml":
				buildTools = appendUnique(buildTools, "Python")
				if version := getPythonVersion(); version != "" {
					versions = appendUnique(versions, fmt.Sprintf("Python %s", version))
				}
				// 检测Python虚拟环境
				if venv := detectPythonVirtualEnv(); venv != "" {
					versions = appendUnique(versions, venv)
				}
			case "pom.xml", "build.gradle":
				buildTools = appendUnique(buildTools, "Java")
				if version := getJavaVersion(); version != "" {
					versions = appendUnique(versions, fmt.Sprintf("Java %s", version))
				}
			case "tsconfig.json":
				buildTools = appendUnique(buildTools, "TypeScript")
				if version := getTypeScriptVersion(dirPath); version != "" {
					versions = appendUnique(versions, fmt.Sprintf("TypeScript %s", version))
				}
			case "CMakeLists.txt":
				buildTools = appendUnique(buildTools, "CMake")
			case "Dockerfile":
				buildTools = appendUnique(buildTools, "Docker")
			}

			// 检测主要文件
			switch file.Name {
			case "main.go", "main.py", "main.js", "main.ts", "main.cpp", "main.c", "main.java", "README.md", "CLAUDE.md":
				mainFiles = append(mainFiles, file.Name)
			}
		}
	}

	// 生成项目信息汇总
	info := fmt.Sprintf("%s (%s project)", projectName, directoryInfo.ProjectType)
	if len(buildTools) > 0 {
		info += fmt.Sprintf(", tools: %s", strings.Join(buildTools, "/"))
	}
	if len(versions) > 0 {
		info += fmt.Sprintf(", versions: %s", strings.Join(versions, ", "))
	}
	if len(mainFiles) > 0 {
		info += fmt.Sprintf(", main files: %s", strings.Join(mainFiles, ", "))
	}

	// 生成系统环境汇总
	var currentUser string
	if user, err := user.Current(); err == nil {
		currentUser = user.Username
	}
	var shell string
	if shellPath := os.Getenv("SHELL"); shellPath != "" {
		shell = filepath.Base(shellPath)
	}

	context := fmt.Sprintf("%s/%s, Go %s, user: %s",
		runtime.GOOS, runtime.GOARCH, runtime.Version(), currentUser)
	if shell != "" {
		context += fmt.Sprintf(", shell: %s", shell)
	}

	return &ProjectSummary{
		Info:    info,
		Context: context,
	}
}

// appendUnique 添加唯一元素到切片
func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}

// getNodeVersion 从 package.json 获取 Node.js 版本要求
func getNodeVersion(dirPath, fileName string) string {
	// 尝试读取 package.json 中的 engines.node 字段
	content, err := os.ReadFile(filepath.Join(dirPath, fileName))
	if err != nil {
		return ""
	}

	// 简单的字符串匹配，避免引入 JSON 解析依赖
	contentStr := string(content)
	if strings.Contains(contentStr, `"node"`) {
		// 查找 "node": "版本" 模式
		if start := strings.Index(contentStr, `"node"`); start != -1 {
			remaining := contentStr[start:]
			if colonPos := strings.Index(remaining, ":"); colonPos != -1 {
				afterColon := remaining[colonPos+1:]
				if quoteStart := strings.Index(afterColon, `"`); quoteStart != -1 {
					versionStart := afterColon[quoteStart+1:]
					if quoteEnd := strings.Index(versionStart, `"`); quoteEnd != -1 {
						version := versionStart[:quoteEnd]
						return strings.TrimSpace(version)
					}
				}
			}
		}
	}
	return ""
}

// getGoModVersion 从 go.mod 获取 Go 版本
func getGoModVersion(dirPath, fileName string) string {
	content, err := os.ReadFile(filepath.Join(dirPath, fileName))
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// getRustVersion 获取 Rust 版本
func getRustVersion() string {
	// 尝试执行 rustc --version
	if version := getCommandVersion("rustc", "--version"); version != "" {
		// rustc 1.70.0 (90c541806 2023-05-31)
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return ""
}

// getPythonVersion 获取 Python 版本
func getPythonVersion() string {
	// 尝试 python3 和 python
	for _, cmd := range []string{"python3", "python"} {
		if version := getCommandVersion(cmd, "--version"); version != "" {
			// Python 3.9.7
			parts := strings.Fields(version)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// getJavaVersion 获取 Java 版本
func getJavaVersion() string {
	if version := getCommandVersion("java", "-version"); version != "" {
		// java version "11.0.16" 2022-07-19 LTS
		// 或者 openjdk version "17.0.4" 2022-07-19
		if strings.Contains(version, "version") {
			// 查找版本号
			if start := strings.Index(version, `"`); start != -1 {
				remaining := version[start+1:]
				if end := strings.Index(remaining, `"`); end != -1 {
					return remaining[:end]
				}
			}
		}
	}
	return ""
}

// getTypeScriptVersion 获取 TypeScript 版本
func getTypeScriptVersion(dirPath string) string {
	// 尝试从 node_modules 获取版本
	packagePath := filepath.Join(dirPath, "node_modules", "typescript", "package.json")
	if content, err := os.ReadFile(packagePath); err == nil {
		contentStr := string(content)
		if strings.Contains(contentStr, `"version"`) {
			if start := strings.Index(contentStr, `"version"`); start != -1 {
				remaining := contentStr[start:]
				if colonPos := strings.Index(remaining, ":"); colonPos != -1 {
					afterColon := remaining[colonPos+1:]
					if quoteStart := strings.Index(afterColon, `"`); quoteStart != -1 {
						versionStart := afterColon[quoteStart+1:]
						if quoteEnd := strings.Index(versionStart, `"`); quoteEnd != -1 {
							return versionStart[:quoteEnd]
						}
					}
				}
			}
		}
	}

	// 备选方案：尝试执行 tsc --version
	if version := getCommandVersion("tsc", "--version"); version != "" {
		// Version 4.8.4
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return ""
}

// getCommandVersion 执行命令获取版本信息
func getCommandVersion(command string, args ...string) string {
	// 设置超时，避免hang住
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)

	// 执行命令并获取输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

// detectPythonVirtualEnv 检测Python虚拟环境
func detectPythonVirtualEnv() string {
	// 检查常见的虚拟环境环境变量
	if venv := os.Getenv("VIRTUAL_ENV"); venv != "" {
		return fmt.Sprintf("venv:%s", filepath.Base(venv))
	}

	if conda := os.Getenv("CONDA_DEFAULT_ENV"); conda != "" && conda != "base" {
		return fmt.Sprintf("conda:%s", conda)
	}

	if poetry := os.Getenv("POETRY_ACTIVE"); poetry == "1" {
		return "poetry:active"
	}

	if pipenv := os.Getenv("PIPENV_ACTIVE"); pipenv == "1" {
		return "pipenv:active"
	}

	// 检查本地虚拟环境目录
	for _, venvDir := range []string{"venv", ".venv", "env", ".env"} {
		if info, err := os.Stat(venvDir); err == nil && info.IsDir() {
			// 检查是否有Python可执行文件
			pythonPath := filepath.Join(venvDir, "bin", "python")
			if runtime.GOOS == "windows" {
				pythonPath = filepath.Join(venvDir, "Scripts", "python.exe")
			}
			if _, err := os.Stat(pythonPath); err == nil {
				return fmt.Sprintf("local:%s", venvDir)
			}
		}
	}

	return ""
}

// detectNodeVirtualEnv 检测Node.js虚拟环境
func detectNodeVirtualEnv() string {
	// 检查node_modules是否存在
	if info, err := os.Stat("node_modules"); err == nil && info.IsDir() {
		return "npm:node_modules"
	}

	// 检查yarn.lock或pnpm-lock.yaml
	if _, err := os.Stat("yarn.lock"); err == nil {
		return "yarn:workspace"
	}

	if _, err := os.Stat("pnpm-lock.yaml"); err == nil {
		return "pnpm:workspace"
	}

	return ""
}

// detectRustVirtualEnv 检测Rust虚拟环境
func detectRustVirtualEnv() string {
	// 检查Cargo.lock和target目录
	if _, err := os.Stat("Cargo.lock"); err == nil {
		if info, err := os.Stat("target"); err == nil && info.IsDir() {
			return "cargo:workspace"
		}
	}

	return ""
}
