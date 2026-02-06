package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- helpers ---

func writeSkillFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill file %s: %v", filename, err)
	}
}

func validSkillContent(name, description string) string {
	return "---\nname: " + name + "\ndescription: " + description + "\n---\n# " + name + "\n\nBody text.\n"
}

// --- LoadCustomSkills ---

func TestLoadCustomSkills_ValidFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "greeting"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "farewell"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(dir, "greeting"), "SKILL.md", validSkillContent("greeting", "A greeting skill"))
	writeSkillFile(t, filepath.Join(dir, "farewell"), "SKILL.md", validSkillContent("farewell", "A farewell skill"))

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) != 0 {
		t.Fatalf("expected no validation errors, got %v", verrs)
	}
	skills := lib.List()
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0].Name != "farewell" {
		t.Errorf("expected first skill 'farewell', got %q", skills[0].Name)
	}
	if skills[1].Name != "greeting" {
		t.Errorf("expected second skill 'greeting', got %q", skills[1].Name)
	}
}

func TestLoadCustomSkills_SkipsInvalidSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "good-skill"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "bad-skill"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	// Valid skill.
	writeSkillFile(t, filepath.Join(dir, "good-skill"), "SKILL.md", validSkillContent("good-skill", "A good skill"))
	// Invalid: no name.
	writeSkillFile(t, filepath.Join(dir, "bad-skill"), "SKILL.md", "---\ndescription: missing name\n---\n# Bad\n\nBody.\n")

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) == 0 {
		t.Fatal("expected validation errors for skill without name")
	}
	skills := lib.List()
	if len(skills) != 1 {
		t.Fatalf("expected 1 valid skill, got %d", len(skills))
	}
	if skills[0].Name != "good-skill" {
		t.Errorf("expected 'good-skill', got %q", skills[0].Name)
	}
}

func TestLoadCustomSkills_EmptyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) != 0 {
		t.Fatalf("expected no validation errors for empty dir, got %v", verrs)
	}
	if len(lib.List()) != 0 {
		t.Fatalf("expected empty library, got %d skills", len(lib.List()))
	}
}

func TestLoadCustomSkills_NonExistentDirectory(t *testing.T) {
	t.Parallel()

	config := CustomSkillConfig{UserDir: "/tmp/nonexistent-skill-dir-abc123xyz"}
	_, verrs := LoadCustomSkills(config)

	if len(verrs) == 0 {
		t.Fatal("expected validation error for non-existent directory")
	}
	found := false
	for _, ve := range verrs {
		if ve.Field == "UserDir" && strings.Contains(ve.Message, "cannot access") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected UserDir error, got %v", verrs)
	}
}

func TestLoadCustomSkills_EmptyUserDir(t *testing.T) {
	t.Parallel()

	config := CustomSkillConfig{UserDir: ""}
	_, verrs := LoadCustomSkills(config)

	if len(verrs) == 0 {
		t.Fatal("expected validation error for empty UserDir")
	}
	if verrs[0].Field != "UserDir" {
		t.Fatalf("expected UserDir field error, got %q", verrs[0].Field)
	}
}

func TestLoadCustomSkills_FileSizeLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "big-skill"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	// Create a skill file that exceeds the size limit.
	content := "---\nname: big-skill\ndescription: Too big.\n---\n# Big\n\n" + strings.Repeat("x", 200)
	writeSkillFile(t, filepath.Join(dir, "big-skill"), "SKILL.md", content)

	config := CustomSkillConfig{
		UserDir:      dir,
		MaxSkillSize: 100, // 100 bytes — the file will exceed this.
	}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) == 0 {
		t.Fatal("expected validation error for oversized file")
	}
	foundSizeErr := false
	for _, ve := range verrs {
		if ve.Field == "file_size" {
			foundSizeErr = true
		}
	}
	if !foundSizeErr {
		t.Fatalf("expected file_size error, got %v", verrs)
	}
	if len(lib.List()) != 0 {
		t.Fatalf("expected no skills loaded, got %d", len(lib.List()))
	}
}

func TestLoadCustomSkills_NonRecursive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a subdirectory with a skill file — should be ignored.
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(subDir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(subDir, "nested"), "SKILL.md", validSkillContent("nested", "Nested skill"))

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) != 0 {
		t.Fatalf("expected no validation errors, got %v", verrs)
	}
	if len(lib.List()) != 0 {
		t.Fatalf("expected 0 skills (non-recursive), got %d", len(lib.List()))
	}
}

