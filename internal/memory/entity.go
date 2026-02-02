package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
)

// EntityType categorizes an entity.
type EntityType string

const (
	EntityPerson       EntityType = "person"
	EntityProject      EntityType = "project"
	EntityOrganization EntityType = "organization"
	EntityConcept      EntityType = "concept"
	EntityTool         EntityType = "tool"
	EntityLocation     EntityType = "location"
)

// Entity represents a known entity extracted from conversations and context.
type Entity struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	Name         string            `json:"name"`
	Type         EntityType        `json:"type"`
	Description  string            `json:"description"`
	Aliases      []string          `json:"aliases,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	Relations    []EntityRelation  `json:"relations,omitempty"`
	FirstSeen    time.Time         `json:"first_seen"`
	LastSeen     time.Time         `json:"last_seen"`
	MentionCount int               `json:"mention_count"`
	Tags         []string          `json:"tags,omitempty"`
}

// EntityRelation describes a directional relationship from one entity to another.
type EntityRelation struct {
	TargetID     string `json:"target_id"`
	RelationType string `json:"relation_type"`
	Description  string `json:"description,omitempty"`
}

// EntityQuery describes a search request against the entity store.
type EntityQuery struct {
	UserID      string       `json:"user_id"`
	Text        string       `json:"text,omitempty"`
	Types       []EntityType `json:"types,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	MinMentions int          `json:"min_mentions,omitempty"`
	Limit       int          `json:"limit,omitempty"`
}

// EntityStore abstracts persistence for entity memory.
type EntityStore interface {
	UpsertEntity(ctx context.Context, entity Entity) (Entity, error)
	GetEntity(ctx context.Context, id string) (Entity, error)
	SearchEntities(ctx context.Context, query EntityQuery) ([]Entity, error)
	AddRelation(ctx context.Context, sourceID string, relation EntityRelation) error
	RecordMention(ctx context.Context, id string) error
	DeleteEntity(ctx context.Context, id string) error
}

// FileEntityStore implements EntityStore by persisting entities as JSON files,
// one file per user at {basePath}/{userID}/entities.json.
type FileEntityStore struct {
	basePath string
	mu       sync.Mutex
	cache    map[string][]Entity // userID -> entities
}

// NewFileEntityStore creates a file-backed entity store rooted at basePath.
func NewFileEntityStore(basePath string) *FileEntityStore {
	return &FileEntityStore{
		basePath: basePath,
		cache:    make(map[string][]Entity),
	}
}

// UpsertEntity creates or updates an entity. If an entity with the same
// UserID+Name+Type already exists, it merges the new data into the existing
// record: description is overwritten if non-empty, new aliases and tags are
// appended (deduplicated), attributes are merged, and LastSeen is updated.
func (s *FileEntityStore) UpsertEntity(_ context.Context, entity Entity) (Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entity.UserID == "" {
		return Entity{}, fmt.Errorf("entity user_id is required")
	}
	if entity.Name == "" {
		return Entity{}, fmt.Errorf("entity name is required")
	}
	if entity.Type == "" {
		return Entity{}, fmt.Errorf("entity type is required")
	}

	entities, err := s.loadEntities(entity.UserID)
	if err != nil {
		return Entity{}, err
	}

	now := time.Now()
	idx := s.findEntity(entities, entity.UserID, entity.Name, entity.Type)
	if idx >= 0 {
		// Merge into existing entity.
		existing := &entities[idx]
		if entity.Description != "" {
			existing.Description = entity.Description
		}
		existing.Aliases = mergeStringSlice(existing.Aliases, entity.Aliases)
		existing.Tags = mergeStringSlice(existing.Tags, entity.Tags)
		existing.Attributes = mergeAttributes(existing.Attributes, entity.Attributes)
		existing.Relations = mergeRelations(existing.Relations, entity.Relations)
		existing.LastSeen = now
		existing.MentionCount++

		if err := s.saveEntities(entity.UserID, entities); err != nil {
			return Entity{}, err
		}
		return *existing, nil
	}

	// Create new entity.
	if entity.ID == "" {
		entity.ID = ksuid.New().String()
	}
	if entity.FirstSeen.IsZero() {
		entity.FirstSeen = now
	}
	if entity.LastSeen.IsZero() {
		entity.LastSeen = now
	}
	if entity.MentionCount == 0 {
		entity.MentionCount = 1
	}
	if entity.Attributes == nil {
		entity.Attributes = map[string]string{}
	}

	entities = append(entities, entity)
	if err := s.saveEntities(entity.UserID, entities); err != nil {
		return Entity{}, err
	}
	return entity, nil
}

