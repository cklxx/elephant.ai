package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"alex/internal/agent/app"
	"alex/internal/agent/ports"
	"alex/internal/storage"
)

// Example: Cost Tracking Integration
//
// This example demonstrates how to use ALEX's cost tracking system
// to monitor and analyze LLM API costs.

func main() {
	fmt.Println("ALEX Cost Tracking Integration Example")
	fmt.Println("=" + string(make([]byte, 40)) + "=")
	fmt.Println()

	// 1. Initialize Cost Store
	fmt.Println("1. Initializing cost store...")
	costStore, err := storage.NewFileCostStore("~/.alex-costs-example")
	if err != nil {
		log.Fatalf("Failed to create cost store: %v", err)
	}
	fmt.Println("   ✓ Cost store initialized at ~/.alex-costs-example")
	fmt.Println()

	// 2. Create Cost Tracker
	fmt.Println("2. Creating cost tracker...")
	costTracker := app.NewCostTracker(costStore)
	fmt.Println("   ✓ Cost tracker created")
	fmt.Println()

	// 3. Simulate LLM Usage
	fmt.Println("3. Simulating LLM API calls...")
	fmt.Println()

	ctx := context.Background()
	sessionID := "example-session-" + fmt.Sprintf("%d", time.Now().Unix())

	// Simulate multiple API calls with different models
	usageRecords := []ports.UsageRecord{
		{
			SessionID:    sessionID,
			Model:        "gpt-4o",
			Provider:     "openrouter",
			InputTokens:  1500,
			OutputTokens: 800,
		},
		{
			SessionID:    sessionID,
			Model:        "gpt-4o",
			Provider:     "openrouter",
			InputTokens:  2000,
			OutputTokens: 1200,
		},
		{
			SessionID:    sessionID,
			Model:        "gpt-4o-mini",
			Provider:     "openrouter",
			InputTokens:  5000,
			OutputTokens: 3000,
		},
		{
			SessionID:    sessionID,
			Model:        "deepseek-chat",
			Provider:     "deepseek",
			InputTokens:  10000,
			OutputTokens: 5000,
		},
	}

	for i, record := range usageRecords {
		fmt.Printf("   API Call %d:\n", i+1)
		fmt.Printf("     Model: %s\n", record.Model)
		fmt.Printf("     Provider: %s\n", record.Provider)
		fmt.Printf("     Tokens: %d input, %d output\n", record.InputTokens, record.OutputTokens)

		// Record usage (costs are calculated automatically)
		if err := costTracker.RecordUsage(ctx, record); err != nil {
			log.Printf("Failed to record usage: %v", err)
			continue
		}

		// Show calculated cost
		inputCost, outputCost, totalCost := ports.CalculateCost(
			record.InputTokens,
			record.OutputTokens,
			record.Model,
		)
		fmt.Printf("     Cost: $%.6f (input: $%.6f, output: $%.6f)\n\n", totalCost, inputCost, outputCost)
	}

	// 4. Query Session Cost
	fmt.Println("4. Querying session cost...")
	fmt.Println()
	sessionSummary, err := costTracker.GetSessionCost(ctx, sessionID)
	if err != nil {
		log.Fatalf("Failed to get session cost: %v", err)
	}

	printCostSummary("Session Cost", sessionSummary)

	// 5. Query Daily Cost
	fmt.Println("5. Querying daily cost...")
	fmt.Println()
	dailySummary, err := costTracker.GetDailyCost(ctx, time.Now())
	if err != nil {
		log.Fatalf("Failed to get daily cost: %v", err)
	}

	printCostSummary("Daily Cost (Today)", dailySummary)

	// 6. Export to CSV
	fmt.Println("6. Exporting data to CSV...")
	fmt.Println()
	csvData, err := costTracker.Export(ctx, ports.ExportFormatCSV, ports.ExportFilter{
		SessionID: sessionID,
	})
	if err != nil {
		log.Fatalf("Failed to export CSV: %v", err)
	}

	csvFile := "/tmp/alex-cost-example.csv"
	if err := os.WriteFile(csvFile, csvData, 0644); err != nil {
		log.Fatalf("Failed to write CSV file: %v", err)
	}
	fmt.Printf("   ✓ CSV exported to: %s\n\n", csvFile)

	// 7. Export to JSON
	fmt.Println("7. Exporting data to JSON...")
	fmt.Println()
	jsonData, err := costTracker.Export(ctx, ports.ExportFormatJSON, ports.ExportFilter{
		SessionID: sessionID,
	})
	if err != nil {
		log.Fatalf("Failed to export JSON: %v", err)
	}

	jsonFile := "/tmp/alex-cost-example.json"
	if err := os.WriteFile(jsonFile, jsonData, 0644); err != nil {
		log.Fatalf("Failed to write JSON file: %v", err)
	}
	fmt.Printf("   ✓ JSON exported to: %s\n\n", jsonFile)

	// 8. Cost Analysis
	fmt.Println("8. Cost Analysis")
	fmt.Println()
	analyzeCosts(sessionSummary)

	fmt.Println("\n" + string(make([]byte, 50)) + "")
	fmt.Println("Example completed successfully!")
	fmt.Printf("Session ID: %s\n", sessionID)
	fmt.Println("\nTo view this data with ALEX CLI:")
	fmt.Printf("  alex cost session %s\n", sessionID)
	fmt.Println("  alex cost day")
	fmt.Printf("  alex cost export --format csv --session %s\n", sessionID)
}

