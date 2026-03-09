package taskfile

import (
	"fmt"
	"strings"
)

// validate checks a TaskFile for structural correctness:
// required fields, valid dependency references, self-dependencies, and cycles.
func validate(tf *TaskFile) error {
	if tf == nil {
		return fmt.Errorf("taskfile is nil")
	}
	if tf.Version == "" {
		return fmt.Errorf("version is required")
	}
	if len(tf.Tasks) == 0 {
		return fmt.Errorf("at least one task is required")
	}

	ids := make(map[string]struct{}, len(tf.Tasks))
	for i, t := range tf.Tasks {
		if t.ID == "" {
			return fmt.Errorf("tasks[%d]: id is required", i)
		}
		if t.Prompt == "" {
			return fmt.Errorf("tasks[%d] (%s): prompt is required", i, t.ID)
		}
		if _, dup := ids[t.ID]; dup {
			return fmt.Errorf("tasks[%d]: duplicate id %q", i, t.ID)
		}
		ids[t.ID] = struct{}{}
	}

	for _, t := range tf.Tasks {
		for _, dep := range t.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == t.ID {
				return fmt.Errorf("task %q depends on itself", t.ID)
			}
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("task %q depends on unknown task %q", t.ID, dep)
			}
		}
	}

	if _, err := topologicalOrder(tf.Tasks); err != nil {
		return err
	}

	return nil
}
