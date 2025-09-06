package builtin

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"alex/internal/tools"
	"alex/internal/types"
)

// FileReadTool implements file reading functionality
type FileReadTool struct{}

func CreateFileReadTool() *FileReadTool {
	return &FileReadTool{}
}

func (t *FileReadTool) Name() string {
	return "file_read"
}

func (t *FileReadTool) Description() string {
	return `Read the contents of a file from the local filesystem with line number formatting and Go code analysis.

Usage:
- Automatically resolves relative paths to absolute paths
- Returns content with line numbers in "lineNum:content" format
- Supports reading specific line ranges with start_line and end_line parameters
- Shows file metadata including size, total lines, and modification time
- Content is limited to 2,500 characters maximum; exceeding files will show truncated content
- **NEW**: For Go files (.go), automatically extracts symbols (functions, structs, interfaces, imports)

Parameters:
- file_path or path: Path to the file to read (relative paths are resolved)
- start_line: Optional starting line number (1-based, inclusive)
- end_line: Optional ending line number (1-based, inclusive)
- analyze_go: Optional boolean to enable Go code analysis for .go files (default: true)

Example output format:
    1:package main
    2:import "fmt"
    3:func main() {

Go Analysis Features (for .go files):
- Extracts function signatures with parameters and return types
- Identifies struct definitions with field information
- Lists interface definitions with method signatures
- Shows import statements with resolved paths
- Detects package declaration and module context

Notes:
- If file doesn't exist, returns an error
- Line numbers start from 1
- When specifying line ranges, both start and end are inclusive
- RECOMMENDED: For large files, use start_line and end_line to read specific sections
- Handles both file_path and legacy path parameter for compatibility
- Files over 2,500 characters will be truncated with a warning message
- Go analysis gracefully handles syntax errors without affecting file reading`
}

func (t *FileReadTool) Parameters() map[string]interface{} {
	schema := tools.NewToolSchema().
		AddParameter("file_path", tools.NewStringParameter("Path to the file to read", false)).
		AddParameter("path", tools.NewStringParameter("Path to the file to read (legacy parameter)", false)).
		AddParameter("start_line", tools.NewIntegerParameter("Starting line number (1-based, optional)", false)).
		AddParameter("end_line", tools.NewIntegerParameter("Ending line number (1-based, optional)", false)).
		AddParameter("analyze_go", tools.NewBooleanParameter("Enable Go code analysis for .go files (default: true)", false))

	// Convert to legacy format for backward compatibility
	return schema.ToLegacyMap()
}

func (t *FileReadTool) Validate(args map[string]interface{}) error {
	// Check if either file_path or path is provided
	if _, hasFilePath := args["file_path"]; !hasFilePath {
		if _, hasPath := args["path"]; !hasPath {
			return fmt.Errorf("either file_path or path is required")
		}
	}

	validator := NewValidationFramework().
		AddOptionalStringField("file_path", "Path to the file to read").
		AddOptionalStringField("path", "Path to the file to read (legacy)").
		AddOptionalIntField("start_line", "Starting line number (1-based)", 1, 0).
		AddOptionalIntField("end_line", "Ending line number (1-based)", 1, 0).
		AddOptionalBooleanField("analyze_go", "Enable Go code analysis")

	return validator.Validate(args)
}

// GoSymbolInfo represents extracted Go code symbols
type GoSymbolInfo struct {
	PackageName string        `json:"package_name"`
	Imports     []GoImport    `json:"imports"`
	Functions   []GoFunction  `json:"functions"`
	Structs     []GoStruct    `json:"structs"`
	Interfaces  []GoInterface `json:"interfaces"`
	Types       []GoTypeDecl  `json:"types"`
	Constants   []GoConstant  `json:"constants"`
	Variables   []GoVariable  `json:"variables"`
}

// GoImport represents an import statement
type GoImport struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
	Line  int    `json:"line"`
}

// GoFunction represents a function or method
type GoFunction struct {
	Name       string    `json:"name"`
	Receiver   string    `json:"receiver,omitempty"`
	Parameters []GoParam `json:"parameters"`
	Returns    []GoParam `json:"returns"`
	Line       int       `json:"line"`
	IsMethod   bool      `json:"is_method"`
}

// GoStruct represents a struct definition
type GoStruct struct {
	Name   string    `json:"name"`
	Fields []GoField `json:"fields"`
	Line   int       `json:"line"`
}

