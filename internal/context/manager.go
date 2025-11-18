package context

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/observability"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/utils"
	"gopkg.in/yaml.v3"
)

type manager struct {
	threshold  float64
	configRoot string
	logger     *utils.Logger
	stateStore sessionstate.Store
	metrics    *observability.ContextMetrics

	static      *staticRegistry
	preloadOnce sync.Once
	preloadErr  error
}

const (
	defaultThreshold = 0.8
	defaultStaticTTL = 30 * time.Minute
)

// Option configures the context manager.
type Option func(*manager)

// WithConfigRoot overrides the directory used for static context files.
func WithConfigRoot(root string) Option {
	return func(m *manager) {
		if root != "" {
			m.configRoot = root
		}
	}
}

// WithStateStore attaches a dynamic state store implementation.
func WithStateStore(store sessionstate.Store) Option {
	return func(m *manager) {
		m.stateStore = store
	}
}

// WithLogger injects a custom logger (used by tests).
func WithLogger(logger *utils.Logger) Option {
	return func(m *manager) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// WithMetrics allows overriding the metrics recorder.
func WithMetrics(metrics *observability.ContextMetrics) Option {
	return func(m *manager) {
		if metrics != nil {
			m.metrics = metrics
		}
	}
}

// NewManager constructs a layered context manager implementation.
func NewManager(opts ...Option) ports.ContextManager {
	root := filepath.Join("configs", "context")
	if envRoot, ok := os.LookupEnv("ALEX_CONTEXT_CONFIG_DIR"); ok {
		if trimmed := strings.TrimSpace(envRoot); trimmed != "" {
			root = trimmed
		}
	}

	m := &manager{
		threshold:  defaultThreshold,
		configRoot: root,
		logger:     utils.NewComponentLogger("ContextManager"),
		metrics:    observability.NewContextMetrics(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	if m.static == nil {
		cfgRoot := m.configRoot
		if cfgRoot == "" {
			cfgRoot = root
		}
		m.static = newStaticRegistry(cfgRoot, defaultStaticTTL, m.logger, m.metrics)
	}
	return m
}

// EstimateTokens approximates token usage by dividing rune count.
func (m *manager) EstimateTokens(messages []ports.Message) int {
	count := 0
	for _, msg := range messages {
		count += len(msg.Content) / 4
	}
	return count
}

// ShouldCompress indicates whether the context needs to be compacted.
func (m *manager) ShouldCompress(messages []ports.Message, limit int) bool {
	if limit <= 0 {
		return false
	}
	return float64(m.EstimateTokens(messages)) > float64(limit)*m.threshold
}

// Compress keeps the system prompt and the last ten messages. When older
// history is trimmed we inject a lightweight structured summary so the model
// still has awareness of what was dropped without wasting tokens on verbose
// placeholders.
func (m *manager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	if targetTokens <= 0 {
		return messages, nil
	}
	current := m.EstimateTokens(messages)
	if current <= targetTokens {
		return messages, nil
	}
	if len(messages) <= 11 {
		return messages, nil
	}
	head := messages[0]
	tail := messages[len(messages)-10:]
	compressed := []ports.Message{head}
	if summary := buildCompressionSummary(messages[1 : len(messages)-10]); summary != "" {
		compressed = append(compressed, ports.Message{
			Role:    "system",
			Content: summary,
			Source:  ports.MessageSourceSystemPrompt,
		})
	}
	compressed = append(compressed, tail...)
	return compressed, nil
}

func buildCompressionSummary(messages []ports.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var userCount, assistantCount, toolMentions int
	var firstUser, lastUser, firstAssistant, lastAssistant string

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		snippet := buildCompressionSnippet(msg.Content, 140)
		switch role {
		case "user":
			userCount++
			if firstUser == "" {
				firstUser = snippet
			}
			lastUser = snippet
		case "assistant":
			assistantCount++
			toolMentions += len(msg.ToolCalls)
			if firstAssistant == "" {
				firstAssistant = snippet
			}
			lastAssistant = snippet
		case "tool":
			toolMentions++
		}
		toolMentions += len(msg.ToolResults)
	}

	parts := []string{fmt.Sprintf("Earlier conversation had %d user message(s) and %d assistant response(s)", userCount, assistantCount)}
	if toolMentions > 0 {
		parts = append(parts, fmt.Sprintf("tools were referenced %d time(s)", toolMentions))
	}

	var contextParts []string
	if firstUser != "" {
		contextParts = append(contextParts, fmt.Sprintf("user first asked: %s", firstUser))
	}
	if firstAssistant != "" {
		contextParts = append(contextParts, fmt.Sprintf("assistant first replied: %s", firstAssistant))
	}
	if lastUser != "" && lastUser != firstUser {
		contextParts = append(contextParts, fmt.Sprintf("recent user request: %s", lastUser))
	}
	if lastAssistant != "" && lastAssistant != firstAssistant {
		contextParts = append(contextParts, fmt.Sprintf("recent assistant reply: %s", lastAssistant))
	}
	if len(contextParts) > 0 {
		parts = append(parts, strings.Join(contextParts, " | "))
	}

	return fmt.Sprintf("[Earlier context compressed] %s.", strings.Join(parts, "; "))
}

func buildCompressionSnippet(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func (m *manager) Preload(ctx context.Context) error {
	m.preloadOnce.Do(func() {
		m.preloadErr = m.static.ensure(ctx)
	})
	return m.preloadErr
}

func (m *manager) BuildWindow(ctx context.Context, session *ports.Session, cfg ports.ContextWindowConfig) (ports.ContextWindow, error) {
	if session == nil {
		return ports.ContextWindow{}, fmt.Errorf("session required")
	}
	if err := m.Preload(ctx); err != nil {
		return ports.ContextWindow{}, err
	}

	staticSnapshot, err := m.static.currentSnapshot(ctx)
	if err != nil {
		return ports.ContextWindow{}, err
	}

	persona := selectPersona(cfg.PersonaKey, session, staticSnapshot.Personas)
	goal := selectGoal(cfg.GoalKey, staticSnapshot.Goals)
	world := selectWorld(cfg.WorldKey, session, staticSnapshot.Worlds)
	policies := mapToSlice(staticSnapshot.Policies)
	knowledge := mapToSlice(staticSnapshot.Knowledge)

	messages := append([]ports.Message(nil), session.Messages...)
	if cfg.TokenLimit > 0 && m.ShouldCompress(messages, cfg.TokenLimit) {
		if compressed, err := m.Compress(messages, int(float64(cfg.TokenLimit)*0.8)); err == nil {
			messages = compressed
		} else if m.logger != nil {
			m.logger.Warn("Context compression failed: %v", err)
		}
	}

	dyn := ports.DynamicContext{}
	if m.stateStore != nil {
		snap, err := m.stateStore.LatestSnapshot(ctx, session.ID)
		if err == nil {
			dyn = convertSnapshotToDynamic(snap)
		} else if !errors.Is(err, sessionstate.ErrSnapshotNotFound) && m.logger != nil {
			m.logger.Warn("State snapshot read failed: %v", err)
		}
	}

	window := ports.ContextWindow{
		SessionID: session.ID,
		Messages:  messages,
		Static: ports.StaticContext{
			Persona:            persona,
			Goal:               goal,
			Policies:           policies,
			Knowledge:          knowledge,
			Tools:              buildToolHints(cfg.ToolPreset),
			World:              world,
			EnvironmentSummary: cfg.EnvironmentSummary,
			Version:            staticSnapshot.Version,
		},
		Dynamic: dyn,
		Meta:    ports.MetaContext{PersonaVersion: persona.ID},
	}
	return window, nil
}

func (m *manager) RecordTurn(ctx context.Context, record ports.ContextTurnRecord) error {
	if m.stateStore == nil || record.SessionID == "" {
		return nil
	}
	snapshot := sessionstate.Snapshot{
		SessionID:     record.SessionID,
		TurnID:        record.TurnID,
		LLMTurnSeq:    record.LLMTurnSeq,
		CreatedAt:     record.Timestamp,
		Summary:       record.Summary,
		Plans:         record.Plans,
		Beliefs:       record.Beliefs,
		World:         record.World,
		Diff:          record.Diff,
		Messages:      record.Messages,
		Feedback:      record.Feedback,
		KnowledgeRefs: record.KnowledgeRefs,
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now()
	}
	if err := m.stateStore.SaveSnapshot(ctx, snapshot); err != nil {
		m.metrics.RecordSnapshotError()
		if m.logger != nil {
			m.logger.Warn("Failed to persist context snapshot: %v", err)
		}
		return err
	}
	return nil
}

// Helper conversions -------------------------------------------------------

func buildToolHints(preset string) []string {
	if preset == "" {
		return nil
	}
	return []string{preset}
}

func convertSnapshotToDynamic(snapshot sessionstate.Snapshot) ports.DynamicContext {
	return ports.DynamicContext{
		TurnID:            snapshot.TurnID,
		LLMTurnSeq:        snapshot.LLMTurnSeq,
		Plans:             snapshot.Plans,
		Beliefs:           snapshot.Beliefs,
		WorldState:        snapshot.World,
		Feedback:          snapshot.Feedback,
		SnapshotTimestamp: snapshot.CreatedAt,
	}
}

func selectPersona(key string, session *ports.Session, personas map[string]ports.PersonaProfile) ports.PersonaProfile {
	if key == "" && session != nil && session.Metadata != nil {
		key = session.Metadata["persona"]
	}
	if persona, ok := personas[key]; ok {
		return persona
	}
	if persona, ok := personas["default"]; ok {
		return persona
	}
	// Fallback to deterministic order
	keys := make([]string, 0, len(personas))
	for id := range personas {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ports.PersonaProfile{ID: "default", Tone: "neutral"}
	}
	return personas[keys[0]]
}

func selectGoal(key string, goals map[string]ports.GoalProfile) ports.GoalProfile {
	if goal, ok := goals[key]; ok {
		return goal
	}
	if goal, ok := goals["default"]; ok {
		return goal
	}
	keys := make([]string, 0, len(goals))
	for id := range goals {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ports.GoalProfile{ID: "default"}
	}
	return goals[keys[0]]
}

func selectWorld(key string, session *ports.Session, worlds map[string]ports.WorldProfile) ports.WorldProfile {
	if key == "" && session != nil && session.Metadata != nil {
		key = session.Metadata["world"]
	}
	if world, ok := worlds[key]; ok {
		return world
	}
	if world, ok := worlds["default"]; ok {
		return world
	}
	keys := make([]string, 0, len(worlds))
	for id := range worlds {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ports.WorldProfile{ID: "default"}
	}
	return worlds[keys[0]]
}

func mapToSlice[T any](input map[string]T) []T {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]T, 0, len(keys))
	for _, k := range keys {
		result = append(result, input[k])
	}
	return result
}

// Static registry ----------------------------------------------------------

type staticRegistry struct {
	root    string
	ttl     time.Duration
	logger  *utils.Logger
	metrics *observability.ContextMetrics

	mu       sync.RWMutex
	snapshot staticSnapshot
	expires  time.Time
}

type staticSnapshot struct {
	Version   string
	LoadedAt  time.Time
	Personas  map[string]ports.PersonaProfile
	Goals     map[string]ports.GoalProfile
	Policies  map[string]ports.PolicyRule
	Knowledge map[string]ports.KnowledgeReference
	Worlds    map[string]ports.WorldProfile
}

func newStaticRegistry(root string, ttl time.Duration, logger *utils.Logger, metrics *observability.ContextMetrics) *staticRegistry {
	if ttl <= 0 {
		ttl = defaultStaticTTL
	}
	if logger == nil {
		logger = utils.NewComponentLogger("ContextStaticRegistry")
	}
	if metrics == nil {
		metrics = observability.NewContextMetrics()
	}
	return &staticRegistry{
		root:    root,
		ttl:     ttl,
		logger:  logger,
		metrics: metrics,
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

	snap, err := r.load(ctx)
	if err != nil {
		return staticSnapshot{}, err
	}
	r.snapshot = snap
	r.expires = time.Now().Add(r.ttl)
	return snap, nil
}

func (r *staticRegistry) load(_ context.Context) (staticSnapshot, error) {
	personas, err := loadPersonas(filepath.Join(r.root, "personas"))
	if err != nil {
		r.metrics.RecordStaticCacheMiss()
		return staticSnapshot{}, err
	}
	goals, err := loadGoals(filepath.Join(r.root, "goals"))
	if err != nil {
		r.metrics.RecordStaticCacheMiss()
		return staticSnapshot{}, err
	}
	policies, err := loadPolicies(filepath.Join(r.root, "policies"))
	if err != nil {
		r.metrics.RecordStaticCacheMiss()
		return staticSnapshot{}, err
	}
	knowledge, err := loadKnowledge(filepath.Join(r.root, "knowledge"))
	if err != nil {
		r.metrics.RecordStaticCacheMiss()
		return staticSnapshot{}, err
	}
	worlds, err := loadWorlds(filepath.Join(r.root, "worlds"))
	if err != nil {
		r.metrics.RecordStaticCacheMiss()
		return staticSnapshot{}, err
	}

	snap := staticSnapshot{
		Version:   fmt.Sprintf("%d", time.Now().Unix()),
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

func loadPersonas(dir string) (map[string]ports.PersonaProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	personas := make(map[string]ports.PersonaProfile, len(entries))
	for _, content := range entries {
		var profile ports.PersonaProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode persona: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		personas[profile.ID] = profile
	}
	if len(personas) == 0 {
		personas["default"] = ports.PersonaProfile{ID: "default", Tone: "neutral"}
	}
	return personas, nil
}

func loadGoals(dir string) (map[string]ports.GoalProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	goals := make(map[string]ports.GoalProfile, len(entries))
	for _, content := range entries {
		var profile ports.GoalProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode goal: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		goals[profile.ID] = profile
	}
	if len(goals) == 0 {
		goals["default"] = ports.GoalProfile{ID: "default"}
	}
	return goals, nil
}

func loadPolicies(dir string) (map[string]ports.PolicyRule, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	policies := make(map[string]ports.PolicyRule, len(entries))
	for _, content := range entries {
		var policy ports.PolicyRule
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

func loadKnowledge(dir string) (map[string]ports.KnowledgeReference, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	knowledge := make(map[string]ports.KnowledgeReference, len(entries))
	for _, content := range entries {
		var ref ports.KnowledgeReference
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

func loadWorlds(dir string) (map[string]ports.WorldProfile, error) {
	entries, err := readYAMLDir(dir)
	if err != nil {
		return nil, err
	}
	worlds := make(map[string]ports.WorldProfile, len(entries))
	for _, content := range entries {
		var profile ports.WorldProfile
		if err := yaml.Unmarshal(content, &profile); err != nil {
			return nil, fmt.Errorf("decode world profile: %w", err)
		}
		if profile.ID == "" {
			profile.ID = filepath.Base(dir)
		}
		worlds[profile.ID] = profile
	}
	if len(worlds) == 0 {
		worlds["default"] = ports.WorldProfile{ID: "default"}
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
