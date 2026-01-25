package react

import "regexp"

// Pre-compiled regexes for hot path performance (avoid recompilation per call)
var cleanToolCallPatterns = []*regexp.Regexp{
	regexp.MustCompile(`<\|tool_call_begin\|>.*`),
	regexp.MustCompile(`<tool_call>.*(?:</tool_call>)?$`),
	regexp.MustCompile(`user<\|tool_call_begin\|>.*`),
	regexp.MustCompile(`functions\.[\w_]+:\d+\(.*`),
	regexp.MustCompile(`(?s)<start_function_call>.*?<end_function_call>`),
}

var genericImageAliasPattern = regexp.MustCompile(`(?i)^image(?:[_\-\s]?(\d+))?(?:\.[a-z0-9]+)?$`)

var contentPlaceholderPattern = regexp.MustCompile(`\[([^\[\]]+)\]`)

const (
	workflowNodeContext  = "react:context"
	workflowNodeFinalize = "react:finalize"

	toolArgInlineLengthLimit        = 256
	toolArgPreviewLength            = 64
	toolArgHistoryInlineLimit       = 256
	maxFeedbackSignals              = 20
	goalPlanPromptDistanceThreshold = 800

	attachmentCatalogMetadataKey = "attachment_catalog"

	attachmentMatchExact           = "exact"
	attachmentMatchCaseInsensitive = "case_insensitive"
	attachmentMatchSeedreamAlias   = "seedream_alias"
	attachmentMatchGeneric         = "generic_alias"

	snapshotSummaryLimit = 160
)
