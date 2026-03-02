//go:build ignore
package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// LearningMemory stores learned patterns and successful strategies
type LearningMemory struct {
	Patterns          map[string]*LearnedPattern   `json:"patterns"`
	StrategyOutcomes  map[string]*StrategyOutcome    `json:"strategy_outcomes"`
	PromptTemplates   map[string]*PromptTemplate     `json:"prompt_templates"`
	DomainKnowledge   map[string]*DomainKnowledge    `json:"domain_knowledge"`
	Version           int                            `json:"version"`
	LastEvolutionTime time.Time                      `json:"last_evolution_time"`
	TotalIterations   int                            `json:"total_iterations"`
	mu                sync.RWMutex                   `json:"-"`
}

// LearnedPattern represents a pattern learned from successful/failed executions
type LearnedPattern struct {
	ID              string            `json:"id"`
	Domain          string            `json:"domain"`
	PatternType     PatternType       `json:"pattern_type"`
	Description     string            `json:"description"`
	SuccessCount    int               `json:"success_count"`
	FailureCount    int               `json:"failure_count"`
	Confidence      float64           `json:"confidence"`
	ContextMatchers []string          `json:"context_matchers"`
	OutcomeHints    map[string]string `json:"outcome_hints"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// StrategyOutcome tracks the effectiveness of different strategies
type StrategyOutcome struct {
	StrategyID    string            `json:"strategy_id"`
	StrategyName  string            `json:"strategy_name"`
	UsedCount     int               `json:"used_count"`
	SuccessCount  int               `json:"success_count"`
	AvgQuality    float64           `json:"avg_quality"`
	ContextScores map[string]float64 `json:"context_scores"`
	LastUsed      time.Time         `json:"last_used"`
}

// PromptTemplate stores optimized prompt templates
type PromptTemplate struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Template        string            `json:"template"`
	Variables       []string          `json:"variables"`
	Domain          string            `json:"domain"`
	SuccessRate     float64           `json:"success_rate"`
	UsageCount      int               `json:"usage_count"`
	QualityHistory  []float64         `json:"quality_history"`
	EvolutionHistory []EvolutionStep  `json:"evolution_history"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// EvolutionStep tracks a single evolution of a prompt
type EvolutionStep struct {
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	ChangeType  string    `json:"change_type"`
	Rationale   string    `json:"rationale"`
	Timestamp   time.Time `json:"timestamp"`
}

// DomainKnowledge represents domain-specific knowledge
type DomainKnowledge struct {
	Domain         string            `json:"domain"`
	Facts          []string          `json:"facts"`
	Heuristics     []Heuristic       `json:"heuristics"`
	CommonErrors   []CommonError     `json:"common_errors"`
	BestPractices  []string          `json:"best_practices"`
	LastUpdated    time.Time         `json:"last_updated"`
	Confidence     float64           `json:"confidence"`
}

// Heuristic represents a learned heuristic
type Heuristic struct {
	ID          string  `json:"id"`
	Condition   string  `json:"condition"`
	Action      string  `json:"action"`
	Priority    int     `json:"priority"`
	SuccessRate float64 `json:"success_rate"`
	UsageCount  int     `json:"usage_count"`
}

// CommonError represents a learned common error pattern
type CommonError struct {
	ErrorPattern    string   `json:"error_pattern"`
	RootCause       string   `json:"root_cause"`
	Solution        string   `json:"solution"`
	PreventionHints []string `json:"prevention_hints"`
	OccurrenceCount int      `json:"occurrence_count"`
}

// PatternType categorizes learned patterns
type PatternType string

const (
	PatternTypeCode       PatternType = "code"
	PatternTypeReasoning  PatternType = "reasoning"
	PatternTypeToolUse    PatternType = "tool_use"
	PatternTypeResponse   PatternType = "response"
	PatternTypePlanning   PatternType = "planning"
	PatternTypeError      PatternType = "error"
)

// NewLearningMemory creates a new learning memory
func NewLearningMemory() *LearningMemory {
	return &LearningMemory{
		Patterns:         make(map[string]*LearnedPattern),
		StrategyOutcomes: make(map[string]*StrategyOutcome),
		PromptTemplates:  make(map[string]*PromptTemplate),
		DomainKnowledge:  make(map[string]*DomainKnowledge),
		Version:          1,
		LastEvolutionTime: time.Now(),
	}
}

// RecordPattern records a learned pattern
func (m *LearningMemory) RecordPattern(pattern *LearnedPattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if pattern.ID == "" {
		pattern.ID = generatePatternID(pattern)
	}
	
	existing, exists := m.Patterns[pattern.ID]
	if exists {
		existing.SuccessCount += pattern.SuccessCount
		existing.FailureCount += pattern.FailureCount
		existing.Confidence = calculateConfidence(existing.SuccessCount, existing.FailureCount)
		existing.UpdatedAt = time.Now()
		mergeOutcomeHints(existing.OutcomeHints, pattern.OutcomeHints)
	} else {
		pattern.Confidence = calculateConfidence(pattern.SuccessCount, pattern.FailureCount)
		pattern.CreatedAt = time.Now()
		pattern.UpdatedAt = time.Now()
		m.Patterns[pattern.ID] = pattern
	}
}

// RecordStrategyOutcome records the outcome of using a strategy
func (m *LearningMemory) RecordStrategyOutcome(strategyID string, success bool, quality float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	outcome, exists := m.StrategyOutcomes[strategyID]
	if !exists {
		return
	}
	
	outcome.UsedCount++
	if success {
		outcome.SuccessCount++
	}
	
	// Update average quality using incremental calculation
	outcome.AvgQuality = (outcome.AvgQuality*float64(outcome.UsedCount-1) + quality) / float64(outcome.UsedCount)
	outcome.LastUsed = time.Now()
}

// UpdatePromptTemplate updates or creates a prompt template
func (m *LearningMemory) UpdatePromptTemplate(template *PromptTemplate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if template.ID == "" {
		template.ID = generateTemplateID(template.Name)
	}
	
	existing, exists := m.PromptTemplates[template.ID]
	if exists {
		// Record evolution
		evolution := EvolutionStep{
			FromVersion: fmt.Sprintf("v%d", len(existing.EvolutionHistory)+1),
			ToVersion:   fmt.Sprintf("v%d", len(existing.EvolutionHistory)+2),
			ChangeType:  "optimization",
			Rationale:   "Improved based on performance feedback",
			Timestamp:   time.Now(),
		}
		existing.EvolutionHistory = append(existing.EvolutionHistory, evolution)
		existing.Template = template.Template
		existing.SuccessRate = template.SuccessRate
		existing.UsageCount++
		existing.QualityHistory = append(existing.QualityHistory, template.SuccessRate)
		existing.UpdatedAt = time.Now()
	} else {
		template.CreatedAt = time.Now()
		template.UpdatedAt = time.Now()
		template.QualityHistory = []float64{template.SuccessRate}
		m.PromptTemplates[template.ID] = template
	}
}

// UpdateDomainKnowledge updates domain-specific knowledge
func (m *LearningMemory) UpdateDomainKnowledge(domain string, updateFn func(*DomainKnowledge)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	dk, exists := m.DomainKnowledge[domain]
	if !exists {
		dk = &DomainKnowledge{
			Domain:      domain,
			Facts:       []string{},
			Heuristics:  []Heuristic{},
			CommonErrors: []CommonError{},
		}
		m.DomainKnowledge[domain] = dk
	}
	
	updateFn(dk)
	dk.LastUpdated = time.Now()
}

// GetRelevantPatterns returns patterns relevant to a context
func (m *LearningMemory) GetRelevantPatterns(domain string, contextHints []string, limit int) []*LearnedPattern {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var relevant []*LearnedPattern
	for _, pattern := range m.Patterns {
		if domain != "" && pattern.Domain != domain {
			continue
		}
		
		score := calculateRelevanceScore(pattern, contextHints)
		if score > 0.3 { // Threshold for relevance
			relevant = append(relevant, pattern)
		}
	}
	
	// Sort by confidence and relevance
	sort.Slice(relevant, func(i, j int) bool {
		return relevant[i].Confidence > relevant[j].Confidence
	})
	
	if limit > 0 && len(relevant) > limit {
		return relevant[:limit]
	}
	return relevant
}

// GetBestStrategy returns the best strategy for a given context
func (m *LearningMemory) GetBestStrategy(contextType string) *StrategyOutcome {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var best *StrategyOutcome
	bestScore := 0.0
	
	for _, outcome := range m.StrategyOutcomes {
		score := outcome.AvgQuality * float64(outcome.SuccessCount) / float64(max(outcome.UsedCount, 1))
		if contextScore, ok := outcome.ContextScores[contextType]; ok {
			score *= (1 + contextScore)
		}
		if score > bestScore {
			bestScore = score
			best = outcome
		}
	}
	
	return best
}

// GetPromptTemplate retrieves a prompt template by ID
func (m *LearningMemory) GetPromptTemplate(id string) *PromptTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.PromptTemplates[id]
}

