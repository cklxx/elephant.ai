package react

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompressGitStatusRemovesHints(t *testing.T) {
	input := `On branch main
Your branch is up to date with 'origin/main'.

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
	modified:   foo.go
	modified:   bar.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)
	new_file.go

no changes added to commit (use "git add" and/or "git commit -a")
`
	got := compressGitStatus(input)
	// Standalone hint lines like (use "git add <file>..." ...) should be removed.
	if strings.Contains(got, `(use "git add <file>..."`) {
		t.Fatalf("expected standalone hint lines removed, got:\n%s", got)
	}
	if strings.Contains(got, `(use "git restore`) {
		t.Fatalf("expected standalone hint lines removed, got:\n%s", got)
	}
	if !strings.Contains(got, "modified:   foo.go") {
		t.Fatal("expected modified files preserved")
	}
	if !strings.Contains(got, "new_file.go") {
		t.Fatal("expected untracked files preserved")
	}
}

func TestCompressGitDiffDropsContextLines(t *testing.T) {
	input := `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -10,7 +10,7 @@ func main() {
 	unchanged line 1
 	unchanged line 2
 	unchanged line 3
-	old line
+	new line
 	unchanged line 4
 	unchanged line 5
`
	got := compressGitDiff(input)
	if !strings.Contains(got, "diff --git") {
		t.Fatal("expected file header preserved")
	}
	if !strings.Contains(got, "-\told line") {
		t.Fatal("expected removed line preserved")
	}
	if !strings.Contains(got, "+\tnew line") {
		t.Fatal("expected added line preserved")
	}
	if strings.Contains(got, "unchanged line") {
		t.Fatal("expected context lines dropped")
	}
}

func TestCompressGoTestAllPass(t *testing.T) {
	input := `=== RUN   TestFoo
--- PASS: TestFoo (0.00s)
=== RUN   TestBar
--- PASS: TestBar (0.01s)
ok  	example.com/pkg	0.015s
`
	got := compressGoTestOutput(input)
	if strings.Contains(got, "=== RUN") {
		t.Fatal("expected RUN lines removed")
	}
	if strings.Contains(got, "--- PASS") {
		t.Fatal("expected PASS lines removed")
	}
	if !strings.Contains(got, "ok") {
		t.Fatal("expected summary line preserved")
	}
}

func TestCompressGoTestWithFailures(t *testing.T) {
	input := `=== RUN   TestFoo
--- PASS: TestFoo (0.00s)
=== RUN   TestBar
    bar_test.go:15: expected 1, got 2
--- FAIL: TestBar (0.01s)
FAIL	example.com/pkg	0.015s
`
	got := compressGoTestOutput(input)
	if strings.Contains(got, "=== RUN") {
		t.Fatal("expected RUN lines removed")
	}
	if strings.Contains(got, "--- PASS") {
		t.Fatal("expected PASS lines removed")
	}
	if !strings.Contains(got, "--- FAIL: TestBar") {
		t.Fatal("expected FAIL block preserved")
	}
	if !strings.Contains(got, "expected 1, got 2") {
		t.Fatal("expected error message preserved")
	}
	if !strings.Contains(got, "FAIL\texample.com/pkg") {
		t.Fatal("expected summary line preserved")
	}
}

func TestCompressGenericDeduplicatesRepeatedLines(t *testing.T) {
	lines := []string{"downloading...", "downloading...", "downloading...", "downloading...", "downloading...", "done"}
	input := strings.Join(lines, "\n")
	got := compressGenericOutput(input)
	if strings.Count(got, "downloading...") > 2 {
		t.Fatalf("expected deduplication, got:\n%s", got)
	}
	if !strings.Contains(got, "repeated") {
		t.Fatal("expected repeat count annotation")
	}
	if !strings.Contains(got, "done") {
		t.Fatal("expected non-repeated line preserved")
	}
}

func TestCompressToolOutputSkipsSmallContent(t *testing.T) {
	small := "hello world"
	got := compressToolOutput("shell", small, nil)
	if got != small {
		t.Fatalf("expected small content returned as-is, got %q", got)
	}
}

func TestCompressToolOutputFallsBackOnLargerResult(t *testing.T) {
	// Content that doesn't compress well — all unique lines, no trailing newline.
	var b strings.Builder
	for i := 0; i < 500; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "unique line %d with padding %s", i, strings.Repeat("x", 20))
	}
	content := b.String()
	got := compressToolOutput("unknown_tool", content, nil)
	// Should return original since compression doesn't help.
	if got != content {
		t.Fatal("expected original content when compression doesn't reduce size")
	}
}

func TestExtractBaseCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"git status", "git"},
		{"FOO=bar git diff", "git"},
		{"cd /tmp && git log", "git"},
		{"CGO_ENABLED=0 go test ./...", "go"},
		{`KEY="val ue" go build`, "go"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractBaseCommand(tt.cmd)
		if got != tt.want {
			t.Errorf("extractBaseCommand(%q) = %q, want %q", tt.cmd, got, tt.want)
		}
	}
}

func TestCompressShellOutputRoutesToGitStatus(t *testing.T) {
	// Large git status output.
	var b strings.Builder
	b.WriteString("On branch main\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "  (use \"git add\" to stage)\n")
		fmt.Fprintf(&b, "\tmodified: file%d.go\n", i)
	}
	content := b.String()
	meta := map[string]any{"command": "git status"}
	got := compressShellOutput(content, meta)
	if strings.Contains(got, `(use "git add"`) {
		t.Fatal("expected git hints removed by git status compressor")
	}
}

func TestCompressShellOutputRoutesToGitDiff(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "diff --git a/f%d.go b/f%d.go\n", i, i)
		fmt.Fprintf(&b, "--- a/f%d.go\n", i)
		fmt.Fprintf(&b, "+++ b/f%d.go\n", i)
		fmt.Fprintf(&b, "@@ -1,5 +1,5 @@\n")
		fmt.Fprintf(&b, " context line\n")
		fmt.Fprintf(&b, "-old\n+new\n")
		fmt.Fprintf(&b, " context line\n")
	}
	content := b.String()
	meta := map[string]any{"command": "git diff --stat"}
	got := compressShellOutput(content, meta)
	if strings.Contains(got, " context line") {
		t.Fatal("expected context lines dropped by git diff compressor")
	}
	if !strings.Contains(got, "-old") {
		t.Fatal("expected changed lines preserved")
	}
}

func TestCompressShellOutputRoutesToGoTest(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "=== RUN   Test%d\n--- PASS: Test%d (0.00s)\n", i, i)
	}
	b.WriteString("ok  \texample.com/pkg\t0.5s\n")
	content := b.String()
	meta := map[string]any{"command": "CGO_ENABLED=0 go test ./..."}
	got := compressShellOutput(content, meta)
	if strings.Contains(got, "=== RUN") {
		t.Fatal("expected RUN lines removed for all-pass")
	}
	if !strings.Contains(got, "ok") {
		t.Fatal("expected summary preserved")
	}
}
