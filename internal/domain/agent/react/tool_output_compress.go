package react

import (
	"regexp"
	"strings"
)

// compressToolOutput applies tool-aware compression before truncation.
// Returns original content if compression fails or produces larger output.
func compressToolOutput(toolName, content string, metadata map[string]any) string {
	if len(content) <= maxToolResultContentChars {
		return content
	}

	var compressed string
	switch {
	case toolName == "shell" || toolName == "bash" || toolName == "execute_command":
		compressed = compressShellOutput(content, metadata)
	default:
		compressed = compressGenericOutput(content)
	}

	if compressed == "" || len(compressed) >= len(content) {
		return content
	}
	return compressed
}

// compressShellOutput routes shell tool output to command-specific compressors
// based on the command string from metadata.
func compressShellOutput(content string, metadata map[string]any) string {
	cmd, _ := metadata["command"].(string)
	if cmd == "" {
		return compressGenericOutput(content)
	}

	base := extractBaseCommand(cmd)
	switch {
	case base == "git" && containsSubcommand(cmd, "status"):
		return compressGitStatus(content)
	case base == "git" && containsSubcommand(cmd, "diff"):
		return compressGitDiff(content)
	case base == "go" && containsSubcommand(cmd, "test"):
		return compressGoTestOutput(content)
	default:
		return compressGenericOutput(content)
	}
}

// extractBaseCommand parses the first meaningful command token from a shell
// command string, handling env var prefixes (FOO=bar cmd), cd prefixes
// (cd /x && cmd), and pipes.
func extractBaseCommand(cmd string) string {
	// Strip leading env var assignments.
	s := strings.TrimSpace(cmd)
	for {
		// Match KEY=value or KEY="value" prefix.
		if i := strings.IndexByte(s, '='); i > 0 && i < strings.IndexAny(s, " \t") {
			rest := s[i+1:]
			// Skip quoted value.
			if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
				q := rest[0]
				end := strings.IndexByte(rest[1:], q)
				if end >= 0 {
					rest = rest[end+2:]
				}
			} else {
				// Skip to next space.
				if sp := strings.IndexAny(rest, " \t"); sp >= 0 {
					rest = rest[sp:]
				} else {
					rest = ""
				}
			}
			s = strings.TrimSpace(rest)
			continue
		}
		break
	}

	// Handle "cd ... &&" prefix.
	if strings.HasPrefix(s, "cd ") {
		if idx := strings.Index(s, "&&"); idx >= 0 {
			s = strings.TrimSpace(s[idx+2:])
		}
	}

	// Take first token (before pipes or semicolons).
	for _, sep := range []string{"|", ";"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}

	// Final token is the command name.
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// containsSubcommand checks if cmd contains the given subcommand token.
func containsSubcommand(cmd, sub string) bool {
	for _, f := range strings.Fields(cmd) {
		if f == sub {
			return true
		}
	}
	return false
}

var gitHintRe = regexp.MustCompile(`(?m)^\s*\(use "git .*\)\s*$`)
var blankRunRe = regexp.MustCompile(`\n{3,}`)

// compressGitStatus removes `(use "git ..." ...)` hint lines and collapses
// runs of blank lines.
func compressGitStatus(content string) string {
	out := gitHintRe.ReplaceAllString(content, "")
	out = blankRunRe.ReplaceAllString(out, "\n\n")
	return strings.TrimSpace(out)
}

// compressGitDiff keeps file headers, hunk headers, and changed (+/-) lines.
// Context lines (unchanged) are dropped when the output exceeds the limit.
func compressGitDiff(content string) string {
	lines := strings.Split(content, "\n")
	var out []string

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			out = append(out, line)
		case strings.HasPrefix(line, "index "):
			out = append(out, line)
		case strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
			out = append(out, line)
		case strings.HasPrefix(line, "@@"):
			out = append(out, line)
		case strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-"):
			out = append(out, line)
		case strings.HasPrefix(line, "Binary files"):
			out = append(out, line)
		case strings.HasPrefix(line, "new file mode") || strings.HasPrefix(line, "deleted file mode"):
			out = append(out, line)
		case strings.HasPrefix(line, "rename ") || strings.HasPrefix(line, "similarity index"):
			out = append(out, line)
			// Drop context lines (lines starting with space) to save tokens.
		}
	}

	result := strings.Join(out, "\n")
	if len(result) >= len(content) {
		return content
	}
	return result
}

var goTestPassRe = regexp.MustCompile(`(?m)^--- PASS:.*$`)
var goTestRunRe = regexp.MustCompile(`(?m)^=== RUN\s+.*$`)

// compressGoTestOutput compresses go test output.
// If all tests pass: keep only the summary lines (ok/FAIL package lines).
// If any test fails: keep FAIL blocks and summary, drop PASS/RUN lines.
func compressGoTestOutput(content string) string {
	hasFailure := strings.Contains(content, "--- FAIL:") || strings.Contains(content, "FAIL\t")

	if !hasFailure {
		// All pass: extract summary lines only.
		lines := strings.Split(content, "\n")
		var summary []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "ok ") || strings.HasPrefix(trimmed, "ok\t") {
				summary = append(summary, line)
			} else if strings.HasPrefix(trimmed, "?") && strings.Contains(trimmed, "[no test files]") {
				summary = append(summary, line)
			}
		}
		if len(summary) == 0 {
			return content
		}
		result := strings.Join(summary, "\n")
		if len(result) >= len(content) {
			return content
		}
		return result
	}

	// Has failures: drop PASS and RUN lines, keep everything else.
	out := goTestPassRe.ReplaceAllString(content, "")
	out = goTestRunRe.ReplaceAllString(out, "")
	out = blankRunRe.ReplaceAllString(out, "\n\n")
	out = strings.TrimSpace(out)

	if len(out) >= len(content) {
		return content
	}
	return out
}

// compressGenericOutput deduplicates consecutive identical lines and collapses
// runs of blank lines.
func compressGenericOutput(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	var prevLine string
	repeatCount := 0

	for _, line := range lines {
		if line == prevLine {
			repeatCount++
			continue
		}
		if repeatCount > 0 {
			out = append(out, prevLine)
			if repeatCount > 1 {
				out = append(out, "  ... (repeated "+itoa(repeatCount)+" more times)")
			}
			repeatCount = 0
		}
		out = append(out, line)
		prevLine = line
	}
	// Flush trailing repeats.
	if repeatCount > 0 {
		if repeatCount > 1 {
			out = append(out, "  ... (repeated "+itoa(repeatCount)+" more times)")
		}
	}

	result := strings.Join(out, "\n")
	result = blankRunRe.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	if len(result) >= len(content) {
		return content
	}
	return result
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
