package hooks

import "alex/internal/agent/textutil"

var stopWords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "is": {}, "are": {},
	"was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"have": {}, "has": {}, "had": {}, "do": {}, "does": {},
	"did": {}, "will": {}, "would": {}, "could": {}, "should": {},
	"may": {}, "might": {}, "shall": {}, "can": {},
	"in": {}, "on": {}, "at": {}, "to": {}, "for": {},
	"of": {}, "with": {}, "by": {}, "from": {}, "as": {},
	"into": {}, "about": {}, "between": {},
	"and": {}, "or": {}, "but": {}, "not": {}, "no": {},
	"if": {}, "then": {}, "else": {}, "when": {}, "while": {},
	"it": {}, "its": {}, "this": {}, "that": {}, "these": {},
	"those": {}, "my": {}, "your": {}, "his": {}, "her": {},
	"me": {}, "you": {}, "we": {}, "they": {}, "them": {},
	"what": {}, "which": {}, "who": {}, "how": {}, "where": {},
	"there": {}, "here": {},
	"all": {}, "each": {}, "every": {}, "some": {}, "any": {},
	"help": {}, "please": {}, "just": {}, "also": {},
	"的": {}, "了": {}, "是": {}, "在": {}, "和": {},
	"我": {}, "你": {}, "他": {}, "她": {}, "它": {},
	"这": {}, "那": {}, "有": {}, "不": {}, "也": {},
	"都": {}, "会": {}, "就": {}, "还": {}, "把": {},
	"吗": {}, "呢": {}, "吧": {}, "啊": {}, "帮": {},
	"请": {}, "要": {}, "能": {}, "可以": {},
}

func extractKeywords(input string) []string {
	return textutil.ExtractKeywords(input, textutil.KeywordOptions{StopWords: stopWords})
}
