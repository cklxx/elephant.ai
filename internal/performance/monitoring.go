package performance

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// AlertLevel defines the severity of performance alerts
type AlertLevel int

const (
	AlertInfo AlertLevel = iota
	AlertWarning
	AlertError
	AlertCritical
)

// Alert represents a performance alert
type Alert struct {
	ID          string                 `json:"id"`
	Level       AlertLevel             `json:"level"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Metrics     PerformanceMetrics     `json:"metrics"`
	Threshold   map[string]interface{} `json:"threshold"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// MonitoringConfig defines monitoring system configuration
type MonitoringConfig struct {
	// Monitoring intervals
	CollectionInterval    time.Duration `json:"collection_interval"`
	AnalysisInterval     time.Duration `json:"analysis_interval"`
	AlertCooldown        time.Duration `json:"alert_cooldown"`
	
	// Thresholds for alerting
	ResponseTimeThreshold     time.Duration `json:"response_time_threshold"`
	MemoryUsageThreshold      int64         `json:"memory_usage_threshold"`
	ThroughputThreshold       float64       `json:"throughput_threshold"`
	ErrorRateThreshold        float64       `json:"error_rate_threshold"`
	
	// Regression detection
	RegressionWindow          time.Duration `json:"regression_window"`
	RegressionThreshold       float64       `json:"regression_threshold"`
	MinDataPointsForRegression int          `json:"min_data_points_for_regression"`
	
	// Rollback configuration
	AutoRollbackEnabled       bool          `json:"auto_rollback_enabled"`
	RollbackThreshold         float64       `json:"rollback_threshold"`
	RollbackConfirmationTime  time.Duration `json:"rollback_confirmation_time"`
	
	// Dashboard and reporting
	DashboardEnabled          bool          `json:"dashboard_enabled"`
	ReportingEnabled          bool          `json:"reporting_enabled"`
	HistoryRetentionDays      int           `json:"history_retention_days"`
}

// PerformanceMonitor provides comprehensive performance monitoring and alerting
type PerformanceMonitor struct {
	config          *MonitoringConfig
	framework       *VerificationFramework
	abTestManager   *ABTestManager
	
	// Alert management
	alerts          map[string]*Alert
	alertHandlers   []AlertHandler
	alertMutex      sync.RWMutex
	
	// Monitoring state
	baseline        *PerformanceMetrics
	history         []PerformanceMetrics
	historyMutex    sync.RWMutex
	
	// Control
	ctx             context.Context
	cancel          context.CancelFunc
	stopChan        chan bool
	
	// Statistics
	stats           MonitoringStats
	statsMutex      sync.RWMutex
}

// MonitoringStats tracks monitoring system statistics
type MonitoringStats struct {
	TotalAlerts         int       `json:"total_alerts"`
	AlertsByLevel       map[AlertLevel]int `json:"alerts_by_level"`
	RegressionsDetected int       `json:"regressions_detected"`
	RollbacksTriggered  int       `json:"rollbacks_triggered"`
	UptimeStart         time.Time `json:"uptime_start"`
	LastCollection      time.Time `json:"last_collection"`
	DataPointsCollected int       `json:"data_points_collected"`
}

// AlertHandler defines interface for handling performance alerts
type AlertHandler interface {
	HandleAlert(alert *Alert) error
}

// NewPerformanceMonitor creates a new performance monitoring system
func NewPerformanceMonitor(config *MonitoringConfig, framework *VerificationFramework) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &PerformanceMonitor{
		config:        config,
		framework:     framework,
		abTestManager: NewABTestManager(),
		alerts:        make(map[string]*Alert),
		alertHandlers: make([]AlertHandler, 0),
		history:       make([]PerformanceMetrics, 0),
		stopChan:      make(chan bool, 1),
		ctx:           ctx,
		cancel:        cancel,
		stats: MonitoringStats{
			AlertsByLevel:       make(map[AlertLevel]int),
			UptimeStart:         time.Now(),
			DataPointsCollected: 0,
		},
	}
}

// Start begins the monitoring system
func (pm *PerformanceMonitor) Start() error {
	// Load baseline from framework
	if err := pm.framework.LoadBaseline(); err != nil {
		return fmt.Errorf("failed to load baseline: %v", err)
	}
	
	pm.baseline = pm.framework.GetBaseline()
	
	// Start monitoring loops
	go pm.collectionLoop()
	go pm.analysisLoop()
	go pm.cleanupLoop()
	
	log.Println("Performance monitoring started")
	return nil
}

