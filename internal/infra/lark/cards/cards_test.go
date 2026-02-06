package cards

import (
	"encoding/json"
	"testing"
)

// helper: unmarshal card JSON and return the top-level map.
func mustParse(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, raw)
	}
	return m
}

// helper: extract the elements array from a parsed card.
func elements(t *testing.T, m map[string]any) []any {
	t.Helper()
	elems, ok := m["elements"].([]any)
	if !ok {
		t.Fatal("missing or invalid elements array")
	}
	return elems
}

// --- Builder tests ---

func TestNewCard_BasicBuild(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "Hello", TitleColor: "blue", EnableForward: true}).Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)

	// Verify header.
	header := m["header"].(map[string]any)
	titleObj := header["title"].(map[string]any)
	if titleObj["content"] != "Hello" {
		t.Errorf("expected title 'Hello', got %v", titleObj["content"])
	}
	if header["template"] != "blue" {
		t.Errorf("expected template 'blue', got %v", header["template"])
	}

	// Verify config.
	cfg := m["config"].(map[string]any)
	if cfg["wide_screen_mode"] != true {
		t.Error("expected wide_screen_mode true")
	}
	if cfg["enable_forward"] != true {
		t.Error("expected enable_forward true")
	}
}

func TestNewCard_DefaultTitleColor(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "No Color"}).Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	header := m["header"].(map[string]any)
	if header["template"] != "blue" {
		t.Errorf("expected default color 'blue', got %v", header["template"])
	}
}

func TestAddMarkdownSection(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "MD"}).
		AddMarkdownSection("**bold** text").
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	div := elems[0].(map[string]any)
	if div["tag"] != "div" {
		t.Errorf("expected tag 'div', got %v", div["tag"])
	}
	text := div["text"].(map[string]any)
	if text["tag"] != "lark_md" {
		t.Errorf("expected text tag 'lark_md', got %v", text["tag"])
	}
	if text["content"] != "**bold** text" {
		t.Errorf("unexpected content: %v", text["content"])
	}
}

func TestAddPlainTextSection(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "PT"}).
		AddPlainTextSection("hello world").
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	div := elems[0].(map[string]any)
	text := div["text"].(map[string]any)
	if text["tag"] != "plain_text" {
		t.Errorf("expected text tag 'plain_text', got %v", text["tag"])
	}
	if text["content"] != "hello world" {
		t.Errorf("unexpected content: %v", text["content"])
	}
}

func TestAddInput(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "Input"}).
		AddInput(InputConfig{
			Name:        "plan_feedback",
			Label:       "修改意见",
			Placeholder: "请输入",
			Required:    true,
		}).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	input := elems[0].(map[string]any)
	if input["tag"] != "input" {
		t.Fatalf("expected input tag, got %v", input["tag"])
	}
	if input["name"] != "plan_feedback" {
		t.Fatalf("expected input name, got %v", input["name"])
	}
	label := input["label"].(map[string]any)
	if label["content"] != "修改意见" {
		t.Fatalf("expected label content, got %v", label["content"])
	}
	placeholder := input["placeholder"].(map[string]any)
	if placeholder["content"] != "请输入" {
		t.Fatalf("expected placeholder content, got %v", placeholder["content"])
	}
	if input["required"] != true {
		t.Fatalf("expected required true, got %v", input["required"])
	}
}

func TestAddDivider(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "Div"}).
		AddDivider().
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	hr := elems[0].(map[string]any)
	if hr["tag"] != "hr" {
		t.Errorf("expected tag 'hr', got %v", hr["tag"])
	}
}

