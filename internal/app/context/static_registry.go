package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/observability"
	"alex/internal/shared/logging"
	"gopkg.in/yaml.v3"
)

// Static registry ----------------------------------------------------------

type staticRegistry struct {
	root     string
	repoRoot string
	ttl      time.Duration
	logger   logging.Logger
	metrics  *observability.ContextMetrics

	mu       sync.RWMutex
	snapshot staticSnapshot
	expires  time.Time
}

type staticSnapshot struct {
	Version   string
	LoadedAt  time.Time
	Personas  map[string]agent.PersonaProfile
	Goals     map[string]agent.GoalProfile
	Policies  map[string]agent.PolicyRule
	Knowledge map[string]agent.KnowledgeReference
	Worlds    map[string]agent.WorldProfile
}

func newStaticRegistry(root string, repoRoot string, ttl time.Duration, logger logging.Logger, metrics *observability.ContextMetrics) *staticRegistry {
	if ttl <= 0 {
		ttl = defaultStaticTTL
	}
	if logging.IsNil(logger) {
		logger = logging.NewComponentLogger("ContextStaticRegistry")
	}
	if metrics == nil {
		metrics = observability.NewContextMetrics()
	}
	if repoRoot == "" {
		repoRoot = deriveRepoRootFromConfigRoot(root)
	}
	return &staticRegistry{
		root:     root,
		repoRoot: repoRoot,
		ttl:      ttl,
		logger:   logger,
		metrics:  metrics,
	}
}

func (r *staticRegistry) ensure(ctx context.Context) error {
	_, err := r.currentSnapshot(ctx)
	return err
}

func (r *staticRegistry) currentSnapshot(ctx context.Context) (staticSnapshot, error) {
	r.mu.RLock()
	if !r.snapshot.LoadedAt.IsZero() && time.Now().Before(r.expires) {
		snap := r.snapshot
		r.mu.RUnlock()
		return snap, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.snapshot.LoadedAt.IsZero() && time.Now().Before(r.expires) {
		return r.snapshot, nil
	}

	if r.metrics != nil {
		r.metrics.RecordStaticCacheMiss()
	}
	snap, err := r.load(ctx)
	if err != nil {
		return staticSnapshot{}, err
	}
	r.snapshot = snap
	r.expires = time.Now().Add(r.ttl)
	return snap, nil
}

func (r *staticRegistry) load(_ context.Context) (staticSnapshot, error) {
	personas, err := loadPersonas(filepath.Join(r.root, "personas"), r.repoRoot)
	if err != nil {
		return staticSnapshot{}, err
	}
	goals, err := loadGoals(filepath.Join(r.root, "goals"))
	if err != nil {
		return staticSnapshot{}, err
	}
	policies, err := loadPolicies(filepath.Join(r.root, "policies"))
	if err != nil {
		return staticSnapshot{}, err
	}
	knowledge, err := loadKnowledge(filepath.Join(r.root, "knowledge"))
	if err != nil {
		return staticSnapshot{}, err
	}
	worlds, err := loadWorlds(filepath.Join(r.root, "worlds"))
	if err != nil {
		return staticSnapshot{}, err
	}

	version := hashStaticSnapshot(personas, goals, policies, knowledge, worlds)
	snap := staticSnapshot{
		Version:   version,
		LoadedAt:  time.Now(),
		Personas:  personas,
		Goals:     goals,
		Policies:  policies,
		Knowledge: knowledge,
		Worlds:    worlds,
	}
	if r.logger != nil {
		r.logger.Info("Static context cache refreshed (personas=%d goals=%d)", len(personas), len(goals))
	}
	return snap, nil
}

// YAML loaders -------------------------------------------------------------

func loadPersonas(dir string, repoRoot string) (map[string]agent.PersonaProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	personas := make(map[string]agent.PersonaProfile, len(entries))
	for _, content := range entries {
		var profile agent.PersonaProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode persona: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}

		// Load voice content from file if VoicePath is set
		if profile.VoicePath != "" && profile.Voice == "" {
			voicePath := filepath.Join(repoRoot, profile.VoicePath)
			voiceData, readErr := os.ReadFile(voicePath)
			if readErr != nil {
				if os.IsNotExist(readErr) {
					return nil, fmt.Errorf("voice file not found: %s (specified in persona %s)", voicePath, profile.ID)
				}
				return nil, fmt.Errorf("read voice file %s: %w", voicePath, readErr)
			}
			profile.Voice = string(voiceData)
		}

		personas[profile.ID] = profile
	}
	if len(personas) == 0 {
		personas["default"] = agent.PersonaProfile{ID: "default", Tone: "neutral"}
	}
	return personas, nil
}

