package swe_bench

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"alex/internal/async"
)

// ProgressReporterImpl implements the ProgressReporter interface
type ProgressReporterImpl struct {
	output     io.Writer
	ticker     *time.Ticker
	stopChan   chan struct{}
	mu         sync.RWMutex
	lastUpdate ProgressUpdate
	isRunning  bool
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter() *ProgressReporterImpl {
	return &ProgressReporterImpl{
		output:   os.Stdout,
		stopChan: make(chan struct{}),
	}
}

// Start starts progress reporting
func (pr *ProgressReporterImpl) Start(ctx context.Context) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pr.isRunning {
		return fmt.Errorf("progress reporter already running")
	}

	pr.ticker = time.NewTicker(10 * time.Second)
	pr.isRunning = true

	async.Go(panicLogger{}, "swe-bench.progress", func() {
		pr.reportingLoop(ctx)
	})

	return nil
}

// Stop stops progress reporting
func (pr *ProgressReporterImpl) Stop() error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if !pr.isRunning {
		return nil
	}

	select {
	case <-pr.stopChan:
		// Channel already closed
	default:
		close(pr.stopChan)
	}

	if pr.ticker != nil {
		pr.ticker.Stop()
		pr.ticker = nil
	}
	pr.isRunning = false

	return nil
}

// Update updates the progress
func (pr *ProgressReporterImpl) Update(update ProgressUpdate) error {
	pr.mu.Lock()
	pr.lastUpdate = update
	pr.mu.Unlock()

	return nil
}

// SetOutput sets the output writer for progress updates
func (pr *ProgressReporterImpl) SetOutput(w io.Writer) {
	pr.mu.Lock()
	pr.output = w
	pr.mu.Unlock()
}

// reportingLoop is the main loop for progress reporting
func (pr *ProgressReporterImpl) reportingLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pr.stopChan:
			return
		case <-func() <-chan time.Time {
			pr.mu.RLock()
			ticker := pr.ticker
			pr.mu.RUnlock()
			if ticker != nil {
				return ticker.C
			}
			return make(chan time.Time) // Return a channel that will never send
		}():
			pr.mu.RLock()
			update := pr.lastUpdate
			output := pr.output
			pr.mu.RUnlock()

			if update.Total > 0 {
				progress := float64(update.Completed+update.Failed) / float64(update.Total) * 100

				if _, err := fmt.Fprintf(output, "[%s] Progress: %.1f%% (%d/%d) | Completed: %d | Failed: %d | Running: %d | Success Rate: %.1f%% | Avg Duration: %s | ETA: %s\n",
					update.Timestamp.Format("15:04:05"),
					progress,
					update.Completed+update.Failed,
					update.Total,
					update.Completed,
					update.Failed,
					update.Running,
					update.SuccessRate,
					update.AvgDuration.String(),
					update.EstimatedETA.String(),
				); err != nil {
					log.Printf("Warning: Failed to write progress: %v", err)
				}
			}
		}
	}
}

// MonitorImpl implements the Monitor interface
type MonitorImpl struct {
	metrics   map[string]float64
	events    []MonitorEvent
	mu        sync.RWMutex
	isRunning bool
	stopChan  chan struct{}

	// File logging
	logFile *os.File
	logPath string
}

// MonitorEvent represents a monitoring event
type MonitorEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Event     string                 `json:"event"`
	Data      map[string]interface{} `json:"data"`
}

// NewMonitor creates a new monitor
func NewMonitor() *MonitorImpl {
	return &MonitorImpl{
		metrics:  make(map[string]float64),
		events:   make([]MonitorEvent, 0),
		stopChan: make(chan struct{}),
	}
}

// StartMonitoring starts monitoring the batch process
func (m *MonitorImpl) StartMonitoring(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return fmt.Errorf("monitor already running")
	}

	// Setup log file
	homeDir, _ := os.UserHomeDir()
	m.logPath = fmt.Sprintf("%s/.alex/logs/batch_monitor_%s.log", homeDir, time.Now().Format("2006-01-02_15-04-05"))

	if err := os.MkdirAll(fmt.Sprintf("%s/.alex/logs", homeDir), 0755); err != nil {
		log.Printf("Warning: Failed to create log directory: %v", err)
	} else {
		logFile, err := os.Create(m.logPath)
		if err != nil {
			log.Printf("Warning: Failed to create log file: %v", err)
		} else {
			m.logFile = logFile
		}
	}

	m.isRunning = true

	// Start monitoring goroutine
	async.Go(panicLogger{}, "swe-bench.monitor", func() {
		m.monitoringLoop(ctx)
	})

	return nil
}

