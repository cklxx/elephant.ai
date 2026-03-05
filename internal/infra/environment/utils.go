package environment

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

var capabilityChecks = []struct {
	Program string
	Args    []string
}{
	{Program: "git", Args: []string{"--version"}},
	{Program: "go", Args: []string{"version"}},
	{Program: "peekaboo", Args: []string{"--version"}},
	{Program: "node", Args: []string{"--version"}},
	{Program: "npm", Args: []string{"--version"}},
	{Program: "python3", Args: []string{"--version"}},
	{Program: "pip3", Args: []string{"--version"}},
	// Removed deno and cargo - uncommon tools that add startup overhead
}

// CollectLocalSummary inspects the current host process environment to produce a summary.
func CollectLocalSummary(maxFileEntries int) Summary {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}

	files, more := listLocalFiles(workingDir, maxFileEntries)

	osDescription := readLocalOSDescription()
	kernel := runLocalCommand("uname", "-sr")

	capabilities := collectLocalCapabilities()
	environmentHints := collectEnvironmentHints(8)

	return Summary{
		WorkingDirectory: workingDir,
		FileEntries:      files,
		HasMoreFiles:     more,
		OperatingSystem:  osDescription,
		Kernel:           kernel,
		Capabilities:     capabilities,
		EnvironmentHints: environmentHints,
	}
}

func listLocalFiles(dir string, maxEntries int) ([]string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	sort.Strings(names)

	more := false
	if maxEntries > 0 && len(names) > maxEntries {
		more = true
		names = names[:maxEntries]
	}

	return names, more
}

func readLocalOSDescription() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	return parseOSRelease(string(data))
}

func parseOSRelease(content string) string {
	if content == "" {
		return ""
	}

	var name, version string
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "NAME="):
			name = trimQuotes(line[5:])
		case strings.HasPrefix(line, "VERSION="):
			version = trimQuotes(line[8:])
		}
	}

	if name != "" && version != "" {
		return fmt.Sprintf("%s %s", name, version)
	}
	return name
}

func trimQuotes(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"")
	return value
}

func collectLocalCapabilities() []string {
	logger := logging.NewComponentLogger("EnvironmentCapabilities")
	// Use a channel to collect results from parallel goroutines
	type capabilityResult struct {
		output string
		index  int
	}
	resultChan := make(chan capabilityResult, len(capabilityChecks))

	// Launch parallel capability checks
	for i, check := range capabilityChecks {
		idx := i
		c := check
		async.Go(logger, "environment.capabilityCheck", func() {
			output := runLocalCommand(c.Program, c.Args...)
			resultChan <- capabilityResult{output: output, index: idx}
		})
	}

	// Collect results in order
	resultMap := make(map[int]string)
	for i := 0; i < len(capabilityChecks); i++ {
		result := <-resultChan
		if result.output != "" {
			normalized := normalizeCapabilityOutput(
				capabilityChecks[result.index].Program,
				capabilityChecks[result.index].Args,
				result.output,
			)
			resultMap[result.index] = normalized
		}
	}

	// Build ordered results list
	results := make([]string, 0, len(resultMap))
	for i := 0; i < len(capabilityChecks); i++ {
		if output, exists := resultMap[i]; exists {
			results = append(results, output)
		}
	}

	return results
}

func runLocalCommand(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func normalizeCapabilityOutput(program string, args []string, output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}

	trimmed = strings.Split(trimmed, "\n")[0]
	if len(args) == 1 && args[0] == "--version" && !strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(program)) {
		return fmt.Sprintf("%s %s", program, trimmed)
	}
	return trimmed
}

var prioritizedEnvironmentKeys = []string{
	"SHELL",
	"LANG",
	"LC_ALL",
	"TZ",
	"TERM",
	"CI",
	"GOROOT",
	"GOPATH",
	"GOOS",
	"GOARCH",
	"GOMOD",
	"GOFLAGS",
	"NODE_ENV",
	"NPM_CONFIG_PREFIX",
	"PNPM_HOME",
	"PYTHONPATH",
	"VIRTUAL_ENV",
	"CONDA_PREFIX",
	"ALEX_CONTEXT_CONFIG_DIR",
	"ALEX_SKILLS_DIR",
	"ALEX_TOOL_MODE",
	"ALEX_TOOL_PRESET",
}

var sensitiveEnvironmentKeyFragments = []string{
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"PASSWD",
	"PASS",
	"API_KEY",
	"ACCESS_KEY",
	"PRIVATE_KEY",
	"SESSION",
	"COOKIE",
	"AUTH",
	"CREDENTIAL",
	"BEARER",
	"SIGNATURE",
	"JWT",
	"OAUTH",
}

func collectEnvironmentHints(limit int) []string {
	if limit == 0 {
		return nil
	}
	envMap := make(map[string]string)
	for _, entry := range os.Environ() {
		idx := strings.Index(entry, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(entry[:idx])
		value := strings.TrimSpace(entry[idx+1:])
		if key == "" || value == "" {
			continue
		}
		envMap[key] = value
	}
	return collectEnvironmentHintsFromMap(envMap, limit)
}

func collectEnvironmentHintsFromMap(env map[string]string, limit int) []string {
	if len(env) == 0 || limit == 0 {
		return nil
	}
	seen := make(map[string]bool, limit)
	hints := make([]string, 0, limit)

	appendHint := func(key, value string) {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" || isSensitiveEnvironmentKey(key) || seen[key] {
			return
		}
		if len(hints) >= limit {
			return
		}
		hints = append(hints, fmt.Sprintf("%s=%s", key, truncateEnvironmentValue(value, 96)))
		seen[key] = true
	}

	for _, key := range prioritizedEnvironmentKeys {
		value, ok := env[key]
		if !ok {
			continue
		}
		appendHint(key, value)
	}

	if len(hints) < limit {
		if pathSummary := summarizePATH(env["PATH"]); pathSummary != "" {
			hints = append(hints, pathSummary)
		}
	}

	return hints
}

func isSensitiveEnvironmentKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if upper == "" {
		return false
	}
	for _, fragment := range sensitiveEnvironmentKeyFragments {
		if strings.Contains(upper, fragment) {
			return true
		}
	}
	return false
}

func summarizePATH(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	segments := strings.Split(value, string(os.PathListSeparator))
	filtered := make([]string, 0, len(segments))
	seen := make(map[string]bool, len(segments))
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		filtered = append(filtered, trimmed)
	}
	if len(filtered) == 0 {
		return ""
	}
	const previewLimit = 3
	previewEnd := previewLimit
	if len(filtered) < previewLimit {
		previewEnd = len(filtered)
	}
	preview := strings.Join(filtered[:previewEnd], ", ")
	if len(filtered) > previewLimit {
		return fmt.Sprintf("PATH entries=%d [%s, ...]", len(filtered), preview)
	}
	return fmt.Sprintf("PATH entries=%d [%s]", len(filtered), preview)
}

func truncateEnvironmentValue(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "..."
}