// GetEntity retrieves an entity by ID across all cached users.
func (s *FileEntityStore) GetEntity(_ context.Context, id string) (Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return Entity{}, fmt.Errorf("entity id is required")
	}

	// Search across all user directories on disk.
	dirEntries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Entity{}, fmt.Errorf("entity not found: %s", id)
		}
		return Entity{}, fmt.Errorf("read base path: %w", err)
	}

	for _, de := range dirEntries {
		if !de.IsDir() {
			continue
		}
		userID := de.Name()
		entities, err := s.loadEntities(userID)
		if err != nil {
			continue
		}
		for _, e := range entities {
			if e.ID == id {
				return e, nil
			}
		}
	}

	return Entity{}, fmt.Errorf("entity not found: %s", id)
}

// SearchEntities finds entities matching the query criteria.
// Results are sorted by MentionCount descending.
func (s *FileEntityStore) SearchEntities(_ context.Context, query EntityQuery) ([]Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if query.UserID == "" {
		return nil, fmt.Errorf("entity query user_id is required")
	}

	entities, err := s.loadEntities(query.UserID)
	if err != nil {
		return nil, err
	}

	textLower := strings.ToLower(strings.TrimSpace(query.Text))
	typeSet := make(map[EntityType]bool, len(query.Types))
	for _, t := range query.Types {
		typeSet[t] = true
	}
	tagSet := make(map[string]bool, len(query.Tags))
	for _, tag := range query.Tags {
		tagSet[strings.ToLower(tag)] = true
	}

	var results []Entity
	for _, e := range entities {
		if len(typeSet) > 0 && !typeSet[e.Type] {
			continue
		}
		if query.MinMentions > 0 && e.MentionCount < query.MinMentions {
			continue
		}
		if len(tagSet) > 0 && !matchAnyTag(e.Tags, tagSet) {
			continue
		}
		if textLower != "" && !matchEntityText(e, textLower) {
			continue
		}
		results = append(results, e)
	}

	// Sort by MentionCount descending (most referenced first).
	sort.Slice(results, func(i, j int) bool {
		return results[i].MentionCount > results[j].MentionCount
	})

	if query.Limit > 0 && len(results) > query.Limit {
		results = results[:query.Limit]
	}
	return results, nil
}

// AddRelation appends a relation to the entity identified by sourceID.
func (s *FileEntityStore) AddRelation(_ context.Context, sourceID string, relation EntityRelation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sourceID == "" {
		return fmt.Errorf("source entity id is required")
	}
	if relation.TargetID == "" {
		return fmt.Errorf("relation target_id is required")
	}

	userID, idx, entities, err := s.findEntityByID(sourceID)
	if err != nil {
		return err
	}

	entity := &entities[idx]
	// Avoid duplicate relations with same TargetID+RelationType.
	for _, r := range entity.Relations {
		if r.TargetID == relation.TargetID && r.RelationType == relation.RelationType {
			return nil
		}
	}
	entity.Relations = append(entity.Relations, relation)

	return s.saveEntities(userID, entities)
}

// RecordMention increments the MentionCount and updates LastSeen for the
// entity identified by id.
func (s *FileEntityStore) RecordMention(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return fmt.Errorf("entity id is required")
	}

	userID, idx, entities, err := s.findEntityByID(id)
	if err != nil {
		return err
	}

	entities[idx].MentionCount++
	entities[idx].LastSeen = time.Now()

	return s.saveEntities(userID, entities)
}

// DeleteEntity removes the entity identified by id.
func (s *FileEntityStore) DeleteEntity(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == "" {
		return fmt.Errorf("entity id is required")
	}

	userID, idx, entities, err := s.findEntityByID(id)
	if err != nil {
		return err
	}

	entities = append(entities[:idx], entities[idx+1:]...)
	return s.saveEntities(userID, entities)
}

// --- internal helpers ---

