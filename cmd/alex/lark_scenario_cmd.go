package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	larkgw "alex/internal/delivery/channels/lark"
	larktesting "alex/internal/delivery/channels/lark/testing"
	runtimeconfig "alex/internal/shared/config"
)

const (
	larkScenarioModeHTTP            = "http"
	larkScenarioModeMock            = "mock"
	defaultLarkScenarioMockDir      = "tests/scenarios/lark"
	defaultLarkScenarioHTTPDir      = "tests/scenarios/lark_http"
	defaultLarkInjectPort           = "9090"
	defaultLarkInjectSenderID       = "ou_inject_user"
	defaultLarkInjectTimeoutSeconds = 300
	scenarioRunUsage                = "usage: alex lark scenario run [--mode http|mock] [--dir path] [--json-out file] [--md-out file]"
)

type larkInjectRequest struct {
	Text               string `json:"text"`
	ChatID             string `json:"chat_id,omitempty"`
	ChatType           string `json:"chat_type,omitempty"`
	SenderID           string `json:"sender_id,omitempty"`
	TimeoutSeconds     int    `json:"timeout_seconds,omitempty"`
	AutoReply          bool   `json:"auto_reply,omitempty"`
	MaxAutoReplyRounds int    `json:"max_auto_reply_rounds,omitempty"`
}

type larkInjectReply struct {
	Method  string `json:"method"`
	Content string `json:"content"`
	MsgType string `json:"msg_type,omitempty"`
	Emoji   string `json:"emoji,omitempty"`
}

type larkInjectResponse struct {
	Replies     []larkInjectReply `json:"replies"`
	DurationMs  int64             `json:"duration_ms"`
	Error       string            `json:"error,omitempty"`
	AutoReplies int               `json:"auto_replies,omitempty"`
}

type larkInjectHTTPResponse struct {
	StatusCode int
	Body       larkInjectResponse
	RawBody    []byte
}

type larkScenarioHTTPRunOptions struct {
	endpoint       string
	timeoutSeconds int
	httpClient     *http.Client
}

func runLarkCommand(args []string) error {
	if len(args) == 0 {
		return &ExitCodeError{Code: 2, Err: errors.New(scenarioRunUsage)}
	}

	switch args[0] {
	case "scenario", "scenarios":
		return runLarkScenarioCommand(args[1:])
	case "inject":
		return runLarkInjectCommand(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown lark subcommand %q (expected: scenario, inject)", args[0])}
	}
}

func runLarkScenarioCommand(args []string) error {
	if len(args) == 0 {
		return &ExitCodeError{Code: 2, Err: errors.New(scenarioRunUsage)}
	}
	switch args[0] {
	case "run":
		return runLarkScenarioRun(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown lark scenario subcommand %q (expected: run)", args[0])}
	}
}

type stringListFlag []string

func (s *stringListFlag) String() string { return strings.Join(*s, ",") }
func (s *stringListFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*s = append(*s, part)
		}
	}
	return nil
}

