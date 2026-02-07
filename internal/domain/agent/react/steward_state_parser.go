package react

import (
	"fmt"
	"strings"
	"unicode/utf8"

	agent "alex/internal/domain/agent/ports/agent"
	jsonx "alex/internal/shared/json"
)

const (
	stewardStateOpenTag  = "<NEW_STATE>"
	stewardStateCloseTag = "</NEW_STATE>"
)

// ExtractNewState scans content for a <NEW_STATE>...</NEW_STATE> block, parses
// the contained JSON into a StewardState, and returns:
//   - state: the parsed state (nil on failure or absence)
//   - cleaned: the content with the <NEW_STATE> block removed
//   - err: non-nil only for structural parse failures (caller should not abort)
func ExtractNewState(content string) (state *agent.StewardState, cleaned string, err error) {
	openIdx := strings.Index(content, stewardStateOpenTag)
	if openIdx == -1 {
		return nil, content, nil
	}

	bodyStart := openIdx + len(stewardStateOpenTag)
	closeIdx := strings.Index(content[bodyStart:], stewardStateCloseTag)
	if closeIdx == -1 {
		// Unclosed tag — treat as absent.
		return nil, content, nil
	}
	closeIdx += bodyStart

	raw := strings.TrimSpace(content[bodyStart:closeIdx])
	if raw == "" {
		return nil, removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag)), nil
	}

	// Parse as JSON.
	var parsed agent.StewardState
	if jsonErr := jsonx.Unmarshal([]byte(raw), &parsed); jsonErr != nil {
		// Try YAML-like (not implemented — JSON only for now).
		return nil, removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag)), jsonErr
	}

	// Validate size.
	rendered := agent.RenderAsReminder(&parsed)
	if utf8.RuneCountInString(rendered) > agent.MaxStewardStateChars {
		return &parsed, removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag)), &stewardStateOversizeError{
			chars: utf8.RuneCountInString(rendered),
			limit: agent.MaxStewardStateChars,
		}
	}

	cleaned = removeBlock(content, openIdx, closeIdx+len(stewardStateCloseTag))
	return &parsed, cleaned, nil
}

// removeBlock excises content[start:end] and collapses surrounding whitespace.
func removeBlock(content string, start, end int) string {
	before := content[:start]
	after := ""
	if end < len(content) {
		after = content[end:]
	}
	result := strings.TrimSpace(before) + "\n" + strings.TrimSpace(after)
	return strings.TrimSpace(result)
}

type stewardStateOversizeError struct {
	chars int
	limit int
}

func (e *stewardStateOversizeError) Error() string {
	return "steward state exceeds character limit"
}

// IsStewardStateOversize reports whether an error indicates the parsed state
// was too large.
func IsStewardStateOversize(err error) bool {
	_, ok := err.(*stewardStateOversizeError)
	return ok
}

// ValidateStewardEvidenceRefs validates that each decision has an evidence_ref
// and every reference is present in evidence_index.
func ValidateStewardEvidenceRefs(state *agent.StewardState) []string {
	if state == nil || len(state.Decisions) == 0 {
		return nil
	}

	index := make(map[string]struct{}, len(state.EvidenceIndex))
	for _, ref := range state.EvidenceIndex {
		key := strings.TrimSpace(ref.Ref)
		if key == "" {
			continue
		}
		index[key] = struct{}{}
	}

	issues := make([]string, 0)
	for i, decision := range state.Decisions {
		ref := strings.TrimSpace(decision.EvidenceRef)
		if ref == "" {
			issues = append(issues, fmt.Sprintf("decision[%d] missing evidence_ref", i))
			continue
		}
		if _, ok := index[ref]; !ok {
			issues = append(issues, fmt.Sprintf("decision[%d] evidence_ref %q not found in evidence_index", i, ref))
		}
	}
	return issues
}

