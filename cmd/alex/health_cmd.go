package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"alex/internal/shared/httpclient"
)

const (
	healthUsage          = "usage: alex health [--json] [--url <server-url>]"
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

type healthFlags struct {
	jsonOutput bool
	url        string
}

func runHealthCommand(args []string) error {
	flags, showUsage, err := parseHealthFlags(args)
	if showUsage {
		fmt.Fprintln(os.Stdout, healthUsage)
		return nil
	}
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	resp, err := fetchHealth(flags.url)
	if err != nil {
		if flags.jsonOutput {
			return printHealthJSON(os.Stdout, &healthResponse{
				Status: "unreachable",
				Components: []healthComponent{{
					Name: "server", Status: "error", Message: err.Error(),
				}},
			})
		}
		fmt.Fprintf(os.Stdout, "Service Status:  DOWN\n")
		fmt.Fprintf(os.Stdout, "  Server at %s is not reachable.\n", flags.url)
		fmt.Fprintf(os.Stdout, "  %v\n", err)
		fmt.Fprintf(os.Stdout, "\nHint: is the server running? Try: alex dev start\n")
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("server unreachable")}
	}

	if flags.jsonOutput {
		return printHealthJSON(os.Stdout, resp)
	}
	printHealthHuman(os.Stdout, resp)
	return nil
}

func parseHealthFlags(args []string) (healthFlags, bool, error) {
	fs, flagBuf := newBufferedFlagSet("alex health")
	jsonOutput := fs.Bool("json", false, "Output health status as JSON")
	url := fs.String("url", defaultHealthURL, "Health endpoint URL")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return healthFlags{}, true, nil
		}
		return healthFlags{}, false, formatBufferedFlagParseError(err, flagBuf)
	}
	if len(fs.Args()) > 0 {
		return healthFlags{}, false, fmt.Errorf("unexpected arguments: %s; %s", strings.Join(fs.Args(), " "), healthUsage)
	}

	return healthFlags{
		jsonOutput: *jsonOutput,
		url:        strings.TrimSpace(*url),
	}, false, nil
}

func fetchHealth(url string) (*healthResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), healthRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	client := httpclient.New(healthRequestTimeout, nil)
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
