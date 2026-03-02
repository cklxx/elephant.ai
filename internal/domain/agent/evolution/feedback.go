//go:build ignore
package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// FeedbackCollector 收集和管理用户反馈
type FeedbackCollector struct {
	mu       sync.RWMutex
	entries  []FeedbackEntry
	config   FeedbackConfig
	logger   Logger
}

// FeedbackConfig 反馈收集配置
type FeedbackConfig struct {
	// EnableExplicitFeedback 是否启用显式反馈（如点赞/点踩）
	EnableExplicitFeedback bool
	// EnableImplicitFeedback 是否启用隐式反馈（基于用户行为分析）
	EnableImplicitFeedback bool
	// MinFeedbackInterval 两次反馈之间的最小间隔
	MinFeedbackInterval time.Duration
	// MaxStoredFeedback 最大存储的反馈条目数
	MaxStoredFeedback int
}

// DefaultFeedbackConfig 返回默认配置
func DefaultFeedbackConfig() FeedbackConfig {
	return FeedbackConfig{
		EnableExplicitFeedback: true,
		EnableImplicitFeedback: true,
		MinFeedbackInterval:    time.Second * 5,
		MaxStoredFeedback:      1000,
	}
}

// NewFeedbackCollector 创建新的反馈收集器
func NewFeedbackCollector(config FeedbackConfig, logger Logger) *FeedbackCollector {
	if logger == nil {
		logger = &noopLogger{}
	}
	return &FeedbackCollector{
		entries: make([]FeedbackEntry, 0),
		config:  config,
		logger:  logger,
	}
}

// SubmitExplicitFeedback 提交显式反馈
func (fc *FeedbackCollector) SubmitExplicitFeedback(ctx context.Context, req ExplicitFeedbackRequest) error {
	if !fc.config.EnableExplicitFeedback {
		return fmt.Errorf("explicit feedback is disabled")
	}

	entry := FeedbackEntry{
		ID:              generateFeedbackID(),
		SessionID:       req.SessionID,
		TurnID:          req.TurnID,
		InteractionType: req.InteractionType,
		Rating:          req.Rating,
		Category:        req.Category,
		Comment:         req.Comment,
		Timestamp:       time.Now(),
		Explicit:        true,
		Metadata:        req.Metadata,
	}

	return fc.addEntry(entry)
}

// RecordImplicitSignal 记录隐式信号
func (fc *FeedbackCollector) RecordImplicitSignal(ctx context.Context, signal ImplicitSignal) error {
	if !fc.config.EnableImplicitFeedback {
		return nil // 静默忽略
	}

	// 将隐式信号转换为反馈条目
	rating := fc.inferRatingFromSignal(signal)
	
	entry := FeedbackEntry{
		ID:              generateFeedbackID(),
		SessionID:       signal.SessionID,
		TurnID:          signal.TurnID,
		InteractionType: "implicit_" + signal.SignalType,
		Rating:          rating,
		Category:        fc.categorizeSignal(signal),
		Timestamp:       time.Now(),
		Explicit:        false,
		Metadata: map[string]interface{}{
			"signal_type":    signal.SignalType,
			"signal_value":   signal.Value,
			"signal_context": signal.Context,
		},
	}

	return fc.addEntry(entry)
}

// GetRecentFeedback 获取最近的反馈
func (fc *FeedbackCollector) GetRecentFeedback(limit int) []FeedbackEntry {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	if limit <= 0 || limit > len(fc.entries) {
		limit = len(fc.entries)
	}

	// 返回最新的条目
	start := len(fc.entries) - limit
	if start < 0 {
		start = 0
	}

	result := make([]FeedbackEntry, limit)
	copy(result, fc.entries[start:])
	return result
}

// GetFeedbackBySession 获取特定会话的反馈
func (fc *FeedbackCollector) GetFeedbackBySession(sessionID string) []FeedbackEntry {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	var result []FeedbackEntry
	for _, entry := range fc.entries {
		if entry.SessionID == sessionID {
			result = append(result, entry)
		}
	}
	return result
}

// GetFeedbackSummary 获取反馈摘要
func (fc *FeedbackCollector) GetFeedbackSummary() FeedbackSummary {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	summary := FeedbackSummary{
		TotalEntries:   len(fc.entries),
		ExplicitCount:  0,
		ImplicitCount:  0,
		RatingSum:      0,
		CategoryCounts: make(map[string]int),
	}

	for _, entry := range fc.entries {
		if entry.Explicit {
			summary.ExplicitCount++
		} else {
			summary.ImplicitCount++
		}
		summary.RatingSum += entry.Rating
		summary.CategoryCounts[entry.Category]++
	}

	if summary.TotalEntries > 0 {
		summary.AverageRating = float64(summary.RatingSum) / float64(summary.TotalEntries)
	}

	return summary
}