// CompressStewardStateForLimit prunes low-priority fields to fit the given
// character budget. It returns the compressed copy and whether compression
// succeeded under the target limit.
func CompressStewardStateForLimit(state *agent.StewardState, maxChars int) (*agent.StewardState, bool) {
	if state == nil {
		return nil, false
	}
	if maxChars <= 0 {
		maxChars = agent.MaxStewardStateChars
	}

	out := cloneStewardState(state)
	if stewardStateFits(out, maxChars) {
		return out, true
	}

	trimDecisionAlternatives(out)
	if stewardStateFits(out, maxChars) {
		return out, true
	}

	out.Artifacts = capArtifacts(out.Artifacts, 4)
	out.Risks = capRisks(out.Risks, 4)
	out.Plan = capPlan(out.Plan, 3)
	out.TaskGraph = compactTaskGraph(out.TaskGraph, 6)
	out.Decisions = capDecisions(out.Decisions, 6)
	out.EvidenceIndex = compactEvidenceIndex(out.Decisions, out.EvidenceIndex, 8)
	if stewardStateFits(out, maxChars) {
		return out, true
	}

	out.TaskGraph = compactTaskGraph(out.TaskGraph, 4)
	out.Decisions = capDecisions(out.Decisions, 4)
	out.EvidenceIndex = compactEvidenceIndex(out.Decisions, out.EvidenceIndex, 6)
	out.Plan = capPlan(out.Plan, 2)
	out.Context = truncateRunes(strings.TrimSpace(out.Context), 180)
	out.Goal = truncateRunes(strings.TrimSpace(out.Goal), 120)
	if stewardStateFits(out, maxChars) {
		return out, true
	}

	out.Artifacts = nil
	out.Risks = nil
	out.TaskGraph = compactTaskGraph(out.TaskGraph, 2)
	out.Decisions = truncateDecisions(capDecisions(out.Decisions, 2), 80)
	out.EvidenceIndex = truncateEvidence(compactEvidenceIndex(out.Decisions, out.EvidenceIndex, 3), 120)
	out.Plan = truncatePlan(capPlan(out.Plan, 1), 80)
	out.Context = truncateRunes(strings.TrimSpace(out.Context), 80)
	out.Goal = truncateRunes(strings.TrimSpace(out.Goal), 80)
	if stewardStateFits(out, maxChars) {
		return out, true
	}

	out.Context = ""
	out.TaskGraph = nil
	out.Artifacts = nil
	out.Risks = nil
	out.Plan = truncatePlan(capPlan(out.Plan, 1), 48)
	out.Decisions = truncateDecisions(capDecisions(out.Decisions, 1), 56)
	out.EvidenceIndex = truncateEvidence(compactEvidenceIndex(out.Decisions, out.EvidenceIndex, 1), 80)
	out.Goal = truncateRunes(strings.TrimSpace(out.Goal), 56)
	return out, stewardStateFits(out, maxChars)
}

func stewardStateFits(state *agent.StewardState, maxChars int) bool {
	return utf8.RuneCountInString(agent.RenderAsReminder(state)) <= maxChars
}

func trimDecisionAlternatives(state *agent.StewardState) {
	for i := range state.Decisions {
		state.Decisions[i].Alternatives = ""
	}
}

func capArtifacts(items []agent.ArtifactRef, max int) []agent.ArtifactRef {
	if len(items) <= max || max <= 0 {
		return items
	}
	return append([]agent.ArtifactRef(nil), items[len(items)-max:]...)
}

func capRisks(items []agent.Risk, max int) []agent.Risk {
	if len(items) <= max || max <= 0 {
		return items
	}
	return append([]agent.Risk(nil), items[len(items)-max:]...)
}

func capPlan(items []agent.StewardAction, max int) []agent.StewardAction {
	if len(items) <= max || max <= 0 {
		return items
	}
	return append([]agent.StewardAction(nil), items[:max]...)
}

func capDecisions(items []agent.Decision, max int) []agent.Decision {
	if len(items) <= max || max <= 0 {
		return items
	}
	return append([]agent.Decision(nil), items[len(items)-max:]...)
}