// Stop shuts down the monitoring system
func (pm *PerformanceMonitor) Stop() {
	pm.cancel()
	pm.stopChan <- true
	log.Println("Performance monitoring stopped")
}

// collectionLoop runs periodic metric collection
func (pm *PerformanceMonitor) collectionLoop() {
	ticker := time.NewTicker(pm.config.CollectionInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.collectAndStore()
		case <-pm.stopChan:
			return
		case <-pm.ctx.Done():
			return
		}
	}
}

// analysisLoop runs periodic performance analysis
func (pm *PerformanceMonitor) analysisLoop() {
	ticker := time.NewTicker(pm.config.AnalysisInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.analyzePerformance()
		case <-pm.stopChan:
			return
		case <-pm.ctx.Done():
			return
		}
	}
}

// cleanupLoop performs periodic cleanup of old data and alerts
func (pm *PerformanceMonitor) cleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour) // Daily cleanup
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.cleanup()
		case <-pm.stopChan:
			return
		case <-pm.ctx.Done():
			return
		}
	}
}

// collectAndStore collects current performance metrics and stores them
func (pm *PerformanceMonitor) collectAndStore() {
	metrics := pm.framework.collectMetrics()
	
	pm.historyMutex.Lock()
	pm.history = append(pm.history, metrics)
	
	// Keep only recent history to prevent memory growth
	maxHistorySize := 10000
	if len(pm.history) > maxHistorySize {
		pm.history = pm.history[len(pm.history)-maxHistorySize:]
	}
	pm.historyMutex.Unlock()
	
	// Update statistics
	pm.statsMutex.Lock()
	pm.stats.LastCollection = time.Now()
	pm.stats.DataPointsCollected++
	pm.statsMutex.Unlock()
	
	// Check for immediate alerts
	pm.checkThresholds(metrics)
}

// analyzePerformance performs deeper analysis of collected metrics
func (pm *PerformanceMonitor) analyzePerformance() {
	pm.historyMutex.RLock()
	recentHistory := make([]PerformanceMetrics, len(pm.history))
	copy(recentHistory, pm.history)
	pm.historyMutex.RUnlock()
	
	if len(recentHistory) < pm.config.MinDataPointsForRegression {
		return
	}
	
	// Check for performance regressions
	pm.detectRegressions(recentHistory)
	
	// Update dashboard data if enabled
	if pm.config.DashboardEnabled {
		pm.updateDashboard(recentHistory)
	}
}

// checkThresholds checks if current metrics exceed configured thresholds
func (pm *PerformanceMonitor) checkThresholds(metrics PerformanceMetrics) {
	// Response time threshold
	if metrics.ResponseTime > pm.config.ResponseTimeThreshold {
		pm.createAlert(AlertError, "High Response Time", 
			fmt.Sprintf("Response time %.2fms exceeds threshold %.2fms", 
				float64(metrics.ResponseTime.Nanoseconds())/1e6, 
				float64(pm.config.ResponseTimeThreshold.Nanoseconds())/1e6),
			metrics, map[string]interface{}{"threshold": pm.config.ResponseTimeThreshold})
	}
	
	// Memory usage threshold
	if metrics.HeapSize > pm.config.MemoryUsageThreshold {
		pm.createAlert(AlertWarning, "High Memory Usage",
			fmt.Sprintf("Memory usage %dMB exceeds threshold %dMB",
				metrics.HeapSize/1024/1024,
				pm.config.MemoryUsageThreshold/1024/1024),
			metrics, map[string]interface{}{"threshold": pm.config.MemoryUsageThreshold})
	}
	
	// Throughput threshold
	if metrics.ThroughputOps < pm.config.ThroughputThreshold {
		pm.createAlert(AlertWarning, "Low Throughput",
			fmt.Sprintf("Throughput %.2f ops/sec below threshold %.2f ops/sec",
				metrics.ThroughputOps, pm.config.ThroughputThreshold),
			metrics, map[string]interface{}{"threshold": pm.config.ThroughputThreshold})
	}
	
	// Error rate threshold
	if metrics.ErrorRate > pm.config.ErrorRateThreshold {
		pm.createAlert(AlertError, "High Error Rate",
			fmt.Sprintf("Error rate %.4f exceeds threshold %.4f",
				metrics.ErrorRate, pm.config.ErrorRateThreshold),
			metrics, map[string]interface{}{"threshold": pm.config.ErrorRateThreshold})
	}
}