// StopMonitoring stops monitoring
func (m *MonitorImpl) StopMonitoring() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	select {
	case <-m.stopChan:
		// Channel already closed
	default:
		close(m.stopChan)
	}

	m.isRunning = false

	if m.logFile != nil {
		if err := m.logFile.Close(); err != nil {
			log.Printf("Warning: Failed to close log file: %v", err)
		}
	}

	return nil
}

// RecordMetric records a metric
func (m *MonitorImpl) RecordMetric(name string, value float64, tags map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create metric key with tags
	key := name
	if len(tags) > 0 {
		key = fmt.Sprintf("%s{", name)
		first := true
		for k, v := range tags {
			if !first {
				key += ","
			}
			key += fmt.Sprintf("%s=\"%s\"", k, v)
			first = false
		}
		key += "}"
	}

	m.metrics[key] = value

	// Log metric
	if m.logFile != nil {
		logEntry := fmt.Sprintf("[%s] METRIC %s = %.4f\n", time.Now().Format(time.RFC3339), key, value)
		if _, err := m.logFile.WriteString(logEntry); err != nil {
			log.Printf("Warning: Failed to write metric to log: %v", err)
		}
	}

	return nil
}

// RecordEvent records an event
func (m *MonitorImpl) RecordEvent(event string, data map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	monitorEvent := MonitorEvent{
		Timestamp: time.Now(),
		Event:     event,
		Data:      data,
	}

	m.events = append(m.events, monitorEvent)

	// Log event
	if m.logFile != nil {
		logEntry := fmt.Sprintf("[%s] EVENT %s: %+v\n", monitorEvent.Timestamp.Format(time.RFC3339), event, data)
		if _, err := m.logFile.WriteString(logEntry); err != nil {
			log.Printf("Warning: Failed to write event to log: %v", err)
		}
	}

	return nil
}

// GetMetrics returns current metrics
func (m *MonitorImpl) GetMetrics() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy of the metrics
	result := make(map[string]float64)
	for k, v := range m.metrics {
		result[k] = v
	}

	return result
}

// GetEvents returns recorded events
func (m *MonitorImpl) GetEvents() []MonitorEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy of the events
	result := make([]MonitorEvent, len(m.events))
	copy(result, m.events)

	return result
}

// monitoringLoop is the main monitoring loop
func (m *MonitorImpl) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			// Record system metrics
			m.recordSystemMetrics()
		}
	}
}

// recordSystemMetrics records system-level metrics
func (m *MonitorImpl) recordSystemMetrics() {
	// This is a simplified implementation
	// In a real system, you would collect actual system metrics
	tags := map[string]string{
		"component": "batch_processor",
	}

	// Record timestamp
	_ = m.RecordMetric("system_uptime", float64(time.Now().Unix()), tags)

	// Record memory usage (simplified)
	_ = m.RecordMetric("memory_usage_mb", 0, tags) // Would use actual memory measurement

	// Record event
	_ = m.RecordEvent("system_health_check", map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

// GetLogPath returns the path to the log file
func (m *MonitorImpl) GetLogPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.logPath
}

// ExportMetrics exports metrics to a file
func (m *MonitorImpl) ExportMetrics(filePath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create metrics file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close metrics file: %v", err)
		}
	}()

	// Write metrics in Prometheus format
	for key, value := range m.metrics {
		line := fmt.Sprintf("%s %.4f\n", key, value)
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write metric: %w", err)
		}
	}

	return nil
}

// ExportEvents exports events to a file
func (m *MonitorImpl) ExportEvents(filePath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create events file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: Failed to close events file: %v", err)
		}
	}()

	// Write events as JSON lines
	for _, event := range m.events {
		line := fmt.Sprintf("{\"timestamp\":\"%s\",\"event\":\"%s\",\"data\":%+v}\n",
			event.Timestamp.Format(time.RFC3339),
			event.Event,
			event.Data,
		)
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
	}

	return nil
}

// ClearMetrics clears all recorded metrics
func (m *MonitorImpl) ClearMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics = make(map[string]float64)
}

// ClearEvents clears all recorded events
func (m *MonitorImpl) ClearEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = make([]MonitorEvent, 0)
}

// GetStats returns monitoring statistics
func (m *MonitorImpl) GetStats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MonitorStats{
		TotalMetrics: len(m.metrics),
		TotalEvents:  len(m.events),
		IsRunning:    m.isRunning,
		LogPath:      m.logPath,
	}
}

// MonitorStats represents monitoring statistics
type MonitorStats struct {
	TotalMetrics int    `json:"total_metrics"`
	TotalEvents  int    `json:"total_events"`
	IsRunning    bool   `json:"is_running"`
	LogPath      string `json:"log_path"`
}
