package builtin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/skills"
)

type skillsTool struct{}

func NewSkills() tools.ToolExecutor {
	return &skillsTool{}
}

func (t *skillsTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "skills",
		Version:  "1.0.0",
		Category: "meta",
		Tags:     []string{"skills", "playbook", "workflow", "guidance"},
	}
}

func (t *skillsTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "skills",
		Description: `Query reusable skill playbooks (Markdown guides).

Use this to list available skills, search by keyword, or show a specific skill by name.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"action": {
					Type:        "string",
					Description: "list|show|search",
					Enum:        []any{"list", "show", "search"},
				},
				"name": {
					Type:        "string",
					Description: "Skill name for action=show.",
				},
				"query": {
					Type:        "string",
					Description: "Search query for action=search.",
				},
			},
			Required: []string{"action"},
		},
	}
}

func (t *skillsTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action, _ := call.Arguments["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))

	library, err := skills.DefaultLibrary()
	if err != nil {
		wrapped := fmt.Errorf("load skills: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	switch action {
	case "list":
		index := skills.IndexMarkdown(library)
		if index == "" {
			return &ports.ToolResult{CallID: call.ID, Content: "No skills found."}, nil
		}
		return &ports.ToolResult{
			CallID:   call.ID,
			Content:  index,
			Metadata: map[string]any{"skills_dir": library.Root(), "count": len(library.List())},
		}, nil

	case "show":
		name, _ := call.Arguments["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			err := errors.New("name is required for action=show")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		skill, ok := library.Get(name)
		if !ok {
			candidates := similarSkillNames(library, name, 8)
			var builder strings.Builder
			builder.WriteString(fmt.Sprintf("Skill not found: %q\n\n", name))
			builder.WriteString("Use `skills` with action=list to see available skills.\n")
			if len(candidates) > 0 {
				builder.WriteString("\nDid you mean:\n")
				for _, c := range candidates {
					builder.WriteString(fmt.Sprintf("- `%s`\n", c))
				}
			}
			content := strings.TrimSpace(builder.String())
			err := fmt.Errorf("skill not found: %s", name)
			return &ports.ToolResult{CallID: call.ID, Content: content, Error: err}, nil
		}

		return &ports.ToolResult{
			CallID:  call.ID,
			Content: skill.Body,
			Metadata: map[string]any{
				"name":        skill.Name,
				"title":       skill.Title,
				"description": skill.Description,
				"source_path": skill.SourcePath,
			},
		}, nil

	case "search":
		query, _ := call.Arguments["query"].(string)
		query = strings.TrimSpace(query)
		if query == "" {
			err := errors.New("query is required for action=search")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		matches := searchSkills(library, query, 10)
		if len(matches) == 0 {
			return &ports.ToolResult{
				CallID:  call.ID,
				Content: fmt.Sprintf("No skills matched %q.", query),
				Metadata: map[string]any{
					"query": query,
					"count": 0,
				},
			}, nil
		}

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("Matches for %q:\n\n", query))
		for _, match := range matches {
			builder.WriteString(fmt.Sprintf("- `%s` — %s\n", match.Name, match.Description))
		}
		builder.WriteString("\nUse `skills` with action=show and the name to view a full playbook.")

		return &ports.ToolResult{
			CallID:  call.ID,
			Content: strings.TrimSpace(builder.String()),
			Metadata: map[string]any{
				"query": query,
				"count": len(matches),
				"names": func() []string {
					out := make([]string, 0, len(matches))
					for _, match := range matches {
						out = append(out, match.Name)
					}
					return out
				}(),
			},
		}, nil

	default:
		err := fmt.Errorf("unsupported action %q (expected list|show|search)", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func searchSkills(library skills.Library, query string, limit int) []skills.Skill {
	if limit <= 0 {
		limit = 10
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	type scored struct {
		skill skills.Skill
		score int
	}

	var matches []scored
	for _, skill := range library.List() {
		name := strings.ToLower(skill.Name)
		desc := strings.ToLower(skill.Description)
		body := strings.ToLower(skill.Body)

		score := 0
		if strings.Contains(name, q) {
			score += 3
		}
		if strings.Contains(desc, q) {
			score += 2
		}
		if strings.Contains(body, q) {
			score += 1
		}
		if score > 0 {
			matches = append(matches, scored{skill: skill, score: score})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].skill.Name < matches[j].skill.Name
	})

	out := make([]skills.Skill, 0, min(len(matches), limit))
	for _, match := range matches {
		out = append(out, match.skill)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func similarSkillNames(library skills.Library, input string, limit int) []string {
	if limit <= 0 {
		limit = 8
	}
	needle := strings.ToLower(strings.TrimSpace(input))
	if needle == "" {
		return nil
	}

	var candidates []string
	for _, skill := range library.List() {
		name := strings.ToLower(skill.Name)
		if strings.Contains(name, needle) || strings.Contains(needle, name) {
			candidates = append(candidates, skill.Name)
		}
	}
	sort.Strings(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}
