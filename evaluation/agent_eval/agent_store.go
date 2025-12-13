package agent_eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// AgentDataStore 负责持久化 agent 画像与历史评估快照
type AgentDataStore struct {
	basePath string

	mu      sync.RWMutex
	indexMu sync.Mutex
}

var safePathComponent = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// NewAgentDataStore 创建持久化存储
func NewAgentDataStore(basePath string) *AgentDataStore {
	return &AgentDataStore{basePath: basePath}
}

// UpsertProfile 按 AgentID 写入或更新画像
func (store *AgentDataStore) UpsertProfile(profile *AgentProfile) (*AgentProfile, error) {
	if profile == nil {
		return nil, fmt.Errorf("profile is nil")
	}
	safeAgentID, err := sanitizePathComponent(profile.AgentID, "agent id")
	if err != nil {
		return nil, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if err := os.MkdirAll(store.basePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create agent store dir: %w", err)
	}

	existing, err := store.loadProfileLocked(safeAgentID)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	now := time.Now()
	incomingCount := profile.EvaluationCount

	merged := *profile
	merged.AgentID = safeAgentID
	merged.UpdatedAt = now

	if existing != nil {
		merged.CreatedAt = existing.CreatedAt
		merged.EvaluationCount = existing.EvaluationCount + incomingCount
		merged.AvgSuccessRate = weightedAverage(existing.AvgSuccessRate, existing.EvaluationCount, profile.AvgSuccessRate, incomingCount)
		merged.AvgExecTime = weightedDuration(existing.AvgExecTime, existing.EvaluationCount, profile.AvgExecTime, incomingCount)
		merged.AvgCostPerTask = weightedAverage(existing.AvgCostPerTask, existing.EvaluationCount, profile.AvgCostPerTask, incomingCount)
		merged.AvgQualityScore = weightedAverage(existing.AvgQualityScore, existing.EvaluationCount, profile.AvgQualityScore, incomingCount)
		merged.LastEvaluated = profile.LastEvaluated
	} else {
		merged.CreatedAt = now
	}

	path := filepath.Join(store.basePath, fmt.Sprintf("%s_profile.json", safeAgentID))
	if err := writeJSON(path, &merged); err != nil {
		return nil, err
	}

	return &merged, nil
}

// LoadProfile 读取画像
func (store *AgentDataStore) LoadProfile(agentID string) (*AgentProfile, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	return store.loadProfileLocked(agentID)
}

func (store *AgentDataStore) loadProfileLocked(agentID string) (*AgentProfile, error) {
	safeAgentID, err := sanitizePathComponent(agentID, "agent id")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(store.basePath, fmt.Sprintf("%s_profile.json", safeAgentID))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var profile AgentProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to decode profile: %w", err)
	}
	return &profile, nil
}

// StoreEvaluation 保存本次评估摘要，便于后续分析
func (store *AgentDataStore) StoreEvaluation(agentID string, results *EvaluationResults) error {
	if results == nil {
		return fmt.Errorf("results is nil")
	}
	if results.JobID == "" {
		return fmt.Errorf("job id is required")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if results.AgentID == "" {
		results.AgentID = agentID
	}
	safeAgentID, err := sanitizePathComponent(results.AgentID, "agent id")
	if err != nil {
		return err
	}
	safeJobID, err := sanitizePathComponent(results.JobID, "job id")
	if err != nil {
		return err
	}
	results.AgentID = safeAgentID
	results.JobID = safeJobID
	if results.Timestamp.IsZero() {
		results.Timestamp = time.Now()
	}

	dir := filepath.Join(store.basePath, safeAgentID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create agent evaluation dir: %w", err)
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.json", safeJobID))
	if err := writeJSON(filename, results); err != nil {
		return err
	}

	return store.updateIndex(safeJobID, safeAgentID)
}

// ListProfiles returns every stored agent profile, if any.
func (store *AgentDataStore) ListProfiles() ([]*AgentProfile, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	entries, err := os.ReadDir(store.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read agent store: %w", err)
	}

	profiles := make([]*AgentProfile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" || !strings.HasSuffix(name, "_profile.json") {
			continue
		}

		profile, err := store.LoadProfile(name[:len(name)-len("_profile.json")])
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

// ListEvaluations enumerates stored evaluation snapshots for an agent.
func (store *AgentDataStore) ListEvaluations(agentID string) ([]*EvaluationResults, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	safeAgentID, err := sanitizePathComponent(agentID, "agent id")
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(store.basePath, safeAgentID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read evaluation dir: %w", err)
	}

	evaluations := make([]*EvaluationResults, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read evaluation %s: %w", entry.Name(), err)
		}

		var result EvaluationResults
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to decode evaluation %s: %w", entry.Name(), err)
		}
		if result.AgentID == "" {
			result.AgentID = agentID
		}
		evaluations = append(evaluations, &result)
	}

	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].Timestamp.After(evaluations[j].Timestamp)
	})

	return evaluations, nil
}