// GoInterface represents an interface definition
type GoInterface struct {
	Name    string       `json:"name"`
	Methods []GoFunction `json:"methods"`
	Line    int          `json:"line"`
}

// GoTypeDecl represents a type declaration
type GoTypeDecl struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

// GoConstant represents a constant declaration
type GoConstant struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
	Line  int    `json:"line"`
}

// GoVariable represents a variable declaration
type GoVariable struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
	Line  int    `json:"line"`
}

// GoParam represents a function parameter or return value
type GoParam struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

// GoField represents a struct field
type GoField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Tag  string `json:"tag,omitempty"`
}

// analyzeGoFile extracts symbols from a Go source file
func (t *FileReadTool) analyzeGoFile(filePath string, content []byte) (*GoSymbolInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		// Return nil for parse errors but don't fail the entire operation
		return nil, err
	}

	symbols := &GoSymbolInfo{
		PackageName: node.Name.Name,
		Imports:     []GoImport{},
		Functions:   []GoFunction{},
		Structs:     []GoStruct{},
		Interfaces:  []GoInterface{},
		Types:       []GoTypeDecl{},
		Constants:   []GoConstant{},
		Variables:   []GoVariable{},
	}

	// Extract imports
	for _, imp := range node.Imports {
		goImport := GoImport{
			Path: strings.Trim(imp.Path.Value, `"`),
			Line: fset.Position(imp.Pos()).Line,
		}
		if imp.Name != nil {
			goImport.Alias = imp.Name.Name
		}
		symbols.Imports = append(symbols.Imports, goImport)
	}

	// Walk the AST to extract symbols
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Extract function/method information
			function := GoFunction{
				Name:       x.Name.Name,
				Parameters: t.extractParams(x.Type.Params),
				Returns:    t.extractParams(x.Type.Results),
				Line:       fset.Position(x.Pos()).Line,
				IsMethod:   x.Recv != nil,
			}

			// Extract receiver information for methods
			if x.Recv != nil && len(x.Recv.List) > 0 {
				if recv := x.Recv.List[0]; recv.Type != nil {
					function.Receiver = t.extractTypeString(recv.Type)
				}
			}

			symbols.Functions = append(symbols.Functions, function)

		case *ast.GenDecl:
			// Handle type, const, var, and import declarations
			for _, spec := range x.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					switch typeExpr := s.Type.(type) {
					case *ast.StructType:
						// Extract struct information
						structInfo := GoStruct{
							Name:   s.Name.Name,
							Fields: t.extractFields(typeExpr.Fields),
							Line:   fset.Position(s.Pos()).Line,
						}
						symbols.Structs = append(symbols.Structs, structInfo)
					case *ast.InterfaceType:
						// Extract interface information
						interfaceInfo := GoInterface{
							Name:    s.Name.Name,
							Methods: t.extractInterfaceMethods(typeExpr.Methods, fset),
							Line:    fset.Position(s.Pos()).Line,
						}
						symbols.Interfaces = append(symbols.Interfaces, interfaceInfo)
					default:
						// Other type declarations (aliases, etc.)
						typeDecl := GoTypeDecl{
							Name: s.Name.Name,
							Type: t.extractTypeString(typeExpr),
							Line: fset.Position(s.Pos()).Line,
						}
						symbols.Types = append(symbols.Types, typeDecl)
					}

				case *ast.ValueSpec:
					// Handle const and var declarations
					for i, name := range s.Names {
						if x.Tok == token.CONST {
							constant := GoConstant{
								Name: name.Name,
								Line: fset.Position(name.Pos()).Line,
							}
							if s.Type != nil {
								constant.Type = t.extractTypeString(s.Type)
							}
							if s.Values != nil && i < len(s.Values) {
								constant.Value = t.extractValueString(s.Values[i])
							}
							symbols.Constants = append(symbols.Constants, constant)
						} else if x.Tok == token.VAR {
							variable := GoVariable{
								Name: name.Name,
								Line: fset.Position(name.Pos()).Line,
							}
							if s.Type != nil {
								variable.Type = t.extractTypeString(s.Type)
							}
							if s.Values != nil && i < len(s.Values) {
								variable.Value = t.extractValueString(s.Values[i])
							}
							symbols.Variables = append(symbols.Variables, variable)
						}
					}
				}
			}
		}
		return true
	})

	return symbols, nil
}

