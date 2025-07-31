package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// KimiCacheManager handles context caching for Kimi API
type KimiCacheManager struct {
	mutex      sync.RWMutex
	caches     map[string]*KimiCache // sessionID -> cache
	llmClient  Client
}

// KimiCache represents a cached context for a session
type KimiCache struct {
	SessionID       string            `json:"session_id"`
	CacheID         string            `json:"cache_id"`
	CachedMessages  []Message         `json:"cached_messages"`  // 完整的缓存消息
	CachedTools     []Tool            `json:"cached_tools"`     // 缓存的 tools
	CreatedAt       time.Time         `json:"created_at"`
	LastUsedAt      time.Time         `json:"last_used_at"`
	RequestCount    int               `json:"request_count"`
	Status          string            `json:"status"` // "active", "expired", "deleted"
	ExtraHeaders    map[string]string `json:"extra_headers,omitempty"`
}

// KimiCacheRequest represents a request with caching enabled
type KimiCacheRequest struct {
	ChatRequest
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl represents cache control parameters for Kimi API
type CacheControl struct {
	Type    string `json:"type"`    // "ephemeral" for context caching
	CacheID string `json:"cache_id,omitempty"`
}

// NewKimiCacheManager creates a new Kimi cache manager
func NewKimiCacheManager(llmClient Client) *KimiCacheManager {
	return &KimiCacheManager{
		caches:    make(map[string]*KimiCache),
		llmClient: llmClient,
	}
}

// IsKimiAPI checks if the base URL is Kimi API
func IsKimiAPI(baseURL string) bool {
	return strings.Contains(baseURL, "api.moonshot.cn")
}

// CreateCacheIfNeeded creates a new cache for the session's cacheable prefix if not exists
func (kcm *KimiCacheManager) CreateCacheIfNeeded(sessionID string, messages []Message, tools []Tool, apiKey string) (*KimiCache, error) {
	kcm.mutex.Lock()
	defer kcm.mutex.Unlock()

	// 提取可缓存的前缀消息（稳定部分）
	cacheableMessages := kcm.extractCacheablePrefix(messages)
	if len(cacheableMessages) == 0 {
		return nil, nil // 没有可缓存的内容
	}

	// Check if cache already exists and verify consistency
	if existingCache, exists := kcm.caches[sessionID]; exists && existingCache.Status == "active" {
		// 只需要验证可缓存前缀是否匹配
		if kcm.messagesMatch(cacheableMessages, existingCache.CachedMessages) && kcm.toolsMatch(tools, existingCache.CachedTools) {
			log.Printf("[KIMI_CACHE] ✅ Using existing cache: %s", existingCache.CacheID)
			return existingCache, nil
		} else {
			// Delete old cache and create new one
			if err := kcm.sendCacheDeletionRequest(existingCache.CacheID, apiKey); err != nil {
				log.Printf("[KIMI_CACHE] ⚠️  Failed to delete old cache: %v", err)
			}
			delete(kcm.caches, sessionID)
		}
	}

	// Create cache using Kimi API for cacheable prefix only
	cacheID, err := kcm.createCacheWithAPI(cacheableMessages, tools, apiKey)
	if err != nil {
		log.Printf("[KIMI_CACHE] ❌ Failed to create cache: %v", err)
		return nil, fmt.Errorf("failed to create cache with Kimi API: %w", err)
	}

	cache := &KimiCache{
		SessionID:      sessionID,
		CacheID:        cacheID,
		CachedMessages: cacheableMessages, // 只缓存稳定前缀
		CachedTools:    tools,
		CreatedAt:      time.Now(),
		LastUsedAt:     time.Now(),
		RequestCount:   0,
		Status:         "active",
	}

	kcm.caches[sessionID] = cache
	log.Printf("[KIMI_CACHE] ✅ Created new cache: %s (%d msgs)", cacheID, len(cacheableMessages))
	return cache, nil
}

// extractCacheablePrefix extracts cacheable prefix from messages
// This should match the compression strategy's cacheable prefix
func (kcm *KimiCacheManager) extractCacheablePrefix(messages []Message) []Message {
	const maxCacheablePrefix = 8 // 与压缩策略保持一致

	// 过滤系统消息，找到用户对话的开始
	var nonSystemMessages []Message
	for _, msg := range messages {
		if msg.Role != "system" {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	if len(nonSystemMessages) == 0 {
		return nil
	}

	// 只取前面稳定的消息作为缓存前缀
	prefixEnd := min(maxCacheablePrefix, len(nonSystemMessages))
	
	// 确保工具调用成对
	prefixEnd = kcm.adjustForToolCallPairing(nonSystemMessages, prefixEnd)
	
	return nonSystemMessages[:prefixEnd]
}

// adjustForToolCallPairing adjusts the prefix end to maintain tool call pairs
func (kcm *KimiCacheManager) adjustForToolCallPairing(messages []Message, prefixEnd int) int {
	if prefixEnd >= len(messages) {
		return len(messages)
	}

	// 向前调整，确保不会在工具调用对中间截断
	for i := prefixEnd - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			// 检查对应的工具结果是否在前缀范围内
			hasToolResultInPrefix := false
			for j := i + 1; j < prefixEnd; j++ {
				if messages[j].Role == "tool" {
					hasToolResultInPrefix = true
					break
				}
			}
			if !hasToolResultInPrefix {
				// 工具调用没有对应结果在前缀中，调整边界
				return i
			}
		}
	}

	return prefixEnd
}

// min helper function for extractCacheablePrefix
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetCache retrieves the cache for a session
func (kcm *KimiCacheManager) GetCache(sessionID string) (*KimiCache, bool) {
	kcm.mutex.RLock()
	defer kcm.mutex.RUnlock()

	cache, exists := kcm.caches[sessionID]
	if !exists || cache.Status != "active" {
		return nil, false
	}

	return cache, true
}

// UseCache marks the cache as used and returns the cache ID for the request
func (kcm *KimiCacheManager) UseCache(sessionID string) string {
	kcm.mutex.Lock()
	defer kcm.mutex.Unlock()

	cache, exists := kcm.caches[sessionID]
	if !exists || cache.Status != "active" {
		return ""
	}

	cache.LastUsedAt = time.Now()
	cache.RequestCount++
	return cache.CacheID
}

// DeleteCache deletes the cache for a session
func (kcm *KimiCacheManager) DeleteCache(sessionID string, apiKey string) error {
	kcm.mutex.Lock()
	defer kcm.mutex.Unlock()

	cache, exists := kcm.caches[sessionID]
	if !exists {
		return nil // Cache doesn't exist, nothing to delete
	}

	// Send cache deletion request to Kimi API if cache ID exists
	if cache.CacheID != "" {
		if err := kcm.sendCacheDeletionRequest(cache.CacheID, apiKey); err != nil {
			log.Printf("WARNING: Failed to delete Kimi cache %s: %v", cache.CacheID, err)
		}
	}

	// Mark cache as deleted
	cache.Status = "deleted"
	delete(kcm.caches, sessionID)

	return nil
}

// CleanupExpiredCaches removes expired caches
func (kcm *KimiCacheManager) CleanupExpiredCaches(maxAge time.Duration, apiKey string) {
	kcm.mutex.Lock()
	defer kcm.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	var toDelete []string

	for sessionID, cache := range kcm.caches {
		if cache.LastUsedAt.Before(cutoff) {
			toDelete = append(toDelete, sessionID)
		}
	}

	for _, sessionID := range toDelete {
		cache := kcm.caches[sessionID]
		if cache.CacheID != "" {
			if err := kcm.sendCacheDeletionRequest(cache.CacheID, apiKey); err != nil {
				log.Printf("WARNING: Failed to delete expired Kimi cache %s: %v", cache.CacheID, err)
			}
		}
		delete(kcm.caches, sessionID)
	}
}

// GetCacheStats returns cache statistics
func (kcm *KimiCacheManager) GetCacheStats() map[string]interface{} {
	kcm.mutex.RLock()
	defer kcm.mutex.RUnlock()

	totalRequests := 0
	activeCaches := 0

	for _, cache := range kcm.caches {
		if cache.Status == "active" {
			activeCaches++
			totalRequests += cache.RequestCount
		}
	}

	return map[string]interface{}{
		"total_caches":    len(kcm.caches),
		"active_caches":   activeCaches,
		"total_requests":  totalRequests,
		"cache_provider":  "kimi",
	}
}

// createCacheWithAPI creates cache using Kimi API's /v1/caching endpoint
func (kcm *KimiCacheManager) createCacheWithAPI(messages []Message, tools []Tool, apiKey string) (string, error) {
	// Convert messages to map format for API
	apiMessages := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		apiMessages[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		// 添加其他字段如果存在
		if msg.Name != "" {
			apiMessages[i]["name"] = msg.Name
		}
		if msg.ToolCallId != "" {
			apiMessages[i]["tool_call_id"] = msg.ToolCallId
		}
		// 处理 tool_calls 如果存在
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCalls[j] = map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":        tc.Function.Name,
						"description": tc.Function.Description,
						"parameters":  tc.Function.Parameters,
					},
				}
			}
			apiMessages[i]["tool_calls"] = toolCalls
		}
	}

	// Prepare cache creation request according to Kimi API documentation
	reqBody := map[string]interface{}{
		"model":    "moonshot-v1", // 模型组名称，不是具体模型
		"messages": apiMessages,
		"ttl":      600, // 缓存存活时间：10分钟
	}

	// Add tools if present
	if len(tools) > 0 {
		apiTools := make([]map[string]interface{}, len(tools))
		for i, tool := range tools {
			apiTools[i] = map[string]interface{}{
				"type": tool.Type,
				"function": map[string]interface{}{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				},
			}
		}
		reqBody["tools"] = apiTools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cache request: %w", err)
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.moonshot.cn/v1/caching", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send cache creation request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing cache creation response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("cache creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read cache creation response: %w", err)
	}

	// Parse response to extract cache ID
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse cache creation response: %w", err)
	}

	// Extract cache ID from response - this depends on Kimi API response format
	cacheID := kcm.extractCacheIDFromResponse(response)
	if cacheID == "" {
		// Generate fallback cache ID if not provided by API
		cacheID = fmt.Sprintf("kimi_cache_%d", time.Now().UnixNano())
	}

	return cacheID, nil
}

