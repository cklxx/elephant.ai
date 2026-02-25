package larktesting

import (
	"testing"
	"time"

	larkgw "alex/internal/delivery/channels/lark"
)

func TestAssertMessengerMinCount(t *testing.T) {
	calls := []larkgw.MessengerCall{
		{Method: "ReplyMessage", Content: `{"text":"hello"}`},
	}

	t.Run("min_count satisfied", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", MinCount: 1},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("min_count not satisfied", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", MinCount: 2},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})
}

func TestAssertMessengerContentContains(t *testing.T) {
	calls := []larkgw.MessengerCall{
		{Method: "ReplyMessage", Content: `{"text":"hello world 42"}`},
	}

	t.Run("content match", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", ContentContains: []string{"hello", "42"}},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("content mismatch", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", ContentContains: []string{"missing"}},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})
}

func TestAssertMessengerContentAbsent(t *testing.T) {
	calls := []larkgw.MessengerCall{
		{Method: "ReplyMessage", Content: `{"text":"secret data"}`},
	}

	t.Run("absent satisfied", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", ContentAbsent: []string{"password"}},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("absent violated", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", ContentAbsent: []string{"secret"}},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})
}

func TestAssertMessengerMaxCount(t *testing.T) {
	calls := []larkgw.MessengerCall{
		{Method: "ReplyMessage", Content: "a"},
		{Method: "ReplyMessage", Content: "b"},
		{Method: "ReplyMessage", Content: "c"},
	}

	t.Run("max_count satisfied", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", MaxCount: 3},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("max_count violated", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "ReplyMessage", MaxCount: 2},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})
}

func TestAssertMessengerEmojiType(t *testing.T) {
	calls := []larkgw.MessengerCall{
		{Method: "AddReaction", Emoji: "SMILE"},
	}

	t.Run("emoji match", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "AddReaction", EmojiType: "SMILE"},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("emoji mismatch", func(t *testing.T) {
		assertions := []MessengerAssertion{
			{Method: "AddReaction", EmojiType: "HEART"},
		}
		errs := assertMessenger(assertions, calls)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})
}

func TestAssertNoCall(t *testing.T) {
	calls := []larkgw.MessengerCall{
		{Method: "ReplyMessage"},
	}

	t.Run("no violation", func(t *testing.T) {
		errs := assertNoCall([]string{"SendMessage"}, calls)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("violation", func(t *testing.T) {
		errs := assertNoCall([]string{"ReplyMessage"}, calls)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})
}

func TestAssertExecutor(t *testing.T) {
	tr := TurnResult{Task: "hello world", Called: true}

	t.Run("task_contains", func(t *testing.T) {
		boolTrue := true
		a := &ExecutorAssertion{TaskContains: []string{"hello"}, Called: &boolTrue}
		errs := assertExecutor(a, tr)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("task_absent violated", func(t *testing.T) {
		a := &ExecutorAssertion{TaskAbsent: []string{"hello"}}
		errs := assertExecutor(a, tr)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("called false check", func(t *testing.T) {
		boolFalse := false
		a := &ExecutorAssertion{Called: &boolFalse}
		errs := assertExecutor(a, tr)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("nil assertion", func(t *testing.T) {
		errs := assertExecutor(nil, tr)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})
}

func TestAssertTiming(t *testing.T) {
	tr := TurnResult{Duration: 100 * time.Millisecond}

	t.Run("within limit", func(t *testing.T) {
		a := &TimingAssertion{MaxMS: 200}
		errs := assertTiming(a, tr)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		a := &TimingAssertion{MaxMS: 50}
		errs := assertTiming(a, tr)
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}
	})

	t.Run("nil assertion", func(t *testing.T) {
		errs := assertTiming(nil, tr)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
	})
}