// ClearOldFeedback 清理旧的反馈
func (fc *FeedbackCollector) ClearOldFeedback(olderThan time.Duration) int {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var newEntries []FeedbackEntry
	removed := 0

	for _, entry := range fc.entries {
		if entry.Timestamp.After(cutoff) {
			newEntries = append(newEntries, entry)
		} else {
			removed++
		}
	}

	fc.entries = newEntries
	return removed
}

// addEntry 添加条目（内部方法）
func (fc *FeedbackCollector) addEntry(entry FeedbackEntry) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// 检查是否需要清理旧条目
	if len(fc.entries) >= fc.config.MaxStoredFeedback {
		// 移除最旧的 20%
		removeCount := fc.config.MaxStoredFeedback / 5
		if removeCount < 1 {
			removeCount = 1
		}
		fc.entries = fc.entries[removeCount:]
	}

	fc.entries = append(fc.entries, entry)
	fc.logger.Debug("Feedback entry added: %s (rating: %d)", entry.ID, entry.Rating)
	return nil
}

// inferRatingFromSignal 从隐式信号推断评分
func (fc *FeedbackCollector) inferRatingFromSignal(signal ImplicitSignal) int {
	switch signal.SignalType {
	case "quick_accept":
		return 5 // 快速接受表示满意
	case "edit_response":
		// 根据编辑程度推断
		if editRatio, ok := signal.Context["edit_ratio"].(float64); ok {
			if editRatio < 0.1 {
				return 4 // 小幅编辑
			} else if editRatio < 0.5 {
				return 3 // 中等编辑
			}
			return 2 // 大幅编辑
		}
		return 3
	case "retry_request":
		return 2 // 重试表示不满意
	case "abandon_session":
		return 1 // 放弃会话表示强烈不满
	case "follow_up_question":
		return 4 // 追问表示有兴趣
	case "copy_response":
		return 5 // 复制表示有价值
	case "share_response":
		return 5 // 分享表示高度认可
	default:
		return 3 // 默认中性
	}
}

// categorizeSignal 分类信号
func (fc *FeedbackCollector) categorizeSignal(signal ImplicitSignal) string {
	switch signal.SignalType {
	case "quick_accept", "copy_response", "share_response":
		return "quality"
	case "edit_response", "retry_request":
		return "accuracy"
	case "abandon_session":
		return "engagement"
	case "follow_up_question":
		return "completeness"
	default:
		return "general"
	}
}

// ExportFeedback 导出反馈数据
func (fc *FeedbackCollector) ExportFeedback(format string) ([]byte, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(fc.entries, "", "  ")
	case "summary":
		summary := fc.GetFeedbackSummary()
		return json.MarshalIndent(summary, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// FeedbackStore 接口定义 — 用于持久化存储
type FeedbackStore interface {
	Save(ctx context.Context, entry FeedbackEntry) error
	LoadBySession(ctx context.Context, sessionID string) ([]FeedbackEntry, error)
	LoadRecent(ctx context.Context, limit int) ([]FeedbackEntry, error)
	DeleteOld(ctx context.Context, olderThan time.Duration) (int, error)
}

// PersistingFeedbackCollector 支持持久化的反馈收集器
type PersistingFeedbackCollector struct {
	*FeedbackCollector
	store FeedbackStore
}

// NewPersistingFeedbackCollector 创建持久化反馈收集器
func NewPersistingFeedbackCollector(config FeedbackConfig, store FeedbackStore, logger Logger) *PersistingFeedbackCollector {
	return &PersistingFeedbackCollector{
		FeedbackCollector: NewFeedbackCollector(config, logger),
		store:             store,
	}
}

// SubmitExplicitFeedback 带持久化的显式反馈
func (pfc *PersistingFeedbackCollector) SubmitExplicitFeedback(ctx context.Context, req ExplicitFeedbackRequest) error {
	if err := pfc.FeedbackCollector.SubmitExplicitFeedback(ctx, req); err != nil {
		return err
	}
	
	// 获取刚添加的条目并持久化
	entries := pfc.FeedbackCollector.GetRecentFeedback(1)
	if len(entries) > 0 && pfc.store != nil {
		if err := pfc.store.Save(ctx, entries[0]); err != nil {
			pfc.logger.Warn("Failed to persist feedback: %v", err)
		}
	}
	return nil
}

// RecordImplicitSignal 带持久化的隐式信号
func (pfc *PersistingFeedbackCollector) RecordImplicitSignal(ctx context.Context, signal ImplicitSignal) error {
	if err := pfc.FeedbackCollector.RecordImplicitSignal(ctx, signal); err != nil {
		return err
	}
	
	entries := pfc.FeedbackCollector.GetRecentFeedback(1)
	if len(entries) > 0 && pfc.store != nil {
		if err := pfc.store.Save(ctx, entries[0]); err != nil {
			pfc.logger.Warn("Failed to persist feedback: %v", err)
		}
	}
	return nil
}