func runLarkScenarioRun(args []string) error {
	fs := flag.NewFlagSet("alex lark scenario run", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	mode := fs.String("mode", larkScenarioModeHTTP, "Scenario execution mode: http (real /api/dev/inject), mock (in-process)")
	dir := fs.String("dir", "", "Directory containing Lark scenario .yaml files (default depends on --mode)")
	jsonOut := fs.String("json-out", "", "Write JSON report to this file (optional)")
	mdOut := fs.String("md-out", "", "Write Markdown report to this file (optional)")
	name := fs.String("name", "", "Run only a single scenario by name (optional)")
	failFast := fs.Bool("fail-fast", false, "Stop after the first failing scenario")
	port := fs.String("port", defaultLarkInjectPort, "Debug server port (for --mode http)")
	baseURL := fs.String("base-url", "", "Debug server base URL for --mode http (overrides --port), e.g. http://127.0.0.1:9090")
	timeout := fs.Int("timeout", defaultLarkInjectTimeoutSeconds, "Per-turn timeout in seconds for --mode http")

	var tags stringListFlag
	fs.Var(&tags, "tag", "Run only scenarios that contain these tag(s). Can be repeated or comma-separated.")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))}
	}

	normalizedMode, err := normalizeLarkScenarioMode(*mode)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	scenarioDir := strings.TrimSpace(*dir)
	if scenarioDir == "" {
		scenarioDir = defaultScenarioDirForMode(normalizedMode)
	}

	scenarios, err := larktesting.LoadScenariosFromDir(scenarioDir)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	filtered := filterScenarios(scenarios, *name, tags)
	if len(filtered) == 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("no scenarios matched (dir=%s name=%q tags=%v)", scenarioDir, *name, []string(tags))}
	}

	ctx := context.Background()
	var results []*larktesting.ScenarioResult

	switch normalizedMode {
	case larkScenarioModeMock:
		results = runMockScenarios(ctx, filtered, *failFast)
	case larkScenarioModeHTTP:
		if strings.TrimSpace(*baseURL) == "" && !flagProvided(fs, "port") {
			*port = detectLarkDebugPort(defaultLarkInjectPort)
		}
		endpoint := larkInjectEndpoint(*baseURL, *port)
		clientTimeout := time.Duration(maxInt(*timeout, defaultLarkInjectTimeoutSeconds)+30) * time.Second
		opts := larkScenarioHTTPRunOptions{
			endpoint:       endpoint,
			timeoutSeconds: maxInt(*timeout, defaultLarkInjectTimeoutSeconds),
			httpClient:     &http.Client{Timeout: clientTimeout},
		}
		results = runHTTPScenarios(ctx, filtered, *failFast, opts)
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unsupported mode %q", normalizedMode)}
	}

	report := larktesting.BuildReport(results)

	if *jsonOut != "" {
		if err := writeFile(*jsonOut, mustJSON(report)); err != nil {
			return &ExitCodeError{Code: 2, Err: err}
		}
	}
	if *mdOut != "" {
		if err := writeFile(*mdOut, []byte(report.ToMarkdown())); err != nil {
			return &ExitCodeError{Code: 2, Err: err}
		}
	}

	fmt.Printf("Lark scenarios (%s): total=%d passed=%d failed=%d duration=%s\n",
		normalizedMode,
		report.Summary.Total,
		report.Summary.Passed,
		report.Summary.Failed,
		report.Duration.Round(0).String(),
	)
	if normalizedMode == larkScenarioModeHTTP {
		fmt.Printf("Inject endpoint: %s\n", larkInjectEndpoint(*baseURL, *port))
	}
	if *jsonOut != "" {
		fmt.Printf("JSON report: %s\n", *jsonOut)
	}
	if *mdOut != "" {
		fmt.Printf("Markdown report: %s\n", *mdOut)
	}

	if report.Summary.Failed > 0 {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("lark scenarios failed: %d", report.Summary.Failed)}
	}
	return nil
}

func normalizeLarkScenarioMode(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", larkScenarioModeHTTP, "inject":
		return larkScenarioModeHTTP, nil
	case larkScenarioModeMock, "local":
		return larkScenarioModeMock, nil
	default:
		return "", fmt.Errorf("invalid --mode %q (expected: http|mock)", mode)
	}
}

func defaultScenarioDirForMode(mode string) string {
	if mode == larkScenarioModeMock {
		return filepath.Clean(defaultLarkScenarioMockDir)
	}
	return filepath.Clean(defaultLarkScenarioHTTPDir)
}

func runMockScenarios(ctx context.Context, scenarios []*larktesting.Scenario, failFast bool) []*larktesting.ScenarioResult {
	runner := larktesting.NewRunner(nil)
	results := make([]*larktesting.ScenarioResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		res := runner.Run(ctx, scenario)
		results = append(results, res)
		if failFast && !res.Passed {
			break
		}
	}
	return results
}

