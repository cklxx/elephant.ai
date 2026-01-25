package app

import (
	"strings"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
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
	if len(snapshot.Beliefs) > 0 {
		state.Beliefs = agent.CloneBeliefs(snapshot.Beliefs)
	}
	if len(snapshot.KnowledgeRefs) > 0 {
		state.KnowledgeRefs = agent.CloneKnowledgeReferences(snapshot.KnowledgeRefs)
	}
	if len(snapshot.WorldState) > 0 {
		state.WorldState = snapshot.WorldState
	}
	if len(snapshot.WorldDiff) > 0 {
		state.WorldDiff = snapshot.WorldDiff
	}
	if len(snapshot.FeedbackSignals) > 0 {
		state.FeedbackSignals = agent.CloneFeedbackSignals(snapshot.FeedbackSignals)
	}
}
