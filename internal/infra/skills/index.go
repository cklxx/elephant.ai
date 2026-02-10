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
		if skill.HasRunScript {
			builder.WriteString(fmt.Sprintf("- `%s` [py] — %s\n", skill.Name, desc))
		} else {
			builder.WriteString(fmt.Sprintf("- `%s` — %s\n", skill.Name, desc))
		}
	}
	return strings.TrimSpace(builder.String())
}

// AvailableSkillsXML renders skills metadata in the Agent Skills XML format.
func AvailableSkillsXML(library Library) string {
	skills := library.List()
	if len(skills) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("<available_skills>\n")
	for _, skill := range skills {
		desc := strings.TrimSpace(skill.Description)
		if desc == "" {
			desc = "(no description)"
		}
		builder.WriteString("  <skill>\n")
		builder.WriteString(fmt.Sprintf("    <name>%s</name>\n", escapeXML(skill.Name)))
		builder.WriteString(fmt.Sprintf("    <description>%s</description>\n", escapeXML(desc)))
		if skill.HasRunScript {
			builder.WriteString("    <type>python</type>\n")
			builder.WriteString(fmt.Sprintf("    <exec>python3 skills/%s/run.py '{...}'</exec>\n", escapeXML(skill.Name)))
		}
		builder.WriteString(fmt.Sprintf("    <location>%s</location>\n", escapeXML(skill.SourcePath)))
		builder.WriteString("  </skill>\n")
	}
	builder.WriteString("</available_skills>")
	return strings.TrimSpace(builder.String())
}

func escapeXML(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch r {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		case '"':
			builder.WriteString("&quot;")
		case '\'':
			builder.WriteString("&apos;")
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