// detectRegressions analyzes trends to detect performance regressions
func (pm *PerformanceMonitor) detectRegressions(history []PerformanceMetrics) {
	if pm.baseline == nil || len(history) < pm.config.MinDataPointsForRegression {
		return
	}
	
	// Analyze recent window
	windowStart := time.Now().Add(-pm.config.RegressionWindow)
	recentMetrics := make([]PerformanceMetrics, 0)
	
	for _, metric := range history {
		if metric.Timestamp.After(windowStart) {
			recentMetrics = append(recentMetrics, metric)
		}
	}
	
	if len(recentMetrics) == 0 {
		return
	}
	
	// Calculate average metrics for the recent window
	avgResponseTime := time.Duration(0)
	avgMemoryUsage := int64(0)
	avgThroughput := 0.0
	avgErrorRate := 0.0
	
	for _, metric := range recentMetrics {
		avgResponseTime += metric.ResponseTime
		avgMemoryUsage += metric.HeapSize
		avgThroughput += metric.ThroughputOps
		avgErrorRate += metric.ErrorRate
	}
	
	count := len(recentMetrics)
	avgResponseTime /= time.Duration(count)
	avgMemoryUsage /= int64(count)
	avgThroughput /= float64(count)
	avgErrorRate /= float64(count)
	
	// Compare with baseline
	responseTimeRegression := float64(avgResponseTime-pm.baseline.ResponseTime) / float64(pm.baseline.ResponseTime)
	memoryRegression := float64(avgMemoryUsage-pm.baseline.HeapSize) / float64(pm.baseline.HeapSize)
	throughputRegression := (pm.baseline.ThroughputOps - avgThroughput) / pm.baseline.ThroughputOps
	
	// Check for significant regressions
	if responseTimeRegression > pm.config.RegressionThreshold {
		pm.createAlert(AlertCritical, "Performance Regression Detected",
			fmt.Sprintf("Response time degraded by %.1f%% over %v window",
				responseTimeRegression*100, pm.config.RegressionWindow),
			recentMetrics[len(recentMetrics)-1],
			map[string]interface{}{
				"regression_percentage": responseTimeRegression * 100,
				"window": pm.config.RegressionWindow.String(),
			})
		
		pm.statsMutex.Lock()
		pm.stats.RegressionsDetected++
		pm.statsMutex.Unlock()
		
		// Consider rollback
		if pm.config.AutoRollbackEnabled && responseTimeRegression > pm.config.RollbackThreshold {
			pm.considerRollback("response time regression", responseTimeRegression)
		}
	}
	
	if memoryRegression > pm.config.RegressionThreshold {
		pm.createAlert(AlertError, "Memory Usage Regression",
			fmt.Sprintf("Memory usage increased by %.1f%% over %v window",
				memoryRegression*100, pm.config.RegressionWindow),
			recentMetrics[len(recentMetrics)-1],
			map[string]interface{}{
				"regression_percentage": memoryRegression * 100,
				"window": pm.config.RegressionWindow.String(),
			})
	}
	
	if throughputRegression > pm.config.RegressionThreshold {
		pm.createAlert(AlertError, "Throughput Regression",
			fmt.Sprintf("Throughput decreased by %.1f%% over %v window",
				throughputRegression*100, pm.config.RegressionWindow),
			recentMetrics[len(recentMetrics)-1],
			map[string]interface{}{
				"regression_percentage": throughputRegression * 100,
				"window": pm.config.RegressionWindow.String(),
			})
	}
}

// createAlert creates and processes a new alert
func (pm *PerformanceMonitor) createAlert(level AlertLevel, title, description string, 
	metrics PerformanceMetrics, threshold map[string]interface{}) {
	
	alertID := fmt.Sprintf("%s_%d", title, time.Now().Unix())
	
	alert := &Alert{
		ID:          alertID,
		Level:       level,
		Title:       title,
		Description: description,
		Metrics:     metrics,
		Threshold:   threshold,
		Timestamp:   time.Now(),
		Resolved:    false,
	}
	
	pm.alertMutex.Lock()
	pm.alerts[alertID] = alert
	pm.alertMutex.Unlock()
	
	// Update statistics
	pm.statsMutex.Lock()
	pm.stats.TotalAlerts++
	pm.stats.AlertsByLevel[level]++
	pm.statsMutex.Unlock()
	
	// Send alert to handlers
	for _, handler := range pm.alertHandlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(alert); err != nil {
				log.Printf("Alert handler error: %v", err)
			}
		}(handler)
	}
}

