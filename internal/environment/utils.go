package environment

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

var capabilityChecks = []struct {
	Program string
	Args    []string
}{
	{Program: "git", Args: []string{"--version"}},
	{Program: "go", Args: []string{"version"}},
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

	return Summary{
		WorkingDirectory: workingDir,
		FileEntries:      files,
		HasMoreFiles:     more,
		OperatingSystem:  osDescription,
		Kernel:           kernel,
		Capabilities:     capabilities,
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
	// Use a channel to collect results from parallel goroutines
	type capabilityResult struct {
		output string
		index  int
	}
	resultChan := make(chan capabilityResult, len(capabilityChecks))

	// Launch parallel capability checks
	for i, check := range capabilityChecks {
		go func(idx int, c struct {
			Program string
			Args    []string
		}) {
			output := runLocalCommand(c.Program, c.Args...)
			resultChan <- capabilityResult{output: output, index: idx}
		}(i, check)
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