// Helper functions for AST extraction
func (t *FileReadTool) extractParams(fieldList *ast.FieldList) []GoParam {
	if fieldList == nil {
		return nil
	}

	var params []GoParam
	for _, field := range fieldList.List {
		typeStr := t.extractTypeString(field.Type)
		if len(field.Names) == 0 {
			// Unnamed parameter (like in return values)
			params = append(params, GoParam{Type: typeStr})
		} else {
			// Named parameters
			for _, name := range field.Names {
				params = append(params, GoParam{Name: name.Name, Type: typeStr})
			}
		}
	}
	return params
}

func (t *FileReadTool) extractFields(fieldList *ast.FieldList) []GoField {
	if fieldList == nil {
		return nil
	}

	var fields []GoField
	for _, field := range fieldList.List {
		typeStr := t.extractTypeString(field.Type)
		tagStr := ""
		if field.Tag != nil {
			tagStr = field.Tag.Value
		}

		if len(field.Names) == 0 {
			// Embedded field
			fields = append(fields, GoField{Name: "", Type: typeStr, Tag: tagStr})
		} else {
			// Named fields
			for _, name := range field.Names {
				fields = append(fields, GoField{Name: name.Name, Type: typeStr, Tag: tagStr})
			}
		}
	}
	return fields
}

func (t *FileReadTool) extractInterfaceMethods(fieldList *ast.FieldList, fset *token.FileSet) []GoFunction {
	if fieldList == nil {
		return nil
	}

	var methods []GoFunction
	for _, field := range fieldList.List {
		if len(field.Names) > 0 {
			// Method declaration
			for _, name := range field.Names {
				if funcType, ok := field.Type.(*ast.FuncType); ok {
					method := GoFunction{
						Name:       name.Name,
						Parameters: t.extractParams(funcType.Params),
						Returns:    t.extractParams(funcType.Results),
						Line:       fset.Position(name.Pos()).Line,
						IsMethod:   false, // Interface methods are not considered "methods"
					}
					methods = append(methods, method)
				}
			}
		}
	}
	return methods
}

func (tool *FileReadTool) extractTypeString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return tool.extractTypeString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + tool.extractTypeString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + tool.extractTypeString(t.Elt)
		}
		return "[" + tool.extractValueString(t.Len) + "]" + tool.extractTypeString(t.Elt)
	case *ast.MapType:
		return "map[" + tool.extractTypeString(t.Key) + "]" + tool.extractTypeString(t.Value)
	case *ast.ChanType:
		prefix := "chan "
		if t.Dir == ast.RECV {
			prefix = "<-chan "
		} else if t.Dir == ast.SEND {
			prefix = "chan<- "
		}
		return prefix + tool.extractTypeString(t.Value)
	case *ast.FuncType:
		params := "("
		if t.Params != nil {
			var paramStrs []string
			for _, param := range t.Params.List {
				paramStrs = append(paramStrs, tool.extractTypeString(param.Type))
			}
			params += strings.Join(paramStrs, ", ")
		}
		params += ")"

		returns := ""
		if t.Results != nil && len(t.Results.List) > 0 {
			var returnStrs []string
			for _, result := range t.Results.List {
				returnStrs = append(returnStrs, tool.extractTypeString(result.Type))
			}
			if len(returnStrs) == 1 {
				returns = " " + returnStrs[0]
			} else {
				returns = " (" + strings.Join(returnStrs, ", ") + ")"
			}
		}

		return "func" + params + returns
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	default:
		// Fallback to string representation
		return fmt.Sprintf("%T", expr)
	}
}

func (tool *FileReadTool) extractValueString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	switch v := expr.(type) {
	case *ast.BasicLit:
		return v.Value
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return tool.extractValueString(v.X) + "." + v.Sel.Name
	default:
		// For complex expressions, return a simplified representation
		return "..."
	}
}