func loadGoals(dir string) (map[string]agent.GoalProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	goals := make(map[string]agent.GoalProfile, len(entries))
	for _, content := range entries {
		var profile agent.GoalProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode goal: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		goals[profile.ID] = profile
	}
	if len(goals) == 0 {
		goals["default"] = agent.GoalProfile{ID: "default"}
	}
	return goals, nil
}

func loadPolicies(dir string) (map[string]agent.PolicyRule, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	policies := make(map[string]agent.PolicyRule, len(entries))
	for _, content := range entries {
		var policy agent.PolicyRule
		if err := yaml.Unmarshal(content, &policy); err != nil {
			return nil, fmt.Errorf("decode policy: %w", err)
		}
		if policy.ID == "" {
			policy.ID = filepath.Base(dir)
		}
		policies[policy.ID] = policy
	}
	return policies, nil
}

func loadKnowledge(dir string) (map[string]agent.KnowledgeReference, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	knowledge := make(map[string]agent.KnowledgeReference, len(entries))
	for _, content := range entries {
		var ref agent.KnowledgeReference
		if err := yaml.Unmarshal(content, &ref); err != nil {
			return nil, fmt.Errorf("decode knowledge pack: %w", err)
		}
		if ref.ID == "" {
			ref.ID = filepath.Base(dir)
		}
		knowledge[ref.ID] = ref
	}
	return knowledge, nil
}

func loadWorlds(dir string) (map[string]agent.WorldProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	worlds := make(map[string]agent.WorldProfile, len(entries))
	for _, content := range entries {
		var profile agent.WorldProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode world profile: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		worlds[profile.ID] = profile
	}
	if len(worlds) == 0 {
		worlds["default"] = agent.WorldProfile{ID: "default"}
	}
	return worlds, nil
}

func readYAMLDir(dir string) ([][]byte, error) {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("context directory %s missing", dir)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	var blobs [][]byte
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".yaml") && !strings.HasSuffix(d.Name(), ".yml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		blobs = append(blobs, data)
		return nil
	})
	return blobs, err
}

func hashStaticSnapshot(
	personas map[string]agent.PersonaProfile,
	goals map[string]agent.GoalProfile,
	policies map[string]agent.PolicyRule,
	knowledge map[string]agent.KnowledgeReference,
	worlds map[string]agent.WorldProfile,
) string {
	h := sha256.New()
	encodeMapForHash(h, "personas", personas)
	encodeMapForHash(h, "goals", goals)
	encodeMapForHash(h, "policies", policies)
	encodeMapForHash(h, "knowledge", knowledge)
	encodeMapForHash(h, "worlds", worlds)
	return hex.EncodeToString(h.Sum(nil))
}

func encodeMapForHash[T any](h hash.Hash, label string, entries map[string]T) {
	if h == nil {
		return
	}
	h.Write([]byte(label))
	if len(entries) == 0 {
		h.Write([]byte{0})
		return
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		data, err := json.Marshal(entries[k])
		if err != nil {
			if _, writeErr := fmt.Fprintf(h, "%v", entries[k]); writeErr != nil {
				_, _ = io.WriteString(h, fmt.Sprint(entries[k]))
			}
			continue
		}
		h.Write(data)
	}
}

// deriveRepoRootFromConfigRoot derives the repository root from the config root path.
// Assumes config root is under "configs/context" in the repo.
func deriveRepoRootFromConfigRoot(configRoot string) string {
	cleaned := filepath.Clean(configRoot)
	suffix := filepath.Join("configs", "context")
	if strings.HasSuffix(cleaned, suffix) {
		return strings.TrimSuffix(cleaned, suffix)
	}
	// Fallback: walk up two levels if the path ends with the expected dirs.
	return filepath.Dir(filepath.Dir(cleaned))
}
