package taskfile

import "testing"

func TestAnalyzeMode_AllIndependent(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B"},
			{ID: "c", Prompt: "do C"},
			{ID: "d", Prompt: "do D"},
			{ID: "e", Prompt: "do E"},
		},
	}
	mode := AnalyzeMode(tf)
	if mode != ModeSwarm {
		t.Errorf("all-independent DAG should select swarm, got %s", mode)
	}
}

func TestAnalyzeMode_DeepChain(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
			{ID: "c", Prompt: "do C", DependsOn: []string{"b"}},
			{ID: "d", Prompt: "do D", DependsOn: []string{"c"}},
		},
	}
	mode := AnalyzeMode(tf)
	if mode != ModeTeam {
		t.Errorf("deep chain (4 layers) should select team, got %s", mode)
	}
}

func TestAnalyzeMode_InheritContext(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", InheritContext: true},
		},
	}
	mode := AnalyzeMode(tf)
	if mode != ModeTeam {
		t.Errorf("inherit_context should force team, got %s", mode)
	}
}

func TestAnalyzeMode_WideDiamond(t *testing.T) {
	// Layer 0: [a], Layer 1: [b,c,d,e], Layer 2: [f]
	// Max width = 4, total = 6, ratio = 0.67 > 0.6 → swarm
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
			{ID: "c", Prompt: "do C", DependsOn: []string{"a"}},
			{ID: "d", Prompt: "do D", DependsOn: []string{"a"}},
			{ID: "e", Prompt: "do E", DependsOn: []string{"a"}},
			{ID: "f", Prompt: "do F", DependsOn: []string{"b", "c", "d", "e"}},
		},
	}
	mode := AnalyzeMode(tf)
	if mode != ModeSwarm {
		t.Errorf("wide diamond should select swarm, got %s", mode)
	}
}

func TestAnalyzeMode_NarrowDAG(t *testing.T) {
	// Layer 0: [a,b], Layer 1: [c,d], Layer 2: [e]
	// Max width = 2, total = 5, ratio = 0.4 < 0.6
	// Depth = 3 → not > 3 → swarm (default)
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A"},
			{ID: "b", Prompt: "do B"},
			{ID: "c", Prompt: "do C", DependsOn: []string{"a"}},
			{ID: "d", Prompt: "do D", DependsOn: []string{"b"}},
			{ID: "e", Prompt: "do E", DependsOn: []string{"c", "d"}},
		},
	}
	mode := AnalyzeMode(tf)
	if mode != ModeSwarm {
		t.Errorf("narrow 3-layer DAG should default to swarm, got %s", mode)
	}
}

func TestAnalyzeMode_Empty(t *testing.T) {
	tf := &TaskFile{Version: "1"}
	mode := AnalyzeMode(tf)
	if mode != ModeSwarm {
		t.Errorf("empty task list should default to swarm, got %s", mode)
	}
}

func TestAnalyzeMode_CycleFallback(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do A", DependsOn: []string{"b"}},
			{ID: "b", Prompt: "do B", DependsOn: []string{"a"}},
		},
	}
	mode := AnalyzeMode(tf)
	if mode != ModeTeam {
		t.Errorf("cycle should fallback to team, got %s", mode)
	}
}
