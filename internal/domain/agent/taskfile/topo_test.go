package taskfile

import (
	"reflect"
	"testing"
)

func TestTopologicalOrder_Linear(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "c", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "a"},
	}
	order, err := topologicalOrder(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"a", "b", "c"}) {
		t.Errorf("got %v, want [a b c]", order)
	}
}

func TestTopologicalOrder_Parallel(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}
	order, err := topologicalOrder(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// a and b can be in either order, but c must be last.
	if order[2] != "c" {
		t.Errorf("c should be last, got %v", order)
	}
	if len(order) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(order))
	}
}

func TestTopologicalOrder_Diamond(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}
	order, err := topologicalOrder(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order[0] != "a" {
		t.Errorf("a should be first, got %v", order)
	}
	if order[3] != "d" {
		t.Errorf("d should be last, got %v", order)
	}
}

func TestTopologicalOrder_Cycle(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}
	_, err := topologicalOrder(tasks)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestTopologicalOrder_SingleTask(t *testing.T) {
	tasks := []TaskSpec{{ID: "only"}}
	order, err := topologicalOrder(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"only"}) {
		t.Errorf("got %v, want [only]", order)
	}
}

// --- topologicalLayers tests ---

func TestTopologicalLayers_Flat(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}
	layers, err := topologicalLayers(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d: %v", len(layers), layers)
	}
	if len(layers[0]) != 3 {
		t.Errorf("layer 0 should have 3 tasks, got %d", len(layers[0]))
	}
}

func TestTopologicalLayers_Chain(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "c", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "a"},
	}
	layers, err := topologicalLayers(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}
	if !reflect.DeepEqual(layers[0], []string{"a"}) {
		t.Errorf("layer 0: got %v, want [a]", layers[0])
	}
	if !reflect.DeepEqual(layers[1], []string{"b"}) {
		t.Errorf("layer 1: got %v, want [b]", layers[1])
	}
	if !reflect.DeepEqual(layers[2], []string{"c"}) {
		t.Errorf("layer 2: got %v, want [c]", layers[2])
	}
}

func TestTopologicalLayers_Diamond(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}
	layers, err := topologicalLayers(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}
	if !reflect.DeepEqual(layers[0], []string{"a"}) {
		t.Errorf("layer 0: got %v, want [a]", layers[0])
	}
	if len(layers[1]) != 2 {
		t.Errorf("layer 1 should have 2 tasks, got %v", layers[1])
	}
	if !reflect.DeepEqual(layers[2], []string{"d"}) {
		t.Errorf("layer 2: got %v, want [d]", layers[2])
	}
}

func TestTopologicalLayers_Wide(t *testing.T) {
	// 5 independent → 1 final
	tasks := []TaskSpec{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"},
		{ID: "f", DependsOn: []string{"a", "b", "c", "d", "e"}},
	}
	layers, err := topologicalLayers(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(layers))
	}
	if len(layers[0]) != 5 {
		t.Errorf("layer 0 should have 5 tasks, got %d", len(layers[0]))
	}
	if len(layers[1]) != 1 {
		t.Errorf("layer 1 should have 1 task, got %d", len(layers[1]))
	}
}

func TestTopologicalLayers_Cycle(t *testing.T) {
	tasks := []TaskSpec{
		{ID: "a", DependsOn: []string{"c"}},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}
	_, err := topologicalLayers(tasks)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestTopologicalLayers_SingleTask(t *testing.T) {
	tasks := []TaskSpec{{ID: "only"}}
	layers, err := topologicalLayers(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 || len(layers[0]) != 1 || layers[0][0] != "only" {
		t.Errorf("got %v, want [[only]]", layers)
	}
}
