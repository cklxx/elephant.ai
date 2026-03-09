package taskfile

// ExecutionMode determines the concurrency strategy for task execution.
type ExecutionMode string

const (
	// ModeTeam uses the existing sequential-dispatch path with inter-task
	// communication via inherit_context and dependency-based blocking.
	ModeTeam ExecutionMode = "team"

	// ModeSwarm uses high fan-out parallel execution with adaptive concurrency.
	// DAG stages are executed as parallel batches sequentially.
	ModeSwarm ExecutionMode = "swarm"

	// ModeAuto analyzes the task DAG to select team or swarm automatically.
	ModeAuto ExecutionMode = "auto"
)

// maxSwarmDepth is the maximum DAG layer depth before analyzeMode prefers team
// execution. Deeply-layered graphs imply tightly-coupled sequential work where
// the overhead of swarm stage-batching outweighs parallelism benefits.
const maxSwarmDepth = 3

// analyzeMode inspects a TaskFile's dependency graph and context flags to
// recommend team or swarm execution.
//
// Decision rules (evaluated in order):
//  1. Any task with InheritContext → team (requires shared state)
//  2. Max chain depth > maxSwarmDepth → team (deeply coupled work)
//  3. Default → swarm (prefer parallelism when ambiguous)
func analyzeMode(tf *TaskFile) ExecutionMode {
	if len(tf.Tasks) == 0 {
		return ModeSwarm
	}

	for _, t := range tf.Tasks {
		if t.InheritContext {
			return ModeTeam
		}
	}

	layers, err := topologicalLayers(tf.Tasks)
	if err != nil {
		return ModeTeam // cycle or broken DAG → play it safe
	}

	if len(layers) > maxSwarmDepth {
		return ModeTeam
	}

	return ModeSwarm
}
