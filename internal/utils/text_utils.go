package utils

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateProjectID 基于当前工作目录生成项目ID
func GenerateProjectID() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 使用MD5哈希生成项目ID
	hash := md5.Sum([]byte(absPath))
	projectID := fmt.Sprintf("project_%x", hash[:8]) // 使用前8个字节

	return projectID, nil
}

// GetProjectDisplayName 获取项目显示名称
func GetProjectDisplayName() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// 返回目录名称作为显示名称
	return filepath.Base(workingDir), nil
}

// formatFileSize formats file size in human-readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
