package channels

import (
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestShapeReply7C_CollapsesWhitespaceAndDuplicates(t *testing.T) {
	input := "  \n进度如下：\n\n\n状态：进行中\n状态：进行中\n\n\n- A\n- B  \n"
	got := ShapeReply7C(input)
	want := "进度如下：\n\n状态：进行中\n\n- A\n- B"
	if got != want {
		t.Fatalf("ShapeReply7C() = %q, want %q", got, want)
	}
}

func TestShapeReply7C_DoesNotDeduplicateStructuredListLines(t *testing.T) {
	input := "- A\n- A\n"
	got := ShapeReply7C(input)
	want := "- A\n- A"
	if got != want {
		t.Fatalf("ShapeReply7C() = %q, want %q", got, want)
	}
}

func TestShapeReply7C_PreservesCodeFenceContent(t *testing.T) {
	input := "结果如下：\n结果如下：\n\n```go\nfmt.Println(\"x\")\nfmt.Println(\"x\")\n```\n\n"
	got := ShapeReply7C(input)
	want := "结果如下：\n\n```go\nfmt.Println(\"x\")\nfmt.Println(\"x\")\n```"
	if got != want {
		t.Fatalf("ShapeReply7C() = %q, want %q", got, want)
	}
}

func TestBuildReplyCore_AppliesSevenCShaping(t *testing.T) {
	cfg := BaseConfig{ReplyPrefix: "[bot] "}
	result := &agent.TaskResult{
		Answer: "  第一行\n\n\n第二行\n第二行\n",
	}
	got := BuildReplyCore(cfg, result, nil)
	want := "[bot] 第一行\n\n第二行"
	if got != want {
		t.Fatalf("BuildReplyCore() = %q, want %q", got, want)
	}
}
