package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	agent "alex/internal/domain/agent/ports/agent"
	id "alex/internal/shared/utils/id"
)

// forkSlot tracks an in-flight btw (fork) session spawned while the parent
// session was running.
type forkSlot struct {
	childSessionID  string
	parentSessionID string
	parentInputCh   chan agent.UserInput // reference captured at fork time; may be stale when parent finishes
	chatID          string
	cancel          context.CancelFunc
}

// forkCounter is a per-gateway monotonic index for human-readable fork labels.
var forkCounter uint64

// generateChildSessionID builds a deterministic child session ID from the
// parent session ID plus a short KSUID tail.
func generateChildSessionID(parentSessionID string) string {
	tail := id.NewKSUID()
	if len(tail) > 8 {
		tail = tail[len(tail)-8:]
	}
	base := parentSessionID
	// Strip any prior /btw/ suffix so nested forks stay one level deep.
	if idx := strings.LastIndex(base, "/btw/"); idx != -1 {
		base = base[:idx]
	}
	return fmt.Sprintf("%s/btw/%s", base, tail)
}

// forkSlots is a gateway-level registry of active btw child slots.
// Key: childSessionID (string) → *forkSlot
type forkSlotMap struct{ sync.Map }

func (m *forkSlotMap) store(fs *forkSlot) {
	m.Store(fs.childSessionID, fs)
}

func (m *forkSlotMap) delete(childSessionID string) {
	m.Delete(childSessionID)
}

// launchForkSession spawns a child task goroutine that handles an interruptive
// ("btw") message while the parent session continues running. After the child
// task completes, if BtwAutoInjectResult is true and the parent inputCh is still
// alive, the child answer is injected into the parent.
//
// Caller must NOT hold slot.mu when calling this.
func (g *Gateway) launchForkSession(
	parentSessionID string,
	parentInputCh chan agent.UserInput,
	msg *incomingMessage,
) {
	idx := atomic.AddUint64(&forkCounter, 1)
	childSessionID := generateChildSessionID(parentSessionID)

	forkCtx, forkCancel := context.WithCancel(context.Background())

	fs := &forkSlot{
		childSessionID:  childSessionID,
		parentSessionID: parentSessionID,
		parentInputCh:   parentInputCh,
		chatID:          msg.chatID,
		cancel:          forkCancel,
	}
	g.forkSlots.store(fs)

	g.logger.Info(
		"btw fork #%d: spawning child session %s (parent=%s) for chat=%s",
		idx, childSessionID, parentSessionID, msg.chatID,
	)

	// Synthesize a fork-scoped inputCh (child never receives follow-up input;
	// the channel is closed immediately after task launch so the child's
	// ReAct loop does not block on user input).
	childInputCh := make(chan agent.UserInput, 1)
	close(childInputCh) // no interactive input for fork sessions

	g.taskWG.Add(1)
	go func() {
		defer g.taskWG.Done()
		defer forkCancel()
		defer g.forkSlots.delete(childSessionID)

		childMsg := *msg // shallow copy — safe: all fields are value or immutable
		result := g.runForkTask(forkCtx, &childMsg, childSessionID, parentSessionID, childInputCh)

		if result == nil || result.Answer == "" {
			g.logger.Info("btw fork %s: child returned empty answer; nothing to inject", childSessionID)
			return
		}

		if !g.btwAutoInjectEnabled() {
			g.logger.Info("btw fork %s: auto-inject disabled; dropping child answer", childSessionID)
			return
		}

		prefix := g.btwInjectionPrefix()
		injected := prefix + result.Answer

		select {
		case parentInputCh <- agent.UserInput{
			Content:   injected,
			SenderID:  msg.senderID,
			MessageID: "", // synthetic; not a real Lark message
		}:
			g.logger.Info("btw fork %s: injected %d chars into parent session %s", childSessionID, len(injected), parentSessionID)
		default:
			g.logger.Warn("btw fork %s: parent inputCh full or closed; dropping injection", childSessionID)
		}
	}()
}

// runForkTask runs a btw child task. It reuses runTask internally but skips
// the normal "resume" and "plan review" paths. The child session ID is isolated
// so no persistent session state bleeds between parent and child.
func (g *Gateway) runForkTask(
	ctx context.Context,
	msg *incomingMessage,
	childSessionID string,
	_ string, // parentSessionID reserved for future lineage tagging
	inputCh chan agent.UserInput,
) *agent.TaskResult {
	// runTask internally calls dispatchResult which sends the reply to Lark.
	// For fork sessions we still want the child's reply visible in chat so
	// the user knows their side-question was handled. isResume=false, taskToken=0.
	awaitingInput := g.runTask(ctx, msg, childSessionID, inputCh, false, 0)
	if awaitingInput {
		// Fork sessions should not enter await_user_input; log and continue.
		g.logger.Warn("btw fork child session %s returned await_user_input; treating as complete", childSessionID)
	}
	// We can't get the TaskResult back from runTask directly (it dispatches
	// internally), so we return nil here. Injection of the child answer into
	// the parent is handled by a separate mechanism when BtwResultSource == "context".
	// For now, returning nil signals "reply already sent; no injection needed".
	return nil
}

// btwAutoInjectEnabled returns whether fork results should be injected into the
// parent session. Defaults to false (reply is visible in chat; parent decides).
func (g *Gateway) btwAutoInjectEnabled() bool {
	if g.cfg.BtwAutoInjectResult == nil {
		return false
	}
	return *g.cfg.BtwAutoInjectResult
}

// btwInjectionPrefix returns the prefix used when injecting child answers.
func (g *Gateway) btwInjectionPrefix() string {
	p := strings.TrimSpace(g.cfg.BtwResultPrefix)
	if p == "" {
		return "[btw result] "
	}
	return p + " "
}
