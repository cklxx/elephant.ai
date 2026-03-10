package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTruncateInlinePreview(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		limit  int
		expect string
	}{
		{"limit<=0 returns full", "hello world", 0, "hello world"},
		{"negative limit returns full", "hello world", -5, "hello world"},
		{"within limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"over limit with ellipsis", "hello world", 6, "hello…"},
		{"limit=1", "hello", 1, "h"},
		{"utf8 multibyte within limit", "こんにちは", 5, "こんにちは"},
		{"utf8 multibyte over limit", "こんにちは", 3, "こん…"},
		{"empty string", "", 5, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, truncateInlinePreview(tc.input, tc.limit))
		})
	}
}

func TestIsConversationalTool(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		expect bool
	}{
		{"plan is conversational", "plan", true},
		{"ask_user is conversational", "ask_user", true},
		{"Plan case-insensitive", "Plan", true},
		{"ASK_USER case-insensitive", "ASK_USER", true},
		{"shell_exec not conversational", "shell_exec", false},
		{"empty not conversational", "", false},
		{"whitespace trimmed", "  plan  ", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, isConversationalTool(tc.tool))
		})
	}
}

func TestDisplayToolName(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		expect string
	}{
		{"channel mapped", "channel", "channel"},
		{"read_file mapped", "read_file", "file.read"},
		{"write_file mapped", "write_file", "file.write"},
		{"replace_in_file mapped", "replace_in_file", "file.replace"},
		{"shell_exec mapped", "shell_exec", "shell.exec"},
		{"unmapped passes through", "unknown_tool", "unknown_tool"},
		{"empty returns empty", "", ""},
		{"case-insensitive lookup", "Read_File", "file.read"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, displayToolName(tc.tool))
		})
	}
}

func TestAppendDurationSuffix(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
		duration time.Duration
		expect   string
	}{
		{"empty rendered returns empty", "", 5 * time.Second, ""},
		{"zero duration returns original", "done", 0, "done"},
		{"negative duration returns original", "done", -1 * time.Second, "done"},
		{"sub-second shows ms", "done", 500 * time.Millisecond, "done (500ms)"},
		{"<10s shows 2 decimal", "done", 3500 * time.Millisecond, "done (3.50s)"},
		{"<60s shows 1 decimal", "done", 45500 * time.Millisecond, "done (45.5s)"},
		{"<1h shows m+s", "done", 3*time.Minute + 5*time.Second, "done (3m05s)"},
		{">=1h shows h+m", "done", 2*time.Hour + 30*time.Minute, "done (2h30m)"},
		{"with newline inserts before newline", "header\nbody", 2 * time.Second, "header (2.00s)\nbody"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, appendDurationSuffix(tc.rendered, tc.duration))
		})
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		limit  int
		expect string
	}{
		{"within limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"over limit with dots", "hello world", 8, "hello..."},
		{"limit<=0 returns full", "hello", 0, "hello"},
		{"negative limit returns full", "hello", -1, "hello"},
		{"limit=3 no room for ellipsis", "hello", 3, "hel"},
		{"limit=2", "hello", 2, "he"},
		{"limit=1", "hello", 1, "h"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, truncateWithEllipsis(tc.input, tc.limit))
		})
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int
	}{
		{"empty returns 0", "", 0},
		{"single line", "hello", 1},
		{"two lines", "hello\nworld", 2},
		{"three lines", "a\nb\nc", 3},
		{"trailing newline counts", "hello\n", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, countLines(tc.input))
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		name   string
		word   string
		count  int
		expect string
	}{
		{"count=1 singular", "file", 1, "file"},
		{"count=2 plural", "file", 2, "files"},
		{"count=0 plural", "file", 0, "files"},
		{"count=100 plural", "item", 100, "items"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, pluralize(tc.word, tc.count))
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name   string
		bytes  int64
		expect string
	}{
		{"bytes under 1024", 512, "512 B"},
		{"zero bytes", 0, "0 B"},
		{"exactly 1024 is KB", 1024, "1.0 KB"},
		{"kilobytes", 2048, "2.0 KB"},
		{"megabytes", 1048576, "1.0 MB"},
		{"gigabytes", 1073741824, "1.0 GB"},
		{"fractional MB", 1572864, "1.5 MB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, formatBytes(tc.bytes))
		})
	}
}

func TestFilterSystemReminders(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			"no reminders passes through",
			"hello world\nfoo bar",
			"hello world\nfoo bar",
		},
		{
			"single-line reminder removed",
			"before\n<system-reminder>secret</system-reminder>\nafter",
			"before\nafter",
		},
		{
			"multi-line reminder removed",
			"before\n<system-reminder>\nline1\nline2\n</system-reminder>\nafter",
			"before\nafter",
		},
		{
			"mixed content",
			"start\n<system-reminder>x</system-reminder>\nmiddle\n<system-reminder>\na\nb\n</system-reminder>\nend",
			"start\nmiddle\nend",
		},
		{
			"empty string",
			"",
			"",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, filterSystemReminders(tc.input))
		})
	}
}