// ListAllEvaluations returns every stored evaluation snapshot across agents.
func (store *AgentDataStore) ListAllEvaluations() ([]*EvaluationResults, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	entries, err := os.ReadDir(store.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read agent store: %w", err)
	}

	var evaluations []*EvaluationResults
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentID := entry.Name()
		evals, err := store.ListEvaluations(agentID)
		if err != nil {
			return nil, err
		}
		evaluations = append(evaluations, evals...)
	}

	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].Timestamp.After(evaluations[j].Timestamp)
	})

	return evaluations, nil
}

// ListRecentEvaluations returns a time-sorted subset of stored evaluations.
// When limit is zero or negative, all evaluations are returned.
func (store *AgentDataStore) ListRecentEvaluations(limit int) ([]*EvaluationResults, error) {
	evaluations, err := store.ListAllEvaluations()
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(evaluations) > limit {
		evaluations = evaluations[:limit]
	}

	return evaluations, nil
}

// QueryEvaluations applies optional filters to stored evaluations across agents.
// When AgentID is set, only that agent's snapshots are considered.
// Results are always sorted by timestamp (newest first) and capped by Limit when provided.
func (store *AgentDataStore) QueryEvaluations(query EvaluationQuery) ([]*EvaluationResults, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	agentIDs := []string{}
	if query.AgentID != "" {
		safeAgentID, err := sanitizePathComponent(query.AgentID, "agent id")
		if err != nil {
			return nil, err
		}
		agentIDs = append(agentIDs, safeAgentID)
	} else {
		entries, err := os.ReadDir(store.basePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to read agent store: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				safeAgentID, err := sanitizePathComponent(entry.Name(), "agent id")
				if err != nil {
					return nil, err
				}
				agentIDs = append(agentIDs, safeAgentID)
			}
		}
	}

	var evaluations []*EvaluationResults
	for _, agentID := range agentIDs {
		var profile *AgentProfile
		if len(query.Tags) > 0 {
			loadedProfile, err := store.loadProfileLocked(agentID)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			profile = loadedProfile
			if !agentMatchesTags(profile, query.Tags) {
				continue
			}
		}

		dir := filepath.Join(store.basePath, agentID)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read evaluation dir for %s: %w", agentID, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}

			eval, err := store.loadEvaluationFromPath(agentID, filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			if matchesEvaluationQuery(eval, query) {
				evaluations = append(evaluations, eval)
			}
		}
	}

	sort.Slice(evaluations, func(i, j int) bool {
		return evaluations[i].Timestamp.After(evaluations[j].Timestamp)
	})

	if query.Limit > 0 && len(evaluations) > query.Limit {
		evaluations = evaluations[:query.Limit]
	}

	return evaluations, nil
}

// LoadEvaluation returns a stored evaluation snapshot by job id, preferring the index file.
func (store *AgentDataStore) LoadEvaluation(jobID string) (*EvaluationResults, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job id is required")
	}
	safeJobID, err := sanitizePathComponent(jobID, "job id")
	if err != nil {
		return nil, err
	}

	store.mu.RLock()
	defer store.mu.RUnlock()

	agentID, err := store.lookupAgent(safeJobID)
	if err != nil {
		return nil, err
	}

	if agentID != "" {
		safeAgentID, err := sanitizePathComponent(agentID, "agent id")
		if err != nil {
			return nil, err
		}
		eval, err := store.loadEvaluationFromPath(safeAgentID, filepath.Join(store.basePath, safeAgentID, fmt.Sprintf("%s.json", safeJobID)))
		if err == nil || os.IsNotExist(err) {
			return eval, err
		}
	}

	evaluations, err := store.ListAllEvaluations()
	if err != nil {
		return nil, err
	}
	for _, eval := range evaluations {
		if eval != nil && eval.JobID == safeJobID {
			return eval, nil
		}
	}

	return nil, os.ErrNotExist
}

// RemoveEvaluation deletes a stored evaluation snapshot and cleans up the index.
func (store *AgentDataStore) RemoveEvaluation(jobID string) error {
	if jobID == "" {
		return fmt.Errorf("job id is required")
	}
	safeJobID, err := sanitizePathComponent(jobID, "job id")
	if err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	agentID, err := store.lookupAgent(safeJobID)
	if err != nil {
		return err
	}

	if agentID != "" {
		safeAgentID, err := sanitizePathComponent(agentID, "agent id")
		if err != nil {
			return err
		}
		if removed, err := store.removeEvaluationFile(safeAgentID, safeJobID); err != nil {
			return err
		} else if removed {
			return store.deleteIndexEntry(safeJobID)
		}
	}

	entries, err := os.ReadDir(store.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return fmt.Errorf("failed to read agent store: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		safeAgentID, err := sanitizePathComponent(entry.Name(), "agent id")
		if err != nil {
			return err
		}
		if removed, err := store.removeEvaluationFile(safeAgentID, safeJobID); err != nil {
			return err
		} else if removed {
			return store.deleteIndexEntry(safeJobID)
		}
	}

	return os.ErrNotExist
}

func (store *AgentDataStore) removeEvaluationFile(agentID, jobID string) (bool, error) {
	safeAgentID, err := sanitizePathComponent(agentID, "agent id")
	if err != nil {
		return false, err
	}
	safeJobID, err := sanitizePathComponent(jobID, "job id")
	if err != nil {
		return false, err
	}
	path := filepath.Join(store.basePath, safeAgentID, fmt.Sprintf("%s.json", safeJobID))
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to remove evaluation %s for %s: %w", jobID, agentID, err)
	}
	return true, nil
}

