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
		metaTags := make([]string, 0, 3)
		if level := strings.TrimSpace(skill.GovernanceLevel); level != "" {
			metaTags = append(metaTags, "gov:"+level)
		}
		if mode := strings.TrimSpace(skill.ActivationMode); mode != "" {
			metaTags = append(metaTags, "mode:"+mode)
		}
		if len(skill.Capabilities) > 0 {
			metaTags = append(metaTags, "cap:"+strings.Join(skill.Capabilities, ","))
		}
		suffix := ""
		if len(metaTags) > 0 {
			suffix = " [" + strings.Join(metaTags, " | ") + "]"
		}
		if skill.HasRunScript {
			builder.WriteString(fmt.Sprintf("- `%s` [py]%s — %s\n", skill.Name, suffix, desc))
		} else {
			builder.WriteString(fmt.Sprintf("- `%s`%s — %s\n", skill.Name, suffix, desc))
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
		if level := strings.TrimSpace(skill.GovernanceLevel); level != "" {
			builder.WriteString(fmt.Sprintf("    <governance_level>%s</governance_level>\n", escapeXML(level)))
		}
		if mode := strings.TrimSpace(skill.ActivationMode); mode != "" {
			builder.WriteString(fmt.Sprintf("    <activation_mode>%s</activation_mode>\n", escapeXML(mode)))
		}
		if len(skill.Capabilities) > 0 {
			builder.WriteString("    <capabilities>\n")
			for _, capability := range skill.Capabilities {
				trimmed := strings.TrimSpace(capability)
				if trimmed == "" {
					continue
				}
				builder.WriteString(fmt.Sprintf("      <capability>%s</capability>\n", escapeXML(trimmed)))
			}
			builder.WriteString("    </capabilities>\n")
		}
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
