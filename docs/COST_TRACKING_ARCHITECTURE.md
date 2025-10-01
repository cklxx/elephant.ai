# Cost Tracking Architecture Diagram

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         ALEX COST TRACKING                       │
│                    Hexagonal Architecture                         │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      Presentation Layer                          │
├─────────────────────────────────────────────────────────────────┤
│  CLI Commands                                                    │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  alex cost show              → Total cost                │  │
│  │  alex cost session <ID>      → Session cost              │  │
│  │  alex cost day <DATE>        → Daily cost                │  │
│  │  alex cost month <YYYY-MM>   → Monthly cost              │  │
│  │  alex cost export [OPTIONS]  → CSV/JSON export           │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                   │
│  File: cmd/alex/cost.go                                          │
└─────────────────────────────┬─────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Application Layer                            │
├─────────────────────────────────────────────────────────────────┤
│  CostTracker                                                     │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  • RecordUsage()          → Save usage records           │  │
│  │  • GetSessionCost()       → Query by session             │  │
│  │  • GetDailyCost()         → Query by day                 │  │
│  │  • GetMonthlyCost()       → Query by month               │  │
│  │  • Export()               → Export CSV/JSON              │  │
│  │                                                           │  │
│  │  + Aggregation Logic                                      │  │
│  │  + Export Formatting                                      │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                   │
│  File: internal/agent/app/cost_tracker.go                        │
└─────────────────────────────┬─────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Domain Layer                              │
├─────────────────────────────────────────────────────────────────┤
│  Interfaces & Models                                             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  CostTracker Interface                                    │  │
│  │  CostStore Interface                                      │  │
│  │  UsageTrackingClient Interface                           │  │
│  │                                                           │  │
│  │  UsageRecord struct                                       │  │
│  │  CostSummary struct                                       │  │
│  │  ExportFormat, ExportFilter                              │  │
│  │                                                           │  │
│  │  GetModelPricing()     → Pricing for 10+ models         │  │
│  │  CalculateCost()       → Input/Output cost calc         │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                   │
│  File: internal/agent/ports/cost.go                              │
└─────────────────────────────┬─────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                          │
├─────────────────────────────────────────────────────────────────┤
│  FileCostStore                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Storage Structure:                                       │  │
│  │  ~/.alex-costs/                                          │  │
│  │  ├── 2025-10-01/                                         │  │
│  │  │   └── records.jsonl    (daily records)               │  │
│  │  ├── 2025-10-02/                                         │  │
│  │  │   └── records.jsonl                                   │  │
│  │  └── _index/                                             │  │
│  │      ├── session-123.json  (session index)              │  │
│  │      └── session-456.json                                │  │
│  │                                                           │  │
│  │  Operations:                                              │  │
│  │  • SaveUsage()       → Append to JSONL                   │  │
│  │  • GetBySession()    → Use index for fast lookup        │  │
│  │  • GetByDateRange()  → Scan date directories            │  │
│  │  • GetByModel()      → Filter records                    │  │
│  │  • ListAll()         → All records sorted                │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                   │
│  File: internal/storage/cost_store.go                            │
└─────────────────────────────────────────────────────────────────┘
```

## Integration Flow

```
┌─────────────────┐
│   User Task     │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│         Coordinator                      │
│  ┌────────────────────────────────────┐ │
│  │  1. Get LLM Client                 │ │
│  │  2. Setup Cost Tracking Callback   │ │
│  │  3. Execute Task (ReAct loop)      │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│         LLM Client (OpenAI)             │
│  ┌────────────────────────────────────┐ │
│  │  Complete() → API Call             │ │
│  │     ↓                              │ │
│  │  Parse Response + Token Usage      │ │
│  │     ↓                              │ │
│  │  Invoke usageCallback()            │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│      Usage Callback (in Coordinator)    │
│  ┌────────────────────────────────────┐ │
│  │  1. Extract token counts           │ │
│  │  2. Calculate costs (via ports)    │ │
│  │  3. Create UsageRecord             │ │
│  │  4. costTracker.RecordUsage()      │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│          CostTracker                     │
│  ┌────────────────────────────────────┐ │
│  │  1. Auto-generate ID & timestamp   │ │
│  │  2. Calculate total tokens         │ │
│  │  3. Calculate costs (if missing)   │ │
│  │  4. costStore.SaveUsage()          │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│          FileCostStore                   │
│  ┌────────────────────────────────────┐ │
│  │  1. Organize by date (YYYY-MM-DD)  │ │
│  │  2. Append to records.jsonl        │ │
│  │  3. Update session index           │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Data Flow - Query Operations

