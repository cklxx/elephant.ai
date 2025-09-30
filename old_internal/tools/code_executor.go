package tools

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// CodeActResult - 代码执行结果
type CodeActResult struct {
	Success       bool          `json:"success"`        // 是否成功
	Output        string        `json:"output"`         // 标准输出
	Error         string        `json:"error"`          // 错误输出
	ExitCode      int           `json:"exit_code"`      // 退出码
	ExecutionTime time.Duration `json:"execution_time"` // 执行时间
	Language      string        `json:"language"`       // 编程语言
	Code          string        `json:"code"`           // 执行的代码
}

// CodeActExecutor - CodeAct执行器（支持代码作为主要行动语言）
type CodeActExecutor struct {
	supportedLanguages map[string]string
	sandboxDir         string
	timeout            time.Duration
	mu                 sync.RWMutex
}

// NewCodeActExecutor - 创建新的CodeActExecutor
func NewCodeActExecutor() *CodeActExecutor {
	sandboxDir := filepath.Join(os.TempDir(), "deep-coding-sandbox")
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		// Fall back to current directory if sandbox creation fails
		sandboxDir = "."
	}

	supportedLanguages := map[string]string{
		"python":     "python3",
		"go":         "go run",
		"bash":       "bash",
		"javascript": "node",
		"js":         "node",
	}

	return &CodeActExecutor{
		supportedLanguages: supportedLanguages,
		sandboxDir:         sandboxDir,
		timeout:            30 * time.Second,
	}
}

// ExecuteCode - 执行代码
func (ce *CodeActExecutor) ExecuteCode(ctx context.Context, language, code string) (*CodeActResult, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	_, exists := ce.supportedLanguages[language]
	if !exists {
		return &CodeActResult{
			Success:  false,
			Error:    fmt.Sprintf("unsupported language: %s", language),
			Language: language,
			Code:     code,
		}, nil
	}

	// 创建临时文件
	var ext string
	switch language {
	case "python":
		ext = ".py"
	case "go":
		ext = ".go"
	case "javascript", "js":
		ext = ".js"
	case "bash":
		ext = ".sh"
	default:
		ext = ".txt"
	}

	tempFile := filepath.Join(ce.sandboxDir, fmt.Sprintf("script_%d%s", time.Now().UnixNano(), ext))
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			log.Printf("Error removing temp file: %v", err)
		}
	}()

	// 写入代码
	if err := os.WriteFile(tempFile, []byte(code), 0644); err != nil {
		return &CodeActResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to write code file: %v", err),
			Language: language,
			Code:     code,
		}, nil
	}

	// 执行代码
	start := time.Now()
	var cmd *exec.Cmd

	switch language {
	case "python":
		cmd = exec.CommandContext(ctx, "python3", tempFile)
	case "go":
		cmd = exec.CommandContext(ctx, "go", "run", tempFile)
	case "javascript", "js":
		cmd = exec.CommandContext(ctx, "node", tempFile)
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", tempFile)
	}

	cmd.Dir = ce.sandboxDir

	output, err := cmd.CombinedOutput()
	executionTime := time.Since(start)

	result := &CodeActResult{
		Language:      language,
		Code:          code,
		ExecutionTime: executionTime,
	}

	if err != nil {
		result.Success = false
		result.Error = string(output)
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}
	} else {
		result.Success = true
		result.Output = string(output)
		result.ExitCode = 0
	}

	return result, nil
}

// GetSupportedLanguages - 获取支持的语言列表
func (ce *CodeActExecutor) GetSupportedLanguages() []string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	languages := make([]string, 0, len(ce.supportedLanguages))
	for lang := range ce.supportedLanguages {
		languages = append(languages, lang)
	}
	return languages
}

// SetTimeout - 设置执行超时时间
func (ce *CodeActExecutor) SetTimeout(timeout time.Duration) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.timeout = timeout
}

// GetTimeout - 获取执行超时时间
func (ce *CodeActExecutor) GetTimeout() time.Duration {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.timeout
}