// loadEntities reads the entities JSON file for a user into the cache and
// returns the cached slice. If the file does not exist, returns an empty slice.
func (s *FileEntityStore) loadEntities(userID string) ([]Entity, error) {
	if cached, ok := s.cache[userID]; ok {
		return cached, nil
	}

	path := s.entityFilePath(userID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.cache[userID] = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read entity file: %w", err)
	}

	var entities []Entity
	if err := json.Unmarshal(data, &entities); err != nil {
		return nil, fmt.Errorf("parse entity file: %w", err)
	}
	s.cache[userID] = entities
	return entities, nil
}

// saveEntities writes the entities slice to disk and updates the cache.
func (s *FileEntityStore) saveEntities(userID string, entities []Entity) error {
	dir := filepath.Join(s.basePath, userID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create entity dir: %w", err)
	}

	data, err := json.MarshalIndent(entities, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal entities: %w", err)
	}

	path := s.entityFilePath(userID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write entity file: %w", err)
	}

	s.cache[userID] = entities
	return nil
}

func (s *FileEntityStore) entityFilePath(userID string) string {
	return filepath.Join(s.basePath, userID, "entities.json")
}

// findEntity locates an entity in the slice by UserID+Name+Type.
// Returns the index, or -1 if not found.
func (s *FileEntityStore) findEntity(entities []Entity, userID string, name string, typ EntityType) int {
	nameLower := strings.ToLower(name)
	for i, e := range entities {
		if e.UserID == userID &&
			strings.ToLower(e.Name) == nameLower &&
			e.Type == typ {
			return i
		}
	}
	return -1
}

// findEntityByID scans all user directories to find an entity by its ID.
// Returns the userID, index within the entities slice, the full slice, or an error.
func (s *FileEntityStore) findEntityByID(id string) (string, int, []Entity, error) {
	dirEntries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, nil, fmt.Errorf("entity not found: %s", id)
		}
		return "", 0, nil, fmt.Errorf("read base path: %w", err)
	}

	for _, de := range dirEntries {
		if !de.IsDir() {
			continue
		}
		userID := de.Name()
		entities, err := s.loadEntities(userID)
		if err != nil {
			continue
		}
		for i, e := range entities {
			if e.ID == id {
				return userID, i, entities, nil
			}
		}
	}

	return "", 0, nil, fmt.Errorf("entity not found: %s", id)
}

// matchEntityText performs case-insensitive substring matching against an
// entity's Name, Description, and Aliases.
func matchEntityText(e Entity, textLower string) bool {
	if strings.Contains(strings.ToLower(e.Name), textLower) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), textLower) {
		return true
	}
	for _, alias := range e.Aliases {
		if strings.Contains(strings.ToLower(alias), textLower) {
			return true
		}
	}
	return false
}

// matchAnyTag returns true if any of the entity's tags appear in the tag set.
func matchAnyTag(tags []string, tagSet map[string]bool) bool {
	for _, tag := range tags {
		if tagSet[strings.ToLower(tag)] {
			return true
		}
	}
	return false
}

// mergeStringSlice appends new unique items from additions into base.
func mergeStringSlice(base, additions []string) []string {
	seen := make(map[string]bool, len(base))
	for _, s := range base {
		seen[strings.ToLower(s)] = true
	}
	merged := make([]string, len(base))
	copy(merged, base)
	for _, s := range additions {
		if !seen[strings.ToLower(s)] {
			seen[strings.ToLower(s)] = true
			merged = append(merged, s)
		}
	}
	return merged
}

// mergeAttributes merges new attributes into existing ones, overwriting on conflict.
func mergeAttributes(base, additions map[string]string) map[string]string {
	if base == nil {
		base = make(map[string]string)
	}
	for k, v := range additions {
		base[k] = v
	}
	return base
}

// mergeRelations appends new relations that don't already exist (by TargetID+RelationType).
func mergeRelations(base, additions []EntityRelation) []EntityRelation {
	type key struct{ targetID, relType string }
	seen := make(map[key]bool, len(base))
	for _, r := range base {
		seen[key{r.TargetID, r.RelationType}] = true
	}
	merged := make([]EntityRelation, len(base))
	copy(merged, base)
	for _, r := range additions {
		k := key{r.TargetID, r.RelationType}
		if !seen[k] {
			seen[k] = true
			merged = append(merged, r)
		}
	}
	return merged
}
