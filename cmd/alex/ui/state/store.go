package state

import (
	"sort"
	"sync"
	"time"
)

// ChatRole represents the speaker role for a chat message.
type ChatRole string

const (
	RoleSystem    ChatRole = "system"
	RoleUser      ChatRole = "user"
	RoleAssistant ChatRole = "assistant"
	RoleTool      ChatRole = "tool"
)

// ToolStatus captures the lifecycle state of a tool run displayed in the UI.
type ToolStatus string

const (
	ToolStatusPending   ToolStatus = "pending"
	ToolStatusRunning   ToolStatus = "running"
	ToolStatusCompleted ToolStatus = "completed"
	ToolStatusFailed    ToolStatus = "failed"
)

// SubtaskStatus enumerates states used by the subagent dashboard.
type SubtaskStatus string

const (
	SubtaskStatusPending   SubtaskStatus = "pending"
	SubtaskStatusRunning   SubtaskStatus = "running"
	SubtaskStatusCompleted SubtaskStatus = "completed"
	SubtaskStatusFailed    SubtaskStatus = "failed"
)

// MCPStatus indicates the lifecycle of an MCP server entry in the UI.
type MCPStatus string

const (
	MCPStatusUnknown  MCPStatus = "unknown"
	MCPStatusStarting MCPStatus = "starting"
	MCPStatusReady    MCPStatus = "ready"
	MCPStatusError    MCPStatus = "error"
	MCPStatusStopped  MCPStatus = "stopped"
)

// ChatMessage is a single entry in the scrollback transcript.
type ChatMessage struct {
	Role      ChatRole
	AgentID   string
	Content   string
	CreatedAt time.Time
}

// ToolRun tracks state for a single tool invocation.
type ToolRun struct {
	ID          string
	AgentID     string
	AgentLevel  string
	ToolName    string
	Arguments   map[string]interface{}
	Status      ToolStatus
	StartedAt   *time.Time
	CompletedAt *time.Time
	Duration    time.Duration
	Result      string
	Error       string
	Stream      []string
	UpdatedAt   time.Time
}

// SubagentTask tracks progress for an individual subagent task.
type SubagentTask struct {
	Index          int
	Total          int
	Preview        string
	Status         SubtaskStatus
	CurrentTool    string
	ToolsCompleted int
	TokensUsed     int
	Error          string
	AgentLevel     string
	StartedAt      *time.Time
	CompletedAt    *time.Time
	Duration       time.Duration
	UpdatedAt      time.Time
}

// MCPServer represents the status of a configured MCP server.
type MCPServer struct {
	Name      string
	Status    MCPStatus
	LastError string
	StartedAt *time.Time
	UpdatedAt time.Time
}

// Metrics captures aggregate session metrics such as token usage.
type Metrics struct {
	TokensByAgent map[string]int
	TotalTokens   int
	CostByModel   map[string]float64
	TotalCost     float64
	UpdatedAt     time.Time
}

// Store maintains the mutable UI state backing the TUI.
type Store struct {
	mu           sync.RWMutex
	messages     []ChatMessage
	toolRuns     map[string]*ToolRun
	subagentRuns map[int]*SubagentTask
	mcpServers   map[string]*MCPServer
	metrics      Metrics
}

// NewStore creates an empty Store instance.
func NewStore() *Store {
	return &Store{
		toolRuns:     make(map[string]*ToolRun),
		subagentRuns: make(map[int]*SubagentTask),
		mcpServers:   make(map[string]*MCPServer),
		metrics: Metrics{
			TokensByAgent: make(map[string]int),
			CostByModel:   make(map[string]float64),
		},
	}
}

// Update represents a mutation applied to the Store.
type Update interface {
	apply(store *Store)
}

// Apply mutates the store using the provided update.
func (s *Store) Apply(update Update) {
	if update == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	update.apply(s)
}

// Snapshot returns a copy of the current store state for safe observation.
type Snapshot struct {
	Messages     []ChatMessage
	ToolRuns     []*ToolRun
	SubagentRuns []*SubagentTask
	MCPServers   []*MCPServer
	Metrics      Metrics
}

// Snapshot copies the current state in a deterministic order to avoid exposing
// internal mutable references.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := Snapshot{
		Messages:     make([]ChatMessage, len(s.messages)),
		ToolRuns:     make([]*ToolRun, 0, len(s.toolRuns)),
		SubagentRuns: make([]*SubagentTask, 0, len(s.subagentRuns)),
		MCPServers:   make([]*MCPServer, 0, len(s.mcpServers)),
		Metrics: Metrics{
			TokensByAgent: make(map[string]int, len(s.metrics.TokensByAgent)),
			CostByModel:   make(map[string]float64, len(s.metrics.CostByModel)),
			TotalTokens:   s.metrics.TotalTokens,
			TotalCost:     s.metrics.TotalCost,
			UpdatedAt:     s.metrics.UpdatedAt,
		},
	}

	copy(snapshot.Messages, s.messages)

	for _, run := range s.toolRuns {
		snapshot.ToolRuns = append(snapshot.ToolRuns, cloneToolRun(run))
	}
	sort.Slice(snapshot.ToolRuns, func(i, j int) bool {
		return snapshot.ToolRuns[i].ID < snapshot.ToolRuns[j].ID
	})

	for _, task := range s.subagentRuns {
		snapshot.SubagentRuns = append(snapshot.SubagentRuns, cloneSubagent(task))
	}
	sort.Slice(snapshot.SubagentRuns, func(i, j int) bool {
		return snapshot.SubagentRuns[i].Index < snapshot.SubagentRuns[j].Index
	})

	for _, server := range s.mcpServers {
		snapshot.MCPServers = append(snapshot.MCPServers, cloneMCPServer(server))
	}
	sort.Slice(snapshot.MCPServers, func(i, j int) bool {
		return snapshot.MCPServers[i].Name < snapshot.MCPServers[j].Name
	})

	for agent, tokens := range s.metrics.TokensByAgent {
		if tokens <= 0 {
			continue
		}
		snapshot.Metrics.TokensByAgent[agent] = tokens
	}

	for model, cost := range s.metrics.CostByModel {
		if cost <= 0 {
			continue
		}
		snapshot.Metrics.CostByModel[model] = cost
	}

	return snapshot
}

func cloneToolRun(run *ToolRun) *ToolRun {
	if run == nil {
		return nil
	}
	clone := *run
	if run.Arguments != nil {
		clone.Arguments = copyInterfaceMap(run.Arguments)
	}
	if run.Stream != nil {
		clone.Stream = append([]string(nil), run.Stream...)
	}
	return &clone
}

func cloneSubagent(task *SubagentTask) *SubagentTask {
	if task == nil {
		return nil
	}
	clone := *task
	if task.StartedAt != nil {
		clone.StartedAt = copyTimePtr(task.StartedAt)
	}
	if task.CompletedAt != nil {
		clone.CompletedAt = copyTimePtr(task.CompletedAt)
	}
	return &clone
}

func cloneMCPServer(server *MCPServer) *MCPServer {
	if server == nil {
		return nil
	}
	clone := *server
	return &clone
}

func copyInterfaceMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
