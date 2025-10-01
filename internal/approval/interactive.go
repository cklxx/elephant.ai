package approval

import (
	"alex/internal/agent/ports"
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

// InteractiveApprover implements approval via terminal prompts
type InteractiveApprover struct {
	timeout      time.Duration
	autoApprove  bool
	colorEnabled bool
}

// NewInteractiveApprover creates a new interactive approver
func NewInteractiveApprover(timeout time.Duration, autoApprove, colorEnabled bool) *InteractiveApprover {
	return &InteractiveApprover{
		timeout:      timeout,
		autoApprove:  autoApprove,
		colorEnabled: colorEnabled,
	}
}

// RequestApproval asks for user approval via terminal
func (a *InteractiveApprover) RequestApproval(ctx context.Context, request *ports.ApprovalRequest) (*ports.ApprovalResponse, error) {
	// Auto-approve if flag is set or request has auto-approve enabled
	if a.autoApprove || request.AutoApprove {
		return &ports.ApprovalResponse{
			Approved: true,
			Action:   "approve",
			Message:  "Auto-approved",
		}, nil
	}

	// Display the diff
	a.displayDiff(request)

	// Prompt for approval with timeout
	response, err := a.promptWithTimeout(ctx)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// displayDiff shows the diff and summary to the user
func (a *InteractiveApprover) displayDiff(request *ports.ApprovalRequest) {
	separator := strings.Repeat("=", 80)

	fmt.Println()
	fmt.Println(a.colorize(separator, color.FgCyan))
	fmt.Println(a.colorize(fmt.Sprintf("File Operation: %s", request.Operation), color.FgYellow, color.Bold))
	fmt.Println(a.colorize(fmt.Sprintf("File: %s", request.FilePath), color.FgWhite))
	fmt.Println(a.colorize(separator, color.FgCyan))
	fmt.Println()

	// Display summary
	if request.Summary != "" {
		fmt.Println(a.colorize("Summary:", color.FgCyan))
		fmt.Println(request.Summary)
		fmt.Println()
	}

	// Display diff
	if request.Diff != "" {
		fmt.Println(a.colorize("Changes:", color.FgCyan))
		fmt.Println(request.Diff)
		fmt.Println()
	}

	fmt.Println(a.colorize(separator, color.FgCyan))
}

// promptWithTimeout prompts the user for approval with a timeout
func (a *InteractiveApprover) promptWithTimeout(ctx context.Context) (*ports.ApprovalResponse, error) {
	// Create channel for user response
	responseChan := make(chan *ports.ApprovalResponse, 1)
	errorChan := make(chan error, 1)

	// Start goroutine to read user input
	go func() {
		response, err := a.readUserInput()
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- response
	}()

	// Wait for response or timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	select {
	case response := <-responseChan:
		return response, nil
	case err := <-errorChan:
		return nil, err
	case <-timeoutCtx.Done():
		// Timeout - default to reject
		fmt.Println()
		fmt.Println(a.colorize("Timeout - operation rejected", color.FgRed))
		return &ports.ApprovalResponse{
			Approved: false,
			Action:   "reject",
			Message:  "Approval timeout",
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// readUserInput reads and parses user input
func (a *InteractiveApprover) readUserInput() (*ports.ApprovalResponse, error) {
	fmt.Println()
	fmt.Println(a.colorize("Apply these changes?", color.FgYellow, color.Bold))
	fmt.Println("  [y] Yes, apply")
	fmt.Println("  [n] No, cancel")
	fmt.Println("  [e] Edit manually")
	fmt.Println("  [q] Quit ALEX")
	fmt.Print(a.colorize("Choice: ", color.FgCyan))

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "y", "yes":
		return &ports.ApprovalResponse{
			Approved: true,
			Action:   "approve",
			Message:  "Approved by user",
		}, nil
	case "n", "no", "":
		return &ports.ApprovalResponse{
			Approved: false,
			Action:   "reject",
			Message:  "Rejected by user",
		}, nil
	case "e", "edit":
		return &ports.ApprovalResponse{
			Approved: false,
			Action:   "edit",
			Message:  "User requested manual edit",
		}, nil
	case "q", "quit":
		return &ports.ApprovalResponse{
			Approved: false,
			Action:   "quit",
			Message:  "User requested quit",
		}, nil
	default:
		fmt.Println(a.colorize("Invalid choice. Please enter y, n, e, or q.", color.FgRed))
		return a.readUserInput() // Recursively ask again
	}
}

// colorize applies color to text if color is enabled
func (a *InteractiveApprover) colorize(text string, attributes ...color.Attribute) string {
	if !a.colorEnabled {
		return text
	}
	c := color.New(attributes...)
	return c.Sprint(text)
}

// NoOpApprover always approves (for testing or auto-approve mode)
type NoOpApprover struct{}

// NewNoOpApprover creates a new no-op approver
func NewNoOpApprover() *NoOpApprover {
	return &NoOpApprover{}
}

// RequestApproval always approves
func (a *NoOpApprover) RequestApproval(ctx context.Context, request *ports.ApprovalRequest) (*ports.ApprovalResponse, error) {
	return &ports.ApprovalResponse{
		Approved: true,
		Action:   "approve",
		Message:  "Auto-approved (no-op)",
	}, nil
}