// extractCacheIDFromResponse extracts cache ID from the Kimi API response
func (kcm *KimiCacheManager) extractCacheIDFromResponse(response map[string]interface{}) string {
	// According to Kimi API docs, cache creation response contains an "id" field
	if id, ok := response["id"].(string); ok {
		return id
	}
	
	return ""
}

// sendCacheDeletionRequest sends a cache deletion request to Kimi API
func (kcm *KimiCacheManager) sendCacheDeletionRequest(cacheID string, apiKey string) error {
	// According to Kimi API docs: DELETE /v1/caching/{cache-id}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://api.moonshot.cn/v1/caching/%s", cacheID), nil)
	if err != nil {
		return fmt.Errorf("failed to create cache deletion request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send cache deletion request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing cache deletion response body: %v", err)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil // Cache doesn't exist, which is fine
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cache deletion failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// messagesMatch verifies that request messages match cached messages
func (kcm *KimiCacheManager) messagesMatch(requestMessages, cachedMessages []Message) bool {
	if len(requestMessages) < len(cachedMessages) {
		return false // 请求消息不能少于缓存消息
	}
	
	// 验证前 N 个消息完全一致 (N = 缓存消息长度)
	for i, cachedMsg := range cachedMessages {
		requestMsg := requestMessages[i]
		if requestMsg.Role != cachedMsg.Role || requestMsg.Content != cachedMsg.Content {
			return false
		}
		// 也可以验证其他字段如 Name, ToolCallId 等
	}
	
	return true
}

// toolsMatch verifies that request tools match cached tools
func (kcm *KimiCacheManager) toolsMatch(requestTools, cachedTools []Tool) bool {
	if len(requestTools) != len(cachedTools) {
		return false
	}
	
	// 简化的工具比较 - 实际应该进行深度比较
	for i, cachedTool := range cachedTools {
		requestTool := requestTools[i]
		if requestTool.Type != cachedTool.Type {
			return false
		}
		// 这里应该进行更深入的 Function 比较
	}
	
	return true
}

// CleanupKimiCacheForSession cleans up Kimi cache for a specific session
// This is a utility function that can be used from anywhere in the application
func CleanupKimiCacheForSession(sessionID string, config *Config) error {
	if sessionID == "" {
		return nil // No session ID, nothing to cleanup
	}

	// Get the LLM client instance
	llmClient, err := GetLLMInstance(BasicModel)
	if err != nil {
		return fmt.Errorf("failed to get LLM instance for cache cleanup: %w", err)
	}

	// Type assert to HTTP client to access Kimi cache manager
	httpClient, ok := llmClient.(*HTTPLLMClient)
	if !ok {
		return nil // Not an HTTP client, no cache to cleanup
	}

	kimiCacheManager := httpClient.GetKimiCacheManager()
	if kimiCacheManager == nil {
		return nil // No Kimi cache manager
	}

	// Get API key from config
	var apiKey string
	if config != nil {
		if config.Models != nil {
			if basicConfig, exists := config.Models[BasicModel]; exists {
				apiKey = basicConfig.APIKey
			}
		}
		if apiKey == "" {
			apiKey = config.APIKey
		}
	}

	if apiKey == "" {
		return fmt.Errorf("no API key available for cache cleanup")
	}

	// Delete the cache
	if err := kimiCacheManager.DeleteCache(sessionID, apiKey); err != nil {
		return fmt.Errorf("failed to cleanup Kimi cache for session %s: %w", sessionID, err)
	}

	log.Printf("[DEBUG] Kimi cache cleaned up for session: %s", sessionID)
	return nil
}

// PrepareRequestWithCache prepares the request to use Kimi cache via Headers
// Only validates the cacheable prefix, allowing the rest to vary
func (kcm *KimiCacheManager) PrepareRequestWithCache(sessionID string, req *ChatRequest) map[string]string {
	cache, exists := kcm.GetCache(sessionID)
	if !exists {
		return nil // No cache available
	}

	// 提取当前请求的可缓存前缀
	currentCacheablePrefix := kcm.extractCacheablePrefix(req.Messages)
	
	// 验证可缓存前缀是否与缓存匹配
	if !kcm.messagesMatch(currentCacheablePrefix, cache.CachedMessages) {
		return nil
	}
	
	// 验证工具是否匹配
	if !kcm.toolsMatch(req.Tools, cache.CachedTools) {
		return nil
	}

	// Mark cache as used
	kcm.UseCache(sessionID)

	// Prepare headers for cache usage
	headers := map[string]string{
		"X-Msh-Context-Cache": cache.CacheID,
		"X-Msh-Context-Cache-Reset-TTL": "600", // 重置缓存过期时间为10分钟
	}
	
	return headers
}