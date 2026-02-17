package lark

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func strPtr(s string) *string { return &s }

// --- withAtPrefix ---

func TestWithAtPrefix_Empty(t *testing.T) {
	if got := withAtPrefix(""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := withAtPrefix("   "); got != "" {
		t.Fatalf("expected empty for whitespace, got %q", got)
	}
}

func TestWithAtPrefix_AlreadyHasPrefix(t *testing.T) {
	if got := withAtPrefix("@alice"); got != "@alice" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestWithAtPrefix_AddPrefix(t *testing.T) {
	if got := withAtPrefix("alice"); got != "@alice" {
		t.Fatalf("expected @alice, got %q", got)
	}
}

func TestWithAtPrefix_TrimmedBeforePrefix(t *testing.T) {
	if got := withAtPrefix("  bob  "); got != "@bob" {
		t.Fatalf("expected @bob, got %q", got)
	}
}

// --- formatReadableMention ---

func TestFormatReadableMention_NameAndID(t *testing.T) {
	got := formatReadableMention("Alice", "ou_alice123", "fallback")
	if got != "@Alice(ou_alice123)" {
		t.Fatalf("expected @Alice(ou_alice123), got %q", got)
	}
}

func TestFormatReadableMention_NameOnly(t *testing.T) {
	got := formatReadableMention("Alice", "", "")
	if got != "@Alice" {
		t.Fatalf("expected @Alice, got %q", got)
	}
}

func TestFormatReadableMention_NameEqualsID(t *testing.T) {
	got := formatReadableMention("alice", "alice", "")
	if got != "@alice" {
		t.Fatalf("expected @alice (no parens when same), got %q", got)
	}
}

func TestFormatReadableMention_IDOnly(t *testing.T) {
	got := formatReadableMention("", "ou_bob", "")
	if got != "@ou_bob" {
		t.Fatalf("expected @ou_bob, got %q", got)
	}
}

func TestFormatReadableMention_FallbackOnly(t *testing.T) {
	got := formatReadableMention("", "", "@_user_1")
	if got != "@_user_1" {
		t.Fatalf("expected @_user_1, got %q", got)
	}
}

func TestFormatReadableMention_AllEmpty(t *testing.T) {
	if got := formatReadableMention("", "", ""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// --- mentionKeyMap ---

func TestMentionKeyMap_Nil(t *testing.T) {
	if got := mentionKeyMap(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestMentionKeyMap_Empty(t *testing.T) {
	if got := mentionKeyMap([]*larkim.MentionEvent{}); got != nil {
		t.Fatalf("expected nil for empty, got %v", got)
	}
}

func TestMentionKeyMap_SkipsNilEntries(t *testing.T) {
	mentions := []*larkim.MentionEvent{nil, nil}
	if got := mentionKeyMap(mentions); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestMentionKeyMap_ExtractsOpenID(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		{
			Key:  strPtr("@_user_1"),
			Name: strPtr("Alice"),
			Id:   &larkim.UserId{OpenId: strPtr("ou_alice")},
		},
	}
	got := mentionKeyMap(mentions)
	if got == nil {
		t.Fatal("expected non-nil map")
	}
	info, ok := got["@_user_1"]
	if !ok {
		t.Fatal("expected key @_user_1")
	}
	if info.Name != "Alice" || info.ID != "ou_alice" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestMentionKeyMap_FallsBackToUserID(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		{
			Key: strPtr("@_user_2"),
			Id:  &larkim.UserId{UserId: strPtr("uid_bob")},
		},
	}
	got := mentionKeyMap(mentions)
	if got["@_user_2"].ID != "uid_bob" {
		t.Fatalf("expected UserId fallback, got %q", got["@_user_2"].ID)
	}
}

func TestMentionKeyMap_SkipsEmptyKey(t *testing.T) {
	mentions := []*larkim.MentionEvent{
		{Key: strPtr(""), Name: strPtr("Alice")},
	}
	if got := mentionKeyMap(mentions); got != nil {
		t.Fatalf("expected nil for empty key, got %v", got)
	}
}

// --- renderIncomingMentionPlaceholders ---

func TestRenderIncomingMentionPlaceholders_Empty(t *testing.T) {
	if got := renderIncomingMentionPlaceholders("", nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestRenderIncomingMentionPlaceholders_NoMentions(t *testing.T) {
	if got := renderIncomingMentionPlaceholders("hello", nil); got != "hello" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestRenderIncomingMentionPlaceholders_Replaces(t *testing.T) {
	mentions := map[string]mentionInfo{
		"@_user_1": {Name: "Alice", ID: "ou_alice"},
	}
	got := renderIncomingMentionPlaceholders("Hello @_user_1!", mentions)
	if got != "Hello @Alice(ou_alice)!" {
		t.Fatalf("expected mention replaced, got %q", got)
	}
}

func TestRenderIncomingMentionPlaceholders_LongerKeysFirst(t *testing.T) {
	// @_user_10 should be replaced before @_user_1
	mentions := map[string]mentionInfo{
		"@_user_1":  {Name: "Alice", ID: "ou_alice"},
		"@_user_10": {Name: "Bob", ID: "ou_bob"},
	}
	got := renderIncomingMentionPlaceholders("Hi @_user_10 and @_user_1", mentions)
	if got != "Hi @Bob(ou_bob) and @Alice(ou_alice)" {
		t.Fatalf("expected longer key replaced first, got %q", got)
	}
}

// --- extractTextContent ---

func TestExtractTextContent_ValidJSON(t *testing.T) {
	raw := `{"text":"hello world"}`
	got := extractTextContent(raw, nil)
	if got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}

func TestExtractTextContent_Empty(t *testing.T) {
	if got := extractTextContent("", nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractTextContent_InvalidJSON(t *testing.T) {
	got := extractTextContent("plain text", nil)
	if got != "plain text" {
		t.Fatalf("expected raw fallback, got %q", got)
	}
}

func TestExtractTextContent_WhitespaceOnly(t *testing.T) {
	raw := `{"text":"   "}`
	if got := extractTextContent(raw, nil); got != "" {
		t.Fatalf("expected empty for whitespace, got %q", got)
	}
}

// --- extractPostContent ---

func TestExtractPostContent_Empty(t *testing.T) {
	if got := extractPostContent("", nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractPostContent_TitleOnly(t *testing.T) {
	raw := `{"title":"My Title","content":[]}`
	got := extractPostContent(raw, nil)
	if got != "My Title" {
		t.Fatalf("expected title, got %q", got)
	}
}

func TestExtractPostContent_TextElements(t *testing.T) {
	raw := `{"title":"","content":[[{"tag":"text","text":"Hello "},{"tag":"text","text":"World"}]]}`
	got := extractPostContent(raw, nil)
	if got != "Hello World" {
		t.Fatalf("expected concatenated text, got %q", got)
	}
}

func TestExtractPostContent_MultiLine(t *testing.T) {
	raw := `{"title":"","content":[[{"tag":"text","text":"line1"}],[{"tag":"text","text":"line2"}]]}`
	got := extractPostContent(raw, nil)
	if got != "line1\nline2" {
		t.Fatalf("expected multiline, got %q", got)
	}
}

func TestExtractPostContent_InvalidJSON(t *testing.T) {
	got := extractPostContent("not json", nil)
	if got != "not json" {
		t.Fatalf("expected raw fallback, got %q", got)
	}
}

// --- renderOutgoingMentions ---

func TestRenderOutgoingMentions_NoMentions(t *testing.T) {
	got := renderOutgoingMentions("hello world")
	if got != "hello world" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestRenderOutgoingMentions_WithMention(t *testing.T) {
	got := renderOutgoingMentions("Hello @Alice(ou_alice123)")
	expected := `Hello <at user_id="ou_alice123">Alice</at>`
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestRenderOutgoingMentions_All(t *testing.T) {
	got := renderOutgoingMentions("@all(all)")
	expected := `<at user_id="all">所有人</at>`
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestRenderOutgoingMentions_Empty(t *testing.T) {
	got := renderOutgoingMentions("")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// --- textContent ---

func TestTextContent_Basic(t *testing.T) {
	got := textContent("hello")
	if got != `{"text":"hello"}` {
		t.Fatalf("expected JSON, got %q", got)
	}
}

// --- imageContent ---

func TestImageContent_Basic(t *testing.T) {
	got := imageContent("img_abc123")
	if got != `{"image_key":"img_abc123"}` {
		t.Fatalf("expected JSON, got %q", got)
	}
}

// --- fileContent ---

func TestFileContent_Basic(t *testing.T) {
	got := fileContent("file_xyz789")
	if got != `{"file_key":"file_xyz789"}` {
		t.Fatalf("expected JSON, got %q", got)
	}
}

// --- extractSenderID ---

func TestExtractSenderID_NilEvent(t *testing.T) {
	if got := extractSenderID(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractSenderID_NilSender(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{},
	}
	if got := extractSenderID(event); got != "" {
		t.Fatalf("expected empty for nil sender, got %q", got)
	}
}

func TestExtractSenderID_OpenID(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{OpenId: strPtr("ou_alice")},
			},
		},
	}
	if got := extractSenderID(event); got != "ou_alice" {
		t.Fatalf("expected ou_alice, got %q", got)
	}
}

// --- extractMentions ---

func TestExtractMentions_Nil(t *testing.T) {
	if got := extractMentions(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestExtractMentions_ExtractsIDs(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				Mentions: []*larkim.MentionEvent{
					{Id: &larkim.UserId{OpenId: strPtr("ou_1")}},
					{Id: &larkim.UserId{UserId: strPtr("uid_2")}},
				},
			},
		},
	}
	got := extractMentions(event)
	if len(got) != 2 || got[0] != "ou_1" || got[1] != "uid_2" {
		t.Fatalf("expected [ou_1, uid_2], got %v", got)
	}
}

// --- isBotSender ---

func TestIsBotSender_Nil(t *testing.T) {
	if isBotSender(nil) {
		t.Fatal("expected false for nil")
	}
}

func TestIsBotSender_App(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{SenderType: strPtr("app")},
		},
	}
	if !isBotSender(event) {
		t.Fatal("expected true for app sender")
	}
}

func TestIsBotSender_User(t *testing.T) {
	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{SenderType: strPtr("user")},
		},
	}
	if isBotSender(event) {
		t.Fatal("expected false for user sender")
	}
}
