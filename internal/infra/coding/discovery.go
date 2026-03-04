package coding

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"alex/internal/shared/utils"
)

const (
	defaultProbeTimeout = 2 * time.Second
)

var (
	discoveryExecCommand = exec.CommandContext
	discoveryReadDir     = os.ReadDir
	discoveryPathEnv     = os.Getenv
	discoveryNow         = time.Now
)

// DiscoveredCLICapability captures probe results for one CLI candidate.
type DiscoveredCLICapability struct {
	ID                  string    `json:"id" yaml:"id"`
	Binary              string    `json:"binary" yaml:"binary"`
	Path                string    `json:"path" yaml:"path"`
	Executable          bool      `json:"executable" yaml:"executable"`
	Version             string    `json:"version,omitempty" yaml:"version,omitempty"`
	AgentType           string    `json:"agent_type,omitempty" yaml:"agent_type,omitempty"`
	AdapterSupport      bool      `json:"adapter_support" yaml:"adapter_support"`
	SupportsPlan        bool      `json:"supports_plan" yaml:"supports_plan"`
	SupportsExecute     bool      `json:"supports_execute" yaml:"supports_execute"`
	SupportsStream      bool      `json:"supports_stream" yaml:"supports_stream"`
	SupportsFilesystem  bool      `json:"supports_filesystem" yaml:"supports_filesystem"`
	SupportsNetwork     bool      `json:"supports_network" yaml:"supports_network"`
	SupportsToolCall    bool      `json:"supports_tool_call" yaml:"supports_tool_call"`
	SupportsLongContext bool      `json:"supports_long_context" yaml:"supports_long_context"`
	AuthReady           bool      `json:"auth_ready" yaml:"auth_ready"`
	FailureReason       string    `json:"failure_reason,omitempty" yaml:"failure_reason,omitempty"`
	ProbeStderr         string    `json:"probe_stderr,omitempty" yaml:"probe_stderr,omitempty"`
	ProbedAt            time.Time `json:"probed_at" yaml:"probed_at"`
}

// DiscoveryOptions controls dynamic CLI discovery.
type DiscoveryOptions struct {
	// Candidates are candidate binary names to probe first.
	Candidates []string
	// IncludePathScan enables scanning PATH for coding-related executables.
	IncludePathScan bool
	// ProbeTimeout controls per-command probe timeout.
	ProbeTimeout time.Duration
}

var codingPathNamePattern = regexp.MustCompile(`(?i)(codex|claude|kimi|gemini|open.?code|aider|cursor|qwen|deepseek|copilot|codegen)`)

