package taskfile

import "fmt"

type topoGraph struct {
	inDegree map[string]int
	adj      map[string][]string
}

func buildTopoGraph(tasks []TaskSpec) topoGraph {
	graph := topoGraph{
		inDegree: make(map[string]int, len(tasks)),
		adj:      make(map[string][]string, len(tasks)),
	}
	for _, t := range tasks {
		graph.inDegree[t.ID] = len(t.DependsOn)
		for _, dep := range t.DependsOn {
			graph.adj[dep] = append(graph.adj[dep], t.ID)
		}
	}
	return graph
}

// topologicalOrder returns task IDs in dependency-respecting execution order
// using Kahn's algorithm. Returns an error if a cycle is detected.
func topologicalOrder(tasks []TaskSpec) ([]string, error) {
	graph := buildTopoGraph(tasks)

	queue := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if graph.inDegree[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}

	order := make([]string, 0, len(tasks))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, next := range graph.adj[current] {
			graph.inDegree[next]--
			if graph.inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(tasks) {
		return nil, fmt.Errorf("dependency cycle detected in task file")
	}
	return order, nil
}

// topologicalLayers groups tasks into dependency layers using Kahn's algorithm.
// Layer 0 contains tasks with no dependencies, layer N contains tasks whose
// dependencies all resolve within layers 0..N-1. Returns an error if a cycle
// is detected.
func topologicalLayers(tasks []TaskSpec) ([][]string, error) {
	graph := buildTopoGraph(tasks)

	var current []string
	for _, t := range tasks {
		if graph.inDegree[t.ID] == 0 {
			current = append(current, t.ID)
		}
	}

	var layers [][]string
	processed := 0
	for len(current) > 0 {
		layers = append(layers, current)
		processed += len(current)

		var next []string
		for _, id := range current {
			for _, child := range graph.adj[id] {
				graph.inDegree[child]--
				if graph.inDegree[child] == 0 {
					next = append(next, child)
				}
			}
		}
		current = next
	}

	if processed != len(tasks) {
		return nil, fmt.Errorf("dependency cycle detected in task file")
	}
	return layers, nil
}