func TestLoadCustomSkills_DuplicateNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dupe-a"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "dupe-b"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	writeSkillFile(t, filepath.Join(dir, "dupe-a"), "SKILL.md", validSkillContent("dupe", "First dupe"))
	writeSkillFile(t, filepath.Join(dir, "dupe-b"), "SKILL.md", validSkillContent("dupe", "Second dupe"))

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	// One should load, one should produce a duplicate error.
	if len(lib.List()) != 1 {
		t.Fatalf("expected 1 skill (first wins), got %d", len(lib.List()))
	}
	foundDupeErr := false
	for _, ve := range verrs {
		if ve.Field == "name" && strings.Contains(ve.Message, "duplicate") {
			foundDupeErr = true
		}
	}
	if !foundDupeErr {
		t.Fatalf("expected duplicate name error, got %v", verrs)
	}
}

func TestLoadCustomSkills_SanitizesBody(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sanitize-test"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := "---\nname: sanitize-test\ndescription: Test sanitization.\n---\n# Test\n\n<script>alert('xss')</script>\n\nSafe content.\n"
	writeSkillFile(t, filepath.Join(dir, "sanitize-test"), "SKILL.md", content)

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) != 0 {
		t.Fatalf("expected no validation errors, got %v", verrs)
	}
	skill, ok := lib.Get("sanitize-test")
	if !ok {
		t.Fatal("expected skill to be loaded")
	}
	if strings.Contains(skill.Body, "<script") {
		t.Fatal("expected script tags to be stripped from body")
	}
	if !strings.Contains(skill.Body, "Safe content") {
		t.Fatal("expected safe content to be preserved")
	}
}

func TestLoadCustomSkills_AllowedTriggers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "trigger-test"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
name: trigger-test
description: Test trigger whitelist.
triggers:
  intent_patterns:
    - "hello"
  tool_signals:
    - web_search
---
# Trigger Test

