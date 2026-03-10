package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/httpclient"
	"alex/internal/shared/utils"
)

const (
	defaultLarkInjectPort           = "9090"
	defaultLarkInjectSenderID       = "ou_inject_user"
	defaultLarkInjectTimeoutSeconds = 300
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

func runLarkCommand(args []string) error {
	if len(args) == 0 {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("usage: alex lark <inject> [...]")}
	}

	switch args[0] {
	case "inject":
		return runLarkInjectCommand(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown lark subcommand %q (expected: inject)", args[0])}
	}
}

func runLarkInjectCommand(args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex lark inject")

	port := fs.String("port", defaultLarkInjectPort, "Debug server port")
	baseURL := fs.String("base-url", "", "Debug server base URL (overrides --port), e.g. http://127.0.0.1:9090")
	chatID := fs.String("chat-id", "", "Chat ID (default: auto-generated)")
	chatType := fs.String("chat-type", "p2p", "Chat type: p2p, group")
	senderID := fs.String("sender-id", defaultLarkInjectSenderID, "Sender open ID")
	timeout := fs.Int("timeout", defaultLarkInjectTimeoutSeconds, "Timeout in seconds")
	autoReply := fs.Bool("auto-reply", false, "Auto-reply when agent asks for clarification")
	maxAutoReplyRounds := fs.Int("max-auto-reply-rounds", 3, "Max auto-reply rounds")

	if err := fs.Parse(args); err != nil {
		return &ExitCodeError{Code: 2, Err: formatBufferedFlagParseError(err, flagBuf)}
	}

	text := strings.Join(fs.Args(), " ")
	if text == "" {
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("usage: alex lark inject [flags] <message>")}
	}
	if utils.IsBlank(*baseURL) && !flagProvided(fs, "port") {
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

	httpClient := httpclient.New(time.Duration(reqBody.TimeoutSeconds+30)*time.Second, nil)
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

func larkInjectEndpoint(baseURL, port string) string {
	trimmedBase := strings.TrimSpace(baseURL)
	if trimmedBase != "" {
		return strings.TrimRight(trimmedBase, "/") + "/api/dev/inject"
	}
	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		trimmedPort = defaultLarkInjectPort
	}
	return fmt.Sprintf("http://127.0.0.1:%s/api/dev/inject", trimmedPort)
}

func postLarkInject(ctx context.Context, httpClient *http.Client, endpoint string, reqBody larkInjectRequest) (*larkInjectHTTPResponse, error) {
	if httpClient == nil {
		httpClient = httpclient.New(time.Duration(defaultLarkInjectTimeoutSeconds+30)*time.Second, nil)
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	const maxAttempts = 3
	var resp *http.Response
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err = httpClient.Do(req)
		if err == nil {
			break
		}
		if attempt == maxAttempts || ctx.Err() != nil || !isTransientInjectTransportError(err) {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}
		time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
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

func isTransientInjectTransportError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.HasSuffix(msg, ": eof") ||
		msg == "eof" ||
		strings.Contains(msg, "broken pipe")
}

func defaultString(value, fallback string) string {
	if utils.IsBlank(value) {
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

	if utils.HasContent(fallback) {
		return strings.TrimSpace(fallback)
	}
	return defaultLarkInjectPort
}