// considerRollback evaluates whether to trigger an automatic rollback
func (pm *PerformanceMonitor) considerRollback(reason string, severity float64) {
	log.Printf("Considering rollback due to %s (severity: %.2f%%)", reason, severity*100)
	
	// Wait for confirmation period
	time.Sleep(pm.config.RollbackConfirmationTime)
	
	// Re-check if the issue persists
	current := pm.framework.GetCurrentMetrics()
	if current != nil && pm.baseline != nil {
		currentSeverity := float64(current.ResponseTime-pm.baseline.ResponseTime) / float64(pm.baseline.ResponseTime)
		
		if currentSeverity > pm.config.RollbackThreshold {
			pm.triggerRollback(reason, currentSeverity)
		} else {
			log.Printf("Rollback cancelled: issue appears to have resolved (current severity: %.2f%%)", 
				currentSeverity*100)
		}
	}
}

// triggerRollback executes the rollback procedure
func (pm *PerformanceMonitor) triggerRollback(reason string, severity float64) {
	log.Printf("TRIGGERING ROLLBACK: %s (severity: %.2f%%)", reason, severity*100)
	
	pm.createAlert(AlertCritical, "Automatic Rollback Triggered",
		fmt.Sprintf("Automatic rollback triggered due to %s (severity: %.1f%%)",
			reason, severity*100),
		*pm.framework.GetCurrentMetrics(),
		map[string]interface{}{
			"reason": reason,
			"severity": severity * 100,
			"automatic": true,
		})
	
	pm.statsMutex.Lock()
	pm.stats.RollbacksTriggered++
	pm.statsMutex.Unlock()
	
	// Here you would integrate with your deployment system to trigger rollback
	pm.framework.triggerRollback()
}

// AddAlertHandler adds a new alert handler
func (pm *PerformanceMonitor) AddAlertHandler(handler AlertHandler) {
	pm.alertHandlers = append(pm.alertHandlers, handler)
}

// GetActiveAlerts returns currently unresolved alerts
func (pm *PerformanceMonitor) GetActiveAlerts() []*Alert {
	pm.alertMutex.RLock()
	defer pm.alertMutex.RUnlock()
	
	active := make([]*Alert, 0)
	for _, alert := range pm.alerts {
		if !alert.Resolved {
			active = append(active, alert)
		}
	}
	
	return active
}

// ResolveAlert marks an alert as resolved
func (pm *PerformanceMonitor) ResolveAlert(alertID string) error {
	pm.alertMutex.Lock()
	defer pm.alertMutex.Unlock()
	
	alert, exists := pm.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}
	
	now := time.Now()
	alert.Resolved = true
	alert.ResolvedAt = &now
	
	return nil
}

// GetMonitoringStats returns current monitoring statistics
func (pm *PerformanceMonitor) GetMonitoringStats() MonitoringStats {
	pm.statsMutex.RLock()
	defer pm.statsMutex.RUnlock()
	
	// Create a copy to avoid race conditions
	stats := pm.stats
	stats.AlertsByLevel = make(map[AlertLevel]int)
	for level, count := range pm.stats.AlertsByLevel {
		stats.AlertsByLevel[level] = count
	}
	
	return stats
}

// updateDashboard updates dashboard data (placeholder for dashboard integration)
func (pm *PerformanceMonitor) updateDashboard(history []PerformanceMetrics) {
	// This would integrate with a dashboard system to provide real-time monitoring
	log.Printf("Dashboard updated with %d data points", len(history))
}

// cleanup removes old data and resolved alerts
func (pm *PerformanceMonitor) cleanup() {
	// Clean up old alerts
	pm.alertMutex.Lock()
	cutoff := time.Now().Add(-time.Duration(pm.config.HistoryRetentionDays) * 24 * time.Hour)
	for id, alert := range pm.alerts {
		if alert.Resolved && alert.ResolvedAt != nil && alert.ResolvedAt.Before(cutoff) {
			delete(pm.alerts, id)
		}
	}
	pm.alertMutex.Unlock()
	
	// Clean up old history
	pm.historyMutex.Lock()
	if len(pm.history) > 0 {
		newHistory := make([]PerformanceMetrics, 0)
		for _, metric := range pm.history {
			if metric.Timestamp.After(cutoff) {
				newHistory = append(newHistory, metric)
			}
		}
		pm.history = newHistory
	}
	pm.historyMutex.Unlock()
	
	log.Printf("Cleanup completed: removed data older than %d days", pm.config.HistoryRetentionDays)
}