// formatGoSymbolSummary creates a human-readable summary of Go symbols
func (t *FileReadTool) formatGoSymbolSummary(symbols *GoSymbolInfo) string {
	if symbols == nil {
		return ""
	}

	var summary strings.Builder
	summary.WriteString("=== GO CODE ANALYSIS ===\n")
	summary.WriteString(fmt.Sprintf("Package: %s\n", symbols.PackageName))

	// Imports summary
	if len(symbols.Imports) > 0 {
		summary.WriteString(fmt.Sprintf("Imports (%d):\n", len(symbols.Imports)))
		for _, imp := range symbols.Imports {
			if imp.Alias != "" {
				summary.WriteString(fmt.Sprintf("  - %s as %s (line %d)\n", imp.Path, imp.Alias, imp.Line))
			} else {
				summary.WriteString(fmt.Sprintf("  - %s (line %d)\n", imp.Path, imp.Line))
			}
		}
	}

	// Functions summary
	if len(symbols.Functions) > 0 {
		summary.WriteString(fmt.Sprintf("\nFunctions & Methods (%d):\n", len(symbols.Functions)))
		for _, fn := range symbols.Functions {
			if fn.IsMethod && fn.Receiver != "" {
				summary.WriteString(fmt.Sprintf("  - (%s) %s", fn.Receiver, fn.Name))
			} else {
				summary.WriteString(fmt.Sprintf("  - %s", fn.Name))
			}

			// Add parameter info
			if len(fn.Parameters) > 0 {
				var params []string
				for _, param := range fn.Parameters {
					if param.Name != "" {
						params = append(params, fmt.Sprintf("%s %s", param.Name, param.Type))
					} else {
						params = append(params, param.Type)
					}
				}
				summary.WriteString(fmt.Sprintf("(%s)", strings.Join(params, ", ")))
			} else {
				summary.WriteString("()")
			}

			// Add return type info
			if len(fn.Returns) > 0 {
				var returns []string
				for _, ret := range fn.Returns {
					returns = append(returns, ret.Type)
				}
				if len(returns) == 1 {
					summary.WriteString(fmt.Sprintf(" %s", returns[0]))
				} else {
					summary.WriteString(fmt.Sprintf(" (%s)", strings.Join(returns, ", ")))
				}
			}
			summary.WriteString(fmt.Sprintf(" (line %d)\n", fn.Line))
		}
	}

	// Structs summary
	if len(symbols.Structs) > 0 {
		summary.WriteString(fmt.Sprintf("\nStructs (%d):\n", len(symbols.Structs)))
		for _, st := range symbols.Structs {
			summary.WriteString(fmt.Sprintf("  - %s", st.Name))
			if len(st.Fields) > 0 {
				summary.WriteString(fmt.Sprintf(" (%d fields)", len(st.Fields)))
			}
			summary.WriteString(fmt.Sprintf(" (line %d)\n", st.Line))

			// Show first few fields
			for i, field := range st.Fields {
				if i >= 3 { // Limit to first 3 fields
					summary.WriteString(fmt.Sprintf("    ... and %d more fields\n", len(st.Fields)-3))
					break
				}
				if field.Name != "" {
					summary.WriteString(fmt.Sprintf("    %s %s", field.Name, field.Type))
				} else {
					summary.WriteString(fmt.Sprintf("    %s (embedded)", field.Type))
				}
				if field.Tag != "" {
					summary.WriteString(fmt.Sprintf(" %s", field.Tag))
				}
				summary.WriteString("\n")
			}
		}
	}

	// Interfaces summary
	if len(symbols.Interfaces) > 0 {
		summary.WriteString(fmt.Sprintf("\nInterfaces (%d):\n", len(symbols.Interfaces)))
		for _, iface := range symbols.Interfaces {
			summary.WriteString(fmt.Sprintf("  - %s", iface.Name))
			if len(iface.Methods) > 0 {
				summary.WriteString(fmt.Sprintf(" (%d methods)", len(iface.Methods)))
			}
			summary.WriteString(fmt.Sprintf(" (line %d)\n", iface.Line))
		}
	}

	// Types summary
	if len(symbols.Types) > 0 {
		summary.WriteString(fmt.Sprintf("\nType Declarations (%d):\n", len(symbols.Types)))
		for _, typ := range symbols.Types {
			summary.WriteString(fmt.Sprintf("  - %s = %s (line %d)\n", typ.Name, typ.Type, typ.Line))
		}
	}

	// Constants summary
	if len(symbols.Constants) > 0 {
		summary.WriteString(fmt.Sprintf("\nConstants (%d):\n", len(symbols.Constants)))
		for _, constant := range symbols.Constants {
			summary.WriteString(fmt.Sprintf("  - %s", constant.Name))
			if constant.Type != "" {
				summary.WriteString(fmt.Sprintf(" %s", constant.Type))
			}
			if constant.Value != "" {
				summary.WriteString(fmt.Sprintf(" = %s", constant.Value))
			}
			summary.WriteString(fmt.Sprintf(" (line %d)\n", constant.Line))
		}
	}

	// Variables summary (only show package-level vars, not local ones)
	if len(symbols.Variables) > 0 {
		summary.WriteString(fmt.Sprintf("\nPackage Variables (%d):\n", len(symbols.Variables)))
		for _, variable := range symbols.Variables {
			summary.WriteString(fmt.Sprintf("  - %s", variable.Name))
			if variable.Type != "" {
				summary.WriteString(fmt.Sprintf(" %s", variable.Type))
			}
			if variable.Value != "" {
				summary.WriteString(fmt.Sprintf(" = %s", variable.Value))
			}
			summary.WriteString(fmt.Sprintf(" (line %d)\n", variable.Line))
		}
	}

	summary.WriteString("=======================")

	return summary.String()
}

