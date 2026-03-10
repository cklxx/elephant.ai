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
	defaultGoal := agent.GoalProfile{ID: "default"}
	goals, err := loadYAMLMap(filepath.Join(r.root, "goals"), "goal", &defaultGoal)
	if err != nil {
		return staticSnapshot{}, err
	}
	policies, err := loadYAMLMap[agent.PolicyRule](filepath.Join(r.root, "policies"), "policy", nil)
	if err != nil {
		return staticSnapshot{}, err
	}
	knowledge, err := loadYAMLMap[agent.KnowledgeReference](filepath.Join(r.root, "knowledge"), "knowledge pack", nil)
	if err != nil {
		return staticSnapshot{}, err
	}
	defaultWorld := agent.WorldProfile{ID: "default"}
	worlds, err := loadYAMLMap(filepath.Join(r.root, "worlds"), "world profile", &defaultWorld)
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

// identifiable is satisfied by domain types that carry an ID field.
type identifiable interface {
	agent.GoalProfile | agent.PolicyRule | agent.KnowledgeReference | agent.WorldProfile
}

// loadYAMLMap reads all YAML files from dir, unmarshals each into T, and
// keys the result map by the item's ID field. When the ID is empty it falls
// back to the directory base name.
func loadYAMLMap[T identifiable](dir, label string, fallback *T) (map[string]T, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	result := make(map[string]T, len(entries))
	for _, content := range entries {
		var item T
		if err := yaml.Unmarshal(content, &item); err != nil {
			return nil, fmt.Errorf("decode %s: %w", label, err)
		}
		id := idOf(&item)
		if id == "" {
			id = filepath.Base(dir)
			setID(&item, id)
		}
		result[id] = item
	}
	if len(result) == 0 && fallback != nil {
		id := idOf(fallback)
		result[id] = *fallback
	}
	return result, nil
}

// idOf returns the ID field from any identifiable type.
func idOf[T identifiable](v *T) string {
	switch p := any(v).(type) {
	case *agent.GoalProfile:
		return p.ID
	case *agent.PolicyRule:
		return p.ID
	case *agent.KnowledgeReference:
		return p.ID
	case *agent.WorldProfile:
		return p.ID
	}
	return ""
}

// setID assigns the ID field on any identifiable type.
func setID[T identifiable](v *T, id string) {
	switch p := any(v).(type) {
	case *agent.GoalProfile:
		p.ID = id
	case *agent.PolicyRule:
		p.ID = id
	case *agent.KnowledgeReference:
		p.ID = id
	case *agent.WorldProfile:
		p.ID = id
	}
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

// deriveRepoRootFromConfigRoot delegates to deriveRepoRoot.
var deriveRepoRootFromConfigRoot = deriveRepoRoot
