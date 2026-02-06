package larktesting

import (
	"fmt"
	"strings"

	larkgw "alex/internal/delivery/channels/lark"
)

// evaluateAssertions checks all assertions for a turn and returns error messages.
func evaluateAssertions(assertions TurnAssertions, tr TurnResult) []string {
	var errs []string

	errs = append(errs, assertMessenger(assertions.Messenger, tr.Calls)...)
	errs = append(errs, assertNoCall(assertions.NoCall, tr.Calls)...)
	errs = append(errs, assertExecutor(assertions.Executor, tr)...)
	errs = append(errs, assertTiming(assertions.Timing, tr)...)

	return errs
}

// assertMessenger checks outbound messenger calls against expectations.
func assertMessenger(assertions []MessengerAssertion, calls []larkgw.MessengerCall) []string {
	var errs []string

	for _, a := range assertions {
		matching := filterCalls(calls, a.Method)

		minCount := a.MinCount
		if minCount == 0 {
			minCount = 1
		}

		if len(matching) < minCount {
			errs = append(errs, fmt.Sprintf(
				"expected at least %d %s call(s), got %d",
				minCount, a.Method, len(matching),
			))
			continue
		}

		if a.MaxCount > 0 && len(matching) > a.MaxCount {
			errs = append(errs, fmt.Sprintf(
				"expected at most %d %s call(s), got %d",
				a.MaxCount, a.Method, len(matching),
			))
		}

		// Check content assertions against any matching call.
		if len(a.ContentContains) > 0 || len(a.ContentAbsent) > 0 {
			for _, sub := range a.ContentContains {
				if !anyCallContains(matching, sub) {
					errs = append(errs, fmt.Sprintf(
						"%s: no call content contains %q (calls: %s)",
						a.Method, sub, summarizeCalls(matching),
					))
				}
			}
			for _, sub := range a.ContentAbsent {
				if anyCallContains(matching, sub) {
					errs = append(errs, fmt.Sprintf(
						"%s: call content should not contain %q",
						a.Method, sub,
					))
				}
			}
		}

		if a.EmojiType != "" {
			if !anyCallHasEmoji(matching, a.EmojiType) {
				errs = append(errs, fmt.Sprintf(
					"%s: no call has emoji %q",
					a.Method, a.EmojiType,
				))
			}
		}
	}

	return errs
}

// assertNoCall verifies that certain methods were NOT called.
func assertNoCall(noCalls []string, calls []larkgw.MessengerCall) []string {
	var errs []string
	for _, method := range noCalls {
		matching := filterCalls(calls, method)
		if len(matching) > 0 {
			errs = append(errs, fmt.Sprintf(
				"expected no %s calls, but got %d",
				method, len(matching),
			))
		}
	}
	return errs
}

// assertExecutor checks executor-level assertions.
func assertExecutor(a *ExecutorAssertion, tr TurnResult) []string {
	if a == nil {
		return nil
	}
	var errs []string

	if a.Called != nil {
		if *a.Called && !tr.Called {
			errs = append(errs, "expected ExecuteTask to be called")
		}
		if !*a.Called && tr.Called {
			errs = append(errs, "expected ExecuteTask NOT to be called")
		}
	}

	for _, sub := range a.TaskContains {
		if !strings.Contains(tr.Task, sub) {
			errs = append(errs, fmt.Sprintf(
				"expected task to contain %q, got %q", sub, tr.Task,
			))
		}
	}

	for _, sub := range a.TaskAbsent {
		if strings.Contains(tr.Task, sub) {
			errs = append(errs, fmt.Sprintf(
				"expected task NOT to contain %q", sub,
			))
		}
	}

	return errs
}

// assertTiming checks response time constraints.
func assertTiming(a *TimingAssertion, tr TurnResult) []string {
	if a == nil || a.MaxMS == 0 {
		return nil
	}
	maxDuration := int(tr.Duration.Milliseconds())
	if maxDuration > a.MaxMS {
		return []string{fmt.Sprintf(
			"turn took %dms, exceeds max %dms",
			maxDuration, a.MaxMS,
		)}
	}
	return nil
}

// --- helpers ---

func filterCalls(calls []larkgw.MessengerCall, method string) []larkgw.MessengerCall {
	var out []larkgw.MessengerCall
	for _, c := range calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

func anyCallContains(calls []larkgw.MessengerCall, sub string) bool {
	for _, c := range calls {
		if strings.Contains(c.Content, sub) {
			return true
		}
	}
	return false
}

func anyCallHasEmoji(calls []larkgw.MessengerCall, emoji string) bool {
	for _, c := range calls {
		if c.Emoji == emoji {
			return true
		}
	}
	return false
}

func summarizeCalls(calls []larkgw.MessengerCall) string {
	if len(calls) == 0 {
		return "(none)"
	}
	var parts []string
	for _, c := range calls {
		content := c.Content
		if len(content) > 80 {
			content = content[:80] + "..."
		}
		parts = append(parts, fmt.Sprintf("[%s: %s]", c.Method, content))
	}
	return strings.Join(parts, ", ")
}
