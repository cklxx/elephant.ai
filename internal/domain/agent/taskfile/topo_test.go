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
	order, err := TopologicalOrder(tasks)
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
	order, err := TopologicalOrder(tasks)
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
	order, err := TopologicalOrder(tasks)
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
	_, err := TopologicalOrder(tasks)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestTopologicalOrder_SingleTask(t *testing.T) {
	tasks := []TaskSpec{{ID: "only"}}
	order, err := TopologicalOrder(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"only"}) {
		t.Errorf("got %v, want [only]", order)
	}
}