Body.
`
	writeSkillFile(t, filepath.Join(dir, "trigger-test"), "SKILL.md", content)

	// Only allow context_signals — intent_patterns and tool_signals should fail.
	config := CustomSkillConfig{
		UserDir:         dir,
		AllowedTriggers: []string{"context_signals"},
	}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) == 0 {
		t.Fatal("expected validation errors for disallowed triggers")
	}
	if len(lib.List()) != 0 {
		t.Fatalf("expected no skills loaded due to trigger errors, got %d", len(lib.List()))
	}

	triggerErrCount := 0
	for _, ve := range verrs {
		if ve.Field == "triggers" {
			triggerErrCount++
		}
	}
	if triggerErrCount != 2 {
		t.Fatalf("expected 2 trigger errors (intent_patterns + tool_signals), got %d", triggerErrCount)
	}
}

// --- ValidateSkill ---

func TestValidateSkill_NameRequired(t *testing.T) {
	t.Parallel()
	skill := Skill{Description: "test"}
	errs := ValidateSkill(skill, CustomSkillConfig{})
	if len(errs) == 0 {
		t.Fatal("expected error for missing name")
	}
	found := false
	for _, e := range errs {
		if e.Field == "name" && strings.Contains(e.Message, "required") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected name required error, got %v", errs)
	}
}

func TestValidateSkill_NameAlphanumericHyphens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"also_valid", false},
		{"CamelCase", false},
		{"name123", false},
		{"a", false},
		{"-starts-with-hyphen", true},
		{"has spaces", true},
		{"special!chars", true},
		{"dot.name", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skill := Skill{Name: tt.name, Description: "test", Priority: 5, MaxTokens: 2000}
			errs := ValidateSkill(skill, CustomSkillConfig{})
			hasNameErr := false
			for _, e := range errs {
				if e.Field == "name" {
					hasNameErr = true
				}
			}
			if tt.wantErr && !hasNameErr {
				t.Errorf("expected name error for %q, got none", tt.name)
			}
			if !tt.wantErr && hasNameErr {
				t.Errorf("unexpected name error for %q: %v", tt.name, errs)
			}
		})
	}
}

func TestValidateSkill_NameMaxLength(t *testing.T) {
	t.Parallel()
	longName := strings.Repeat("a", 65)
	skill := Skill{Name: longName, Description: "test", Priority: 5, MaxTokens: 2000}
	errs := ValidateSkill(skill, CustomSkillConfig{})
	found := false
	for _, e := range errs {
		if e.Field == "name" && strings.Contains(e.Message, "exceeds maximum 64") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected name length error, got %v", errs)
	}
}

func TestValidateSkill_DescriptionRequired(t *testing.T) {
	t.Parallel()
	skill := Skill{Name: "test", Priority: 5, MaxTokens: 2000}
	errs := ValidateSkill(skill, CustomSkillConfig{})
	found := false
	for _, e := range errs {
		if e.Field == "description" && strings.Contains(e.Message, "required") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected description required error, got %v", errs)
	}
}

func TestValidateSkill_DescriptionMaxLength(t *testing.T) {
	t.Parallel()
	skill := Skill{Name: "test", Description: strings.Repeat("d", 501), Priority: 5, MaxTokens: 2000}
	errs := ValidateSkill(skill, CustomSkillConfig{})
	found := false
	for _, e := range errs {
		if e.Field == "description" && strings.Contains(e.Message, "exceeds maximum 500") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected description length error, got %v", errs)
	}
}

func TestValidateSkill_MaxTokensLimit(t *testing.T) {
	t.Parallel()
	skill := Skill{Name: "test", Description: "test", MaxTokens: 200000, Priority: 5}
	errs := ValidateSkill(skill, CustomSkillConfig{})
	found := false
	for _, e := range errs {
		if e.Field == "max_tokens" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected max_tokens error, got %v", errs)
	}
}

func TestValidateSkill_PriorityRange(t *testing.T) {
	t.Parallel()

	// Note: Priority is int, and parseSkillFile sets default to 5 for 0 values.
	// But ValidateSkill checks the value as-is. Priority 101 should fail.
	skill := Skill{Name: "test", Description: "test", Priority: 101, MaxTokens: 2000}
	errs := ValidateSkill(skill, CustomSkillConfig{})
	found := false
	for _, e := range errs {
		if e.Field == "priority" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected priority error for 101, got %v", errs)
	}
}

func TestValidateSkill_ValidSkillNoErrors(t *testing.T) {
	t.Parallel()
	skill := Skill{
		Name:        "my-skill",
		Description: "A perfectly valid skill.",
		Priority:    10,
		MaxTokens:   5000,
	}
	errs := ValidateSkill(skill, CustomSkillConfig{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid skill, got %v", errs)
	}
}

// --- MergeLibraries ---

func TestMergeLibraries_NoOverride(t *testing.T) {
	t.Parallel()

	builtinDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(builtinDir, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(builtinDir, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(builtinDir, "alpha"), "SKILL.md", validSkillContent("alpha", "Built-in alpha"))
	writeSkillFile(t, filepath.Join(builtinDir, "beta"), "SKILL.md", validSkillContent("beta", "Built-in beta"))
	builtin, err := Load(builtinDir)
	if err != nil {
		t.Fatalf("load builtin: %v", err)
	}

	customDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(customDir, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(customDir, "gamma"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(customDir, "beta"), "SKILL.md", validSkillContent("beta", "Custom beta"))
	writeSkillFile(t, filepath.Join(customDir, "gamma"), "SKILL.md", validSkillContent("gamma", "Custom gamma"))
	custom, verrs := LoadCustomSkills(CustomSkillConfig{UserDir: customDir})
	if len(verrs) != 0 {
		t.Fatalf("unexpected validation errors: %v", verrs)
	}

	merged := MergeLibraries(builtin, custom, false)
	skills := merged.List()
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills (alpha, beta-builtin, gamma), got %d", len(skills))
	}

	// beta should be the built-in version.
	beta, ok := merged.Get("beta")
	if !ok {
		t.Fatal("expected beta to be present")
	}
	if beta.Description != "Built-in beta" {
		t.Fatalf("expected built-in beta description, got %q", beta.Description)
	}
}

func TestMergeLibraries_WithOverride(t *testing.T) {
	t.Parallel()

	builtinDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(builtinDir, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(builtinDir, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(builtinDir, "alpha"), "SKILL.md", validSkillContent("alpha", "Built-in alpha"))
	writeSkillFile(t, filepath.Join(builtinDir, "beta"), "SKILL.md", validSkillContent("beta", "Built-in beta"))
	builtin, err := Load(builtinDir)
	if err != nil {
		t.Fatalf("load builtin: %v", err)
	}

	customDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(customDir, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(customDir, "gamma"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(customDir, "beta"), "SKILL.md", validSkillContent("beta", "Custom beta override"))
	writeSkillFile(t, filepath.Join(customDir, "gamma"), "SKILL.md", validSkillContent("gamma", "Custom gamma"))
	custom, verrs := LoadCustomSkills(CustomSkillConfig{UserDir: customDir})
	if len(verrs) != 0 {
		t.Fatalf("unexpected validation errors: %v", verrs)
	}

	merged := MergeLibraries(builtin, custom, true)
	skills := merged.List()
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills (alpha, beta-custom, gamma), got %d", len(skills))
	}

	// beta should be the custom version.
	beta, ok := merged.Get("beta")
	if !ok {
		t.Fatal("expected beta to be present")
	}
	if beta.Description != "Custom beta override" {
		t.Fatalf("expected custom beta description, got %q", beta.Description)
	}
}

func TestMergeLibraries_EmptyLibraries(t *testing.T) {
	t.Parallel()

	merged := MergeLibraries(Library{}, Library{}, false)
	if len(merged.List()) != 0 {
		t.Fatalf("expected empty merged library, got %d skills", len(merged.List()))
	}
}

func TestMergeLibraries_Sorted(t *testing.T) {
	t.Parallel()

	builtinDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(builtinDir, "charlie"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(builtinDir, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(builtinDir, "charlie"), "SKILL.md", validSkillContent("charlie", "C"))
	writeSkillFile(t, filepath.Join(builtinDir, "alpha"), "SKILL.md", validSkillContent("alpha", "A"))
	builtin, err := Load(builtinDir)
	if err != nil {
		t.Fatalf("load builtin: %v", err)
	}

	customDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(customDir, "bravo"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(customDir, "bravo"), "SKILL.md", validSkillContent("bravo", "B"))
	custom, verrs := LoadCustomSkills(CustomSkillConfig{UserDir: customDir})
	if len(verrs) != 0 {
		t.Fatalf("unexpected validation errors: %v", verrs)
	}

	merged := MergeLibraries(builtin, custom, false)
	skills := merged.List()
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	expected := []string{"alpha", "bravo", "charlie"}
	for i, n := range names {
		if n != expected[i] {
			t.Fatalf("expected sorted order %v, got %v", expected, names)
		}
	}
}

// --- SanitizeSkillBody ---

func TestSanitizeSkillBody_RemovesScriptTags(t *testing.T) {
	t.Parallel()

	input := "Hello\n<script>alert('xss')</script>\nWorld"
	result := SanitizeSkillBody(input)
	if strings.Contains(result, "<script") {
		t.Fatalf("expected script tags removed, got %q", result)
	}
	if !strings.Contains(result, "Hello") || !strings.Contains(result, "World") {
		t.Fatalf("expected safe content preserved, got %q", result)
	}
}

func TestSanitizeSkillBody_RemovesScriptTagsCaseInsensitive(t *testing.T) {
	t.Parallel()

	input := "Before\n<SCRIPT type='text/javascript'>evil()</SCRIPT>\nAfter"
	result := SanitizeSkillBody(input)
	if strings.Contains(strings.ToLower(result), "script") {
		t.Fatalf("expected script tags removed (case-insensitive), got %q", result)
	}
}

func TestSanitizeSkillBody_RemovesTemplateInjection(t *testing.T) {
	t.Parallel()

	input := "Hello {{ .Secret }} World"
	result := SanitizeSkillBody(input)
	if strings.Contains(result, "{{") {
		t.Fatalf("expected template injection removed, got %q", result)
	}
	if !strings.Contains(result, "Hello") || !strings.Contains(result, "World") {
		t.Fatalf("expected safe content preserved, got %q", result)
	}
}

func TestSanitizeSkillBody_TrimsExcessiveWhitespace(t *testing.T) {
	t.Parallel()

	input := "Line 1\n\n\n\n\nLine 2"
	result := SanitizeSkillBody(input)
	if strings.Contains(result, "\n\n\n") {
		t.Fatalf("expected excessive newlines collapsed, got %q", result)
	}
	if !strings.Contains(result, "Line 1\n\nLine 2") {
		t.Fatalf("expected two-newline gap, got %q", result)
	}
}

func TestSanitizeSkillBody_EmptyInput(t *testing.T) {
	t.Parallel()

	result := SanitizeSkillBody("")
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestSanitizeSkillBody_NoChangesNeeded(t *testing.T) {
	t.Parallel()

	input := "# Title\n\nSafe Markdown content.\n\n- Item 1\n- Item 2"
	result := SanitizeSkillBody(input)
	if result != input {
		t.Fatalf("expected no changes, got %q", result)
	}
}

func TestSanitizeSkillBody_MultilineScript(t *testing.T) {
	t.Parallel()

	input := "Before\n<script>\nvar x = 1;\nalert(x);\n</script>\nAfter"
	result := SanitizeSkillBody(input)
	if strings.Contains(result, "<script") || strings.Contains(result, "alert") {
		t.Fatalf("expected multiline script removed, got %q", result)
	}
	if !strings.Contains(result, "Before") || !strings.Contains(result, "After") {
		t.Fatalf("expected surrounding content preserved, got %q", result)
	}
}

// --- ValidationError ---

func TestValidationError_ErrorString(t *testing.T) {
	t.Parallel()

	ve := ValidationError{SkillName: "my-skill", Field: "name", Message: "too long"}
	s := ve.Error()
	if !strings.Contains(s, "my-skill") {
		t.Fatalf("expected skill name in error, got %q", s)
	}
	if !strings.Contains(s, "name") {
		t.Fatalf("expected field in error, got %q", s)
	}
	if !strings.Contains(s, "too long") {
		t.Fatalf("expected message in error, got %q", s)
	}
}

func TestValidationError_ErrorStringNoSkillName(t *testing.T) {
	t.Parallel()

	ve := ValidationError{Field: "UserDir", Message: "empty"}
	s := ve.Error()
	if strings.Contains(s, "skill") {
		t.Fatalf("expected no skill prefix when name is empty, got %q", s)
	}
	if !strings.Contains(s, "UserDir") {
		t.Fatalf("expected field in error, got %q", s)
	}
}

// --- CustomSkillConfig ---

func TestCustomSkillConfig_EffectiveMaxSize(t *testing.T) {
	t.Parallel()

	c1 := CustomSkillConfig{}
	if c1.effectiveMaxSize() != DefaultMaxSkillSize {
		t.Fatalf("expected default max size %d, got %d", DefaultMaxSkillSize, c1.effectiveMaxSize())
	}

	c2 := CustomSkillConfig{MaxSkillSize: 500}
	if c2.effectiveMaxSize() != 500 {
		t.Fatalf("expected 500, got %d", c2.effectiveMaxSize())
	}
}

// --- Integration: Load skill with triggers and full validation ---

func TestLoadCustomSkills_WithTriggers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "search-skill"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
name: search-skill
description: A search skill with triggers.
triggers:
  intent_patterns:
    - "search|find"
  tool_signals:
    - web_search
  context_signals:
    keywords: ["search", "find"]
  confidence_threshold: 0.5
priority: 8
max_tokens: 3000
---
# Search Skill

Search body.
`
	writeSkillFile(t, filepath.Join(dir, "search-skill"), "SKILL.md", content)

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) != 0 {
		t.Fatalf("expected no validation errors, got %v", verrs)
	}

	skill, ok := lib.Get("search-skill")
	if !ok {
		t.Fatal("expected search-skill to be loaded")
	}
	if skill.Triggers == nil {
		t.Fatal("expected triggers to be populated")
	}
	if len(skill.Triggers.IntentPatterns) != 1 {
		t.Fatalf("expected 1 intent pattern, got %d", len(skill.Triggers.IntentPatterns))
	}
	if skill.Priority != 8 {
		t.Fatalf("expected priority 8, got %d", skill.Priority)
	}
	if skill.MaxTokens != 3000 {
		t.Fatalf("expected max_tokens 3000, got %d", skill.MaxTokens)
	}
}

func TestLoadCustomSkills_IgnoresNonMarkdownFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real-skill"), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	writeSkillFile(t, filepath.Join(dir, "real-skill"), "SKILL.md", validSkillContent("real-skill", "A real skill"))
	writeSkillFile(t, dir, "notes.txt", "not a skill")
	writeSkillFile(t, dir, "data.json", `{"key": "value"}`)

	config := CustomSkillConfig{UserDir: dir}
	lib, verrs := LoadCustomSkills(config)

	if len(verrs) != 0 {
		t.Fatalf("expected no errors, got %v", verrs)
	}
	if len(lib.List()) != 1 {
		t.Fatalf("expected 1 skill (skill dir only), got %d", len(lib.List()))
	}
}

func TestLoadCustomSkills_PathIsNotDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir.md")
	writeSkillFile(t, dir, "not-a-dir.md", "content")

	config := CustomSkillConfig{UserDir: filePath}
	_, verrs := LoadCustomSkills(config)

	if len(verrs) == 0 {
		t.Fatal("expected validation error for non-directory path")
	}
	if verrs[0].Field != "UserDir" || !strings.Contains(verrs[0].Message, "not a directory") {
		t.Fatalf("expected 'not a directory' error, got %v", verrs)
	}
}
