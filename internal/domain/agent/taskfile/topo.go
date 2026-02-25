package taskfile

import "fmt"

// TopologicalOrder returns task IDs in dependency-respecting execution order
// using Kahn's algorithm. Returns an error if a cycle is detected.
func TopologicalOrder(tasks []TaskSpec) ([]string, error) {
	inDegree := make(map[string]int, len(tasks))
	adj := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		inDegree[t.ID] = len(t.DependsOn)
		for _, dep := range t.DependsOn {
			adj[dep] = append(adj[dep], t.ID)
		}
	}

	queue := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if inDegree[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}

	order := make([]string, 0, len(tasks))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, next := range adj[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(tasks) {
		return nil, fmt.Errorf("dependency cycle detected in task file")
	}
	return order, nil
}
