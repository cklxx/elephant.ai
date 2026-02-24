package lark

import (
	"strings"
	"testing"
)

func TestParseLarkTextPayload(t *testing.T) {
	t.Parallel()

	text, ok := parseLarkTextPayload(`{"text":"  hello  "}`)
	if !ok || text != "hello" {
		t.Fatalf("expected trimmed text payload, got ok=%v text=%q", ok, text)
	}

	emptyText, emptyOK := parseLarkTextPayload(`{"text":""}`)
	if !emptyOK || emptyText != "" {
		t.Fatalf("expected empty text payload to parse, got ok=%v text=%q", emptyOK, emptyText)
	}

	if _, ok := parseLarkTextPayload(`{invalid`); ok {
		t.Fatalf("expected invalid payload parse failure")
	}
}

func TestParseLarkPostPayload(t *testing.T) {
	t.Parallel()

	payload, ok := parseLarkPostPayload(`{"title":"Title","content":[[{"tag":"text","text":"hello"}]]}`)
	if !ok {
		t.Fatalf("expected post payload to parse")
	}
	if payload.Title != "Title" || len(payload.Content) != 1 || len(payload.Content[0]) != 1 {
		t.Fatalf("unexpected post payload: %+v", payload)
	}

	if _, ok := parseLarkPostPayload(`not json`); ok {
		t.Fatalf("expected invalid payload parse failure")
	}
}

func TestFlattenLarkPostPayload(t *testing.T) {
	t.Parallel()

	payload := larkPostPayload{
		Title: "Title",
		Content: [][]larkPostElement{
			{{Tag: "text", Text: "hello"}},
			{{Tag: "at", UserID: "ou_1", UserName: "Alice"}},
			{{Tag: "unknown", Text: "!"}},
		},
	}

	got := flattenLarkPostPayload(
		payload,
		strings.ToUpper,
		func(el larkPostElement) string {
			if strings.TrimSpace(el.UserID) != "" {
				return "@" + el.UserID
			}
			return "@" + strings.TrimSpace(el.UserName)
		},
	)
	if got != "Title\nHELLO\n@ou_1\n!" {
		t.Fatalf("unexpected flattened payload: %q", got)
	}

	gotNoMention := flattenLarkPostPayload(payload, nil, nil)
	if gotNoMention != "Title\nhello\n\n!" {
		t.Fatalf("unexpected payload with nil renderers: %q", gotNoMention)
	}
}
