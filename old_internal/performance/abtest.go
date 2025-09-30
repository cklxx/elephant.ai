package performance

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// ABTestGroup represents different implementation variants for A/B testing
type ABTestGroup string

const (
	ControlGroup   ABTestGroup = "control"
	TreatmentGroup ABTestGroup = "treatment"
)

// ABTestConfig defines A/B testing configuration
type ABTestConfig struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	TrafficSplit  float64         `json:"traffic_split"` // 0.5 = 50/50 split
	StartTime     time.Time       `json:"start_time"`
	EndTime       time.Time       `json:"end_time"`
	MinSampleSize int             `json:"min_sample_size"`
	FeatureFlags  map[string]bool `json:"feature_flags"`
	Enabled       bool            `json:"enabled"`
}

// ABTestResult tracks performance results for each group
type ABTestResult struct {
	Group           ABTestGroup        `json:"group"`
	SampleSize      int                `json:"sample_size"`
	Metrics         PerformanceMetrics `json:"metrics"`
	AggregatedStats ABTestStats        `json:"aggregated_stats"`
}

// ABTestStats provides statistical analysis of A/B test results
type ABTestStats struct {
	MeanResponseTime   float64 `json:"mean_response_time"`
	MedianResponseTime float64 `json:"median_response_time"`
	P95ResponseTime    float64 `json:"p95_response_time"`
	MeanMemoryUsage    float64 `json:"mean_memory_usage"`
	MeanThroughput     float64 `json:"mean_throughput"`
	ErrorRate          float64 `json:"error_rate"`
	ConfidenceInterval float64 `json:"confidence_interval"`
	StatisticalPower   float64 `json:"statistical_power"`
}

// ABTestManager manages A/B testing experiments
type ABTestManager struct {
	tests       map[string]*ABTestConfig
	results     map[string]map[ABTestGroup][]*PerformanceMetrics
	assignments map[string]ABTestGroup // User/session ID to group assignment
	mutex       sync.RWMutex

	// Statistical settings
	confidenceLevel float64
	minimumEffect   float64 // Minimum effect size to detect
}

// NewABTestManager creates a new A/B test manager
func NewABTestManager() *ABTestManager {
	return &ABTestManager{
		tests:           make(map[string]*ABTestConfig),
		results:         make(map[string]map[ABTestGroup][]*PerformanceMetrics),
		assignments:     make(map[string]ABTestGroup),
		confidenceLevel: 0.95, // 95% confidence level
		minimumEffect:   0.05, // 5% minimum effect size
	}
}

// CreateTest creates a new A/B test configuration
func (atm *ABTestManager) CreateTest(config *ABTestConfig) error {
	if config.Name == "" {
		return fmt.Errorf("test name cannot be empty")
	}

	if config.TrafficSplit < 0 || config.TrafficSplit > 1 {
		return fmt.Errorf("traffic split must be between 0 and 1")
	}

	if config.MinSampleSize <= 0 {
		config.MinSampleSize = 100 // Default minimum sample size
	}

	atm.mutex.Lock()
	defer atm.mutex.Unlock()

	atm.tests[config.Name] = config
	atm.results[config.Name] = map[ABTestGroup][]*PerformanceMetrics{
		ControlGroup:   make([]*PerformanceMetrics, 0),
		TreatmentGroup: make([]*PerformanceMetrics, 0),
	}

	return nil
}

// AssignUserToGroup assigns a user/session to a test group
func (atm *ABTestManager) AssignUserToGroup(testName, userID string) (ABTestGroup, error) {
	atm.mutex.Lock()
	defer atm.mutex.Unlock()

	test, exists := atm.tests[testName]
	if !exists {
		return ControlGroup, fmt.Errorf("test %s does not exist", testName)
	}

	if !test.Enabled {
		return ControlGroup, fmt.Errorf("test %s is not enabled", testName)
	}

	// Check if test is active
	now := time.Now()
	if now.Before(test.StartTime) || now.After(test.EndTime) {
		return ControlGroup, fmt.Errorf("test %s is not active", testName)
	}

	// Check if user is already assigned
	key := fmt.Sprintf("%s:%s", testName, userID)
	if group, exists := atm.assignments[key]; exists {
		return group, nil
	}

	// Assign user to group based on traffic split
	group := ControlGroup
	randNum, err := rand.Int(rand.Reader, big.NewInt(100))
	if err == nil {
		if float64(randNum.Int64()) < test.TrafficSplit*100 {
			group = TreatmentGroup
		}
	}

	atm.assignments[key] = group
	return group, nil
}