func (t *FileReadTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Convert args to typed parameters for safer access
	params := types.ToolParameters(args)

	// Get file path using typed parameter access
	var filePath string
	if fp, ok := params.GetString("file_path"); ok {
		filePath = fp
	} else if p, ok := params.GetString("path"); ok {
		filePath = p
	} else {
		return nil, fmt.Errorf("either file_path or path is required")
	}

	// 解析路径（处理相对路径）
	resolver := GetPathResolverFromContext(ctx)
	resolvedPath := resolver.ResolvePath(filePath)

	// Check if file exists
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read file content
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	const maxChars = 2500
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	var formattedLines []string
	startLineNum := 1
	endLineNum := len(lines)
	truncated := false
	truncationWarning := ""

	// Handle line range if specified using typed parameter access
	if startLine, ok := params.GetInt("start_line"); ok {
		start := startLine - 1 // Convert to 0-based
		end := len(lines)

		if endLine, ok := params.GetInt("end_line"); ok {
			end = endLine
		}

		if start < 0 {
			start = 0
		}
		if start >= len(lines) {
			return &ToolResult{
				Content: "",
				Data: map[string]interface{}{
					"file_path":     filePath,
					"resolved_path": resolvedPath,
					"total_lines":   len(lines),
					"error":         "start_line exceeds file length",
				},
			}, nil
		}

		if end > len(lines) {
			end = len(lines)
		}
		if end <= start {
			end = start + 1
		}

		lines = lines[start:end]
		startLineNum = start + 1
		endLineNum = end
	}

	// Add line numbers to each line and check character limit
	totalChars := 0
	for i, line := range lines {
		lineNum := startLineNum + i
		formattedLine := fmt.Sprintf("%5d:%s", lineNum, line)

		// Check if adding this line would exceed the character limit
		if totalChars+len(formattedLine)+1 > maxChars { // +1 for newline
			truncated = true
			truncationWarning = fmt.Sprintf("\n\n[TRUNCATED] File content exceeds %d characters. Total file size: %d characters (%d lines). Consider using start_line and end_line parameters to read specific sections.", maxChars, len(content), len(strings.Split(string(content), "\n")))
			break
		}

		formattedLines = append(formattedLines, formattedLine)
		totalChars += len(formattedLine) + 1 // +1 for newline
	}

	contentStr = strings.Join(formattedLines, "\n") + truncationWarning

	// Get file info
	fileInfo, _ := os.Stat(resolvedPath)

	// Prepare result data
	resultData := map[string]interface{}{
		"file_path":       filePath,
		"resolved_path":   resolvedPath,
		"file_size":       len(content),
		"lines":           len(strings.Split(string(content), "\n")),
		"modified":        fileInfo.ModTime().Unix(),
		"start_line":      startLineNum,
		"end_line":        endLineNum,
		"displayed_lines": len(formattedLines),
		"truncated":       truncated,
		"content":         contentStr,
	}

	// Check if this is a Go file and analyze_go is enabled
	analyzeGo := true // Default to true
	if analyzeGoParam, ok := params.GetBool("analyze_go"); ok {
		analyzeGo = analyzeGoParam
	}

	if analyzeGo && strings.HasSuffix(strings.ToLower(resolvedPath), ".go") {
		symbols, err := t.analyzeGoFile(resolvedPath, content)
		if err == nil {
			// Add Go analysis results to the data
			resultData["go_analysis"] = symbols
			resultData["is_go_file"] = true

			// Add summary information to the content
			summary := t.formatGoSymbolSummary(symbols)
			if summary != "" {
				contentStr = summary + "\n\n" + contentStr
				resultData["content"] = contentStr
			}
		} else {
			// If Go analysis fails, still include basic info
			resultData["go_analysis_error"] = err.Error()
			resultData["is_go_file"] = true
		}
	} else {
		resultData["is_go_file"] = false
	}

	return &ToolResult{
		Content: contentStr,
		Data:    resultData,
	}, nil
}