func TestAddActionButtons(t *testing.T) {
	btn1 := NewPrimaryButton("OK", "act_ok").
		WithValue("id", "123").
		WithValue("extra", "yes")
	btn2 := NewDangerButton("Cancel", "act_cancel")

	raw, err := NewCard(CardConfig{Title: "Btns"}).
		AddActionButtons(btn1, btn2).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	actionElem := elems[0].(map[string]any)
	if actionElem["tag"] != "action" {
		t.Errorf("expected tag 'action', got %v", actionElem["tag"])
	}
	actions := actionElem["actions"].([]any)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	// First button: primary with values.
	a1 := actions[0].(map[string]any)
	if a1["type"] != "primary" {
		t.Errorf("expected type 'primary', got %v", a1["type"])
	}
	a1Text := a1["text"].(map[string]any)
	if a1Text["content"] != "OK" {
		t.Errorf("expected button text 'OK', got %v", a1Text["content"])
	}
	vals := a1["value"].(map[string]any)
	if vals["id"] != "123" {
		t.Errorf("expected value id='123', got %v", vals["id"])
	}
	if vals["extra"] != "yes" {
		t.Errorf("expected value extra='yes', got %v", vals["extra"])
	}
	if a1["action_tag"] != "act_ok" {
		t.Errorf("expected action_tag 'act_ok', got %v", a1["action_tag"])
	}

	// Second button: danger, no values.
	a2 := actions[1].(map[string]any)
	if a2["type"] != "danger" {
		t.Errorf("expected type 'danger', got %v", a2["type"])
	}
	if _, hasVal := a2["value"]; hasVal {
		t.Error("expected no value on cancel button")
	}
}

func TestAddNote(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "Note"}).
		AddNote("This is a footer").
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	note := elems[0].(map[string]any)
	if note["tag"] != "note" {
		t.Errorf("expected tag 'note', got %v", note["tag"])
	}
	noteElems := note["elements"].([]any)
	if len(noteElems) != 1 {
		t.Fatalf("expected 1 note element, got %d", len(noteElems))
	}
	ne := noteElems[0].(map[string]any)
	if ne["content"] != "This is a footer" {
		t.Errorf("unexpected note content: %v", ne["content"])
	}
}

func TestPlanReviewCard(t *testing.T) {
	raw, err := PlanReviewCard(PlanReviewParams{
		Goal:                 "ship feature",
		PlanMarkdown:         "- step1\n- step2",
		RunID:                "run-1",
		RequireConfirmation:  true,
		IncludeFeedbackInput: true,
	})
	if err != nil {
		t.Fatalf("PlanReviewCard failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
	foundApprove := false
	foundInput := false
	for _, elem := range elems {
		e := elem.(map[string]any)
		switch e["tag"] {
		case "input":
			foundInput = true
		case "action":
			actions := e["actions"].([]any)
			for _, a := range actions {
				action := a.(map[string]any)
				if action["action_tag"] == "plan_review_approve" {
					foundApprove = true
				}
			}
		}
	}
	if !foundInput {
		t.Fatal("expected feedback input element")
	}
	if !foundApprove {
		t.Fatal("expected approve action_tag")
	}
}

func TestResultCard(t *testing.T) {
	raw, err := ResultCard(ResultParams{
		Title:   "完成",
		Summary: "ok",
		Footer:  "footer",
	})
	if err != nil {
		t.Fatalf("ResultCard failed: %v", err)
	}
	m := mustParse(t, raw)
	header := m["header"].(map[string]any)
	if header["template"] != "green" {
		t.Fatalf("expected green template, got %v", header["template"])
	}
}

func TestErrorCard(t *testing.T) {
	raw, err := ErrorCard("失败", "boom")
	if err != nil {
		t.Fatalf("ErrorCard failed: %v", err)
	}
	m := mustParse(t, raw)
	header := m["header"].(map[string]any)
	if header["template"] != "red" {
		t.Fatalf("expected red template, got %v", header["template"])
	}
}

func TestEmptyCardBuild(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "Empty"}).Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := m["elements"]
	// An empty card should have a nil elements array (JSON null) or empty.
	if elems != nil {
		arr, ok := elems.([]any)
		if ok && len(arr) != 0 {
			t.Errorf("expected empty elements, got %d", len(arr))
		}
	}
}

