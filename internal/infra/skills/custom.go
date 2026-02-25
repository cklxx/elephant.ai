package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DefaultMaxSkillSize is the default maximum file size for a custom skill (100KB).
const DefaultMaxSkillSize int64 = 100 * 1024

// namePattern validates skill names: alphanumeric, hyphens, and underscores.
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// CustomSkillConfig controls how user-defined custom skills are loaded.
type CustomSkillConfig struct {
	// UserDir is the path to the directory containing user skill folders.
	UserDir string
	// AllowOverride controls whether user skills can override built-in skill names.
	AllowOverride bool
	// MaxSkillSize is the maximum file size in bytes per skill file. Zero uses DefaultMaxSkillSize.
	MaxSkillSize int64
	// AllowedTriggers is a whitelist of trigger types the user can define.
	// Valid values: "intent_patterns", "tool_signals", "context_signals".
	// An empty slice means all trigger types are allowed.
	AllowedTriggers []string
}

// effectiveMaxSize returns MaxSkillSize if set, otherwise DefaultMaxSkillSize.
func (c CustomSkillConfig) effectiveMaxSize() int64 {
	if c.MaxSkillSize > 0 {
		return c.MaxSkillSize
	}
	return DefaultMaxSkillSize
}

// ValidationError describes a validation failure for a custom skill.
type ValidationError struct {
	SkillName string
	Field     string
	Message   string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.SkillName != "" {
		return fmt.Sprintf("skill %q field %q: %s", e.SkillName, e.Field, e.Message)
	}
	return fmt.Sprintf("field %q: %s", e.Field, e.Message)
}

// LoadCustomSkills scans UserDir for skill directories that contain SKILL.md
// (or SKILL.mdx), parses them using the same YAML frontmatter format as built-in
// skills, validates each skill, and returns the loaded library together with
// any validation errors. Invalid skills are skipped (soft errors).
func LoadCustomSkills(config CustomSkillConfig) (Library, []ValidationError) {
	dir := strings.TrimSpace(config.UserDir)
	if dir == "" {
		return Library{}, []ValidationError{{Field: "UserDir", Message: "user skill directory is empty"}}
	}

	info, err := os.Stat(dir)
	if err != nil {
		msg := fmt.Sprintf("cannot access directory: %v", err)
		return Library{}, []ValidationError{{Field: "UserDir", Message: msg}}
	}
	if !info.IsDir() {
		return Library{}, []ValidationError{{Field: "UserDir", Message: "path is not a directory"}}
	}

	// Discover .md files non-recursively for security.
	entries, err := os.ReadDir(dir)
	if err != nil {
		msg := fmt.Sprintf("cannot read directory: %v", err)
		return Library{}, []ValidationError{{Field: "UserDir", Message: msg}}
	}

	var allErrors []ValidationError
	var validSkills []Skill
	byName := make(map[string]Skill)
	maxSize := config.effectiveMaxSize()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(dir, entry.Name())
		var path string
		var info fs.FileInfo
		var statErr error
		for _, candidate := range []string{"SKILL.md", "SKILL.mdx"} {
			candidatePath := filepath.Join(skillDir, candidate)
			fi, err := os.Stat(candidatePath)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					statErr = err
				}
				continue
			}
			if fi.IsDir() {
				continue
			}
			path = candidatePath
			info = fi
			break
		}
		if path == "" {
			if statErr != nil {
				allErrors = append(allErrors, ValidationError{
					SkillName: entry.Name(),
					Field:     "file",
					Message:   fmt.Sprintf("cannot stat skill file: %v", statErr),
				})
			}
			continue
		}

		// Check file size before reading.
		if info.Size() > maxSize {
			allErrors = append(allErrors, ValidationError{
				SkillName: entry.Name(),
				Field:     "file_size",
				Message:   fmt.Sprintf("file size %d exceeds limit %d", info.Size(), maxSize),
			})
			continue
		}

		// Parse using the same logic as the built-in Load.
		skill, err := parseSkillFile(path)
		if err != nil {
			allErrors = append(allErrors, ValidationError{
				SkillName: entry.Name(),
				Field:     "parse",
				Message:   fmt.Sprintf("parse error: %v", err),
			})
			continue
		}

		// Validate.
		verrs := ValidateSkill(skill, config)
		if len(verrs) > 0 {
			allErrors = append(allErrors, verrs...)
			continue
		}

		// Sanitize body.
		skill.Body = SanitizeSkillBody(skill.Body)

		// Check for duplicates within custom skills.
		key := NormalizeName(skill.Name)
		if _, exists := byName[key]; exists {
			allErrors = append(allErrors, ValidationError{
				SkillName: skill.Name,
				Field:     "name",
				Message:   "duplicate custom skill name",
			})
			continue
		}

		byName[key] = skill
		validSkills = append(validSkills, skill)
	}

	sort.Slice(validSkills, func(i, j int) bool {
		return validSkills[i].Name < validSkills[j].Name
	})

	return Library{skills: validSkills, byName: byName, root: dir}, allErrors
}

