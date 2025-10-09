package state

import (
	"strings"
	"time"
)

// MessageAppend appends a chat message to the transcript.
type MessageAppend struct {
	Message ChatMessage
}

func (u MessageAppend) apply(store *Store) {
	if stringsTrimmed(u.Message.Content) == "" {
		return
	}
	store.messages = append(store.messages, u.Message)
}

// Reset clears all state within the store, reinitialising backing collections.
type Reset struct{}

func (Reset) apply(store *Store) {
	store.messages = nil
	store.toolRuns = make(map[string]*ToolRun)
	store.subagentRuns = make(map[int]*SubagentTask)
	store.mcpServers = make(map[string]*MCPServer)
	store.metrics = Metrics{
		TokensByAgent: make(map[string]int),
		CostByModel:   make(map[string]float64),
	}
}

// ToolRunDelta mutates a tracked tool run entry.
type ToolRunDelta struct {
	CallID       string
	ToolName     string
	AgentID      string
	AgentLevel   string
	Arguments    map[string]interface{}
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Duration     *time.Duration
	Status       *ToolStatus
	Result       *string
	Error        *string
	AppendStream string
	Timestamp    time.Time
}

func (u ToolRunDelta) apply(store *Store) {
	if u.CallID == "" {
		return
	}
	run, exists := store.toolRuns[u.CallID]
	if !exists {
		run = &ToolRun{ID: u.CallID, Status: ToolStatusPending}
		store.toolRuns[u.CallID] = run
	}
	if u.ToolName != "" {
		run.ToolName = u.ToolName
	}
	if u.AgentID != "" {
		run.AgentID = u.AgentID
	}
	if u.AgentLevel != "" {
		run.AgentLevel = u.AgentLevel
	}
	if u.Arguments != nil {
		run.Arguments = copyInterfaceMap(u.Arguments)
	}
	if u.StartedAt != nil {
		run.StartedAt = copyTimePtr(u.StartedAt)
	}
	if u.CompletedAt != nil {
		run.CompletedAt = copyTimePtr(u.CompletedAt)
	}
	if u.Duration != nil {
		run.Duration = *u.Duration
	}
	if u.Status != nil {
		run.Status = *u.Status
	}
	if u.Result != nil {
		run.Result = *u.Result
	}
	if u.Error != nil {
		run.Error = *u.Error
	}
	if u.AppendStream != "" {
		run.Stream = append(run.Stream, u.AppendStream)
	}
	if !u.Timestamp.IsZero() {
		run.UpdatedAt = u.Timestamp
	}
}

// SubtaskDelta updates subagent progress data.
type SubtaskDelta struct {
	Index               int
	Total               int
	Preview             string
	Status              *SubtaskStatus
	CurrentTool         *string
	ToolsCompletedDelta int
	TokensUsed          *int
	Error               *string
	AgentLevel          string
	StartedAt           *time.Time
	CompletedAt         *time.Time
	Timestamp           time.Time
}

func (u SubtaskDelta) apply(store *Store) {
	if u.Index < 0 {
		return
	}
	task, exists := store.subagentRuns[u.Index]
	if !exists {
		task = &SubagentTask{Index: u.Index, Status: SubtaskStatusPending}
		store.subagentRuns[u.Index] = task
	}
	if u.Total > 0 {
		task.Total = u.Total
	}
	if u.Preview != "" {
		task.Preview = u.Preview
	}
	if u.AgentLevel != "" {
		task.AgentLevel = u.AgentLevel
	}
	if u.Status != nil {
		task.Status = *u.Status
	}
	if u.CurrentTool != nil {
		task.CurrentTool = *u.CurrentTool
	}
	if u.ToolsCompletedDelta != 0 {
		task.ToolsCompleted += u.ToolsCompletedDelta
		if task.ToolsCompleted < 0 {
			task.ToolsCompleted = 0
		}
		if task.Total > 0 && task.ToolsCompleted > task.Total {
			task.ToolsCompleted = task.Total
		}
	}
	if u.TokensUsed != nil {
		task.TokensUsed = *u.TokensUsed
	}
	if u.Error != nil {
		task.Error = *u.Error
	}
	if u.StartedAt != nil {
		task.StartedAt = copyTimePtr(u.StartedAt)
	} else if task.StartedAt == nil && !u.Timestamp.IsZero() {
		ts := u.Timestamp
		task.StartedAt = &ts
	}
	if u.CompletedAt != nil {
		task.CompletedAt = copyTimePtr(u.CompletedAt)
	}
	if !u.Timestamp.IsZero() {
		task.UpdatedAt = u.Timestamp
		if task.StartedAt == nil {
			ts := u.Timestamp
			task.StartedAt = &ts
		}
	}

	if u.Status != nil {
		switch *u.Status {
		case SubtaskStatusCompleted, SubtaskStatusFailed:
			if task.CompletedAt == nil {
				if u.CompletedAt != nil {
					task.CompletedAt = copyTimePtr(u.CompletedAt)
				} else if !u.Timestamp.IsZero() {
					ts := u.Timestamp
					task.CompletedAt = &ts
				}
			}
		case SubtaskStatusPending, SubtaskStatusRunning:
			task.CompletedAt = nil
			task.Duration = 0
		}
	}

	if task.StartedAt != nil && task.CompletedAt != nil {
		if !task.CompletedAt.Before(*task.StartedAt) {
			task.Duration = task.CompletedAt.Sub(*task.StartedAt)
		} else {
			task.Duration = 0
		}
	}
}

