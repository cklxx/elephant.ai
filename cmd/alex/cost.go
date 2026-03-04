package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	agentstorage "alex/internal/domain/agent/ports/storage"
)

// handleCostCommand handles all cost-related subcommands
func (c *CLI) handleCostCommand(args []string) error {
	if c.container.Container.CostTracker == nil {
		return fmt.Errorf("cost tracking is not enabled")
	}

	ctx := cliBaseContext()

	// Parse subcommand
	if len(args) == 0 {
		// No args - show usage
		return c.showCostUsage()
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "show", "summary":
		return c.handleCostShow(ctx, cmdArgs)
	case "session":
		return c.handleCostSession(ctx, cmdArgs)
	case "day", "daily":
		return c.handleCostDaily(ctx, cmdArgs)
	case "month", "monthly":
		return c.handleCostMonthly(ctx, cmdArgs)
	case "export":
		return c.handleCostExport(ctx, cmdArgs)
	default:
		// Treat as session ID if it starts with "session-"
		if strings.HasPrefix(cmd, "session-") {
			return c.handleCostSession(ctx, []string{cmd})
		}
		return c.showCostUsage()
	}
}

func (c *CLI) showCostUsage() error {
	fmt.Print(`
elephant.ai Cost Tracking Commands

Usage:
  alex cost                              Show this help
  alex cost show                         Show total cost across all sessions
  alex cost session <SESSION_ID>         Show cost for a specific session
  alex cost day <YYYY-MM-DD>            Show cost for a specific day
  alex cost month <YYYY-MM>              Show cost for a specific month
  alex cost export [OPTIONS]             Export cost data

Export Options:
  --format <csv|json>                    Export format (default: csv)
  --session <SESSION_ID>                 Filter by session
  --model <MODEL>                        Filter by model
  --start <YYYY-MM-DD>                   Start date
  --end <YYYY-MM-DD>                     End date
  --output <FILE>                        Output file (default: stdout)

Examples:
  alex cost session session-1727890123
  alex cost day 2025-10-01
  alex cost month 2025-10
  alex cost export --format csv --output costs.csv
  alex cost export --format json --session session-1727890123
`)
	return nil
}

func (c *CLI) handleCostShow(ctx context.Context, _ []string) error {
	// Show total cost across all time
	start := time.Unix(0, 0)
	end := time.Now()

	summary, err := c.container.Container.CostTracker.GetDateRangeCost(ctx, start, end)
	if err != nil {
		return fmt.Errorf("get cost summary: %w", err)
	}

	c.printCostSummary(summary, "Total Cost (All Time)")
	return nil
}

func (c *CLI) handleCostSession(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session ID required")
	}

	sessionID := args[0]
	summary, err := c.container.Container.CostTracker.GetSessionCost(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session cost: %w", err)
	}

	c.printCostSummary(summary, fmt.Sprintf("Session: %s", sessionID))
	return nil
}

func (c *CLI) handleCostDaily(ctx context.Context, args []string) error {
	var date time.Time
	var err error

	if len(args) == 0 {
		// Default to today
		date = time.Now()
	} else {
		date, err = time.Parse("2006-01-02", args[0])
		if err != nil {
			return fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
		}
	}

	summary, err := c.container.Container.CostTracker.GetDailyCost(ctx, date)
	if err != nil {
		return fmt.Errorf("get daily cost: %w", err)
	}

	c.printCostSummary(summary, fmt.Sprintf("Daily Cost: %s", date.Format("2006-01-02")))
	return nil
}

func (c *CLI) handleCostMonthly(ctx context.Context, args []string) error {
	var year, month int
	var err error

	if len(args) == 0 {
		// Default to current month
		now := time.Now()
		year = now.Year()
		month = int(now.Month())
	} else {
		var t time.Time
		t, err = time.Parse("2006-01", args[0])
		if err != nil {
			return fmt.Errorf("invalid month format, expected YYYY-MM: %w", err)
		}
		year = t.Year()
		month = int(t.Month())
	}

	summary, err := c.container.Container.CostTracker.GetMonthlyCost(ctx, year, month)
	if err != nil {
		return fmt.Errorf("get monthly cost: %w", err)
	}

	c.printCostSummary(summary, fmt.Sprintf("Monthly Cost: %04d-%02d", year, month))
	return nil
}