```
┌─────────────────┐
│   CLI Command   │
│  alex cost ...  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│     CLI Handler (cost.go)               │
│  ┌────────────────────────────────────┐ │
│  │  Parse arguments                   │ │
│  │  Call costTracker method           │ │
│  │  Format output                      │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│          CostTracker                     │
│  ┌────────────────────────────────────┐ │
│  │  GetSessionCost()                  │ │
│  │      → costStore.GetBySession()    │ │
│  │      → aggregateRecords()          │ │
│  │                                     │ │
│  │  GetDailyCost()                    │ │
│  │      → costStore.GetByDateRange()  │ │
│  │      → aggregateRecords()          │ │
│  │                                     │ │
│  │  Export()                          │ │
│  │      → getFilteredRecords()        │ │
│  │      → exportJSON() / exportCSV()  │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│          FileCostStore                   │
│  ┌────────────────────────────────────┐ │
│  │  GetBySession():                   │ │
│  │    1. Read session index           │ │
│  │    2. Load relevant date files     │ │
│  │    3. Filter by session ID         │ │
│  │                                     │ │
│  │  GetByDateRange():                 │ │
│  │    1. Iterate date directories     │ │
│  │    2. Read records.jsonl           │ │
│  │    3. Filter by timestamp          │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│           CostSummary                    │
│  ┌────────────────────────────────────┐ │
│  │  TotalCost                         │ │
│  │  InputTokens, OutputTokens         │ │
│  │  RequestCount                       │ │
│  │  ByModel: map[string]float64       │ │
│  │  ByProvider: map[string]float64    │ │
│  │  StartTime, EndTime                │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

## Model Pricing Lookup

```
┌─────────────────────────────────────────┐
│      GetModelPricing(model string)      │
├─────────────────────────────────────────┤
│                                          │
│  Pricing Map:                            │
│  ┌────────────────────────────────────┐ │
│  │  "gpt-4"           → 0.03 / 0.06   │ │
│  │  "gpt-4-turbo"     → 0.01 / 0.03   │ │
│  │  "gpt-4o"          → 0.005 / 0.015 │ │
│  │  "gpt-4o-mini"     → 0.00015 / ... │ │
│  │  "deepseek-chat"   → 0.00014 / ... │ │
│  │  "deepseek-reasoner" → 0.00055 /...│ │
│  │  ...                                │ │
│  └────────────────────────────────────┘ │
│                                          │
│  Unknown models:                         │
│  → Default: 0.001 / 0.002                │
│                                          │
│  Returns: ModelPricing{                  │
│    InputPer1K:  float64                  │
│    OutputPer1K: float64                  │
│  }                                        │
└─────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│  CalculateCost(in, out int, model str)  │
├─────────────────────────────────────────┤
│                                          │
│  pricing := GetModelPricing(model)       │
│                                          │
│  inputCost  = (in / 1000) * pricing.In   │
│  outputCost = (out / 1000) * pricing.Out │
│  totalCost  = inputCost + outputCost     │
│                                          │
│  return inputCost, outputCost, totalCost │
└─────────────────────────────────────────┘
```

## Export Flow

```
┌─────────────────────────────────────────┐
│     alex cost export [OPTIONS]          │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│         Parse Export Options             │
│  ┌────────────────────────────────────┐ │
│  │  --format csv|json                 │ │
│  │  --session <ID>                    │ │
│  │  --model <MODEL>                   │ │
│  │  --provider <PROVIDER>             │ │
│  │  --start <DATE>                    │ │
│  │  --end <DATE>                      │ │
│  │  --output <FILE>                   │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│       costTracker.Export()              │
│  ┌────────────────────────────────────┐ │
│  │  1. Get filtered records           │ │
│  │     (session/model/date range)     │ │
│  │                                     │ │
│  │  2. Format based on type:          │ │
│  │     CSV:  exportCSV()              │ │
│  │     JSON: exportJSON()             │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│           Export Output                  │
│  ┌────────────────────────────────────┐ │
│  │  CSV Format:                       │ │
│  │  ID,Timestamp,SessionID,Provider,  │ │
│  │  Model,InputTokens,OutputTokens,   │ │
│  │  TotalTokens,InputCost,OutputCost, │ │
│  │  TotalCost                          │ │
│  │  ...data rows...                    │ │
│  │                                     │ │
│  │  JSON Format:                       │ │
│  │  [                                  │ │
│  │    {                                │ │
│  │      "id": "...",                   │ │
│  │      "timestamp": "...",            │ │
│  │      "session_id": "...",           │ │
│  │      ...                             │ │
│  │    },                               │ │
│  │    ...                              │ │
│  │  ]                                  │ │
│  └────────────────────────────────────┘ │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│       Write to File or Stdout           │
└─────────────────────────────────────────┘
```

## Component Dependencies

```
┌─────────────────────────────────────────┐
│              Coordinator                 │
│  (Application Orchestration)             │
└─────────────┬───────────────────────────┘
              │ depends on
              ▼
