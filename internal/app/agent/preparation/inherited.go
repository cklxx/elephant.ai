package preparation

import (
	"strings"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

func (s *ExecutionPreparationService) applyInheritedStateSnapshot(state *domain.TaskState, inherited *agent.TaskState) {
	if state == nil || inherited == nil {
		return
	}
	snapshot := agent.CloneTaskState(inherited)
	if trimmed := strings.TrimSpace(snapshot.SystemPrompt); trimmed != "" {
		state.SystemPrompt = trimmed
	}
	if len(snapshot.Messages) > 0 {
		state.Messages = agent.CloneMessages(snapshot.Messages)
	}
	if len(snapshot.Attachments) > 0 {
		if state.Attachments == nil {
			state.Attachments = make(map[string]ports.Attachment)
		}
		mergeAttachmentMaps(state.Attachments, snapshot.Attachments)
	}
	if len(snapshot.Important) > 0 {
		if state.Important == nil {
			state.Important = make(map[string]ports.ImportantNote)
		}
		mergeImportantNotes(state.Important, snapshot.Important)
	}
	if len(snapshot.AttachmentIterations) > 0 {
		if state.AttachmentIterations == nil {
			state.AttachmentIterations = make(map[string]int)
		}
		for key, iter := range snapshot.AttachmentIterations {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			state.AttachmentIterations[name] = iter
		}
	}
	if len(snapshot.Plans) > 0 {
		state.Plans = agent.ClonePlanNodes(snapshot.Plans)
	}
	if snapshot.Cognitive != nil {
		src := snapshot.Cognitive
		dst := state.EnsureCognitive()
		if len(src.Beliefs) > 0 {
			dst.Beliefs = agent.CloneBeliefs(src.Beliefs)
		}
		if len(src.KnowledgeRefs) > 0 {
			dst.KnowledgeRefs = agent.CloneKnowledgeReferences(src.KnowledgeRefs)
		}
		if len(src.WorldState) > 0 {
			dst.WorldState = src.WorldState
		}
		if len(src.WorldDiff) > 0 {
			dst.WorldDiff = src.WorldDiff
		}
		if len(src.FeedbackSignals) > 0 {
			dst.FeedbackSignals = agent.CloneFeedbackSignals(src.FeedbackSignals)
		}
	}
}