// SearchPromptTemplates searches for templates matching criteria
func (m *LearningMemory) SearchPromptTemplates(domain string, minSuccessRate float64) []*PromptTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var matches []*PromptTemplate
	for _, template := range m.PromptTemplates {
		if domain != "" && template.Domain != domain {
			continue
		}
		if template.SuccessRate >= minSuccessRate {
			matches = append(matches, template)
		}
	}
	
	// Sort by success rate
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].SuccessRate > matches[j].SuccessRate
	})
	
	return matches
}

// Serialize serializes the learning memory to JSON
func (m *LearningMemory) Serialize() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.MarshalIndent(m, "", "  ")
}

// Deserialize deserializes learning memory from JSON
func DeserializeLearningMemory(data []byte) (*LearningMemory, error) {
	var memory LearningMemory
	if err := json.Unmarshal(data, &memory); err != nil {
		return nil, err
	}
	
	// Initialize maps if nil
	if memory.Patterns == nil {
		memory.Patterns = make(map[string]*LearnedPattern)
	}
	if memory.StrategyOutcomes == nil {
		memory.StrategyOutcomes = make(map[string]*StrategyOutcome)
	}
	if memory.PromptTemplates == nil {
		memory.PromptTemplates = make(map[string]*PromptTemplate)
	}
	if memory.DomainKnowledge == nil {
		memory.DomainKnowledge = make(map[string]*DomainKnowledge)
	}
	
	return &memory, nil
}

