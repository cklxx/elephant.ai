commit d40ba894dd2c833380be09ccf0e92c6582a4475b
Author: cklxx <q1293822641@gmail.com>
Date:   Thu Mar 5 11:49:14 2026 +0800

    refactor(prompt): guide llm to read workspace memory files directly

diff --git a/internal/app/agent/preparation/default_prompt_test.go b/internal/app/agent/preparation/default_prompt_test.go
index a4037810..6cc3f92b 100644
--- a/internal/app/agent/preparation/default_prompt_test.go
+++ b/internal/app/agent/preparation/default_prompt_test.go
@@ -11,7 +11,7 @@ func TestDefaultSystemPromptIncludesRoutingBoundaries(t *testing.T) {
 	for _, snippet := range []string{
 		"do not use clarify for explicit operational asks",
 		"exhaust safe deterministic attempts before asking questions",
-		"inspect injected memory snapshot and thread context first",
+		"inspect workspace memory files and thread context first",
 		"ask one minimal blocking question only then",
 		"search/install suitable skills or tools from trusted sources",
 		"explicit approval/consent/manual gates",
diff --git a/internal/app/agent/preparation/service.go b/internal/app/agent/preparation/service.go
index 8997c50e..b83205dd 100644
--- a/internal/app/agent/preparation/service.go
+++ b/internal/app/agent/preparation/service.go
@@ -34,7 +34,7 @@ const (
 
 Output quality (priority: Clear > Coherent > Concise > Concrete): Lead with result first, key evidence second, supporting detail only on demand. Avoid emojis in responses unless the user explicitly requests them.
 
-Execution: Always execute first and exhaust safe deterministic attempts before asking questions. If intent is unclear, inspect injected memory snapshot and thread context first (then local chat context snapshots when available). For explicit low-risk read-only inspection asks (view/check/list/inspect project state, files, branch, workspace), execute directly with read/list/shell tools and report findings; do not ask for reconfirmation. Use clarify(needs_user_input=true) only when requirements are missing/contradictory after all viable attempts fail; ask one minimal blocking question only then, and do not use clarify for explicit operational asks. For explicit approval/consent/manual gates (login, 2FA, CAPTCHA, external confirmation), call request_user with clear steps and wait. Treat explicit user delegation signals ("you decide", "anything works", "use your judgment") as authorization for low-risk reversible actions: choose a sensible default, execute, and report instead of asking again.
+Execution: Always execute first and exhaust safe deterministic attempts before asking questions. If intent is unclear, inspect workspace memory files and thread context first (use read_file for memory files, then local chat context snapshots when available). For explicit low-risk read-only inspection asks (view/check/list/inspect project state, files, branch, workspace), execute directly with read/list/shell tools and report findings; do not ask for reconfirmation. Use clarify(needs_user_input=true) only when requirements are missing/contradictory after all viable attempts fail; ask one minimal blocking question only then, and do not use clarify for explicit operational asks. For explicit approval/consent/manual gates (login, 2FA, CAPTCHA, external confirmation), call request_user with clear steps and wait. Treat explicit user delegation signals ("you decide", "anything works", "use your judgment") as authorization for low-risk reversible actions: choose a sensible default, execute, and report instead of asking again.
 
 Tools: Use web_search when no URL is fixed and source discovery is needed; use web_fetch after a URL is chosen. Avoid assuming interactive browser automation capabilities unless matching browser tools are explicitly present in the runtime tool list. When capability is missing, proactively search/install suitable skills or tools from trusted sources before escalating. For Lark/Feishu operations, run local skill CLIs via shell_exec (for example python3 skills/feishu-cli/run.py); do not assume a dedicated channel tool exists. Use /tmp as the default location for temporary/generated files unless the user specifies another path. Use artifacts_list for inventory and artifacts_write for creating/updating durable outputs. Use write_attachment only to materialize an existing attachment into a downloadable file path.`
 )
@@ -486,7 +486,7 @@ func (s *ExecutionPreparationService) Prepare(ctx context.Context, task string,
 - When producing long-form deliverables (reports, articles, specs), write them to a Markdown file via write_file.
 - Use /tmp as the default location for temporary/generated files unless the user requests another path.
 - Always execute first. Exhaust all safe deterministic attempts before asking follow-up questions.
-- If intent is unclear, inspect injected memory snapshot and thread context first (then local chat context snapshots when available).
+- If intent is unclear, inspect workspace memory files and thread context first (use read_file for memory files, then local chat context snapshots when available).
 - Ask only after all viable attempts fail and missing critical input still blocks progress.
 - In Lark chats, use shell_exec + skill CLIs (for example skills/feishu-cli/run.py) for both text updates and file delivery.
 - Provide a short summary in the final answer and point the user to the generated file path instead of pasting the full content.`)
diff --git a/internal/app/context/manager_memory.go b/internal/app/context/manager_memory.go
index e58f9aca..159abc87 100644
--- a/internal/app/context/manager_memory.go
+++ b/internal/app/context/manager_memory.go
@@ -221,7 +221,7 @@ func buildKernelDailyLogPromptChunk(now time.Time, today, yesterday string) stri
 	if len(lines) == 0 {
 		return ""
 	}
-	lines = append(lines, "Use injected memory snapshot sections for full details.")
+	lines = append(lines, "Use read_file to open workspace memory files for full details.")
 	return fmt.Sprintf("## Daily Log Digest (Kernel only)\n%s", strings.Join(lines, "\n"))
 }
 
@@ -231,7 +231,7 @@ func summarizeKernelDailyLog(content string) string {
 		return "daily memory entry available"
 	}
 	if containsNonASCII(snippet) {
-		return "non-English daily memory available in injected snapshot."
+		return "non-English daily memory available in workspace memory files."
 	}
 	return snippet
 }
diff --git a/internal/app/context/manager_memory_test.go b/internal/app/context/manager_memory_test.go
index bf5d1bde..263ffe41 100644
--- a/internal/app/context/manager_memory_test.go
+++ b/internal/app/context/manager_memory_test.go
@@ -127,7 +127,7 @@ func TestLoadMemorySnapshotBootstrapsSoulAndUserFiles(t *testing.T) {
 func TestKernelDailyDigestMasksNonEnglishRawContent(t *testing.T) {
 	now := time.Date(2026, time.February, 24, 9, 0, 0, 0, time.UTC)
 	digest := buildKernelDailyLogPromptChunk(now, "你是 elephant.ai 的 kernel 自主代理", "yesterday note")
-	if !strings.Contains(digest, "non-English daily memory available in injected snapshot.") {
+	if !strings.Contains(digest, "non-English daily memory available in workspace memory files.") {
 		t.Fatalf("expected non-English daily content to be masked into English summary, got: %s", digest)
 	}
 }
