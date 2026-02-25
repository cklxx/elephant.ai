package lark

import (
	"fmt"
	"strconv"
	"strings"
)

// formatNumberedOptions formats a question with numbered options for text-based selection.
// Example output:
//
//	Which env?
//
//	[1] dev
//	[2] staging
//
//	回复数字选择，或直接输入内容。
func formatNumberedOptions(question string, options []string) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(question))
	sb.WriteString("\n")
	for i, opt := range options {
		sb.WriteString(fmt.Sprintf("\n[%d] %s", i+1, strings.TrimSpace(opt)))
	}
	sb.WriteString("\n\n回复数字选择，或直接输入内容。")
	return sb.String()
}

// parseNumberedReply resolves a numeric reply against a pending option list.
// If input is a 1-indexed number within range, the corresponding option is returned.
// Otherwise the raw input is returned as-is (free text fallback).
func parseNumberedReply(input string, options []string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || len(options) == 0 {
		return trimmed
	}
	n, err := strconv.Atoi(trimmed)
	if err != nil || n < 1 || n > len(options) {
		return trimmed
	}
	return options[n-1]
}