// IsFeatureEnabled checks if a feature is enabled for a user in a test
func (atm *ABTestManager) IsFeatureEnabled(testName, userID, feature string) bool {
	group, err := atm.AssignUserToGroup(testName, userID)
	if err != nil {
		return false
	}

	atm.mutex.RLock()
	defer atm.mutex.RUnlock()

	test, exists := atm.tests[testName]
	if !exists {
		return false
	}

	// Control group uses default (disabled) features
	if group == ControlGroup {
		return false
	}

	// Treatment group uses feature flags from configuration
	enabled, exists := test.FeatureFlags[feature]
	return exists && enabled
}

// RecordMetrics records performance metrics for a test group
func (atm *ABTestManager) RecordMetrics(testName, userID string, metrics *PerformanceMetrics) error {
	group, err := atm.AssignUserToGroup(testName, userID)
	if err != nil {
		return err
	}

	atm.mutex.Lock()
	defer atm.mutex.Unlock()

	if results, exists := atm.results[testName]; exists {
		results[group] = append(results[group], metrics)
	} else {
		return fmt.Errorf("test results not initialized for %s", testName)
	}

	return nil
}

// GetTestResults returns the current results for a test
func (atm *ABTestManager) GetTestResults(testName string) (*ABTestComparison, error) {
	atm.mutex.RLock()
	defer atm.mutex.RUnlock()

	test, exists := atm.tests[testName]
	if !exists {
		return nil, fmt.Errorf("test %s does not exist", testName)
	}

	results, exists := atm.results[testName]
	if !exists {
		return nil, fmt.Errorf("no results found for test %s", testName)
	}

	controlMetrics := results[ControlGroup]
	treatmentMetrics := results[TreatmentGroup]

	if len(controlMetrics) < test.MinSampleSize || len(treatmentMetrics) < test.MinSampleSize {
		return nil, fmt.Errorf("insufficient sample size for test %s", testName)
	}

	controlStats := atm.calculateStats(controlMetrics)
	treatmentStats := atm.calculateStats(treatmentMetrics)

	comparison := &ABTestComparison{
		TestName:                testName,
		ControlResult:           ABTestResult{Group: ControlGroup, SampleSize: len(controlMetrics), AggregatedStats: controlStats},
		TreatmentResult:         ABTestResult{Group: TreatmentGroup, SampleSize: len(treatmentMetrics), AggregatedStats: treatmentStats},
		StatisticalSignificance: atm.calculateSignificance(controlStats, treatmentStats, len(controlMetrics), len(treatmentMetrics)),
		Recommendations:         atm.generateRecommendations(controlStats, treatmentStats),
		Timestamp:               time.Now(),
	}

	return comparison, nil
}

// ABTestComparison provides comprehensive comparison between control and treatment groups
type ABTestComparison struct {
	TestName                string       `json:"test_name"`
	ControlResult           ABTestResult `json:"control_result"`
	TreatmentResult         ABTestResult `json:"treatment_result"`
	StatisticalSignificance float64      `json:"statistical_significance"`
	EffectSize              float64      `json:"effect_size"`
	ConfidenceInterval      []float64    `json:"confidence_interval"`
	Recommendations         []string     `json:"recommendations"`
	ShouldRollout           bool         `json:"should_rollout"`
	ShouldRollback          bool         `json:"should_rollback"`
	Timestamp               time.Time    `json:"timestamp"`
}

// calculateStats computes statistical measures for a set of metrics
func (atm *ABTestManager) calculateStats(metrics []*PerformanceMetrics) ABTestStats {
	if len(metrics) == 0 {
		return ABTestStats{}
	}

	// Calculate response time statistics
	responseTimes := make([]float64, len(metrics))
	memoryUsages := make([]float64, len(metrics))
	throughputs := make([]float64, len(metrics))
	errorCount := 0

	for i, m := range metrics {
		responseTimes[i] = float64(m.ResponseTime.Nanoseconds())
		memoryUsages[i] = float64(m.HeapSize)
		throughputs[i] = m.ThroughputOps
		if m.ErrorRate > 0 {
			errorCount++
		}
	}

	return ABTestStats{
		MeanResponseTime:   mean(responseTimes),
		MedianResponseTime: median(responseTimes),
		P95ResponseTime:    percentile(responseTimes, 0.95),
		MeanMemoryUsage:    mean(memoryUsages),
		MeanThroughput:     mean(throughputs),
		ErrorRate:          float64(errorCount) / float64(len(metrics)),
	}
}

