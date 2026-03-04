package ports

// CompressionPlan describes which messages should be compacted and which
// messages should be summarized for synthetic replacement.
type CompressionPlan struct {
	CompressibleIndexes map[int]struct{}
	SummarySource       []Message
}

// CompressionPlanOptions controls compression plan behavior.
type CompressionPlanOptions struct {
	KeepRecentTurns int
	PreserveSource  func(MessageSource) bool
	IsSynthetic     func(Message) bool
}

// BuildCompressionPlan computes a deterministic compaction plan shared by
// context manager and react runtime.
func BuildCompressionPlan(messages []Message, opts CompressionPlanOptions) CompressionPlan {
	plan := CompressionPlan{CompressibleIndexes: map[int]struct{}{}}
	if len(messages) == 0 {
		return plan
	}

	keepRecentTurns := opts.KeepRecentTurns
	if keepRecentTurns <= 0 {
		keepRecentTurns = 1
	}

	preserveSource := opts.PreserveSource
	if preserveSource == nil {
		preserveSource = IsPreservedSource
	}

	isSynthetic := opts.IsSynthetic
	if isSynthetic == nil {
		isSynthetic = func(Message) bool { return false }
	}

	conversation := make([]Message, 0, len(messages))
	conversationIndexes := make([]int, 0, len(messages))
	for idx, msg := range messages {
		if preserveSource(msg.Source) {
			continue
		}
		conversation = append(conversation, msg)
		conversationIndexes = append(conversationIndexes, idx)
	}
	if len(conversation) == 0 {
		return plan
	}

	keptConversation := KeepRecentTurns(conversation, keepRecentTurns)
	compressibleCount := len(conversation) - len(keptConversation)
	if compressibleCount <= 0 {
		return plan
	}

	plan.CompressibleIndexes = make(map[int]struct{}, compressibleCount)
	plan.SummarySource = make([]Message, 0, compressibleCount)
	for idx := 0; idx < compressibleCount; idx++ {
		plan.CompressibleIndexes[conversationIndexes[idx]] = struct{}{}
		msg := conversation[idx]
		if isSynthetic(msg) {
			continue
		}
		plan.SummarySource = append(plan.SummarySource, msg)
	}

	return plan
}
