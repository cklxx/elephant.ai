package llm

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// SessionCache represents cached session state for API optimization
type SessionCache struct {
	SessionID    string                 `json:"session_id"`
	CacheKey     string                 `json:"cache_key"` // Hash of conversation state
	Messages     []Message              `json:"messages"`  // Cached messages
	Context      string                 `json:"context"`   // Additional context
	LastUsed     time.Time              `json:"last_used"`
	TokensUsed   int                    `json:"tokens_used"`   // Accumulated token usage
	RequestCount int                    `json:"request_count"` // Number of API calls made
	Metadata     map[string]interface{} `json:"metadata"`
}

// CacheManager manages session caches for API optimization
type CacheManager struct {
	caches map[string]*SessionCache
	mutex  sync.RWMutex

	// Configuration
	maxCacheSize     int           // Maximum number of cached sessions
	maxMessageCount  int           // Maximum messages per session cache
	cacheExpiry      time.Duration // Cache expiry time
	compressionRatio float64       // When to compress old messages
}

// NewCacheManager creates a new cache manager
func NewCacheManager() *CacheManager {
	return &CacheManager{
		caches:           make(map[string]*SessionCache),
		maxCacheSize:     100,            // Max 100 cached sessions
		maxMessageCount:  50,             // Max 50 messages per cache
		cacheExpiry:      24 * time.Hour, // 24 hour expiry
		compressionRatio: 0.7,            // Compress when 70% full
	}
}

// GetOrCreateCache gets existing cache or creates new one for session
func (cm *CacheManager) GetOrCreateCache(sessionID string) *SessionCache {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Check if cache exists
	if cache, exists := cm.caches[sessionID]; exists {
		cache.LastUsed = time.Now()
		return cache
	}

	// Create new cache
	cache := &SessionCache{
		SessionID:    sessionID,
		Messages:     make([]Message, 0),
		LastUsed:     time.Now(),
		TokensUsed:   0,
		RequestCount: 0,
		Metadata:     make(map[string]interface{}),
	}

	// Cleanup old caches if needed
	cm.cleanupIfNeeded()

	cm.caches[sessionID] = cache
	return cache
}

// UpdateCache updates cache with new messages and response
func (cm *CacheManager) UpdateCache(sessionID string, newMessages []Message, tokensUsed int) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cache, exists := cm.caches[sessionID]
	if !exists {
		log.Printf("[WARNING] CacheManager: Attempting to update non-existent cache: %s", sessionID)
		return
	}

	// Add new messages
	cache.Messages = append(cache.Messages, newMessages...)
	cache.TokensUsed += tokensUsed
	cache.RequestCount++
	cache.LastUsed = time.Now()

	// Generate new cache key
	cache.CacheKey = cm.generateCacheKey(cache.Messages)

	// Compress messages if cache is getting too large
	if len(cache.Messages) > int(float64(cm.maxMessageCount)*cm.compressionRatio) {
		cm.compressMessages(cache)
	}

}

// GetOptimizedMessages returns optimized message list for API call
// This reduces the number of messages sent to the API by using cache context
func (cm *CacheManager) GetOptimizedMessages(sessionID string, newMessages []Message) []Message {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	cache, exists := cm.caches[sessionID]
	if !exists {
		// No cache, return all messages
		return newMessages
	}

	// If we have cached context, we can send fewer messages
	if len(cache.Messages) > 0 {
		// Strategy: Send summary of old context + recent messages + new messages
		optimized := make([]Message, 0)

		// Add system message with conversation summary
		if len(cache.Messages) > 5 {
			summary := cm.generateConversationSummary(cache.Messages)
			optimized = append(optimized, Message{
				Role:    "system",
				Content: fmt.Sprintf("Previous conversation summary: %s", summary),
			})

			// Add only the last few messages for immediate context
			recentStart := len(cache.Messages) - 3
			if recentStart < 0 {
				recentStart = 0
			}
			optimized = append(optimized, cache.Messages[recentStart:]...)
		} else {
			// If cache is small, include all cached messages
			optimized = append(optimized, cache.Messages...)
		}

		// Add new messages
		optimized = append(optimized, newMessages...)

		return optimized
	}

	return newMessages
}

// generateCacheKey creates a hash key for the current conversation state
func (cm *CacheManager) generateCacheKey(messages []Message) string {
	data, _ := json.Marshal(messages)
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// generateConversationSummary creates a summary of the conversation
func (cm *CacheManager) generateConversationSummary(messages []Message) string {
	if len(messages) == 0 {
		return "No previous conversation"
	}

	// Simple summarization strategy
	var parts []string
	userMessages := 0
	assistantMessages := 0
	toolCalls := 0

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userMessages++
		case "assistant":
			assistantMessages++
			if len(msg.ToolCalls) > 0 {
				toolCalls += len(msg.ToolCalls)
			}
		}
	}

	parts = append(parts, fmt.Sprintf("Conversation had %d user messages and %d assistant responses",
		userMessages, assistantMessages))

	if toolCalls > 0 {
		parts = append(parts, fmt.Sprintf("Used %d tool calls for various operations", toolCalls))
	}

	// Add context from first and last messages
	if len(messages) > 1 {
		first := messages[0]
		last := messages[len(messages)-1]

		// Use rune-based slicing to properly handle UTF-8 characters in message content
		firstRunes := []rune(first.Content)
		if len(firstRunes) > 100 {
			parts = append(parts, fmt.Sprintf("Started with: %s...", string(firstRunes[:100])))
		} else {
			parts = append(parts, fmt.Sprintf("Started with: %s", first.Content))
		}

		// Use rune-based slicing to properly handle UTF-8 characters in message content
		lastRunes := []rune(last.Content)
		if len(lastRunes) > 100 {
			parts = append(parts, fmt.Sprintf("Last message: %s...", string(lastRunes[:100])))
		} else if last.Content != first.Content {
			parts = append(parts, fmt.Sprintf("Last message: %s", last.Content))
		}
	}

	return parts[0]
}