func runHTTPScenarios(ctx context.Context, scenarios []*larktesting.Scenario, failFast bool, opts larkScenarioHTTPRunOptions) []*larktesting.ScenarioResult {
	results := make([]*larktesting.ScenarioResult, 0, len(scenarios))
	for _, scenario := range scenarios {
		res := runHTTPScenario(ctx, scenario, opts)
		results = append(results, res)
		if failFast && !res.Passed {
			break
		}
	}
	return results
}

func runHTTPScenario(ctx context.Context, scenario *larktesting.Scenario, opts larkScenarioHTTPRunOptions) *larktesting.ScenarioResult {
	start := time.Now()
	result := &larktesting.ScenarioResult{
		Name: scenario.Name,
	}

	if opts.httpClient == nil {
		opts.httpClient = &http.Client{Timeout: time.Duration(maxInt(opts.timeoutSeconds, defaultLarkInjectTimeoutSeconds)+30) * time.Second}
	}
	if strings.TrimSpace(opts.endpoint) == "" {
		opts.endpoint = larkInjectEndpoint("", defaultLarkInjectPort)
	}
	if opts.timeoutSeconds <= 0 {
		opts.timeoutSeconds = defaultLarkInjectTimeoutSeconds
	}

	for i, turn := range scenario.Turns {
		if turn.DelayMS > 0 {
			time.Sleep(time.Duration(turn.DelayMS) * time.Millisecond)
		}

		turnStart := time.Now()
		tr := larktesting.TurnResult{
			TurnIndex: i,
		}

		if turn.MockResponse != nil {
			tr.Errors = append(tr.Errors, "mock_response is not supported in --mode http; run with --mode mock")
		}
		assertions := turn.Assertions
		if assertions.Executor != nil {
			tr.Errors = append(tr.Errors, "executor assertions are not supported in --mode http; run with --mode mock")
			assertions.Executor = nil
		}

		if len(tr.Errors) == 0 {
			injectReq := larkInjectRequest{
				Text:               turn.Content,
				ChatID:             strings.TrimSpace(turn.ChatID),
				ChatType:           defaultString(strings.TrimSpace(turn.ChatType), "p2p"),
				SenderID:           defaultString(strings.TrimSpace(turn.SenderID), defaultLarkInjectSenderID),
				TimeoutSeconds:     opts.timeoutSeconds,
				AutoReply:          turn.AutoReply,
				MaxAutoReplyRounds: turn.MaxAutoReplyRounds,
			}

			injectResp, err := postLarkInject(ctx, opts.httpClient, opts.endpoint, injectReq)
			if err != nil {
				tr.Errors = append(tr.Errors, fmt.Sprintf("inject request failed: %v", err))
			} else {
				tr.Calls = injectRepliesToMessengerCalls(injectResp.Body.Replies)
				if injectResp.Body.DurationMs > 0 {
					tr.Duration = time.Duration(injectResp.Body.DurationMs) * time.Millisecond
				}
				if injectResp.StatusCode >= http.StatusBadRequest {
					errText := strings.TrimSpace(injectResp.Body.Error)
					if errText == "" {
						errText = strings.TrimSpace(string(injectResp.RawBody))
					}
					if errText == "" {
						errText = http.StatusText(injectResp.StatusCode)
					}
					tr.Errors = append(tr.Errors, fmt.Sprintf("inject endpoint returned HTTP %d: %s", injectResp.StatusCode, errText))
				} else if injectResp.Body.Error != "" {
					tr.Errors = append(tr.Errors, fmt.Sprintf("inject endpoint returned error: %s", injectResp.Body.Error))
				}

				tr.Errors = append(tr.Errors, larktesting.EvaluateAssertions(assertions, tr)...)
			}
		}

		if tr.Duration <= 0 {
			tr.Duration = time.Since(turnStart)
		}
		result.Turns = append(result.Turns, tr)
	}

	result.Duration = time.Since(start)
	result.Passed = true
	for _, tr := range result.Turns {
		if len(tr.Errors) > 0 {
			result.Passed = false
			break
		}
	}

	return result
}

