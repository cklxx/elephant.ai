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

// AnalyzeMode inspects a TaskFile's dependency graph and context flags to
// recommend team or swarm execution.
//
// Decision rules (evaluated in order):
//  1. Any task with InheritContext → team (requires shared state)
//  2. Max chain depth > 3 → team (deeply coupled work)
//  3. Max layer width / total tasks > 0.6 → swarm (wide parallel DAG)
//  4. Default → swarm (prefer parallelism when ambiguous)
func AnalyzeMode(tf *TaskFile) ExecutionMode {
	if len(tf.Tasks) == 0 {
		return ModeSwarm
	}

	for _, t := range tf.Tasks {
		if t.InheritContext {
			return ModeTeam
		}
	}

	layers, err := TopologicalLayers(tf.Tasks)
	if err != nil {
		return ModeTeam // cycle or broken DAG → play it safe
	}

	if len(layers) > 3 {
		return ModeTeam
	}

	maxWidth := 0
	for _, layer := range layers {
		if len(layer) > maxWidth {
			maxWidth = len(layer)
		}
	}

	if float64(maxWidth)/float64(len(tf.Tasks)) > 0.6 {
		return ModeSwarm
	}

	return ModeSwarm
}