// compressMessages removes older messages to keep cache size manageable
func (cm *CacheManager) compressMessages(cache *SessionCache) {
	if len(cache.Messages) <= cm.maxMessageCount {
		return
	}

	// Keep recent messages and create a summary of older ones
	keepCount := cm.maxMessageCount / 2
	oldMessages := cache.Messages[:len(cache.Messages)-keepCount]
	recentMessages := cache.Messages[len(cache.Messages)-keepCount:]

	// Create summary message
	summary := cm.generateConversationSummary(oldMessages)
	summaryMessage := Message{
		Role:    "system",
		Content: fmt.Sprintf("[COMPRESSED HISTORY] %s", summary),
	}

	// Replace old messages with summary + recent messages
	cache.Messages = make([]Message, 0, keepCount+1)
	cache.Messages = append(cache.Messages, summaryMessage)
	cache.Messages = append(cache.Messages, recentMessages...)
}

// cleanupIfNeeded removes expired or excess caches
func (cm *CacheManager) cleanupIfNeeded() {
	if len(cm.caches) < cm.maxCacheSize {
		return
	}

	now := time.Now()
	var toDelete []string

	// Find expired caches
	for sessionID, cache := range cm.caches {
		if now.Sub(cache.LastUsed) > cm.cacheExpiry {
			toDelete = append(toDelete, sessionID)
		}
	}

	// If still too many, remove least recently used
	if len(cm.caches)-len(toDelete) >= cm.maxCacheSize {
		type cacheAge struct {
			sessionID string
			lastUsed  time.Time
		}

		var ages []cacheAge
		for sessionID, cache := range cm.caches {
			skip := false
			for _, delID := range toDelete {
				if delID == sessionID {
					skip = true
					break
				}
			}
			if !skip {
				ages = append(ages, cacheAge{sessionID, cache.LastUsed})
			}
		}

		// Sort by last used (oldest first)
		for i := 0; i < len(ages)-1; i++ {
			for j := i + 1; j < len(ages); j++ {
				if ages[i].lastUsed.After(ages[j].lastUsed) {
					ages[i], ages[j] = ages[j], ages[i]
				}
			}
		}

		// Add oldest to delete list
		needed := len(cm.caches) - len(toDelete) - cm.maxCacheSize + 10 // Keep some buffer
		for i := 0; i < needed && i < len(ages); i++ {
			toDelete = append(toDelete, ages[i].sessionID)
		}
	}

	// Delete caches
	for _, sessionID := range toDelete {
		delete(cm.caches, sessionID)
	}
}

// GetCacheStats returns statistics about cache usage
func (cm *CacheManager) GetCacheStats() map[string]interface{} {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	totalMessages := 0
	totalTokens := 0
	totalRequests := 0

	for _, cache := range cm.caches {
		totalMessages += len(cache.Messages)
		totalTokens += cache.TokensUsed
		totalRequests += cache.RequestCount
	}

	return map[string]interface{}{
		"total_sessions":        len(cm.caches),
		"total_cached_messages": totalMessages,
		"total_tokens_saved":    totalTokens,
		"total_requests":        totalRequests,
		"cache_hit_ratio":       cm.calculateCacheHitRatio(),
	}
}

// calculateCacheHitRatio calculates the effectiveness of caching
func (cm *CacheManager) calculateCacheHitRatio() float64 {
	if len(cm.caches) == 0 {
		return 0.0
	}

	totalRequests := 0
	for _, cache := range cm.caches {
		totalRequests += cache.RequestCount
	}

	if totalRequests == 0 {
		return 0.0
	}

	// Estimate savings: each request after the first saves tokens
	totalSavings := 0
	for _, cache := range cm.caches {
		if cache.RequestCount > 1 {
			totalSavings += (cache.RequestCount - 1) * len(cache.Messages)
		}
	}

	return float64(totalSavings) / float64(totalRequests*50) // Assume 50 avg messages per full context
}

// ClearCache removes cache for a specific session
func (cm *CacheManager) ClearCache(sessionID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	delete(cm.caches, sessionID)
}

// ClearAllCaches removes all caches
func (cm *CacheManager) ClearAllCaches() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.caches = make(map[string]*SessionCache)
	// All caches cleared
}

// Global cache manager instance
var globalCacheManager *CacheManager
var cacheManagerOnce sync.Once

// GetGlobalCacheManager returns the singleton cache manager
func GetGlobalCacheManager() *CacheManager {
	cacheManagerOnce.Do(func() {
		globalCacheManager = NewCacheManager()
	})
	return globalCacheManager
}