// Helper functions

func generateTemplateID(name string) string {
	return fmt.Sprintf("%s_%d", sanitizeID(name), time.Now().Unix())
}

func calculateConfidence(success, failure int) float64 {
	total := success + failure
	if total == 0 {
		return 0.5
	}
	// Wilson score interval for confidence
	p := float64(success) / float64(total)
	z := 1.96 // 95% confidence
	n := float64(total)
	
	wilson := (p + z*z/(2*n) - z*sqrt(p*(1-p)/n+z*z/(4*n*n))) / (1 + z*z/n)
	return wilson
}

func calculateRelevanceScore(pattern *LearnedPattern, contextHints []string) float64 {
	if len(contextHints) == 0 || len(pattern.ContextMatchers) == 0 {
		return pattern.Confidence
	}
	
	matchCount := 0
	for _, hint := range contextHints {
		hintLower := strings.ToLower(hint)
		for _, matcher := range pattern.ContextMatchers {
			if strings.Contains(hintLower, strings.ToLower(matcher)) {
				matchCount++
				break
			}
		}
	}
	
	matchRatio := float64(matchCount) / float64(len(contextHints))
	return pattern.Confidence * (0.5 + 0.5*matchRatio)
}

func mergeOutcomeHints(existing, new map[string]string) {
	for k, v := range new {
		existing[k] = v
	}
}

func sanitizeID(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "_")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// Ensure LearningMemory implements MemoryStore interface
type MemoryStore interface {
	RecordPattern(pattern *LearnedPattern)
	RecordStrategyOutcome(strategyID string, success bool, quality float64)
	UpdatePromptTemplate(template *PromptTemplate)
	UpdateDomainKnowledge(domain string, updateFn func(*DomainKnowledge))
	GetRelevantPatterns(domain string, contextHints []string, limit int) []*LearnedPattern
	GetBestStrategy(contextType string) *StrategyOutcome
	GetPromptTemplate(id string) *PromptTemplate
	SearchPromptTemplates(domain string, minSuccessRate float64) []*PromptTemplate
	Serialize() ([]byte, error)
}

var _ MemoryStore = (*LearningMemory)(nil)

// StorageBackend defines the interface for persistent storage
type StorageBackend interface {
	Save(ctx context.Context, key string, data []byte) error
	Load(ctx context.Context, key string) ([]byte, error)
	List(ctx context.Context, prefix string) ([]string, error)
}

// PersistentLearningMemory wraps LearningMemory with persistence
type PersistentLearningMemory struct {
	*LearningMemory
	backend  StorageBackend
	key      string
	saveMu   sync.Mutex
}

// NewPersistentLearningMemory creates a new persistent learning memory
func NewPersistentLearningMemory(backend StorageBackend, key string) (*PersistentLearningMemory, error) {
	data, err := backend.Load(context.Background(), key)
	if err != nil {
		// Create new if load fails
		return &PersistentLearningMemory{
			LearningMemory: NewLearningMemory(),
			backend:        backend,
			key:            key,
		}, nil
	}
	
	memory, err := DeserializeLearningMemory(data)
	if err != nil {
		return nil, err
	}
	
	return &PersistentLearningMemory{
		LearningMemory: memory,
		backend:        backend,
		key:            key,
	}, nil
}

// Save persists the learning memory
func (p *PersistentLearningMemory) Save(ctx context.Context) error {
	p.saveMu.Lock()
	defer p.saveMu.Unlock()
	
	data, err := p.Serialize()
	if err != nil {
		return err
	}
	
	return p.backend.Save(ctx, p.key, data)
}

// AutoSaveDecorator wraps operations with automatic saving
type AutoSaveDecorator struct {
	*PersistentLearningMemory
	saveDelay time.Duration
}

// NewAutoSaveDecorator creates an auto-saving decorator
func NewAutoSaveDecorator(plm *PersistentLearningMemory, saveDelay time.Duration) *AutoSaveDecorator {
	return &AutoSaveDecorator{
		PersistentLearningMemory: plm,
		saveDelay:                saveDelay,
	}
}

func (a *AutoSaveDecorator) triggerSave() {
	time.AfterFunc(a.saveDelay, func() {
		a.Save(context.Background())
	})
}
