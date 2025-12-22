package skills

import (
	"context"
	"fmt"
	"strings"
)

// DefaultLibrary loads skills from the discovered default directory.
func DefaultLibrary() (Library, error) {
	baseLibrary, err := Load(LocateDefaultDir())
	if err != nil {
		return Library{}, err
	}

	combined := append([]Skill(nil), baseLibrary.List()...)
	roots := []string{baseLibrary.Root()}

	fabricLibrary, err := loadFabricLibrary(context.Background())
	if err != nil {
		return Library{}, err
	}
	if skills := fabricLibrary.List(); len(skills) > 0 {
		combined = append(combined, skills...)
		roots = append(roots, fabricLibrary.Root())
	}

	return buildLibrary(combined, strings.Join(compactStrings(roots), ","))
}

// IndexMarkdown renders a compact index for the given library (names + descriptions).
func IndexMarkdown(library Library) string {
	skills := library.List()
	if len(skills) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("# Skills Catalog\n\n")
	builder.WriteString("Reusable playbooks available to the agent. Use the `skills` tool to view full details.\n\n")
	builder.WriteString("Example: `skills({\"action\":\"show\",\"name\":\"video_production\"})`\n\n")
	builder.WriteString("Available skills:\n")
	for _, skill := range skills {
		desc := strings.TrimSpace(skill.Description)
		if desc == "" {
			desc = "(no description)"
		}
		builder.WriteString(fmt.Sprintf("- `%s` — %s\n", skill.Name, desc))
	}
	return strings.TrimSpace(builder.String())
}

func compactStrings(values []string) []string {
	var out []string
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
