package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	tools "alex/internal/domain/agent/ports/tools"

	"golang.org/x/term"
)

type cliApprover struct {
	sessionID      string
	in             io.Reader
	out            io.Writer
	interactive    bool
	autoApproveAll bool
	mu             sync.Mutex
}

func newCLIApprover(sessionID string) *cliApprover {
	return newCLIApproverWithIO(sessionID, os.Stdin, os.Stderr, detectInteractive(os.Stdin))
}

func newCLIApproverWithIO(sessionID string, in io.Reader, out io.Writer, interactive bool) *cliApprover {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stderr
	}
	return &cliApprover{
		sessionID:   sessionID,
		in:          in,
		out:         out,
		interactive: interactive,
	}
}

func (a *cliApprover) RequestApproval(ctx context.Context, request *tools.ApprovalRequest) (*tools.ApprovalResponse, error) {
	if request == nil {
		return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}
	if request.AutoApprove {
		return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.autoApproveAll {
		return &tools.ApprovalResponse{Approved: true, Action: "approve_all"}, nil
	}

	if !a.interactive {
		return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}

	prompt := approvalPrompt(request)
	if _, err := fmt.Fprint(a.out, prompt); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(a.in)
	line, err := reader.ReadString('\n')
	if err != nil {
		return &tools.ApprovalResponse{Approved: false, Action: "reject"}, nil
	}

	decision, ok := parseApprovalDecision(line)
	if !ok {
		return &tools.ApprovalResponse{Approved: false, Action: "reject"}, nil
	}
	if decision.AutoApproveAll {
		a.autoApproveAll = true
	}
	return &tools.ApprovalResponse{Approved: decision.Approved, Action: decision.Action}, nil
}

func approvalPrompt(request *tools.ApprovalRequest) string {
	var b strings.Builder
	if request.ToolName != "" {
		fmt.Fprintf(&b, "\nApproval required for %s", request.ToolName)
	} else if request.Operation != "" {
		fmt.Fprintf(&b, "\nApproval required for %s", request.Operation)
	} else {
		b.WriteString("\nApproval required")
	}
	if request.FilePath != "" {
		fmt.Fprintf(&b, "\nPath: %s", request.FilePath)
	}
	if request.Summary != "" {
		fmt.Fprintf(&b, "\nSummary: %s", request.Summary)
	}
	if request.SafetyLevel > 0 {
		fmt.Fprintf(&b, "\nSafety: L%d", request.SafetyLevel)
	}
	if request.RollbackSteps != "" {
		fmt.Fprintf(&b, "\nRollback: %s", request.RollbackSteps)
	}
	if request.AlternativePlan != "" {
		fmt.Fprintf(&b, "\nAlternative: %s", request.AlternativePlan)
	}
	b.WriteString("\nAllow? [y]es / [a]ll (session) / [n]o: ")
	return b.String()
}

func detectInteractive(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

type cliApproverStore struct {
	mu        sync.Mutex
	approvers map[string]*cliApprover
}

var defaultCLIApproverStore = newCLIApproverStore()

func newCLIApproverStore() *cliApproverStore {
	return &cliApproverStore{approvers: make(map[string]*cliApprover)}
}

func (s *cliApproverStore) forSession(sessionID string) *cliApprover {
	if strings.TrimSpace(sessionID) == "" {
		return newCLIApprover("")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if approver, ok := s.approvers[sessionID]; ok {
		return approver
	}
	approver := newCLIApprover(sessionID)
	s.approvers[sessionID] = approver
	return approver
}

func cliApproverForSession(sessionID string) *cliApprover {
	return defaultCLIApproverStore.forSession(sessionID)
}