func (c *CLI) handleCostExport(ctx context.Context, args []string) error {
	// Parse export options
	format := agentstorage.ExportFormatCSV
	filter := agentstorage.ExportFilter{}
	outputFile := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			value, err := requireOptionValue(args, &i, "--format")
			if err != nil {
				return err
			}
			switch value {
			case "csv":
				format = agentstorage.ExportFormatCSV
			case "json":
				format = agentstorage.ExportFormatJSON
			default:
				return fmt.Errorf("invalid format: %s (must be csv or json)", value)
			}

		case "--session":
			value, err := requireOptionValue(args, &i, "--session")
			if err != nil {
				return err
			}
			filter.SessionID = value

		case "--model":
			value, err := requireOptionValue(args, &i, "--model")
			if err != nil {
				return err
			}
			filter.Model = value

		case "--provider":
			value, err := requireOptionValue(args, &i, "--provider")
			if err != nil {
				return err
			}
			filter.Provider = value

		case "--start":
			value, err := requireOptionValue(args, &i, "--start")
			if err != nil {
				return err
			}
			t, err := time.Parse("2006-01-02", value)
			if err != nil {
				return fmt.Errorf("invalid start date: %w", err)
			}
			filter.StartDate = t

		case "--end":
			value, err := requireOptionValue(args, &i, "--end")
			if err != nil {
				return err
			}
			t, err := time.Parse("2006-01-02", value)
			if err != nil {
				return fmt.Errorf("invalid end date: %w", err)
			}
			filter.EndDate = t

		case "--output", "-o":
			value, err := requireOptionValue(args, &i, "--output")
			if err != nil {
				return err
			}
			outputFile = value

		default:
			return fmt.Errorf("unknown option: %s", args[i])
		}
	}

	// Export data
	data, err := c.container.Container.CostTracker.Export(ctx, format, filter)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Write to file or stdout
	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Printf("Cost data exported to: %s\n", outputFile)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func requireOptionValue(args []string, index *int, flag string) (string, error) {
	if *index+1 >= len(args) {
		return "", fmt.Errorf("%s requires a value", flag)
	}
	*index += 1
	return args[*index], nil
}

func (c *CLI) printCostSummary(summary *agentstorage.CostSummary, title string) {
	fmt.Printf("\n%s\n", title)
	fmt.Println(strings.Repeat("=", len(title)))
	fmt.Printf("Total Cost:      $%.6f\n", summary.TotalCost)
	fmt.Printf("Requests:        %d\n", summary.RequestCount)
	fmt.Printf("Input Tokens:    %d (%.1fK)\n", summary.InputTokens, float64(summary.InputTokens)/1000.0)
	fmt.Printf("Output Tokens:   %d (%.1fK)\n", summary.OutputTokens, float64(summary.OutputTokens)/1000.0)
	fmt.Printf("Total Tokens:    %d (%.1fK)\n", summary.TotalTokens, float64(summary.TotalTokens)/1000.0)

	if len(summary.ByModel) > 0 {
		fmt.Println("\nCost by Model:")
		for model, cost := range summary.ByModel {
			fmt.Printf("  %-40s $%.6f\n", model, cost)
		}
	}

	if len(summary.ByProvider) > 0 {
		fmt.Println("\nCost by Provider:")
		for provider, cost := range summary.ByProvider {
			fmt.Printf("  %-20s $%.6f\n", provider, cost)
		}
	}

	if !summary.StartTime.IsZero() && !summary.EndTime.IsZero() {
		fmt.Printf("\nTime Range:      %s to %s\n",
			summary.StartTime.Format("2006-01-02 15:04:05"),
			summary.EndTime.Format("2006-01-02 15:04:05"))
	}

	fmt.Println()
}
