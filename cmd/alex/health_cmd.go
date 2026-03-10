package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultHealthURL     = "http://localhost:8080/health"
	healthRequestTimeout = 5 * time.Second
)

// healthResponse mirrors the JSON returned by GET /health.
type healthResponse struct {
	Status     string            `json:"status"`
	Components []healthComponent `json:"components"`
}

type healthComponent struct {
	Name    string      `json:"name"`
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

func runHealthCommand(args []string) error {
	jsonOutput := false
	url := defaultHealthURL
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--url":
			if i+1 < len(args) {
				i++
				url = args[i]
			} else {
				return fmt.Errorf("--url requires a value")
			}
		}
	}

	resp, err := fetchHealth(url)
	if err != nil {
		if jsonOutput {
			return printHealthJSON(os.Stdout, &healthResponse{
				Status: "unreachable",
				Components: []healthComponent{{
					Name: "server", Status: "error", Message: err.Error(),
				}},
			})
		}
		fmt.Fprintf(os.Stdout, "Service Status:  DOWN\n")
		fmt.Fprintf(os.Stdout, "  Server at %s is not reachable.\n", url)
		fmt.Fprintf(os.Stdout, "  %v\n", err)
		fmt.Fprintf(os.Stdout, "\nHint: is the server running? Try: alex dev start\n")
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("server unreachable")}
	}

	if jsonOutput {
		return printHealthJSON(os.Stdout, resp)
	}
	printHealthHuman(os.Stdout, resp)
	return nil
}

func fetchHealth(url string) (*healthResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), healthRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	client := &http.Client{Timeout: healthRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var hr healthResponse
	if err := json.Unmarshal(body, &hr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &hr, nil
}

func printHealthJSON(w io.Writer, resp *healthResponse) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func printHealthHuman(w io.Writer, resp *healthResponse) {
	icon := "✓"
	label := strings.ToUpper(resp.Status)
	if resp.Status == "unhealthy" {
		icon = "✗"
	} else if resp.Status == "degraded" {
		icon = "!"
	}
	fmt.Fprintf(w, "Service Status:  %s %s\n", icon, label)

	if len(resp.Components) == 0 {
		return
	}

	fmt.Fprintf(w, "\nComponents:\n")
	for _, c := range resp.Components {
		cIcon := statusIcon(c.Status)
		fmt.Fprintf(w, "  %s %-20s %s", cIcon, c.Name, c.Status)
		if c.Message != "" {
			fmt.Fprintf(w, "  — %s", c.Message)
		}
		fmt.Fprintln(w)
	}
}

func statusIcon(status string) string {
	switch status {
	case "ready":
		return "✓"
	case "disabled":
		return "-"
	case "error":
		return "✗"
	default:
		return "!"
	}
}