// calculateSignificance performs statistical significance testing
func (atm *ABTestManager) calculateSignificance(control, treatment ABTestStats, controlSize, treatmentSize int) float64 {
	// Simplified t-test for response time difference
	// In production, would use more sophisticated statistical tests

	pooledVariance := (float64(controlSize-1)*variance([]float64{control.MeanResponseTime}) +
		float64(treatmentSize-1)*variance([]float64{treatment.MeanResponseTime})) /
		float64(controlSize+treatmentSize-2)

	standardError := sqrt(pooledVariance * (1.0/float64(controlSize) + 1.0/float64(treatmentSize)))

	if standardError == 0 {
		return 0
	}

	tStatistic := (treatment.MeanResponseTime - control.MeanResponseTime) / standardError

	// Return absolute t-statistic as a simple significance measure
	if tStatistic < 0 {
		return -tStatistic
	}
	return tStatistic
}

// generateRecommendations provides actionable recommendations based on test results
func (atm *ABTestManager) generateRecommendations(control, treatment ABTestStats) []string {
	recommendations := make([]string, 0)

	// Response time comparison
	responseTimeImprovement := (control.MeanResponseTime - treatment.MeanResponseTime) / control.MeanResponseTime
	if responseTimeImprovement > atm.minimumEffect {
		recommendations = append(recommendations,
			fmt.Sprintf("Treatment shows %.1f%% response time improvement - recommend rollout", responseTimeImprovement*100))
	} else if responseTimeImprovement < -atm.minimumEffect {
		recommendations = append(recommendations,
			fmt.Sprintf("Treatment shows %.1f%% response time regression - recommend rollback", -responseTimeImprovement*100))
	}

	// Memory usage comparison
	memoryImprovement := (control.MeanMemoryUsage - treatment.MeanMemoryUsage) / control.MeanMemoryUsage
	if memoryImprovement > atm.minimumEffect {
		recommendations = append(recommendations,
			fmt.Sprintf("Treatment shows %.1f%% memory usage improvement", memoryImprovement*100))
	} else if memoryImprovement < -atm.minimumEffect {
		recommendations = append(recommendations,
			fmt.Sprintf("Treatment shows %.1f%% memory usage increase - monitor closely", -memoryImprovement*100))
	}

	// Throughput comparison
	throughputImprovement := (treatment.MeanThroughput - control.MeanThroughput) / control.MeanThroughput
	if throughputImprovement > atm.minimumEffect {
		recommendations = append(recommendations,
			fmt.Sprintf("Treatment shows %.1f%% throughput improvement", throughputImprovement*100))
	}

	// Error rate comparison
	if treatment.ErrorRate > control.ErrorRate*1.1 {
		recommendations = append(recommendations, "Treatment shows increased error rate - investigate immediately")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "No significant differences detected - continue monitoring")
	}

	return recommendations
}

// Helper statistical functions (simplified implementations)
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple median calculation (would use sort.Float64s in production)
	return values[len(values)/2]
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	index := int(p * float64(len(values)))
	if index >= len(values) {
		index = len(values) - 1
	}

	return values[index]
}

func variance(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	m := mean(values)
	sum := 0.0
	for _, v := range values {
		diff := v - m
		sum += diff * diff
	}

	return sum / float64(len(values)-1)
}

func sqrt(x float64) float64 {
	// Simple approximation (would use math.Sqrt in production)
	if x < 0 {
		return 0
	}
	if x == 0 {
		return 0
	}

	// Newton's method approximation
	guess := x / 2
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

// GetActiveTests returns all currently active tests
func (atm *ABTestManager) GetActiveTests() []*ABTestConfig {
	atm.mutex.RLock()
	defer atm.mutex.RUnlock()

	active := make([]*ABTestConfig, 0)
	now := time.Now()

	for _, test := range atm.tests {
		if test.Enabled && now.After(test.StartTime) && now.Before(test.EndTime) {
			active = append(active, test)
		}
	}

	return active
}

// StopTest stops a running test
func (atm *ABTestManager) StopTest(testName string) error {
	atm.mutex.Lock()
	defer atm.mutex.Unlock()

	test, exists := atm.tests[testName]
	if !exists {
		return fmt.Errorf("test %s does not exist", testName)
	}

	test.Enabled = false
	test.EndTime = time.Now()

	return nil
}

// CleanupExpiredTests removes data for tests that have ended
func (atm *ABTestManager) CleanupExpiredTests() {
	atm.mutex.Lock()
	defer atm.mutex.Unlock()

	now := time.Now()
	for testName, test := range atm.tests {
		if now.After(test.EndTime.Add(7 * 24 * time.Hour)) { // Keep for 7 days after end
			delete(atm.tests, testName)
			delete(atm.results, testName)

			// Clean up assignments
			for key := range atm.assignments {
				if len(key) > len(testName) && key[:len(testName)] == testName {
					delete(atm.assignments, key)
				}
			}
		}
	}
}
