package skills

import (
	"fmt"
	"strings"
)

// ResolveChain expands a declared skill chain into a single instruction block.
func (l *Library) ResolveChain(chain SkillChain) (string, error) {
	if l == nil {
		return "", fmt.Errorf("skills library is nil")
	}
	if len(chain.Steps) == 0 {
		return "", fmt.Errorf("empty skill chain")
	}

	var sb strings.Builder
	sb.WriteString("## Multi-Step Workflow\n\n")
	sb.WriteString("Execute the following steps in order. Each step's output feeds into the next.\n\n")

	for i, step := range chain.Steps {
		skill, ok := l.Get(step.SkillName)
		if !ok {
			return "", fmt.Errorf("chain step %d references unknown skill: %s", i+1, step.SkillName)
		}

		sb.WriteString(fmt.Sprintf("### Step %d: %s\n\n", i+1, skill.Name))
		if step.InputFrom != "" {
			sb.WriteString(fmt.Sprintf("**Input:** Use output from `%s`\n\n", step.InputFrom))
		}
		if step.OutputAs != "" {
			sb.WriteString(fmt.Sprintf("**Output:** Save as `%s` for subsequent steps\n\n", step.OutputAs))
		}
		if len(step.Params) > 0 {
			sb.WriteString("**Parameters:**\n")
			for key, value := range step.Params {
				if strings.TrimSpace(key) == "" {
					continue
				}
				sb.WriteString(fmt.Sprintf("- %s: %s\n", key, value))
			}
			sb.WriteString("\n")
		}

		if strings.TrimSpace(skill.Body) != "" {
			sb.WriteString(skill.Body)
			sb.WriteString("\n\n")
		}
		sb.WriteString("---\n\n")
	}

	return strings.TrimSpace(sb.String()), nil
}
