package main

import "alex/internal/shared/utils"

type approvalDecision struct {
	Approved       bool
	Action         string
	AutoApproveAll bool
}

func parseApprovalDecision(input string) (approvalDecision, bool) {
	choice := utils.TrimLower(input)
	switch choice {
	case "", "y", "yes", "allow":
		return approvalDecision{Approved: true, Action: "approve"}, true
	case "a", "all", "always":
		return approvalDecision{Approved: true, Action: "approve_all", AutoApproveAll: true}, true
	case "n", "no", "reject":
		return approvalDecision{Approved: false, Action: "reject"}, true
	case "q", "quit", "exit":
		return approvalDecision{Approved: false, Action: "quit"}, true
	default:
		return approvalDecision{}, false
	}
}
