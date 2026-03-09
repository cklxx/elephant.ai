package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type sourceSpec struct {
	Dir    string
	Type   string
	Prefix string
}

type graphNode struct {
	ID      string   `yaml:"id"`
	Path    string   `yaml:"path"`
	Type    string   `yaml:"type"`
	Date    string   `yaml:"date,omitempty"`
	Tags    []string `yaml:"tags,omitempty"`
	Summary string   `yaml:"summary,omitempty"`
}

type graphEdge struct {
	From      string `yaml:"from"`
	To        string `yaml:"to"`
	Type      string `yaml:"type"`
	Direction string `yaml:"direction"`
}

type tagDef struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Aliases     []string `yaml:"aliases,omitempty"`
}

type indexYAML struct {
	Version int         `yaml:"version"`
	Updated string      `yaml:"updated"`
	Nodes   []graphNode `yaml:"nodes"`
	Schema  any         `yaml:"schema"`
}

type edgesYAML struct {
	Version int         `yaml:"version"`
	Updated string      `yaml:"updated"`
	Edges   []graphEdge `yaml:"edges"`
	Schema  any         `yaml:"schema"`
}

type tagsYAML struct {
	Version int      `yaml:"version"`
	Updated string   `yaml:"updated"`
	Tags    []tagDef `yaml:"tags"`
	Schema  any      `yaml:"schema"`
}

