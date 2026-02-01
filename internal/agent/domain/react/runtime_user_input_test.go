package react

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"

	"github.com/stretchr/testify/require"
)

func newEngineForUserInputTest() *ReactEngine {
	return &ReactEngine{logger: agent.NoopLogger{}}
}

func TestInjectUserInput_NilChannel(t *testing.T) {
	r := &reactRuntime{
		engine: newEngineForUserInputTest(),
		state:  &TaskState{},
	}
	r.injectUserInput()
	require.Empty(t, r.state.Messages)
}

func TestInjectUserInput_EmptyChannel(t *testing.T) {
	ch := make(chan agent.UserInput, 4)
	r := &reactRuntime{
		engine:      newEngineForUserInputTest(),
		state:       &TaskState{},
		userInputCh: ch,
	}
	r.injectUserInput()
	require.Empty(t, r.state.Messages)
}

func TestInjectUserInput_SingleMessage(t *testing.T) {
	ch := make(chan agent.UserInput, 4)
	ch <- agent.UserInput{Content: "follow up", SenderID: "user1", MessageID: "msg1"}

	r := &reactRuntime{
		engine:      newEngineForUserInputTest(),
		state:       &TaskState{},
		userInputCh: ch,
	}
	r.injectUserInput()

	require.Len(t, r.state.Messages, 1)
	require.Equal(t, "user", r.state.Messages[0].Role)
	require.Equal(t, "follow up", r.state.Messages[0].Content)
	require.Equal(t, ports.MessageSourceUserInput, r.state.Messages[0].Source)
}

func TestInjectUserInput_MultipleMessages(t *testing.T) {
	ch := make(chan agent.UserInput, 4)
	ch <- agent.UserInput{Content: "msg A", SenderID: "u1"}
	ch <- agent.UserInput{Content: "msg B", SenderID: "u2"}
	ch <- agent.UserInput{Content: "msg C", SenderID: "u1"}

	r := &reactRuntime{
		engine:      newEngineForUserInputTest(),
		state:       &TaskState{},
		userInputCh: ch,
	}
	r.injectUserInput()

	require.Len(t, r.state.Messages, 3)
	require.Equal(t, "msg A", r.state.Messages[0].Content)
	require.Equal(t, "msg B", r.state.Messages[1].Content)
	require.Equal(t, "msg C", r.state.Messages[2].Content)
}

func TestInjectUserInput_ClosedChannel(t *testing.T) {
	ch := make(chan agent.UserInput, 4)
	ch <- agent.UserInput{Content: "before close"}
	close(ch)

	r := &reactRuntime{
		engine:      newEngineForUserInputTest(),
		state:       &TaskState{},
		userInputCh: ch,
	}
	r.injectUserInput()

	require.Len(t, r.state.Messages, 1)
	require.Equal(t, "before close", r.state.Messages[0].Content)
	require.Nil(t, r.userInputCh, "channel should be set to nil after close")
}

func TestInjectUserInput_AppendsToExistingMessages(t *testing.T) {
	ch := make(chan agent.UserInput, 4)
	ch <- agent.UserInput{Content: "new input"}

	r := &reactRuntime{
		engine: newEngineForUserInputTest(),
		state: &TaskState{
			Messages: []ports.Message{
				{Role: "user", Content: "original task"},
				{Role: "assistant", Content: "thinking..."},
			},
		},
		userInputCh: ch,
	}
	r.injectUserInput()

	require.Len(t, r.state.Messages, 3)
	require.Equal(t, "original task", r.state.Messages[0].Content)
	require.Equal(t, "thinking...", r.state.Messages[1].Content)
	require.Equal(t, "new input", r.state.Messages[2].Content)
}

func TestInjectUserInput_ContextHelper(t *testing.T) {
	require.Nil(t, agent.UserInputChFromContext(context.Background()))

	ch := make(chan agent.UserInput, 1)
	ctx := agent.WithUserInputCh(context.Background(), ch)
	got := agent.UserInputChFromContext(ctx)
	require.NotNil(t, got)
}

func TestInjectUserInput_ContextHelperNilContext(t *testing.T) {
	require.Nil(t, agent.UserInputChFromContext(context.TODO()))
}
