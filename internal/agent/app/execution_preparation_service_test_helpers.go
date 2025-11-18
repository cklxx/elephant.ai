package app

const historySummaryResponseStub = "History Summary: Marketing experiments and automation context recalled."

// historySummaryResponse provides a shared stubbed summary string so history-aware
// execution-preparation tests don't redeclare their own copies (which previously
// triggered golangci-lint duplicate symbol errors when merged).
func historySummaryResponse() string {
	return historySummaryResponseStub
}