func compactTaskGraph(items []agent.TaskGraphNode, max int) []agent.TaskGraphNode {
	if len(items) <= max || max <= 0 {
		return items
	}
	filtered := make([]agent.TaskGraphNode, 0, len(items))
	for _, node := range items {
		status := strings.ToLower(strings.TrimSpace(node.Status))
		if status == "done" || status == "completed" {
			continue
		}
		filtered = append(filtered, node)
	}
	if len(filtered) == 0 {
		filtered = items
	}
	if len(filtered) <= max {
		return append([]agent.TaskGraphNode(nil), filtered...)
	}
	return append([]agent.TaskGraphNode(nil), filtered[len(filtered)-max:]...)
}

func compactEvidenceIndex(decisions []agent.Decision, index []agent.EvidenceRef, max int) []agent.EvidenceRef {
	if len(index) <= max || max <= 0 {
		return index
	}

	referenced := make(map[string]struct{}, len(decisions))
	for _, decision := range decisions {
		ref := strings.TrimSpace(decision.EvidenceRef)
		if ref == "" {
			continue
		}
		referenced[ref] = struct{}{}
	}

	selected := make([]agent.EvidenceRef, 0, len(index))
	for _, ref := range index {
		key := strings.TrimSpace(ref.Ref)
		if key == "" {
			continue
		}
		if _, ok := referenced[key]; ok {
			selected = append(selected, ref)
		}
	}
	if len(selected) == 0 {
		selected = append(selected, index...)
	}
	if len(selected) <= max {
		return append([]agent.EvidenceRef(nil), selected...)
	}
	return append([]agent.EvidenceRef(nil), selected[len(selected)-max:]...)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func truncatePlan(items []agent.StewardAction, limit int) []agent.StewardAction {
	if len(items) == 0 {
		return items
	}
	truncated := append([]agent.StewardAction(nil), items...)
	for i := range truncated {
		truncated[i].Input = truncateRunes(strings.TrimSpace(truncated[i].Input), limit)
		truncated[i].Tool = truncateRunes(strings.TrimSpace(truncated[i].Tool), 24)
		truncated[i].Checkpoint = truncateRunes(strings.TrimSpace(truncated[i].Checkpoint), limit)
		truncated[i].OutputID = truncateRunes(strings.TrimSpace(truncated[i].OutputID), 32)
	}
	return truncated
}

func truncateDecisions(items []agent.Decision, limit int) []agent.Decision {
	if len(items) == 0 {
		return items
	}
	truncated := append([]agent.Decision(nil), items...)
	for i := range truncated {
		truncated[i].Conclusion = truncateRunes(strings.TrimSpace(truncated[i].Conclusion), limit)
		truncated[i].Alternatives = ""
		truncated[i].EvidenceRef = truncateRunes(strings.TrimSpace(truncated[i].EvidenceRef), 32)
	}
	return truncated
}

func truncateEvidence(items []agent.EvidenceRef, summaryLimit int) []agent.EvidenceRef {
	if len(items) == 0 {
		return items
	}
	truncated := append([]agent.EvidenceRef(nil), items...)
	for i := range truncated {
		truncated[i].Ref = truncateRunes(strings.TrimSpace(truncated[i].Ref), 32)
		truncated[i].Source = truncateRunes(strings.TrimSpace(truncated[i].Source), 64)
		truncated[i].Summary = truncateRunes(strings.TrimSpace(truncated[i].Summary), summaryLimit)
	}
	return truncated
}

func cloneStewardState(state *agent.StewardState) *agent.StewardState {
	if state == nil {
		return nil
	}

	cloned := *state
	cloned.Plan = append([]agent.StewardAction(nil), state.Plan...)
	cloned.TaskGraph = append([]agent.TaskGraphNode(nil), state.TaskGraph...)
	cloned.Decisions = append([]agent.Decision(nil), state.Decisions...)
	cloned.Artifacts = append([]agent.ArtifactRef(nil), state.Artifacts...)
	cloned.Risks = append([]agent.Risk(nil), state.Risks...)
	cloned.EvidenceIndex = append([]agent.EvidenceRef(nil), state.EvidenceIndex...)
	return &cloned
}