func injectRepliesToMessengerCalls(replies []larkInjectReply) []larkgw.MessengerCall {
	calls := make([]larkgw.MessengerCall, 0, len(replies))
	for _, reply := range replies {
		calls = append(calls, larkgw.MessengerCall{
			Method:  reply.Method,
			Content: reply.Content,
			MsgType: reply.MsgType,
			Emoji:   reply.Emoji,
		})
	}
	return calls
}

func larkInjectEndpoint(baseURL, port string) string {
	trimmedBase := strings.TrimSpace(baseURL)
	if trimmedBase != "" {
		return strings.TrimRight(trimmedBase, "/") + "/api/dev/inject"
	}
	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		trimmedPort = defaultLarkInjectPort
	}
	return fmt.Sprintf("http://localhost:%s/api/dev/inject", trimmedPort)
}

func postLarkInject(ctx context.Context, httpClient *http.Client, endpoint string, reqBody larkInjectRequest) (*larkInjectHTTPResponse, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Duration(defaultLarkInjectTimeoutSeconds+30) * time.Second}
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var parsed larkInjectResponse
	if len(bytes.TrimSpace(rawBody)) > 0 {
		if err := json.Unmarshal(rawBody, &parsed); err != nil {
			return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(rawBody))
		}
	}

	return &larkInjectHTTPResponse{
		StatusCode: resp.StatusCode,
		Body:       parsed,
		RawBody:    rawBody,
	}, nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func filterScenarios(all []*larktesting.Scenario, name string, tags []string) []*larktesting.Scenario {
	var out []*larktesting.Scenario
	manualRequested := hasTag(tags, "manual")
	for _, s := range all {
		if name != "" && s.Name != name {
			continue
		}
		if hasTag(s.Tags, "manual") && name == "" && !manualRequested {
			continue
		}
		if len(tags) > 0 && !scenarioHasAnyTag(s, tags) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func hasTag(tags []string, needle string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), needle) {
			return true
		}
	}
	return false
}

func scenarioHasAnyTag(s *larktesting.Scenario, tags []string) bool {
	if s == nil {
		return false
	}
	tagSet := make(map[string]bool, len(s.Tags))
	for _, t := range s.Tags {
		tagSet[t] = true
	}
	for _, t := range tags {
		if tagSet[t] {
			return true
		}
	}
	return false
}

func mustJSON(report *larktesting.TestReport) []byte {
	b, err := report.ToJSON()
	if err != nil {
		// Should never happen; report is a simple struct.
		return []byte("{}")
	}
	return b
}