┌─────────────────────────────────────────┐
│             CostTracker                  │
│  (Application Service)                   │
└─────────────┬───────────────────────────┘
              │ implements
              ▼
┌─────────────────────────────────────────┐
│         CostTracker Interface            │
│  (Domain Port)                           │
└─────────────────────────────────────────┘
              ▲ depends on
              │
┌─────────────┴───────────────────────────┐
│          FileCostStore                   │
│  (Infrastructure Adapter)                │
└─────────────┬───────────────────────────┘
              │ implements
              ▼
┌─────────────────────────────────────────┐
│          CostStore Interface             │
│  (Domain Port)                           │
└─────────────────────────────────────────┘
```

## Key Design Patterns

### 1. Dependency Injection
```go
// Container builds and wires components
costStore := storage.NewFileCostStore("~/.alex-costs")
costTracker := app.NewCostTracker(costStore)

coordinator := app.NewAgentCoordinator(
    ...,
    costTracker,  // Injected
    config,
)
```

### 2. Callback Pattern
```go
// LLM client invokes callback after each API call
if c.usageCallback != nil {
    c.usageCallback(result.Usage, c.model, provider)
}

// Coordinator sets up callback
trackingClient.SetUsageCallback(func(usage, model, provider) {
    // Record usage in cost tracker
    costTracker.RecordUsage(ctx, record)
})
```

### 3. Repository Pattern
```go
// CostStore provides data access abstraction
type CostStore interface {
    SaveUsage(ctx, record) error
    GetBySession(ctx, sessionID) ([]UsageRecord, error)
    GetByDateRange(ctx, start, end) ([]UsageRecord, error)
    ...
}
```

### 4. Strategy Pattern
```go
// Different export strategies
switch format {
case ExportFormatJSON:
    return t.exportJSON(records)
case ExportFormatCSV:
    return t.exportCSV(records)
}
```

## Storage Optimization

### Indexing Strategy
```
Session Index (~/.alex-costs/_index/session-123.json):
["2025-10-01", "2025-10-02", "2025-10-05"]

Benefits:
✅ O(1) session lookup
✅ Minimal storage overhead
✅ Fast multi-day queries
```

### Date Partitioning
```
~/.alex-costs/
├── 2025-10-01/records.jsonl  ← Day 1
├── 2025-10-02/records.jsonl  ← Day 2
└── 2025-10-03/records.jsonl  ← Day 3

Benefits:
✅ Efficient date range queries
✅ Natural data organization
✅ Easy archival/cleanup
```

### JSONL Format
```json
{"id":"1","session_id":"s1","model":"gpt-4o",...}
{"id":"2","session_id":"s1","model":"gpt-4o",...}

Benefits:
✅ Append-only writes (fast)
✅ Line-by-line parsing (memory efficient)
✅ Human-readable
```

## Thread Safety

```
┌─────────────────────────────────────────┐
│          FileCostStore                   │
│                                          │
│  var mu sync.RWMutex                     │
│                                          │
│  SaveUsage(...)                          │
│    mu.Lock()           ← Exclusive lock │
│    defer mu.Unlock()                     │
│    // write operations                   │
│                                          │
│  GetBySession(...)                       │
│    mu.RLock()          ← Shared lock    │
│    defer mu.RUnlock()                    │
│    // read operations                    │
└─────────────────────────────────────────┘
```

## Error Handling Strategy

```
Coordinator → CostTracker → CostStore
     │              │            │
     │              │            └─→ Error: log & return
     │              │
     │              └─→ Error: wrap with context & return
     │
     └─→ Error: log warning, continue execution
         (cost tracking failures are non-fatal)
```

This ensures that cost tracking never blocks or fails the main task execution.
