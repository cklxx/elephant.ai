package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// AstGrepTool implements AST-based code search functionality using ast-grep
type AstGrepTool struct{}

// AstGrepResult represents a single match result from ast-grep JSON output
type AstGrepResult struct {
	Text         string                 `json:"text"`
	File         string                 `json:"file"`
	Lines        string                 `json:"lines"`
	Language     string                 `json:"language"`
	Range        AstGrepRange           `json:"range"`
	MetaVariables AstGrepMetaVariables   `json:"metaVariables"`
}

type AstGrepRange struct {
	Start AstGrepPosition `json:"start"`
	End   AstGrepPosition `json:"end"`
}

type AstGrepPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type AstGrepMetaVariables struct {
	Single map[string]AstGrepVariable `json:"single"`
}

type AstGrepVariable struct {
	Text  string       `json:"text"`
	Range AstGrepRange `json:"range"`
}

func CreateAstGrepTool() *AstGrepTool {
	return &AstGrepTool{}
}

func (t *AstGrepTool) Name() string {
	return "ast_grep"
}

func (t *AstGrepTool) Description() string {
	return `Search for code patterns using AST (Abstract Syntax Tree) structure with ast-grep.

Usage:
- Searches for structural patterns in code using AST-based matching
- More precise than text-based search - matches code semantics, not just text
- Supports meta-variables ($VAR, $FUNC, $$$ARGS) for flexible pattern matching
- Works with multiple programming languages (Go, JavaScript, Python, etc.)
- Can perform code transformations with rewrite patterns

Parameters:
- pattern: AST pattern to search for (required) - use code-like syntax with meta-variables
- language: Programming language to search in (auto-detected if not specified)
- path: Directory or file to search in (defaults to current directory)  
- rewrite: Optional rewrite pattern for code transformation
- output_mode: Output format - "matches" (default), "json", or "count"
- context: Lines of context around matches (default: 0)
- max_results: Maximum number of results to return (default: 100)

Meta-variable examples:
- $VAR: matches any single variable/expression  
- $FUNC: matches any function call or expression
- $$$ARGS: matches zero or more arguments/statements
- $NAME: matches identifiers

Pattern examples:
- Go: "func $NAME($PARAMS) $RETURN" - find function definitions
- JavaScript: "const $VAR = $VALUE" - find const declarations
- Python: "def $NAME($ARGS):" - find function definitions
- General: "$OBJ.$METHOD($ARGS)" - find method calls

Output format (matches mode):
filename:line:matched_code_with_context

Notes:
- Requires ast-grep command-line tool to be installed
- More accurate than regex-based search for code patterns
- Supports structural transformations via rewrite patterns
- Language auto-detection works for most common file extensions`
}

func (t *AstGrepTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "AST pattern to search for (use meta-variables like $VAR, $FUNC)",
			},
			"language": map[string]interface{}{
				"type":        "string",
				"description": "Programming language (go, javascript, python, etc.) - auto-detected if not specified",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to search in",
				"default":     ".",
			},
			"rewrite": map[string]interface{}{
				"type":        "string",
				"description": "Optional rewrite pattern for code transformation",
			},
			"output_mode": map[string]interface{}{
				"type":        "string",
				"description": "Output format: matches, json, count",
				"enum":        []string{"matches", "json", "count"},
				"default":     "matches",
			},
			"context": map[string]interface{}{
				"type":        "integer",
				"description": "Lines of context around matches",
				"default":     0,
				"minimum":     0,
				"maximum":     10,
			},
			"max_results": map[string]interface{}{
				"type":        "integer", 
				"description": "Maximum number of results",
				"default":     100,
				"minimum":     1,
				"maximum":     500,
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *AstGrepTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("pattern", "AST pattern to search for").
		AddOptionalStringField("language", "Programming language").
		AddOptionalStringField("path", "Path to search in").
		AddOptionalStringField("rewrite", "Rewrite pattern").
		AddOptionalIntField("context", "Lines of context", 0, 10).
		AddOptionalIntField("max_results", "Maximum results", 1, 500).
		AddCustomValidator("output_mode", "Output format", false, func(value interface{}) error {
			if value == nil {
				return nil // Optional field
			}
			if str, ok := value.(string); ok {
				validModes := []string{"matches", "json", "count"}
				for _, mode := range validModes {
					if str == mode {
						return nil
					}
				}
				return fmt.Errorf("output_mode must be one of: %s", strings.Join(validModes, ", "))
			}
			return fmt.Errorf("output_mode must be a string")
		})

	return validator.Validate(args)
}