func (store *AgentDataStore) loadEvaluationFromPath(agentID, path string) (*EvaluationResults, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var eval EvaluationResults
	if err := json.Unmarshal(data, &eval); err != nil {
		return nil, fmt.Errorf("failed to decode evaluation: %w", err)
	}
	if eval.AgentID == "" {
		eval.AgentID = agentID
	}
	return &eval, nil
}

func (store *AgentDataStore) indexPath() string {
	return filepath.Join(store.basePath, "index.json")
}

func (store *AgentDataStore) loadIndex() (map[string]string, error) {
	data, err := os.ReadFile(store.indexPath())
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var index map[string]string
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to decode index: %w", err)
	}
	return index, nil
}

func (store *AgentDataStore) updateIndex(jobID, agentID string) error {
	store.indexMu.Lock()
	defer store.indexMu.Unlock()

	index, err := store.loadIndex()
	if err != nil {
		return err
	}

	if existing, ok := index[jobID]; ok && existing == agentID {
		return nil
	}

	index[jobID] = agentID
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(store.indexPath(), data, 0o644)
}

func (store *AgentDataStore) deleteIndexEntry(jobID string) error {
	store.indexMu.Lock()
	defer store.indexMu.Unlock()

	index, err := store.loadIndex()
	if err != nil {
		return err
	}

	if _, ok := index[jobID]; !ok {
		return nil
	}

	delete(index, jobID)
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(store.indexPath(), data, 0o644)
}

func (store *AgentDataStore) lookupAgent(jobID string) (string, error) {
	store.indexMu.Lock()
	defer store.indexMu.Unlock()

	index, err := store.loadIndex()
	if err != nil {
		return "", err
	}
	return index[jobID], nil
}

func agentMatchesTags(profile *AgentProfile, tags []string) bool {
	if len(tags) == 0 {
		return true
	}
	if profile == nil || len(profile.Tags) == 0 {
		return false
	}

	normalized := make(map[string]struct{}, len(profile.Tags))
	for _, tag := range profile.Tags {
		cleaned := strings.ToLower(strings.TrimSpace(tag))
		if cleaned == "" {
			continue
		}
		normalized[cleaned] = struct{}{}
	}

	for _, tag := range tags {
		cleaned := strings.ToLower(strings.TrimSpace(tag))
		if cleaned == "" {
			continue
		}
		if _, ok := normalized[cleaned]; !ok {
			return false
		}
	}

	return true
}

func matchesEvaluationQuery(eval *EvaluationResults, query EvaluationQuery) bool {
	if eval == nil {
		return false
	}

	if query.AgentID != "" && eval.AgentID != "" && eval.AgentID != query.AgentID {
		return false
	}
	if !query.After.IsZero() && (eval.Timestamp.IsZero() || eval.Timestamp.Before(query.After)) {
		return false
	}
	if !query.Before.IsZero() && (eval.Timestamp.IsZero() || eval.Timestamp.After(query.Before)) {
		return false
	}

	if query.MinScore > 0 {
		overall := evaluationOverallScore(eval)
		if overall < query.MinScore {
			return false
		}
	}

	if query.DatasetPath != "" {
		if eval.Config == nil || !strings.Contains(eval.Config.DatasetPath, query.DatasetPath) {
			return false
		}
	}

	if query.DatasetType != "" {
		if eval.Config == nil || !strings.EqualFold(eval.Config.DatasetType, query.DatasetType) {
			return false
		}
	}

	return true
}

func sanitizePathComponent(value, field string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if value != filepath.Base(value) {
		return "", fmt.Errorf("%s contains path separators", field)
	}
	if !safePathComponent.MatchString(value) {
		return "", fmt.Errorf("%s contains invalid characters", field)
	}
	return value, nil
}

func evaluationOverallScore(eval *EvaluationResults) float64 {
	if eval == nil || eval.Analysis == nil {
		return 0
	}
	return eval.Analysis.Summary.OverallScore
}

func writeJSON(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func weightedAverage(prev float64, prevCount int, value float64, valueCount int) float64 {
	total := prev*float64(prevCount) + value*float64(valueCount)
	denom := float64(prevCount + valueCount)
	if denom == 0 {
		return value
	}
	return total / denom
}

func weightedDuration(prev time.Duration, prevCount int, value time.Duration, valueCount int) time.Duration {
	total := prev*time.Duration(prevCount) + value*time.Duration(valueCount)
	denom := time.Duration(prevCount + valueCount)
	if denom == 0 {
		return value
	}
	return total / denom
}
