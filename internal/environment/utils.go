package environment

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	api "github.com/agent-infra/sandbox-sdk-go"
	"github.com/agent-infra/sandbox-sdk-go/shell"

	"alex/internal/tools"
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
	{Program: "deno", Args: []string{"--version"}},
	{Program: "cargo", Args: []string{"--version"}},
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

// CollectSandboxSummary uses the sandbox manager to query environment details from the remote runtime.
func CollectSandboxSummary(ctx context.Context, manager *tools.SandboxManager, maxFileEntries int) (Summary, error) {
	if manager == nil {
		return Summary{}, fmt.Errorf("sandbox manager not available")
	}

	if err := manager.Initialize(ctx); err != nil {
		return Summary{}, err
	}

	shellClient := manager.Shell()
	if shellClient == nil {
		return Summary{}, fmt.Errorf("sandbox shell not available")
	}

	workingDir := runSandboxCommand(ctx, shellClient, "pwd")
	files, more := listSandboxFiles(ctx, shellClient, maxFileEntries)

	osDescription := parseOSRelease(runSandboxCommand(ctx, shellClient, "cat /etc/os-release"))
	kernel := runSandboxCommand(ctx, shellClient, "uname -sr")

	capabilities := collectSandboxCapabilities(ctx, shellClient)

	return Summary{
		WorkingDirectory: workingDir,
		FileEntries:      files,
		HasMoreFiles:     more,
		OperatingSystem:  osDescription,
		Kernel:           kernel,
		Capabilities:     capabilities,
	}, nil
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

func listSandboxFiles(ctx context.Context, shellClient *shell.Client, maxEntries int) ([]string, bool) {
	if maxEntries <= 0 {
		return nil, false
	}

	cmd := fmt.Sprintf("ls -p | head -n %d", maxEntries+1)
	output := runSandboxCommand(ctx, shellClient, cmd)
	if output == "" {
		return nil, false
	}

	lines := filterNonEmpty(strings.Split(output, "\n"))
	more := false
	if len(lines) > maxEntries {
		more = true
		lines = lines[:maxEntries]
	}

	return lines, more
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
	results := make([]string, 0, len(capabilityChecks))
	for _, check := range capabilityChecks {
		output := runLocalCommand(check.Program, check.Args...)
		if output == "" {
			continue
		}
		results = append(results, normalizeCapabilityOutput(check.Program, check.Args, output))
	}
	return results
}

func collectSandboxCapabilities(ctx context.Context, shellClient *shell.Client) []string {
	results := make([]string, 0, len(capabilityChecks))
	for _, check := range capabilityChecks {
		command := fmt.Sprintf("if command -v %s >/dev/null 2>&1; then %s %s; fi", check.Program, check.Program, strings.Join(check.Args, " "))
		output := runSandboxCommand(ctx, shellClient, command)
		if output == "" {
			continue
		}
		firstLine := strings.Split(strings.TrimSpace(output), "\n")[0]
		results = append(results, normalizeCapabilityOutput(check.Program, check.Args, firstLine))
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

func runSandboxCommand(ctx context.Context, shellClient *shell.Client, command string) string {
	if shellClient == nil || strings.TrimSpace(command) == "" {
		return ""
	}

	req := &api.ShellExecRequest{Command: command}
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := shellClient.ExecCommand(cmdCtx, req)
	if err != nil {
		return ""
	}
	data := resp.GetData()
	if data == nil || data.GetOutput() == nil {
		return ""
	}
	output := strings.TrimSpace(*data.GetOutput())
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(filterNonEmpty(lines), "\n")
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

func filterNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