func (t *AstGrepTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Check if ast-grep is available
	if !isAstGrepInstalled() {
		return nil, fmt.Errorf("ast-grep is not installed or not accessible in PATH. Install with: npm install -g @ast-grep/cli")
	}

	// Extract and validate required parameters
	if args == nil {
		return nil, fmt.Errorf("arguments cannot be nil")
	}

	patternValue, exists := args["pattern"]
	if !exists {
		return nil, fmt.Errorf("pattern parameter is required")
	}

	pattern, ok := patternValue.(string)
	if !ok {
		return nil, fmt.Errorf("pattern must be a string")
	}

	if pattern == "" {
		return nil, fmt.Errorf("pattern cannot be empty")
	}

	// Extract optional parameters with defaults
	path := "."
	if p, ok := args["path"].(string); ok && p != "" {
		path = p
	}

	var language string
	if l, ok := args["language"].(string); ok {
		language = l
	}

	var rewrite string
	if r, ok := args["rewrite"].(string); ok {
		rewrite = r
	}

	outputMode := "matches"
	if om, ok := args["output_mode"].(string); ok {
		outputMode = om
	}

	context := 0
	if c, ok := args["context"].(float64); ok {
		context = int(c)
	}

	maxResults := 100
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	// Build ast-grep command
	cmdArgs := []string{"run", "--pattern", pattern}

	// Add language if specified
	if language != "" {
		cmdArgs = append(cmdArgs, "--lang", language)
	}

	// Add rewrite if specified
	if rewrite != "" {
		cmdArgs = append(cmdArgs, "--rewrite", rewrite)
	}

	// Always use JSON output for structured parsing
	cmdArgs = append(cmdArgs, "--json")

	// Add path
	cmdArgs = append(cmdArgs, path)

	// Execute ast-grep command
	cmd := exec.CommandContext(ctx, "ast-grep", cmdArgs...)
	output, err := cmd.Output()

	if err != nil {
		// Handle ast-grep specific errors
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "No such file or directory") {
				return nil, fmt.Errorf("path not found: %s", path)
			}
			if strings.Contains(stderr, "ERROR node") {
				return &ToolResult{
					Content: fmt.Sprintf("Pattern syntax error: %s\nTip: Use meta-variables like $VAR, $FUNC for flexible matching", stderr),
					Data: map[string]interface{}{
						"pattern": pattern,
						"error":   "pattern_syntax_error",
						"stderr":  stderr,
					},
				}, nil
			}
		}
		return nil, fmt.Errorf("ast-grep command failed: %w", err)
	}

	// Parse JSON output
	var results []AstGrepResult
	if len(output) > 0 {
		if err := json.Unmarshal(output, &results); err != nil {
			return nil, fmt.Errorf("failed to parse ast-grep JSON output: %w", err)
		}
	}

	// Handle no matches
	if len(results) == 0 {
		return &ToolResult{
			Content: "No matches found",
			Data: map[string]interface{}{
				"pattern":     pattern,
				"path":        path,
				"language":    language,
				"matches":     0,
				"output_mode": outputMode,
			},
		}, nil
	}

	// Apply result limit
	originalCount := len(results)
	truncated := false
	if len(results) > maxResults {
		results = results[:maxResults]
		truncated = true
	}

	// Format output based on mode
	switch outputMode {
	case "count":
		countMsg := fmt.Sprintf("Found %d matches", len(results))
		if truncated {
			countMsg += fmt.Sprintf(" (showing first %d of %d total)", maxResults, originalCount)
		}
		
		return &ToolResult{
			Content: countMsg,
			Data: map[string]interface{}{
				"pattern":          pattern,
				"matches":          len(results),
				"original_matches": originalCount,
				"truncated":        truncated,
				"output_mode":      outputMode,
			},
		}, nil

	case "json":
		jsonData, _ := json.MarshalIndent(results, "", "  ")
		return &ToolResult{
			Content: string(jsonData),
			Data: map[string]interface{}{
				"pattern":          pattern,
				"matches":          len(results),
				"original_matches": originalCount,
				"truncated":        truncated,
				"results":          results,
				"output_mode":      outputMode,
			},
		}, nil

	default: // "matches" mode
		var formattedResults []string
		var files []string
		fileMap := make(map[string]bool)

		for _, result := range results {
			// Get relative path for cleaner output
			relPath, _ := filepath.Rel(".", result.File)
			if relPath == "" {
				relPath = result.File
			}

			// Track modified files
			if !fileMap[result.File] {
				files = append(files, result.File)
				fileMap[result.File] = true
			}

			// Format: filename:line:matched_content
			line := result.Range.Start.Line + 1 // ast-grep uses 0-based, we want 1-based
			matchLine := strings.TrimSpace(result.Lines)
			
			// Add context if requested
			if context > 0 {
				// For now, just show the matched line - context would require additional file reading
				formattedResults = append(formattedResults, fmt.Sprintf("%s:%d:%s", relPath, line, matchLine))
			} else {
				formattedResults = append(formattedResults, fmt.Sprintf("%s:%d:%s", relPath, line, matchLine))
			}
		}

		// Build final content
		var warningMsg string
		if truncated {
			warningMsg = fmt.Sprintf("\n\n[TRUNCATED] Showing %d of %d total matches (limit: %d)", len(results), originalCount, maxResults)
		}

		finalContent := fmt.Sprintf("Found %d matches:\n%s%s", len(results), strings.Join(formattedResults, "\n"), warningMsg)

		return &ToolResult{
			Content: finalContent,
			Data: map[string]interface{}{
				"pattern":          pattern,
				"path":             path,
				"language":         language,
				"rewrite":          rewrite,
				"matches":          len(results),
				"original_matches": originalCount,
				"truncated":        truncated,
				"output_mode":      outputMode,
				"results":          formattedResults,
				"context":          context,
				"max_results":      maxResults,
			},
			Files: files,
		}, nil
	}
}

// isAstGrepInstalled checks if ast-grep command is available
func isAstGrepInstalled() bool {
	_, err := exec.LookPath("ast-grep")
	if err == nil {
		return true
	}

	// Test actual functionality
	cmd := exec.Command("ast-grep", "--version")
	err = cmd.Run()
	return err == nil
}