func printCostSummary(title string, summary *ports.CostSummary) {
	fmt.Printf("   %s:\n", title)
	fmt.Printf("   %s\n", string(make([]byte, len(title)+3)))
	fmt.Printf("   Total Cost:      $%.6f\n", summary.TotalCost)
	fmt.Printf("   Requests:        %d\n", summary.RequestCount)
	fmt.Printf("   Input Tokens:    %d (%.1fK)\n", summary.InputTokens, float64(summary.InputTokens)/1000.0)
	fmt.Printf("   Output Tokens:   %d (%.1fK)\n", summary.OutputTokens, float64(summary.OutputTokens)/1000.0)
	fmt.Printf("   Total Tokens:    %d (%.1fK)\n", summary.TotalTokens, float64(summary.TotalTokens)/1000.0)

	if len(summary.ByModel) > 0 {
		fmt.Println("\n   Cost by Model:")
		for model, cost := range summary.ByModel {
			fmt.Printf("     %-30s $%.6f\n", model, cost)
		}
	}

	if len(summary.ByProvider) > 0 {
		fmt.Println("\n   Cost by Provider:")
		for provider, cost := range summary.ByProvider {
			fmt.Printf("     %-20s $%.6f\n", provider, cost)
		}
	}
	fmt.Println()
}

func analyzeCosts(summary *ports.CostSummary) {
	// Calculate average cost per request
	avgCostPerRequest := summary.TotalCost / float64(summary.RequestCount)
	fmt.Printf("   Average cost per request: $%.6f\n", avgCostPerRequest)

	// Calculate token efficiency
	if summary.InputTokens > 0 {
		inputOutputRatio := float64(summary.OutputTokens) / float64(summary.InputTokens)
		fmt.Printf("   Output/Input token ratio: %.2f\n", inputOutputRatio)
	}

	// Find most expensive model
	var maxModel string
	var maxCost float64
	for model, cost := range summary.ByModel {
		if cost > maxCost {
			maxCost = cost
			maxModel = model
		}
	}
	if maxModel != "" {
		percentage := (maxCost / summary.TotalCost) * 100
		fmt.Printf("   Most expensive model: %s ($%.6f, %.1f%%)\n", maxModel, maxCost, percentage)
	}

	// Cost optimization suggestions
	fmt.Println("\n   Cost Optimization Tips:")

	// Check if using expensive models
	if _, ok := summary.ByModel["gpt-4"]; ok {
		fmt.Println("     • Consider using gpt-4o or gpt-4o-mini for better cost efficiency")
	}

	// Check output token ratio
	if summary.OutputTokens > 0 && summary.InputTokens > 0 {
		ratio := float64(summary.OutputTokens) / float64(summary.InputTokens)
		if ratio > 1.5 {
			fmt.Println("     • High output/input ratio - consider using more concise prompts")
		}
	}

	// Suggest cheaper alternatives
	if summary.TotalCost > 0.01 {
		potentialSavings := 0.0
		for model, cost := range summary.ByModel {
			switch model {
			case "gpt-4":
				// gpt-4 to gpt-4o is ~5x cheaper
				potentialSavings += cost * 0.8
			case "gpt-4o":
				// gpt-4o to gpt-4o-mini is ~30x cheaper
				potentialSavings += cost * 0.95
			}
		}
		if potentialSavings > 0 {
			fmt.Printf("     • Potential savings with cheaper models: $%.6f (%.1f%%)\n",
				potentialSavings, (potentialSavings/summary.TotalCost)*100)
		}
	}
}
