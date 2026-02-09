package claudecode

import "encoding/json"

// SDKEventType identifies the kind of JSONL event emitted by the Python bridge.
type SDKEventType string

const (
	SDKEventTool   SDKEventType = "tool"
	SDKEventResult SDKEventType = "result"
	SDKEventError  SDKEventType = "error"
)

// SDKEvent is a single JSONL line produced by scripts/cc_bridge/cc_bridge.py.
type SDKEvent struct {
	Type     SDKEventType `json:"type"`
	ToolName string       `json:"tool_name,omitempty"`
	Summary  string       `json:"summary,omitempty"`
	Files    []string     `json:"files,omitempty"`
	Iter     int          `json:"iter,omitempty"`
	Answer   string       `json:"answer,omitempty"`
	Tokens   int          `json:"tokens,omitempty"`
	Cost     float64      `json:"cost,omitempty"`
	Iters    int          `json:"iters,omitempty"`
	IsError  bool         `json:"is_error,omitempty"`
	Message  string       `json:"message,omitempty"`
}

// ParseSDKEvent parses a JSONL line into an SDKEvent.
func ParseSDKEvent(line []byte) (SDKEvent, error) {
	var ev SDKEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return SDKEvent{}, err
	}
	return ev, nil
}