func writeFile(path string, contents []byte) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty output path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func runLarkInjectCommand(args []string) error {
	fs := flag.NewFlagSet("alex lark inject", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)

	port := fs.String("port", defaultLarkInjectPort, "Debug server port")
	baseURL := fs.String("base-url", "", "Debug server base URL (overrides --port), e.g. http://127.0.0.1:9090")
	chatID := fs.String("chat-id", "", "Chat ID (default: auto-generated)")
	chatType := fs.String("chat-type", "p2p", "Chat type: p2p, group")
	senderID := fs.String("sender-id", defaultLarkInjectSenderID, "Sender open ID")
	timeout := fs.Int("timeout", defaultLarkInjectTimeoutSeconds, "Timeout in seconds")
	autoReply := fs.Bool("auto-reply", false, "Auto-reply when agent asks for clarification")
	maxAutoReplyRounds := fs.Int("max-auto-reply-rounds", 3, "Max auto-reply rounds")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))}
	}

	text := strings.Join(fs.Args(), " ")
	if text == "" {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("usage: alex lark inject [flags] <message>")}
	}
	if strings.TrimSpace(*baseURL) == "" && !flagProvided(fs, "port") {
		*port = detectLarkDebugPort(defaultLarkInjectPort)
	}

	endpoint := larkInjectEndpoint(*baseURL, *port)
	reqBody := larkInjectRequest{
		Text:               text,
		ChatID:             strings.TrimSpace(*chatID),
		ChatType:           defaultString(strings.TrimSpace(*chatType), "p2p"),
		SenderID:           defaultString(strings.TrimSpace(*senderID), defaultLarkInjectSenderID),
		TimeoutSeconds:     maxInt(*timeout, defaultLarkInjectTimeoutSeconds),
		AutoReply:          *autoReply,
		MaxAutoReplyRounds: *maxAutoReplyRounds,
	}

	displayChatID := reqBody.ChatID
	if displayChatID == "" {
		displayChatID = "(auto)"
	}
	fmt.Printf("Injecting message to Lark gateway (chat=%s, type=%s)...\n\n", displayChatID, reqBody.ChatType)

	httpClient := &http.Client{Timeout: time.Duration(reqBody.TimeoutSeconds+30) * time.Second}
	resp, err := postLarkInject(context.Background(), httpClient, endpoint, reqBody)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}

	if resp.Body.Error != "" {
		fmt.Printf("Error: %s\n", resp.Body.Error)
	}

	duration := time.Duration(resp.Body.DurationMs) * time.Millisecond
	if duration <= 0 {
		duration = time.Duration(reqBody.TimeoutSeconds) * time.Second
	}
	if len(resp.Body.Replies) == 0 {
		fmt.Printf("No bot replies captured.\n\nDuration: %s\n", duration.Round(time.Millisecond))
		if resp.StatusCode >= http.StatusBadRequest {
			return &ExitCodeError{Code: 1, Err: fmt.Errorf("inject failed with HTTP %d", resp.StatusCode)}
		}
		if resp.Body.Error != "" {
			return &ExitCodeError{Code: 1, Err: fmt.Errorf("inject failed: %s", resp.Body.Error)}
		}
		return nil
	}

	fmt.Printf("Bot replies (%d):\n", len(resp.Body.Replies))
	for i, r := range resp.Body.Replies {
		fmt.Println("──────────────────────────────")
		label := r.Method
		if r.MsgType != "" {
			label += " (" + r.MsgType + ")"
		}
		fmt.Printf("[%d] %s\n", i+1, label)

		if r.Content != "" {
			fmt.Println(extractLarkReplyText(r.Content))
		}
		if r.Emoji != "" {
			fmt.Printf("emoji: %s\n", r.Emoji)
		}
	}
	if resp.Body.AutoReplies > 0 {
		fmt.Printf("\nAuto-replies: %d\n", resp.Body.AutoReplies)
	}
	fmt.Printf("\nDuration: %s\n", duration.Round(time.Millisecond))

	if resp.StatusCode >= http.StatusBadRequest {
		if resp.Body.Error != "" {
			return &ExitCodeError{Code: 1, Err: fmt.Errorf("inject completed with HTTP %d: %s", resp.StatusCode, resp.Body.Error)}
		}
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("inject completed with HTTP %d", resp.StatusCode)}
	}
	if resp.Body.Error != "" {
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("inject completed with error: %s", resp.Body.Error)}
	}
	return nil
}

func extractLarkReplyText(content string) string {
	var textObj struct {
		Text string `json:"text"`
	}
	if json.Unmarshal([]byte(content), &textObj) == nil && textObj.Text != "" {
		return textObj.Text
	}
	return content
}

func flagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	if fs == nil {
		return false
	}
	fs.Visit(func(f *flag.Flag) {
		if f != nil && f.Name == name {
			provided = true
		}
	})
	return provided
}

func detectLarkDebugPort(fallback string) string {
	envLookup := runtimeEnvLookup()

	if envLookup != nil {
		if raw, ok := envLookup("ALEX_DEBUG_PORT"); ok {
			if trimmed := strings.TrimSpace(raw); trimmed != "" {
				return trimmed
			}
		}
	}

	cfg, _, err := runtimeconfig.LoadFileConfig(runtimeconfig.WithEnv(envLookup))
	if err == nil && cfg.Server != nil {
		if trimmed := strings.TrimSpace(cfg.Server.DebugPort); trimmed != "" {
			return trimmed
		}
	}

	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return defaultLarkInjectPort
}