// MCPServerDelta updates MCP server status information.
type MCPServerDelta struct {
	Name      string
	Status    *MCPStatus
	LastError *string
	StartedAt *time.Time
	Timestamp time.Time
}

func (u MCPServerDelta) apply(store *Store) {
	if u.Name == "" {
		return
	}
	server, exists := store.mcpServers[u.Name]
	if !exists {
		server = &MCPServer{Name: u.Name, Status: MCPStatusUnknown}
		store.mcpServers[u.Name] = server
	}
	if u.Status != nil {
		server.Status = *u.Status
	}
	if u.LastError != nil {
		server.LastError = *u.LastError
	}
	if u.StartedAt != nil {
		server.StartedAt = copyTimePtr(u.StartedAt)
	}
	if !u.Timestamp.IsZero() {
		server.UpdatedAt = u.Timestamp
	}
}

// MetricsDelta updates aggregate metrics such as token usage.
type MetricsDelta struct {
	AgentLevel string
	Tokens     int
	Timestamp  time.Time
}

func (u MetricsDelta) apply(store *Store) {
	agent := stringsTrimmed(u.AgentLevel)
	if agent == "" {
		return
	}

	if store.metrics.TokensByAgent == nil {
		store.metrics.TokensByAgent = make(map[string]int)
	}
	if store.metrics.CostByModel == nil {
		store.metrics.CostByModel = make(map[string]float64)
	}

	if u.Tokens <= 0 {
		if !u.Timestamp.IsZero() {
			store.metrics.UpdatedAt = u.Timestamp
		}
		return
	}

	store.metrics.TokensByAgent[agent] += u.Tokens

	total := 0
	for _, value := range store.metrics.TokensByAgent {
		if value > 0 {
			total += value
		}
	}
	store.metrics.TotalTokens = total
	if !u.Timestamp.IsZero() {
		store.metrics.UpdatedAt = u.Timestamp
	}
}

// MetricsCostSummary replaces the stored cost metrics with the provided totals.
type MetricsCostSummary struct {
	TotalCost   float64
	CostByModel map[string]float64
	Timestamp   time.Time
}

func (u MetricsCostSummary) apply(store *Store) {
	if store.metrics.CostByModel == nil {
		store.metrics.CostByModel = make(map[string]float64)
	}

	// Clear existing entries before applying the new summary.
	for key := range store.metrics.CostByModel {
		delete(store.metrics.CostByModel, key)
	}

	store.metrics.TotalCost = 0

	if u.CostByModel != nil {
		for model, cost := range u.CostByModel {
			trimmed := stringsTrimmed(model)
			if trimmed == "" || cost <= 0 {
				continue
			}
			store.metrics.CostByModel[trimmed] = cost
			store.metrics.TotalCost += cost
		}
	}

	if u.TotalCost > 0 {
		store.metrics.TotalCost = u.TotalCost
	}

	if !u.Timestamp.IsZero() {
		store.metrics.UpdatedAt = u.Timestamp
	}
}

func copyTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	t := *src
	return &t
}

func stringsTrimmed(s string) string {
	return strings.TrimSpace(s)
}
