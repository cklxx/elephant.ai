// Package larktesting provides a YAML-driven scenario test framework for the
// Lark gateway. Scenarios declare user messages, mock LLM responses, and
// assertions that are evaluated against a RecordingMessenger.
package larktesting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scenario is the top-level structure of a YAML scenario file.
type Scenario struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Tags        []string    `yaml:"tags"`
	Setup       SetupConfig `yaml:"setup"`
	Turns       []Turn      `yaml:"turns"`
}

// SetupConfig declares gateway and LLM configuration for the scenario.
type SetupConfig struct {
	Config  GatewayConfig `yaml:"config"`
	LLMMode string        `yaml:"llm_mode"` // "mock" (default) or "real"
}

// GatewayConfig is the subset of lark.Config that scenarios can override.
type GatewayConfig struct {
	SessionPrefix     string `yaml:"session_prefix"`
	AllowDirect       bool   `yaml:"allow_direct"`
	AllowGroups       bool   `yaml:"allow_groups"`
	ShowToolProgress  bool   `yaml:"show_tool_progress"`
	ShowPlanClarify   bool   `yaml:"show_plan_clarify_messages"`
	ReactEmoji        string `yaml:"react_emoji"`
	PlanReviewEnabled bool   `yaml:"plan_review_enabled"`
	AutoChatContext   bool   `yaml:"auto_chat_context"`
	MemoryEnabled     bool   `yaml:"memory_enabled"`
}

// Turn represents a single user message and its expected outcomes.
type Turn struct {
	SenderID  string `yaml:"sender_id"`
	ChatID    string `yaml:"chat_id"`
	ChatType  string `yaml:"chat_type"` // "p2p" (default) or "group"
	MessageID string `yaml:"message_id"`
	Content   string `yaml:"content"`
	DelayMS   int    `yaml:"delay_ms"`

	// MockResponse defines the agent's canned response for this turn (mock mode).
	MockResponse *MockResponse `yaml:"mock_response"`

	// Assertions to evaluate after this turn completes.
	Assertions TurnAssertions `yaml:"assertions"`
}

// MockResponse configures what the mock executor returns for a turn.
type MockResponse struct {
	Answer     string `yaml:"answer"`
	Error      string `yaml:"error"`
	StopReason string `yaml:"stop_reason"`
	// SystemMessages are injected as system-role messages in the result.
	SystemMessages []string `yaml:"system_messages"`
}

// TurnAssertions declares all checks for a single turn.
type TurnAssertions struct {
	Messenger []MessengerAssertion `yaml:"messenger"`
	NoCall    []string             `yaml:"no_call"` // methods that must NOT be called
	Executor  *ExecutorAssertion   `yaml:"executor"`
	Timing    *TimingAssertion     `yaml:"timing"`
}

// MessengerAssertion checks a specific outbound messenger call.
type MessengerAssertion struct {
	Method          string   `yaml:"method"`           // "ReplyMessage", "SendMessage", "UpdateMessage", "AddReaction", etc.
	ContentContains []string `yaml:"content_contains"` // substrings that must appear in content
	ContentAbsent   []string `yaml:"content_absent"`   // substrings that must NOT appear in content
	EmojiType       string   `yaml:"emoji_type"`       // for AddReaction assertions
	MinCount        int      `yaml:"min_count"`        // minimum number of matching calls (default 1)
	MaxCount        int      `yaml:"max_count"`        // maximum number of matching calls (0 = unlimited)
}

// ExecutorAssertion checks properties of the task passed to ExecuteTask.
type ExecutorAssertion struct {
	TaskContains []string `yaml:"task_contains"`
	TaskAbsent   []string `yaml:"task_absent"`
	Called       *bool    `yaml:"called"` // explicitly assert whether ExecuteTask was called
}

// TimingAssertion checks response timing.
type TimingAssertion struct {
	MaxMS int `yaml:"max_ms"`
}

// LoadScenario reads and parses a single YAML scenario file.
func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenario %s: %w", path, err)
	}
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse scenario %s: %w", path, err)
	}
	if s.Name == "" {
		s.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return &s, nil
}

// LoadScenariosFromDir loads all .yaml files from a directory.
func LoadScenariosFromDir(dir string) ([]*Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read scenario dir %s: %w", dir, err)
	}
	var scenarios []*Scenario
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		s, err := LoadScenario(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, s)
	}
	return scenarios, nil
}
