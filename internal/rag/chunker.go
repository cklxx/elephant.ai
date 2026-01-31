package rag

import (
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// ChunkerConfig holds chunking configuration
type ChunkerConfig struct {
	ChunkSize    int // Tokens per chunk (default: 512)
	ChunkOverlap int // Token overlap between chunks (default: 50)
}

// Chunk represents a text chunk with metadata
type Chunk struct {
	Text      string
	StartLine int
	EndLine   int
	Metadata  map[string]string
}

// Chunker splits text into chunks
type Chunker interface {
	// ChunkText splits text into chunks
	ChunkText(text string, metadata map[string]string) ([]Chunk, error)

	// CountTokens returns token count for text
	CountTokens(text string) (int, error)
}

// recursiveChunker implements recursive character-based chunking
type recursiveChunker struct {
	config   ChunkerConfig
	encoding *tiktoken.Tiktoken
}

// NewChunker creates a new chunker
func NewChunker(config ChunkerConfig) (Chunker, error) {
	if config.ChunkSize == 0 {
		config.ChunkSize = 512
	}
	if config.ChunkOverlap == 0 {
		config.ChunkOverlap = 50
	}

	// Use cl100k_base encoding (GPT-3.5/4)
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("get encoding: %w", err)
	}

	return &recursiveChunker{
		config:   config,
		encoding: encoding,
	}, nil
}

// ChunkText splits text into chunks
func (c *recursiveChunker) ChunkText(text string, metadata map[string]string) ([]Chunk, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Split by lines to track line numbers
	lines := strings.Split(text, "\n")

	var chunks []Chunk
	var currentChunk strings.Builder
	var currentStartLine int
	var currentTokens int

	for lineNum, line := range lines {
		lineText := line + "\n"
		lineTokens, err := c.CountTokens(lineText)
		if err != nil {
			return nil, err
		}

		// If single line exceeds chunk size, split it
		if lineTokens > c.config.ChunkSize {
			// Save current chunk if not empty
			if currentChunk.Len() > 0 {
				chunks = append(chunks, Chunk{
					Text:      currentChunk.String(),
					StartLine: currentStartLine,
					EndLine:   lineNum - 1,
					Metadata:  cloneMetadata(metadata),
				})
				currentChunk.Reset()
				currentTokens = 0
			}

			// Split long line by characters
			longLineChunks := c.splitLongLine(lineText, lineNum, metadata)
			chunks = append(chunks, longLineChunks...)
			currentStartLine = lineNum + 1
			continue
		}

		// Check if adding this line would exceed chunk size
		if currentTokens+lineTokens > c.config.ChunkSize && currentChunk.Len() > 0 {
			// Save current chunk
			chunks = append(chunks, Chunk{
				Text:      currentChunk.String(),
				StartLine: currentStartLine,
				EndLine:   lineNum - 1,
				Metadata:  cloneMetadata(metadata),
			})

			// Start new chunk with overlap
			if c.config.ChunkOverlap > 0 {
				overlapText, overlapStart := c.getOverlap(lines, lineNum, currentStartLine)
				currentChunk.Reset()
				currentChunk.WriteString(overlapText)
				currentStartLine = overlapStart
				currentTokens, _ = c.CountTokens(overlapText)
			} else {
				currentChunk.Reset()
				currentTokens = 0
				currentStartLine = lineNum
			}
		}

		// Add line to current chunk
		currentChunk.WriteString(lineText)
		currentTokens += lineTokens
	}

	// Add final chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, Chunk{
			Text:      currentChunk.String(),
			StartLine: currentStartLine,
			EndLine:   len(lines) - 1,
			Metadata:  cloneMetadata(metadata),
		})
	}

	return chunks, nil
}

// splitLongLine splits a very long line into character-based chunks
func (c *recursiveChunker) splitLongLine(line string, lineNum int, metadata map[string]string) []Chunk {
	var chunks []Chunk

	// Estimate characters per token (roughly 4)
	charsPerChunk := c.config.ChunkSize * 4

	for start := 0; start < len(line); start += charsPerChunk {
		end := start + charsPerChunk
		if end > len(line) {
			end = len(line)
		}

		chunkText := line[start:end]
		chunks = append(chunks, Chunk{
			Text:      chunkText,
			StartLine: lineNum,
			EndLine:   lineNum,
			Metadata:  cloneMetadata(metadata),
		})
	}

	return chunks
}

func cloneMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

// getOverlap returns overlap text from previous chunk
func (c *recursiveChunker) getOverlap(lines []string, currentLineNum, chunkStartLine int) (string, int) {
	var overlap strings.Builder
	tokens := 0
	startLine := currentLineNum

	// Work backwards to collect overlap
	for i := currentLineNum - 1; i >= chunkStartLine && tokens < c.config.ChunkOverlap; i-- {
		lineText := lines[i] + "\n"
		lineTokens, err := c.CountTokens(lineText)
		if err != nil || tokens+lineTokens > c.config.ChunkOverlap {
			break
		}

		overlap.WriteString(lineText)
		tokens += lineTokens
		startLine = i
	}

	// Reverse the overlap text (we built it backwards)
	overlapLines := strings.Split(overlap.String(), "\n")
	var result strings.Builder
	for i := len(overlapLines) - 1; i >= 0; i-- {
		if overlapLines[i] != "" || i > 0 {
			result.WriteString(overlapLines[i])
			if i > 0 {
				result.WriteString("\n")
			}
		}
	}

	return result.String(), startLine
}

// CountTokens returns token count for text
func (c *recursiveChunker) CountTokens(text string) (int, error) {
	tokens := c.encoding.Encode(text, nil, nil)
	return len(tokens), nil
}
