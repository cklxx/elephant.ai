package skills

import (
	"fmt"
	"strings"
)

// DefaultLibrary loads skills from the discovered default directory.
func DefaultLibrary() (Library, error) {
	return Load(LocateDefaultDir())
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
	builder.WriteString("Example: `skills({\"action\":\"show\",\"name\":\"ppt-deck\"})`\n\n")
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
