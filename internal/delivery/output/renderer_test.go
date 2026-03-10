package output

import (
	"errors"
	"fmt"
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// OutputManager
// ---------------------------------------------------------------------------

func TestOutputManager_RegisterAndGet(t *testing.T) {
	mgr := NewOutputManager()
	r := NewLLMRenderer()
	mgr.RegisterRenderer(r)

	got := mgr.GetRenderer(TargetLLM)
	assert.Equal(t, r, got)
}

func TestOutputManager_GetUnregistered(t *testing.T) {
	mgr := NewOutputManager()
	assert.Nil(t, mgr.GetRenderer(TargetCLI))
}

func TestOutputManager_RenderFor(t *testing.T) {
	mgr := NewOutputManager()
	mgr.RegisterRenderer(NewLLMRenderer())

	result := mgr.RenderFor(TargetLLM, func(r Renderer) string {
		return "rendered"
	})
	assert.Equal(t, "rendered", result)
}

func TestOutputManager_RenderForMissing(t *testing.T) {
	mgr := NewOutputManager()
	result := mgr.RenderFor(TargetCLI, func(r Renderer) string {
		return "should not reach"
	})
	assert.Equal(t, "", result)
}

func TestOutputManager_RegisterOverwrites(t *testing.T) {
	mgr := NewOutputManager()
	mgr.RegisterRenderer(NewLLMRenderer())
	mgr.RegisterRenderer(NewLLMRenderer()) // overwrite
	assert.NotNil(t, mgr.GetRenderer(TargetLLM))
}

// ---------------------------------------------------------------------------
// CategorizeToolName
// ---------------------------------------------------------------------------

func TestCategorizeToolName(t *testing.T) {
	tests := []struct {
		tool     string
		expected types.ToolCategory
	}{
		{"read_file", types.CategoryFile},
		{"write_file", types.CategoryFile},
		{"replace_in_file", types.CategoryFile},
		{"shell_exec", types.CategoryShell},
		{"web_search", types.CategoryWeb},
		{"ask_user", types.CategoryTask},
		{"unknown_tool", types.CategoryOther},
		{"", types.CategoryOther},
	}
	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			assert.Equal(t, tc.expected, CategorizeToolName(tc.tool))
		})
	}
}

// ---------------------------------------------------------------------------
// LLMRenderer
// ---------------------------------------------------------------------------

func TestLLMRenderer_Target(t *testing.T) {
	r := NewLLMRenderer()
	assert.Equal(t, TargetLLM, r.Target())
}

func TestLLMRenderer_ToolCallStartReturnsEmpty(t *testing.T) {
	r := NewLLMRenderer()
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := r.RenderToolCallStart(ctx, "shell_exec", map[string]interface{}{"cmd": "ls"})
	assert.Equal(t, "", result)
}

func TestLLMRenderer_ToolCallComplete_Success(t *testing.T) {
	r := NewLLMRenderer()

	t.Run("core level", func(t *testing.T) {
		ctx := &types.OutputContext{Level: types.LevelCore}
		result := r.RenderToolCallComplete(ctx, "shell_exec", "file1.go\nfile2.go", nil, time.Second)
		assert.Equal(t, "file1.go\nfile2.go", result)
	})

	t.Run("subagent level", func(t *testing.T) {
		ctx := &types.OutputContext{Level: types.LevelSubagent, AgentID: "agent-1"}
		result := r.RenderToolCallComplete(ctx, "shell_exec", "output", nil, time.Second)
		assert.Contains(t, result, "[Subagent agent-1]")
		assert.Contains(t, result, "output")
	})
}

func TestLLMRenderer_ToolCallComplete_Error(t *testing.T) {
	r := NewLLMRenderer()

	t.Run("core level", func(t *testing.T) {
		ctx := &types.OutputContext{Level: types.LevelCore}
		result := r.RenderToolCallComplete(ctx, "shell_exec", "", errors.New("timeout"), time.Second)
		assert.Equal(t, "Error executing shell_exec: timeout", result)
	})

	t.Run("subagent level", func(t *testing.T) {
		ctx := &types.OutputContext{Level: types.LevelSubagent, AgentID: "agent-2"}
		result := r.RenderToolCallComplete(ctx, "read_file", "", errors.New("not found"), time.Second)
		assert.Contains(t, result, "[Subagent agent-2]")
		assert.Contains(t, result, "not found")
	})
}

func TestLLMRenderer_TaskComplete(t *testing.T) {
	r := NewLLMRenderer()
	ctx := &types.OutputContext{Level: types.LevelCore}
	taskResult := &domain.TaskResult{Answer: "The answer is 42"}
	result := r.RenderTaskComplete(ctx, taskResult)
	assert.Equal(t, "The answer is 42", result)
}

func TestLLMRenderer_RenderError(t *testing.T) {
	r := NewLLMRenderer()

	t.Run("core level", func(t *testing.T) {
		ctx := &types.OutputContext{Level: types.LevelCore}
		result := r.RenderError(ctx, "execution", fmt.Errorf("crashed"))
		assert.Equal(t, "Error in execution: crashed", result)
	})

	t.Run("subagent level", func(t *testing.T) {
		ctx := &types.OutputContext{Level: types.LevelSubagent, AgentID: "worker-1"}
		result := r.RenderError(ctx, "planning", fmt.Errorf("no plan"))
		assert.Contains(t, result, "[Subagent worker-1]")
		assert.Contains(t, result, "planning")
	})
}

func TestLLMRenderer_SubagentProgressReturnsEmpty(t *testing.T) {
	r := NewLLMRenderer()
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := r.RenderSubagentProgress(ctx, 3, 5, 1000, 10)
	assert.Equal(t, "", result)
}

func TestLLMRenderer_SubagentComplete(t *testing.T) {
	r := NewLLMRenderer()
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := r.RenderSubagentComplete(ctx, 5, 4, 1, 2000, 20)
	require.NotEmpty(t, result)
	assert.Contains(t, result, "4/5")
	assert.Contains(t, result, "1 failed")
}
