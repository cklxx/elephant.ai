# Cost Tracking and Token Usage Analytics
> Last updated: 2025-11-18


ALEX includes comprehensive cost tracking and token usage analytics to help you monitor and manage LLM API costs.

## Overview

The cost tracking system:
- **Automatically tracks** every LLM API call with token counts and costs
- **Supports multiple models** with accurate pricing (GPT-4, GPT-4o, GPT-4o-mini, DeepSeek, etc.)
- **Aggregates costs** by session, day, month, or custom date ranges
- **Exports data** in CSV or JSON format for billing and analysis
- **Real-time monitoring** with zero configuration required

## Architecture

The cost tracking system follows ALEX's hexagonal architecture:

```
Domain Layer (Pure Logic)
├── ports/cost.go          - CostTracker, CostStore interfaces
└── ports/cost.go          - Model pricing and cost calculation

Application Layer
└── app/cost_tracker.go    - CostTracker implementation

Infrastructure Layer
└── storage/cost_store.go  - File-based cost storage

Presentation Layer
└── cmd/alex/cost.go       - CLI cost commands
```

## Usage

### CLI Commands

#### Show Total Cost
```bash
# Show cost across all sessions
alex cost show

# Output:
# Total Cost (All Time)
# ====================
# Total Cost:      $0.123456
# Requests:        42
# Input Tokens:    12000 (12.0K)
# Output Tokens:   6000 (6.0K)
# Total Tokens:    18000 (18.0K)
```

#### Show Session Cost
```bash
# Show cost for a specific session
alex cost session session-1727890123

# Output shows detailed breakdown by model and provider
```

#### Show Daily Cost
```bash
# Show cost for today
alex cost day

# Show cost for specific date
alex cost day 2025-10-01
```

#### Show Monthly Cost
```bash
# Show cost for current month
alex cost month

# Show cost for specific month
alex cost month 2025-10
```

#### Export Data

Export to CSV:
```bash
# Export all data to CSV
alex cost export --format csv --output costs.csv

# Export specific session
alex cost export --format csv --session session-1727890123 --output session-costs.csv

# Export date range
alex cost export --format csv --start 2025-10-01 --end 2025-10-31 --output october-costs.csv
```

Export to JSON:
```bash
# Export to JSON
alex cost export --format json --output costs.json

# Export filtered by model
alex cost export --format json --model gpt-4o --output gpt4o-costs.json
```

### Programmatic Usage

#### Recording Usage in Custom Code

```go
import (
    "alex/internal/agent/ports"
    "alex/internal/agent/app"
    "alex/internal/storage"
)

// Create cost store
costStore, err := storage.NewFileCostStore("~/.alex-costs")
if err != nil {
    log.Fatal(err)
}

// Create cost tracker
costTracker := app.NewCostTracker(costStore)

// Record usage
ctx := context.Background()
record := ports.UsageRecord{
    SessionID:    "my-session",
    Model:        "gpt-4o",
    Provider:     "openrouter",
    InputTokens:  1000,
    OutputTokens: 500,
}

err = costTracker.RecordUsage(ctx, record)
```

#### Querying Costs

```go
// Get session cost
summary, err := costTracker.GetSessionCost(ctx, "my-session")
fmt.Printf("Session cost: $%.6f\n", summary.TotalCost)

// Get daily cost
date := time.Now()
summary, err = costTracker.GetDailyCost(ctx, date)
fmt.Printf("Today's cost: $%.6f\n", summary.TotalCost)

// Get monthly cost
summary, err = costTracker.GetMonthlyCost(ctx, 2025, 10)
fmt.Printf("October cost: $%.6f\n", summary.TotalCost)
```

#### Exporting Data

```go
// Export to CSV
filter := ports.ExportFilter{
    StartDate: time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC),
    EndDate:   time.Date(2025, 10, 31, 23, 59, 59, 0, time.UTC),
}

data, err := costTracker.Export(ctx, ports.ExportFormatCSV, filter)
if err != nil {
    log.Fatal(err)
}

// Write to file
os.WriteFile("october-costs.csv", data, 0644)
```

## Model Pricing

The system includes up-to-date pricing for common models (as of 2025):

| Model | Input (per 1K tokens) | Output (per 1K tokens) |
|-------|----------------------|------------------------|
| gpt-4 | $0.03 | $0.06 |
| gpt-4-turbo | $0.01 | $0.03 |
| gpt-4o | $0.005 | $0.015 |
| gpt-4o-mini | $0.00015 | $0.0006 |
| deepseek-chat | $0.00014 | $0.00028 |
| deepseek-reasoner | $0.00055 | $0.00219 |
| claude-3-5-sonnet | $0.003 | $0.015 |

Unknown models default to: $0.001 (input) / $0.002 (output) per 1K tokens.

## Storage

Cost data is stored in `~/.alex-costs/` with the following structure:

```
~/.alex-costs/
├── 2025-10-01/
│   └── records.jsonl      # Daily records in JSON Lines format
├── 2025-10-02/
│   └── records.jsonl
└── _index/
    ├── session-123.json   # Session index (dates where session has records)
    └── session-456.json
```

### Data Format

Each record contains:
```json
{
  "id": "usage-1727890123456-session-123",
  "session_id": "session-123",
  "model": "gpt-4o",
  "provider": "openrouter",
  "input_tokens": 1000,
  "output_tokens": 500,
  "total_tokens": 1500,
  "input_cost": 0.005,
  "output_cost": 0.0075,
  "total_cost": 0.0125,
  "timestamp": "2025-10-01T10:30:00Z"
}
```

## Integration with LLM Clients

Cost tracking is automatically enabled for all LLM clients that implement `UsageTrackingClient`:

```go
// In openai_client.go
type openaiClient struct {
    // ...
    usageCallback func(usage ports.TokenUsage, model string, provider string)
}

// SetUsageCallback implements UsageTrackingClient
func (c *openaiClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
    c.usageCallback = callback
}

// After each API call
if c.usageCallback != nil {
    c.usageCallback(result.Usage, c.model, provider)
}
```

The Coordinator automatically sets up cost tracking:

```go
// In coordinator.go
if c.costTracker != nil {
    if trackingClient, ok := llmClient.(ports.UsageTrackingClient); ok {
        trackingClient.SetUsageCallback(func(usage ports.TokenUsage, model string, provider string) {
            // Record usage with calculated costs
            record := ports.UsageRecord{
                SessionID:    sessionID,
                Model:        model,
                Provider:     provider,
                InputTokens:  usage.PromptTokens,
                OutputTokens: usage.CompletionTokens,
                // ... costs calculated automatically
            }
            c.costTracker.RecordUsage(ctx, record)
        })
    }
}
```

## Testing

Run tests to verify cost tracking:

```bash
# Test cost interfaces and calculations
go test ./internal/agent/ports/ -v -run TestCost

# Test cost tracker
go test ./internal/agent/app/ -v -run TestCostTracker

# Test cost storage
go test ./internal/storage/ -v -run TestFileCostStore
```

## Best Practices

1. **Monitor regularly**: Check daily costs with `alex cost day` to catch unexpected usage
2. **Export monthly**: Use `alex cost export --format csv --month 2025-10` for billing records
3. **Track by session**: Use meaningful session IDs to track costs by project or feature
4. **Set budgets**: Monitor costs and set alerts when they exceed expected values
5. **Optimize models**: Use cheaper models (e.g., gpt-4o-mini) for simple tasks

## Cost Optimization Tips

1. **Use smaller models** for simple tasks:
   - gpt-4o-mini: ~75x cheaper than gpt-4
   - deepseek-chat: ~200x cheaper than gpt-4

2. **Reduce output tokens**:
   - Output tokens cost 2-4x more than input tokens
   - Use concise prompts and limit response length

3. **Cache responses** when appropriate:
   - Reuse results for repeated queries
   - Implement caching at application level

4. **Monitor token usage**:
   - Track ratio of input/output tokens
   - Optimize prompts to reduce unnecessary tokens

## Troubleshooting

### Cost tracking not working
- Ensure cost store was initialized in container
- Check logs for error messages
- Verify LLM client implements `UsageTrackingClient`

### Incorrect costs
- Verify model pricing is up-to-date
- Check that token counts match API response
- Ensure costs are calculated with correct pricing

### Missing records
- Check storage directory: `ls ~/.alex-costs/`
- Verify session index: `ls ~/.alex-costs/_index/`
- Look for error logs in application output

## API Reference

### CostTracker Interface

```go
type CostTracker interface {
    RecordUsage(ctx context.Context, usage UsageRecord) error
    GetSessionCost(ctx context.Context, sessionID string) (*CostSummary, error)
    GetDailyCost(ctx context.Context, date time.Time) (*CostSummary, error)
    GetMonthlyCost(ctx context.Context, year int, month int) (*CostSummary, error)
    GetDateRangeCost(ctx context.Context, start, end time.Time) (*CostSummary, error)
    Export(ctx context.Context, format ExportFormat, filter ExportFilter) ([]byte, error)
}
```

### CostStore Interface

```go
type CostStore interface {
    SaveUsage(ctx context.Context, record UsageRecord) error
    GetBySession(ctx context.Context, sessionID string) ([]UsageRecord, error)
    GetByDateRange(ctx context.Context, start, end time.Time) ([]UsageRecord, error)
    GetByModel(ctx context.Context, model string) ([]UsageRecord, error)
    ListAll(ctx context.Context) ([]UsageRecord, error)
}
```

## Future Enhancements

Planned improvements:
- [ ] Real-time cost alerts when budget exceeded
- [ ] Cost prediction based on historical usage
- [ ] Integration with billing systems (Stripe, etc.)
- [ ] Dashboard UI for visualizing costs
- [ ] Automatic model selection based on budget constraints
- [ ] Cost optimization recommendations
