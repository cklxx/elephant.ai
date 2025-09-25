package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"alex/internal/config"
)

// ConfigHandler - 配置处理器
type ConfigHandler struct {
	configMgr *config.Manager
}

// NewConfigHandler - 创建新的配置处理器
func NewConfigHandler(configMgr *config.Manager) *ConfigHandler {
	return &ConfigHandler{
		configMgr: configMgr,
	}
}

// GetConfig - 获取当前配置
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	cfg := h.configMgr.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "failed to get configuration",
		})
		return
	}

	// 构建配置响应 (隐藏敏感信息)
	configData := map[string]interface{}{
		"base_url":           cfg.BaseURL,
		"model":              cfg.Model,
		"max_tokens":         cfg.MaxTokens,
		"temperature":        cfg.Temperature,
		"max_turns":          cfg.MaxTurns,
		"default_model_type": cfg.DefaultModelType,
		"models":             cfg.Models, // 这里可能包含敏感信息，实际使用时需要过滤
		"mcp":                cfg.MCP,
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    configData,
	})
}

// UpdateConfig - 更新配置 (简化版本)
func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var req ConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	// 暂时只支持读取配置，不支持动态更新
	// 在实际应用中，需要实现配置保存功能
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "configuration update not implemented yet",
	})
}

// ResetConfig - 重置配置到默认值 (简化版本)
func (h *ConfigHandler) ResetConfig(c *gin.Context) {
	// 暂时不支持配置重置
	c.JSON(http.StatusNotImplemented, APIResponse{
		Success: false,
		Error:   "configuration reset not implemented yet",
	})
}