func TestMethodChaining(t *testing.T) {
	raw, err := NewCard(CardConfig{Title: "Chain"}).
		AddMarkdownSection("line 1").
		AddDivider().
		AddPlainTextSection("line 2").
		AddActionButtons(NewButton("Go", "act_go")).
		AddNote("footer").
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	if len(elems) != 5 {
		t.Errorf("expected 5 elements, got %d", len(elems))
	}

	// Verify order: div, hr, div, action, note.
	tags := []string{"div", "hr", "div", "action", "note"}
	for i, tag := range tags {
		e := elems[i].(map[string]any)
		if e["tag"] != tag {
			t.Errorf("element %d: expected tag %q, got %v", i, tag, e["tag"])
		}
	}
}

// --- Template tests ---

func TestApprovalCard(t *testing.T) {
	raw, err := ApprovalCard("Leave Request", "John wants 3 days off", "req-001")
	if err != nil {
		t.Fatalf("ApprovalCard failed: %v", err)
	}
	m := mustParse(t, raw)

	// Header color should be orange.
	header := m["header"].(map[string]any)
	if header["template"] != "orange" {
		t.Errorf("expected template 'orange', got %v", header["template"])
	}

	// Should contain an action element with 2 buttons.
	elems := elements(t, m)
	var actionElem map[string]any
	for _, e := range elems {
		em := e.(map[string]any)
		if em["tag"] == "action" {
			actionElem = em
			break
		}
	}
	if actionElem == nil {
		t.Fatal("no action element found")
	}
	actions := actionElem["actions"].([]any)
	if len(actions) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(actions))
	}

	// Approve button.
	approve := actions[0].(map[string]any)
	approveText := approve["text"].(map[string]any)
	if approveText["content"] != "Approve" {
		t.Errorf("expected 'Approve', got %v", approveText["content"])
	}
	if approve["type"] != "primary" {
		t.Errorf("expected type 'primary', got %v", approve["type"])
	}
	av := approve["value"].(map[string]any)
	if av["approval_id"] != "req-001" {
		t.Errorf("expected approval_id 'req-001', got %v", av["approval_id"])
	}

	// Reject button.
	reject := actions[1].(map[string]any)
	rejectText := reject["text"].(map[string]any)
	if rejectText["content"] != "Reject" {
		t.Errorf("expected 'Reject', got %v", rejectText["content"])
	}
	if reject["type"] != "danger" {
		t.Errorf("expected type 'danger', got %v", reject["type"])
	}
}

func TestConfirmationCard(t *testing.T) {
	raw, err := ConfirmationCard("Delete file?", "This cannot be undone.", "Delete", "Keep")
	if err != nil {
		t.Fatalf("ConfirmationCard failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)

	var actionElem map[string]any
	for _, e := range elems {
		em := e.(map[string]any)
		if em["tag"] == "action" {
			actionElem = em
			break
		}
	}
	if actionElem == nil {
		t.Fatal("no action element found")
	}
	actions := actionElem["actions"].([]any)
	if len(actions) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(actions))
	}

	confirm := actions[0].(map[string]any)
	confirmText := confirm["text"].(map[string]any)
	if confirmText["content"] != "Delete" {
		t.Errorf("expected 'Delete', got %v", confirmText["content"])
	}
	if confirm["type"] != "primary" {
		t.Errorf("expected type 'primary', got %v", confirm["type"])
	}

	cancel := actions[1].(map[string]any)
	cancelText := cancel["text"].(map[string]any)
	if cancelText["content"] != "Keep" {
		t.Errorf("expected 'Keep', got %v", cancelText["content"])
	}
	if cancel["type"] != "default" {
		t.Errorf("expected type 'default', got %v", cancel["type"])
	}
}

