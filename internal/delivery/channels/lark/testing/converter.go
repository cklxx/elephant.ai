package larktesting

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// TraceToScenario converts a conversation trace into a YAML scenario.
// It extracts inbound messages as turns and outbound calls as assertions.
// User IDs and chat IDs are anonymized by default.
func TraceToScenario(name, description string, entries []TraceEntry, anonymize bool) *Scenario {
	scenario := &Scenario{
		Name:        name,
		Description: description,
		Tags:        []string{"auto-generated"},
		Setup: SetupConfig{
			Config: GatewayConfig{
				SessionPrefix: "test-lark",
				AllowDirect:   true,
				AllowGroups:   true,
			},
			LLMMode: "mock",
		},
	}

	anonMap := make(map[string]string)
	anonCounter := 0

	anonymizeID := func(id, prefix string) string {
		if !anonymize || id == "" {
			return id
		}
		if anon, ok := anonMap[id]; ok {
			return anon
		}
		anonCounter++
		anon := fmt.Sprintf("%s_%03d", prefix, anonCounter)
		anonMap[id] = anon
		return anon
	}

	// Group entries by inbound messages and their subsequent outbound responses.
	type turnData struct {
		inbound   TraceEntry
		outbound  []TraceEntry
	}

	var turns []turnData
	var current *turnData

	for _, entry := range entries {
		if entry.Direction == "inbound" {
			if current != nil {
				turns = append(turns, *current)
			}
			current = &turnData{inbound: entry}
		} else if current != nil {
			current.outbound = append(current.outbound, entry)
		}
	}
	if current != nil {
		turns = append(turns, *current)
	}

	for _, td := range turns {
		in := td.inbound
		text := extractTextFromContent(in.Content)

		turn := Turn{
			SenderID:  anonymizeID(in.SenderID, "ou_user"),
			ChatID:    anonymizeID(in.ChatID, "oc_chat"),
			ChatType:  in.ChatType,
			MessageID: anonymizeID(in.MessageID, "om_msg"),
			Content:   text,
		}

		// Build mock response from first outbound reply.
		for _, out := range td.outbound {
			if out.Method == "ReplyMessage" || out.Method == "SendMessage" {
				replyText := extractTextFromContent(out.Content)
				turn.MockResponse = &MockResponse{Answer: replyText}
				break
			}
		}

		// Build assertions from outbound calls.
		var assertions []MessengerAssertion
		methodCounts := make(map[string]int)

		for _, out := range td.outbound {
			if out.Direction != "outbound" {
				continue
			}
			methodCounts[out.Method]++

			switch out.Method {
			case "ReplyMessage", "SendMessage":
				replyText := extractTextFromContent(out.Content)
				keywords := extractKeywords(replyText, 3)
				if len(keywords) > 0 {
					assertions = append(assertions, MessengerAssertion{
						Method:          out.Method,
						ContentContains: keywords,
					})
				} else {
					assertions = append(assertions, MessengerAssertion{
						Method: out.Method,
					})
				}
			case "AddReaction":
				assertions = append(assertions, MessengerAssertion{
					Method:    out.Method,
					EmojiType: out.Emoji,
				})
			case "UploadImage", "UploadFile":
				assertions = append(assertions, MessengerAssertion{
					Method: out.Method,
				})
			}
		}

		// Deduplicate assertions by method — keep unique content checks.
		turn.Assertions.Messenger = deduplicateAssertions(assertions)
		scenario.Turns = append(scenario.Turns, turn)
	}

	return scenario
}

// ScenarioToYAML serializes a scenario to YAML bytes.
func ScenarioToYAML(s *Scenario) ([]byte, error) {
	return yaml.Marshal(s)
}

// AnonymizeID creates a deterministic anonymous ID from an input.
func AnonymizeID(input, prefix string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%s_%x", prefix, hash[:6])
}

// extractTextFromContent tries to parse Lark text JSON format {"text":"..."}.
// Falls back to raw content if parsing fails.
func extractTextFromContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err == nil && parsed.Text != "" {
		return parsed.Text
	}
	return content
}

// extractKeywords picks representative words from text for content_contains assertions.
func extractKeywords(text string, maxWords int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Split into words, filter short ones, pick first N significant ones.
	words := strings.Fields(text)
	var keywords []string
	seen := make(map[string]bool)

	for _, w := range words {
		// Skip very short words and common particles.
		if len(w) < 2 {
			continue
		}
		lower := strings.ToLower(w)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		keywords = append(keywords, w)
		if len(keywords) >= maxWords {
			break
		}
	}
	return keywords
}

// deduplicateAssertions merges assertions with the same method.
func deduplicateAssertions(assertions []MessengerAssertion) []MessengerAssertion {
	type key struct {
		method string
		emoji  string
	}
	seen := make(map[key]bool)
	var result []MessengerAssertion

	for _, a := range assertions {
		k := key{method: a.Method, emoji: a.EmojiType}
		if seen[k] {
			continue
		}
		seen[k] = true
		result = append(result, a)
	}
	return result
}
