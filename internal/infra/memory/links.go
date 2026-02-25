package memory

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	memoryLinkPattern   = regexp.MustCompile(`\[\[memory:([^\]#\s]+)(?:#([^\]]+))?\]\]`)
	markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)#\s]+\.md)(?:#([^)]+))?\)`)
)

func extractMemoryEdges(text string) []MemoryEdge {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]MemoryEdge, 0, 4)
	appendEdge := func(dstPath, dstAnchor string) {
		dstPath = normalizeLinkedPath(dstPath)
		if dstPath == "" {
			return
		}
		dstAnchor = strings.TrimSpace(dstAnchor)
		key := dstPath + "|" + dstAnchor
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, MemoryEdge{
			DstPath:   dstPath,
			DstAnchor: dstAnchor,
			EdgeType:  "related",
			Direction: "directed",
		})
	}

	for _, m := range memoryLinkPattern.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		anchor := ""
		if len(m) > 2 {
			anchor = m[2]
		}
		appendEdge(m[1], anchor)
	}
	for _, m := range markdownLinkPattern.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 {
			continue
		}
		anchor := ""
		if len(m) > 2 {
			anchor = m[2]
		}
		appendEdge(m[1], anchor)
	}
	return out
}

func normalizeLinkedPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.Contains(path, "://") {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "." || path == ".." || path == "/" {
		return ""
	}
	if strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		return ""
	}
	if strings.HasPrefix(path, "/") {
		return ""
	}
	return path
}

func buildNodeID(path string, startLine, endLine int) string {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "" || path == "." {
		path = "memory"
	}
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 {
		endLine = startLine
	}
	return path + ":" + strconv.Itoa(startLine) + "-" + strconv.Itoa(endLine)
}