// DiscoverCodingCLIs dynamically discovers coding CLI binaries, probes their
// runtime capabilities, and returns a sorted capability matrix.
func DiscoverCodingCLIs(ctx context.Context, opts DiscoveryOptions) []DiscoveredCLICapability {
	timeout := opts.ProbeTimeout
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}

	nameSet := make(map[string]struct{})
	for _, seed := range defaultDiscoverySeeds() {
		if trimmed := strings.TrimSpace(seed); trimmed != "" {
			nameSet[trimmed] = struct{}{}
		}
	}
	for _, seed := range opts.Candidates {
		if trimmed := strings.TrimSpace(seed); trimmed != "" {
			nameSet[trimmed] = struct{}{}
		}
	}

	if opts.IncludePathScan {
		for _, name := range scanPATHExecutableNames() {
			nameSet[name] = struct{}{}
		}
	}

	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]DiscoveredCLICapability, 0, len(names))
	for _, name := range names {
		cap := probeSingleCLI(ctx, name, timeout)
		if !cap.Executable {
			continue
		}
		out = append(out, cap)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AdapterSupport != out[j].AdapterSupport {
			return out[i].AdapterSupport
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func defaultDiscoverySeeds() []string {
	return []string{
		"codex",
		"claude",
		"claude-code",
		"kimi",
		"kimi-cli",
		"k2",
		"gemini",
		"gemini-cli",
		"opencode",
		"aider",
		"cursor-agent",
		"cursor",
	}
}

func scanPATHExecutableNames() []string {
	pathEnv := strings.TrimSpace(discoveryPathEnv("PATH"))
	if pathEnv == "" {
		return nil
	}
	seen := make(map[string]struct{})
	for _, dir := range filepath.SplitList(pathEnv) {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}
		entries, err := discoveryReadDir(trimmed)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.TrimSpace(entry.Name())
			if name == "" {
				continue
			}
			if !codingPathNamePattern.MatchString(name) {
				continue
			}
			fullPath := filepath.Join(trimmed, name)
			if !isExecutableFile(fullPath) {
				continue
			}
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func probeSingleCLI(ctx context.Context, binary string, timeout time.Duration) DiscoveredCLICapability {
	id := normalizeCLIID(binary)
	now := discoveryNow()
	out := DiscoveredCLICapability{
		ID:        id,
		Binary:    binary,
		AuthReady: true,
		ProbedAt:  now,
	}

	resolved, err := resolveLocalBinaryPath(binary)
	if err != nil {
		out.FailureReason = classifyResolveFailure(err)
		return out
	}

	out.Path = resolved
	out.Executable = true

	versionOut, versionErr := runProbeCommand(ctx, timeout, resolved, "--version")
	if versionErr != nil {
		altOut, altErr := runProbeCommand(ctx, timeout, resolved, "-V")
		if altErr == nil && strings.TrimSpace(altOut) != "" {
			versionOut = altOut
			versionErr = nil
		} else {
			altOut, altErr = runProbeCommand(ctx, timeout, resolved, "version")
			if altErr == nil {
				versionOut = altOut
				versionErr = nil
			}
		}
	}
	if versionErr != nil {
		out.ProbeStderr = compactProbeText(versionErr.Error())
		if errors.Is(versionErr, context.DeadlineExceeded) {
			out.FailureReason = "probe_timeout"
		}
	}
	out.Version = compactProbeText(versionOut)

	helpOut, helpErr := runProbeCommand(ctx, timeout, resolved, "--help")
	if helpErr != nil {
		altOut, altErr := runProbeCommand(ctx, timeout, resolved, "-h")
		if altErr == nil {
			helpOut = altOut
			helpErr = nil
		}
	}
	if helpErr != nil && out.FailureReason == "" {
		if errors.Is(helpErr, context.DeadlineExceeded) {
			out.FailureReason = "probe_timeout"
		} else {
			out.FailureReason = "probe_error"
		}
		if out.ProbeStderr == "" {
			out.ProbeStderr = compactProbeText(helpErr.Error())
		}
	}

	combined := strings.ToLower(strings.TrimSpace(versionOut + "\n" + helpOut))
	if strings.Contains(combined, "not logged in") || strings.Contains(combined, "login required") {
		out.AuthReady = false
		out.FailureReason = "not_logged_in"
	}
	if strings.Contains(combined, "unauthorized") || strings.Contains(combined, "authentication failed") {
		out.AuthReady = false
		out.FailureReason = "unauthorized"
	}

	out.AdapterSupport, out.AgentType = detectLegacyAdapterSupport(id, binary)
	out.SupportsExecute = true
	out.SupportsPlan = inferCapability(combined, []string{" plan", "planning", "dry-run"})
	out.SupportsStream = inferCapability(combined, []string{"stream", "watch", "jsonl"})
	out.SupportsFilesystem = inferCapability(combined, []string{"file", "write", "edit", "sandbox", "workspace"})
	out.SupportsNetwork = inferCapability(combined, []string{"web", "search", "http", "internet"})
	out.SupportsToolCall = inferCapability(combined, []string{"tool", "mcp", "function call"})
	out.SupportsLongContext = inferCapability(combined, []string{"context", "token", "long"})

	// Known CLIs get pragmatic defaults even when help/version output is sparse.
	switch out.ID {
	case "codex", "claude_code", "kimi", "gemini", "opencode":
		out.SupportsPlan = true
		out.SupportsExecute = true
	}
	if out.ID == "codex" || out.ID == "claude_code" || out.ID == "kimi" {
		out.SupportsStream = true
		out.SupportsFilesystem = true
	}

	return out
}

func classifyResolveFailure(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, os.ErrNotExist) {
		return "not_found"
	}
	return "not_executable"
}

func runProbeCommand(ctx context.Context, timeout time.Duration, binary string, arg string) (string, error) {
	pctx := ctx
	cancel := func() {}
	if timeout > 0 {
		pctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	cmd := discoveryExecCommand(pctx, binary, arg)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if errors.Is(pctx.Err(), context.DeadlineExceeded) {
			return text, context.DeadlineExceeded
		}
		if text != "" {
			return text, err
		}
		return "", err
	}
	return text, nil
}

func inferCapability(text string, keywords []string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func compactProbeText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if utf8Len(trimmed) <= 320 {
		return trimmed
	}
	return strings.TrimSpace(string([]rune(trimmed)[:320])) + "…"
}

func utf8Len(s string) int {
	return len([]rune(s))
}

func normalizeCLIID(binary string) string {
	trimmed := strings.TrimSpace(strings.ToLower(binary))
	switch trimmed {
	case "claude", "claude-code":
		return "claude_code"
	case "kimi-cli", "k2", "kimi cli":
		return "kimi"
	}

	// Keep arbitrary binaries discoverable while normalizing separators.
	normalized := strings.NewReplacer(" ", "_", "-", "_").Replace(trimmed)
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func detectLegacyAdapterSupport(id, binary string) (bool, string) {
	switch normalizeCLIID(id) {
	case "codex":
		return true, "codex"
	case "claude_code":
		return true, "claude_code"
	case "kimi":
		return true, "kimi"
	}

	// Keep compatibility with prior hard-coded adapter types.
	switch utils.TrimLower(binary) {
	case "codex":
		return true, "codex"
	case "claude", "claude-code":
		return true, "claude_code"
	case "kimi", "kimi-cli", "k2":
		return true, "kimi"
	}
	return false, ""
}