var (
	dateFromNamePattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})(?:-(.+))?$`)
	tokenPattern        = regexp.MustCompile(`[a-z0-9]+`)
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "backfill_networked: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	specs := []sourceSpec{
		{Dir: "docs/error-experience/entries", Type: "error_entry", Prefix: "err"},
		{Dir: "docs/error-experience/summary/entries", Type: "error_summary", Prefix: "errsum"},
		{Dir: "docs/good-experience/entries", Type: "good_entry", Prefix: "good"},
		{Dir: "docs/good-experience/summary/entries", Type: "good_summary", Prefix: "goodsum"},
	}

	nodesByPath := map[string]graphNode{}
	idByPath := map[string]string{}
	tagSet := map[string]struct{}{}
	for _, spec := range specs {
		if err := collectNodes(root, spec, nodesByPath, idByPath, tagSet); err != nil {
			return err
		}
	}
	if err := collectLongTermNode(root, nodesByPath, idByPath, tagSet); err != nil {
		return err
	}
	if err := collectPlanNodes(root, nodesByPath, idByPath, tagSet); err != nil {
		return err
	}

	nodes := make([]graphNode, 0, len(nodesByPath))
	for _, node := range nodesByPath {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Date == nodes[j].Date {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Date > nodes[j].Date
	})

	edges := buildEdges(nodesByPath, idByPath)
	tags := buildTagDefs(tagSet)
	updated := time.Now().Format("2006-01-02")

	if err := writeYAML(filepath.Join(root, "docs/memory/index.yaml"), indexYAML{
		Version: 1,
		Updated: updated,
		Nodes:   nodes,
		Schema: map[string]any{
			"id":      "string",
			"path":    "string",
			"type":    "string",
			"date":    "YYYY-MM-DD",
			"tags":    []string{"string"},
			"summary": "string",
		},
	}); err != nil {
		return err
	}
	if err := writeYAML(filepath.Join(root, "docs/memory/edges.yaml"), edgesYAML{
		Version: 1,
		Updated: updated,
		Edges:   edges,
		Schema: map[string]any{
			"from":      "string",
			"to":        "string",
			"type":      "related|supersedes|see_also|derived_from",
			"direction": "directed|bidirectional",
		},
	}); err != nil {
		return err
	}
	if err := writeYAML(filepath.Join(root, "docs/memory/tags.yaml"), tagsYAML{
		Version: 1,
		Updated: updated,
		Tags:    tags,
		Schema: map[string]any{
			"name":        "string",
			"description": "string",
			"aliases":     []string{"string"},
		},
	}); err != nil {
		return err
	}
	return nil
}

func collectNodes(root string, spec sourceSpec, nodesByPath map[string]graphNode, idByPath map[string]string, tagSet map[string]struct{}) error {
	dir := filepath.Join(root, spec.Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "README.md" {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		relPath, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		base := strings.TrimSuffix(entry.Name(), ".md")
		date, slug := parseDateSlug(base)
		id := buildID(spec.Prefix, date, slug)
		summary, fileTags, err := extractSummaryAndTags(filePath, spec.Type, slug)
		if err != nil {
			return err
		}
		node := graphNode{
			ID:      id,
			Path:    relPath,
			Type:    spec.Type,
			Date:    date,
			Tags:    fileTags,
			Summary: summary,
		}
		nodesByPath[relPath] = node
		idByPath[relPath] = id
		for _, tag := range fileTags {
			tagSet[tag] = struct{}{}
		}
	}
	return nil
}

func collectLongTermNode(root string, nodesByPath map[string]graphNode, idByPath map[string]string, tagSet map[string]struct{}) error {
	path := filepath.Join(root, "docs/memory/long-term.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	content := string(data)
	summary := "Durable long-term memory rules and active memory set."
	if idx := strings.Index(content, "## Active Memory"); idx >= 0 {
		summary = "Long-term durable memory rules plus currently active memory set."
	}
	relPath := "docs/memory/long-term.md"
	node := graphNode{
		ID:      "ltm-long-term-memory",
		Path:    relPath,
		Type:    "long_term",
		Date:    extractUpdatedDate(content),
		Tags:    []string{"memory", "long_term"},
		Summary: summary,
	}
	nodesByPath[relPath] = node
	idByPath[relPath] = node.ID
	tagSet["memory"] = struct{}{}
	tagSet["long_term"] = struct{}{}
	return nil
}

func collectPlanNodes(root string, nodesByPath map[string]graphNode, idByPath map[string]string, tagSet map[string]struct{}) error {
	dir := filepath.Join(root, "docs/plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		lower := strings.ToLower(entry.Name())
		if !strings.Contains(lower, "memory") && !strings.Contains(lower, "lark") {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		relPath, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		base := strings.TrimSuffix(entry.Name(), ".md")
		date, slug := parseDateSlug(base)
		if strings.TrimSpace(date) == "" {
			date = "1970-01-01"
		}
		id := buildID("plan", date, slug)
		summary, fileTags, err := extractSummaryAndTags(filePath, "plan", slug)
		if err != nil {
			return err
		}
		node := graphNode{
			ID:      id,
			Path:    relPath,
			Type:    "plan",
			Date:    date,
			Tags:    fileTags,
			Summary: summary,
		}
		nodesByPath[relPath] = node
		idByPath[relPath] = id
		for _, tag := range fileTags {
			tagSet[tag] = struct{}{}
		}
	}
	return nil
}

func buildEdges(nodesByPath map[string]graphNode, idByPath map[string]string) []graphEdge {
	edgeSet := map[string]graphEdge{}
	add := func(from, to, edgeType, direction string) {
		if from == "" || to == "" || from == to {
			return
		}
		key := from + "|" + to + "|" + edgeType + "|" + direction
		edgeSet[key] = graphEdge{From: from, To: to, Type: edgeType, Direction: direction}
	}

	for path, node := range nodesByPath {
		if node.Type != "error_summary" && node.Type != "good_summary" {
			continue
		}
		base := filepath.Base(path)
		var peerPath string
		if node.Type == "error_summary" {
			peerPath = filepath.ToSlash(filepath.Join("docs/error-experience/entries", base))
		} else {
			peerPath = filepath.ToSlash(filepath.Join("docs/good-experience/entries", base))
		}
		peerID := idByPath[peerPath]
		if peerID == "" {
			continue
		}
		add(node.ID, peerID, "derived_from", "directed")
		add(peerID, node.ID, "see_also", "directed")
	}

	// Link long-term memory to memory/lark plans for traversal.
	ltmID := idByPath["docs/memory/long-term.md"]
	if ltmID != "" {
		for _, node := range nodesByPath {
			if node.Type != "plan" {
				continue
			}
			if !hasAnyTag(node.Tags, "memory", "lark") {
				continue
			}
			add(ltmID, node.ID, "see_also", "directed")
		}
	}

	edges := make([]graphEdge, 0, len(edgeSet))
	for _, edge := range edgeSet {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			if edges[i].To == edges[j].To {
				return edges[i].Type < edges[j].Type
			}
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})
	return edges
}

func hasAnyTag(tags []string, wants ...string) bool {
	set := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		set[tag] = struct{}{}
	}
	for _, want := range wants {
		if _, ok := set[want]; ok {
			return true
		}
	}
	return false
}

func buildTagDefs(tagSet map[string]struct{}) []tagDef {
	defs := []tagDef{}
	desc := map[string]string{
		"memory":        "Memory retrieval, storage, or continuity concerns.",
		"long_term":     "Durable memory rules and long-lived guidance.",
		"lark":          "Lark channel, gateway, callback, or chat integration behavior.",
		"error":         "Error experience and failure analysis.",
		"good":          "Good experience and successful implementation patterns.",
		"summary":       "Condensed summary node derived from a full entry.",
		"plan":          "Implementation planning or execution tracking.",
		"config":        "Configuration, env, profile, or key management concerns.",
		"session":       "Session identity, reuse, or state recovery concerns.",
		"observability": "Metrics, logs, tracing, and runtime diagnosis.",
		"concurrency":   "Concurrency, race, or synchronization behavior.",
		"performance":   "Performance, memory growth, latency, or throughput.",
	}
	for tag := range tagSet {
		description := desc[tag]
		if description == "" {
			description = "Auto-derived keyword tag for networked memory indexing."
		}
		defs = append(defs, tagDef{Name: tag, Description: description})
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}

func extractSummaryAndTags(path, nodeType, slug string) (string, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	summary := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		summary = strings.TrimPrefix(line, "- ")
		break
	}
	if summary == "" {
		summary = "See entry content for details."
	}
	summary = truncateRunes(summary, 180)
	tags := inferTags(nodeType, slug, content)
	return summary, tags, nil
}

func inferTags(nodeType, slug, content string) []string {
	set := map[string]struct{}{}
	switch nodeType {
	case "error_entry", "error_summary":
		set["error"] = struct{}{}
	case "good_entry", "good_summary":
		set["good"] = struct{}{}
	}
	if strings.Contains(nodeType, "summary") {
		set["summary"] = struct{}{}
	}
	if nodeType == "plan" {
		set["plan"] = struct{}{}
	}
	text := strings.ToLower(slug + " " + content)
	for _, token := range tokenPattern.FindAllString(text, -1) {
		switch token {
		case "memory", "lark", "config", "session", "observability", "concurrency", "performance":
			set[token] = struct{}{}
		case "authdb", "database", "db":
			set["config"] = struct{}{}
		case "eval", "e2e":
			set["observability"] = struct{}{}
		}
	}
	tags := make([]string, 0, len(set))
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func extractUpdatedDate(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Updated:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "Updated:"))
		if len(value) >= 10 {
			return value[:10]
		}
	}
	return time.Now().Format("2006-01-02")
}

func parseDateSlug(base string) (string, string) {
	m := dateFromNamePattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(base)))
	if len(m) == 0 {
		return "", sanitizeSlug(base)
	}
	date := m[1]
	slug := sanitizeSlug(m[2])
	if slug == "" {
		slug = "entry"
	}
	return date, slug
}

func buildID(prefix, date, slug string) string {
	prefix = sanitizeSlug(prefix)
	if prefix == "" {
		prefix = "node"
	}
	if date == "" {
		return prefix + "-" + slug
	}
	return prefix + "-" + date + "-" + slug
}

func sanitizeSlug(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.Trim(raw, "-_ ")
	if raw == "" {
		return ""
	}
	parts := tokenPattern.FindAllString(raw, -1)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}

func writeYAML(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max])) + "..."
}
