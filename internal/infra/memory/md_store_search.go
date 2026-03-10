package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"alex/internal/shared/utils"
)

// Search scans MEMORY.md + daily logs for the query and returns ranked hits.
func (e *MarkdownEngine) Search(ctx context.Context, _ string, query string, maxResults int, minScore float64) ([]SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = defaultSearchMax
	}
	if minScore <= 0 {
		minScore = defaultSearchMinScore
	}

	root, err := e.requireRoot()
	if err != nil {
		return nil, err
	}

	if e.indexer != nil {
		results, err := e.indexer.Search(ctx, "", query, maxResults, minScore)
		if err == nil {
			return results, nil
		}
	}
	paths, err := collectMemoryFilesForRoot(root)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}

	queryTerms := normalizeTerms(tokenize(query))
	queryLower := strings.ToLower(query)
	if len(queryTerms) == 0 && queryLower == "" {
		return nil, nil
	}

	var hits []SearchHit
	for _, file := range paths {
		h, err := searchFile(file, root, queryTerms, queryLower, minScore, e.chunkTokens, e.chunkOverlap)
		if err != nil {
			continue
		}
		hits = append(hits, h...)
	}

	if len(hits) == 0 {
		return nil, nil
	}

	return selectTopHits(hits, maxResults), nil
}

// Related returns graph-adjacent memory entries for a path/range.
func (e *MarkdownEngine) Related(ctx context.Context, _ string, path string, fromLine, toLine, maxResults int) ([]RelatedHit, error) {
	if utils.IsBlank(path) {
		return nil, fmt.Errorf("path is required")
	}
	if maxResults <= 0 {
		maxResults = defaultSearchMax
	}

	if e.indexer != nil {
		results, err := e.indexer.Related(ctx, "", path, fromLine, toLine, maxResults)
		if err == nil {
			return results, nil
		}
	}

	absPath, err := e.resolvePath(path)
	if err != nil {
		return nil, err
	}
	lines, err := readLines(absPath)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}
	if fromLine <= 0 {
		fromLine = 1
	}
	if toLine <= 0 || toLine < fromLine {
		toLine = len(lines)
	}
	if fromLine > len(lines) {
		return nil, nil
	}
	if toLine > len(lines) {
		toLine = len(lines)
	}
	text := strings.Join(lines[fromLine-1:toLine], "\n")
	edges := extractMemoryEdges(text)
	if len(edges) == 0 {
		return nil, nil
	}

	results := make([]RelatedHit, 0, len(edges))
	for _, edge := range edges {
		cleanPath := normalizeLinkedPath(edge.DstPath)
		if cleanPath == "" {
			continue
		}
		related := RelatedHit{
			Path:         cleanPath,
			StartLine:    1,
			EndLine:      1,
			Score:        1.0,
			Snippet:      "",
			RelationType: edge.EdgeType,
			NodeID:       buildNodeID(cleanPath, 1, 1),
		}
		if relatedAbs, relErr := e.resolvePath(cleanPath); relErr == nil {
			if relLines, readErr := readLines(relatedAbs); readErr == nil && len(relLines) > 0 {
				related.EndLine = minInt(20, len(relLines))
				related.Snippet = buildSnippet(strings.Join(relLines[:related.EndLine], "\n"))
				related.NodeID = buildNodeID(cleanPath, related.StartLine, related.EndLine)
			}
		}
		results = append(results, related)
		if len(results) >= maxResults {
			break
		}
	}
	return results, nil
}

func searchFile(path, root string, queryTerms map[string]struct{}, queryLower string, minScore float64, chunkTokens, chunkOverlap int) ([]SearchHit, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	lineTokens := make([][]string, len(lines))
	lineCounts := make([]int, len(lines))
	for i, line := range lines {
		tokens := tokenize(line)
		lineTokens[i] = tokens
		lineCounts[i] = len(tokens)
	}

	var hits []SearchHit
	if chunkTokens <= 0 {
		chunkTokens = chunkTokenSize
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	start := 0
	for start < len(lines) {
		end, _ := nextChunkEnd(start, lineCounts, chunkTokens)
		if end <= start {
			break
		}
		chunkText := strings.Join(lines[start:end], "\n")
		chunkTerms := mergeTokens(lineTokens[start:end])
		score := scoreChunk(queryTerms, queryLower, chunkTerms, chunkText)
		if score >= minScore {
			relPath := path
			if rel, err := filepath.Rel(root, path); err == nil {
				relPath = rel
			}
			snippet := buildSnippet(chunkText)
			source := "memory"
			if filepath.Base(path) == memoryFileName {
				source = "long_term"
			}
			hits = append(hits, SearchHit{
				Path:      relPath,
				StartLine: start + 1,
				EndLine:   end,
				Score:     score,
				Snippet:   snippet,
				Source:    source,
				NodeID:    buildNodeID(relPath, start+1, end),
			})
		}
		start = nextChunkStart(start, end, lineCounts, chunkOverlap)
		if start <= 0 {
			break
		}
	}

	return hits, nil
}

func selectTopHits(hits []SearchHit, limit int) []SearchHit {
	if limit <= 0 || len(hits) <= limit {
		return hits
	}
	best := make([]SearchHit, 0, limit)
	for _, hit := range hits {
		if len(best) < limit {
			best = append(best, hit)
			continue
		}
		minIdx := 0
		for i := 1; i < len(best); i++ {
			if best[i].Score < best[minIdx].Score {
				minIdx = i
			}
		}
		if hit.Score > best[minIdx].Score {
			best[minIdx] = hit
		}
	}
	for i := 1; i < len(best); i++ {
		j := i
		for j > 0 {
			swap := best[j].Score > best[j-1].Score
			if !swap && best[j].Score == best[j-1].Score {
				swap = best[j].Path < best[j-1].Path
			}
			if !swap {
				break
			}
			best[j], best[j-1] = best[j-1], best[j]
			j--
		}
	}
	return best
}

func scoreChunk(queryTerms map[string]struct{}, queryLower string, chunkTerms []string, chunkText string) float64 {
	if len(queryTerms) == 0 {
		return 0
	}
	chunkSet := make(map[string]struct{}, len(chunkTerms))
	for _, term := range chunkTerms {
		trimmed := utils.TrimLower(term)
		if trimmed == "" {
			continue
		}
		chunkSet[trimmed] = struct{}{}
	}
	matches := 0
	for term := range queryTerms {
		if _, ok := chunkSet[term]; ok {
			matches++
		}
	}
	termScore := float64(matches) / float64(len(queryTerms))
	exactScore := 0.0
	if queryLower != "" {
		textLower := strings.ToLower(chunkText)
		if len(queryLower) >= 3 && strings.Contains(textLower, queryLower) {
			exactScore = 1.0
		}
	}
	return (0.7 * termScore) + (0.3 * exactScore)
}