func TestProgressCard(t *testing.T) {
	steps := []ProgressStep{
		{Name: "Design", Status: "done"},
		{Name: "Implement", Status: "active"},
		{Name: "Test", Status: "pending"},
	}
	raw, err := ProgressCard("Sprint 1", steps, 1)
	if err != nil {
		t.Fatalf("ProgressCard failed: %v", err)
	}
	m := mustParse(t, raw)

	// Header color should be green.
	header := m["header"].(map[string]any)
	if header["template"] != "green" {
		t.Errorf("expected template 'green', got %v", header["template"])
	}

	elems := elements(t, m)
	// Should have a markdown section and a note.
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}

	// Check markdown content contains step names.
	div := elems[0].(map[string]any)
	text := div["text"].(map[string]any)
	content := text["content"].(string)
	if !containsAll(content, "Design", "Implement", "Test") {
		t.Errorf("markdown content missing step names: %s", content)
	}
	if !containsAll(content, "[done]", "[active]", "[pending]") {
		t.Errorf("markdown content missing status icons: %s", content)
	}

	// Check note footer.
	note := elems[len(elems)-1].(map[string]any)
	if note["tag"] != "note" {
		t.Errorf("expected last element to be note, got %v", note["tag"])
	}
	noteElems := note["elements"].([]any)
	ne := noteElems[0].(map[string]any)
	if ne["content"] != "1/3 steps completed" {
		t.Errorf("unexpected note content: %v", ne["content"])
	}
}

func TestSummaryCard(t *testing.T) {
	sections := map[string]string{
		"Status":   "Active",
		"Assignee": "Alice",
		"Priority": "High",
	}
	raw, err := SummaryCard("Task Summary", sections)
	if err != nil {
		t.Fatalf("SummaryCard failed: %v", err)
	}
	m := mustParse(t, raw)
	elems := elements(t, m)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}

	div := elems[0].(map[string]any)
	text := div["text"].(map[string]any)
	content := text["content"].(string)

	// Keys should be sorted alphabetically.
	if !containsAll(content, "**Assignee**: Alice", "**Priority**: High", "**Status**: Active") {
		t.Errorf("unexpected summary content: %s", content)
	}

	// Verify ordering: Assignee < Priority < Status.
	idxA := indexOf(content, "Assignee")
	idxP := indexOf(content, "Priority")
	idxS := indexOf(content, "Status")
	if !(idxA < idxP && idxP < idxS) {
		t.Errorf("keys not sorted: Assignee@%d, Priority@%d, Status@%d", idxA, idxP, idxS)
	}
}

func TestButtonWithValue(t *testing.T) {
	b := NewButton("Click", "act_click").
		WithValue("a", "1").
		WithValue("b", "2")
	if b.Value["a"] != "1" || b.Value["b"] != "2" {
		t.Errorf("unexpected values: %v", b.Value)
	}
	// WithValue should not mutate the original map reference unexpectedly.
	b2 := b.WithValue("c", "3")
	if b2.Value["c"] != "3" {
		t.Errorf("expected c=3, got %v", b2.Value["c"])
	}
}

func TestOutputIsValidJSON(t *testing.T) {
	// Build a complex card and confirm it round-trips through JSON.
	raw, err := NewCard(CardConfig{Title: "JSON Test", TitleColor: "red", EnableForward: true}).
		AddMarkdownSection("**heading**\nline two").
		AddPlainTextSection("plain").
		AddDivider().
		AddActionButtons(
			NewPrimaryButton("Go", "go").WithValue("k", "v"),
			NewDangerButton("Stop", "stop"),
		).
		AddNote("note text").
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Round-trip: unmarshal then marshal again.
	var intermediate any
	if err := json.Unmarshal([]byte(raw), &intermediate); err != nil {
		t.Fatalf("first unmarshal failed: %v", err)
	}
	reJSON, err := json.Marshal(intermediate)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if len(reJSON) == 0 {
		t.Error("re-marshaled JSON is empty")
	}
}

// --- helpers ---

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if indexOf(s, sub) < 0 {
			return false
		}
	}
	return true
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
