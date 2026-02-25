package agent

// CognitiveExtension groups sparse cognitive architecture fields that are
// optionally attached to a TaskState. Extracting them behind a pointer keeps
// TaskState slim for the common case where these fields are unused.
type CognitiveExtension struct {
	Beliefs         []Belief
	KnowledgeRefs   []KnowledgeReference
	WorldState      map[string]any
	WorldDiff       map[string]any
	FeedbackSignals []FeedbackSignal
}

// EnsureCognitive lazily initialises and returns the CognitiveExtension on
// the given TaskState. It is safe to call from concurrent write paths because
// the TaskState is never shared across goroutines during a single ReAct
// iteration.
func (s *TaskState) EnsureCognitive() *CognitiveExtension {
	if s.Cognitive == nil {
		s.Cognitive = &CognitiveExtension{}
	}
	return s.Cognitive
}

// CognitiveOrEmpty returns the CognitiveExtension if present, or a static
// empty value for nil-safe reads. The returned pointer must not be mutated
// when Cognitive is nil.
func (s *TaskState) CognitiveOrEmpty() *CognitiveExtension {
	if s.Cognitive != nil {
		return s.Cognitive
	}
	return &emptyCognitive
}

// emptyCognitive is a zero-value sentinel used by CognitiveOrEmpty so callers
// can safely read fields without nil checks.
var emptyCognitive = CognitiveExtension{}

// CloneCognitiveExtension deep copies the provided extension. Returns nil
// when the input is nil or entirely empty.
func CloneCognitiveExtension(ext *CognitiveExtension) *CognitiveExtension {
	if ext == nil {
		return nil
	}
	hasContent := len(ext.Beliefs) > 0 ||
		len(ext.KnowledgeRefs) > 0 ||
		len(ext.WorldState) > 0 ||
		len(ext.WorldDiff) > 0 ||
		len(ext.FeedbackSignals) > 0
	if !hasContent {
		return nil
	}
	cloned := &CognitiveExtension{}
	if len(ext.Beliefs) > 0 {
		cloned.Beliefs = CloneBeliefs(ext.Beliefs)
	}
	if len(ext.KnowledgeRefs) > 0 {
		cloned.KnowledgeRefs = CloneKnowledgeReferences(ext.KnowledgeRefs)
	}
	if len(ext.WorldState) > 0 {
		cloned.WorldState = CloneMapAny(ext.WorldState)
	}
	if len(ext.WorldDiff) > 0 {
		cloned.WorldDiff = CloneMapAny(ext.WorldDiff)
	}
	if len(ext.FeedbackSignals) > 0 {
		cloned.FeedbackSignals = CloneFeedbackSignals(ext.FeedbackSignals)
	}
	return cloned
}
