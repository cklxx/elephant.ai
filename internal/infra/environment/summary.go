package environment

import (
	"fmt"
	"sort"
	"strings"
)

// Summary captures high-level details about an execution environment.
type Summary struct {
	WorkingDirectory string
	FileEntries      []string
	HasMoreFiles     bool
	OperatingSystem  string
	Kernel           string
	Capabilities     []string
}

// IsEmpty reports whether the summary has any populated fields.
func (s Summary) IsEmpty() bool {
	return strings.TrimSpace(s.WorkingDirectory) == "" &&
		len(s.FileEntries) == 0 &&
		strings.TrimSpace(s.OperatingSystem) == "" &&
		strings.TrimSpace(s.Kernel) == "" &&
		len(s.Capabilities) == 0
}

// FormatSummary renders the summary into a human-readable multi-line description.
func FormatSummary(summary Summary) string {
	if summary.IsEmpty() {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Environment context:\n")

	if summary.WorkingDirectory != "" {
		builder.WriteString(fmt.Sprintf("- Working directory: %s\n", summary.WorkingDirectory))
	}

	if len(summary.FileEntries) > 0 {
		entries := append([]string(nil), summary.FileEntries...)
		sort.Strings(entries)
		builder.WriteString(fmt.Sprintf("- Project files: %s", strings.Join(entries, ", ")))
		if summary.HasMoreFiles {
			builder.WriteString(" …")
		}
		builder.WriteString("\n")
	}

	if summary.OperatingSystem != "" {
		builder.WriteString(fmt.Sprintf("- Operating system: %s\n", summary.OperatingSystem))
	}

	if summary.Kernel != "" {
		builder.WriteString(fmt.Sprintf("- Kernel: %s\n", summary.Kernel))
	}

	if len(summary.Capabilities) > 0 {
		capabilities := append([]string(nil), summary.Capabilities...)
		sort.Strings(capabilities)
		builder.WriteString(fmt.Sprintf("- Capabilities: %s\n", strings.Join(capabilities, ", ")))
	}

	return strings.TrimSpace(builder.String())
}

// SummaryMap converts the summary into a string map suitable for diagnostics payloads.
func SummaryMap(summary Summary) map[string]string {
	if summary.IsEmpty() {
		return nil
	}

	result := map[string]string{
		"summary": FormatSummary(summary),
	}

	if summary.WorkingDirectory != "" {
		result["working_directory"] = summary.WorkingDirectory
	}
	if len(summary.FileEntries) > 0 {
		entries := append([]string(nil), summary.FileEntries...)
		sort.Strings(entries)
		result["project_files"] = strings.Join(entries, ", ")
		if summary.HasMoreFiles {
			result["project_files_more"] = "true"
		}
	}
	if summary.OperatingSystem != "" {
		result["operating_system"] = summary.OperatingSystem
	}
	if summary.Kernel != "" {
		result["kernel"] = summary.Kernel
	}
	if len(summary.Capabilities) > 0 {
		capabilities := append([]string(nil), summary.Capabilities...)
		sort.Strings(capabilities)
		result["capabilities"] = strings.Join(capabilities, ", ")
	}

	return result
}