// ValidateSkill checks a single skill against the given config constraints.
// It returns a slice of validation errors (empty if the skill is valid).
func ValidateSkill(skill Skill, config CustomSkillConfig) []ValidationError {
	var errs []ValidationError

	// Name: required, alphanumeric + hyphens + underscores, max 64 chars.
	name := strings.TrimSpace(skill.Name)
	if name == "" {
		errs = append(errs, ValidationError{
			SkillName: skill.SourcePath,
			Field:     "name",
			Message:   "name is required",
		})
	} else {
		if len(name) > 64 {
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "name",
				Message:   fmt.Sprintf("name length %d exceeds maximum 64 characters", len(name)),
			})
		}
		if !namePattern.MatchString(name) {
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "name",
				Message:   "name must be alphanumeric with hyphens or underscores, starting with alphanumeric",
			})
		}
	}

	// Description: required, max 500 chars.
	desc := strings.TrimSpace(skill.Description)
	if desc == "" {
		errs = append(errs, ValidationError{
			SkillName: name,
			Field:     "description",
			Message:   "description is required",
		})
	} else if len(desc) > 500 {
		errs = append(errs, ValidationError{
			SkillName: name,
			Field:     "description",
			Message:   fmt.Sprintf("description length %d exceeds maximum 500 characters", len(desc)),
		})
	}

	// MaxTokens: if set, must be <= 100000.
	if skill.MaxTokens > 100000 {
		errs = append(errs, ValidationError{
			SkillName: name,
			Field:     "max_tokens",
			Message:   fmt.Sprintf("max_tokens %d exceeds limit 100000", skill.MaxTokens),
		})
	}

	// Priority: must be 0-100.
	if skill.Priority < 0 || skill.Priority > 100 {
		errs = append(errs, ValidationError{
			SkillName: name,
			Field:     "priority",
			Message:   fmt.Sprintf("priority %d must be between 0 and 100", skill.Priority),
		})
	}
	if level := strings.TrimSpace(strings.ToLower(skill.GovernanceLevel)); level != "" {
		switch level {
		case "low", "medium", "high", "critical":
		default:
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "governance_level",
				Message:   fmt.Sprintf("invalid governance_level %q", skill.GovernanceLevel),
			})
		}
	}
	if mode := strings.TrimSpace(strings.ToLower(skill.ActivationMode)); mode != "" {
		switch mode {
		case "auto", "semi_auto", "manual":
		default:
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "activation_mode",
				Message:   fmt.Sprintf("invalid activation_mode %q", skill.ActivationMode),
			})
		}
	}
	for _, dep := range skill.DependsOnSkills {
		trimmed := strings.TrimSpace(dep)
		if trimmed == "" {
			continue
		}
		if !namePattern.MatchString(trimmed) {
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "depends_on_skills",
				Message:   fmt.Sprintf("invalid dependency name %q", dep),
			})
		}
	}

	// Trigger type whitelist.
	if len(config.AllowedTriggers) > 0 && skill.Triggers != nil {
		allowed := make(map[string]bool, len(config.AllowedTriggers))
		for _, t := range config.AllowedTriggers {
			allowed[t] = true
		}

		if len(skill.Triggers.IntentPatterns) > 0 && !allowed["intent_patterns"] {
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "triggers",
				Message:   "trigger type \"intent_patterns\" is not allowed",
			})
		}
		if len(skill.Triggers.ToolSignals) > 0 && !allowed["tool_signals"] {
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "triggers",
				Message:   "trigger type \"tool_signals\" is not allowed",
			})
		}
		if skill.Triggers.ContextSignals != nil && !allowed["context_signals"] {
			errs = append(errs, ValidationError{
				SkillName: name,
				Field:     "triggers",
				Message:   "trigger type \"context_signals\" is not allowed",
			})
		}
	}

	return errs
}

// MergeLibraries merges a built-in library with a custom library into a single
// library. If allowOverride is false, custom skills whose names conflict with
// built-in skills are skipped. If allowOverride is true, the custom skill
// replaces the built-in one. The result is sorted by name.
func MergeLibraries(builtin Library, custom Library, allowOverride bool) Library {
	merged := make(map[string]Skill, len(builtin.skills)+len(custom.skills))

	// Start with all built-in skills.
	for _, s := range builtin.skills {
		merged[NormalizeName(s.Name)] = s
	}

	// Merge custom skills.
	for _, s := range custom.skills {
		key := NormalizeName(s.Name)
		if _, exists := merged[key]; exists && !allowOverride {
			continue
		}
		merged[key] = s
	}

	skills := make([]Skill, 0, len(merged))
	byName := make(map[string]Skill, len(merged))
	for key, s := range merged {
		skills = append(skills, s)
		byName[key] = s
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	// Use the built-in root if available, otherwise custom root.
	root := builtin.root
	if root == "" {
		root = custom.root
	}

	return Library{skills: skills, byName: byName, root: root}
}

// scriptTagPattern matches <script> ... </script> blocks (case-insensitive).
var scriptTagPattern = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

// templateInjectionPattern matches {{ ... }} template injection patterns.
var templateInjectionPattern = regexp.MustCompile(`\{\{.*?\}\}`)

// excessiveWhitespacePattern matches 3+ consecutive newlines.
var excessiveWhitespacePattern = regexp.MustCompile(`\n{3,}`)

// SanitizeSkillBody removes potentially dangerous content from a skill body.
// It strips <script> tags, {{ template injection patterns, and trims excessive
// whitespace.
func SanitizeSkillBody(body string) string {
	// Strip <script> tags and their contents.
	body = scriptTagPattern.ReplaceAllString(body, "")

	// Strip {{ ... }} template injection patterns.
	body = templateInjectionPattern.ReplaceAllString(body, "")

	// Collapse excessive newlines (3+ consecutive) to 2.
	body = excessiveWhitespacePattern.ReplaceAllString(body, "\n\n")

	return strings.TrimSpace(body)
}
