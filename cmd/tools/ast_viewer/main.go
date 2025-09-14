package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"reflect"
	"strings"
)

// ASTNode represents a node in the AST for JSON serialization
type ASTNode struct {
	Type     string      `json:"type"`
	Position string      `json:"position,omitempty"`
	Value    interface{} `json:"value,omitempty"`
	Children []*ASTNode  `json:"children,omitempty"`
}

// ViewerConfig holds configuration for AST viewing
type ViewerConfig struct {
	Mode        string // standard, tree, compact, json
	ShowPos     bool   // show position information
	MaxDepth    int    // maximum depth to show (-1 for unlimited)
	Filter      string // filter node types (comma-separated)
	NoColor     bool   // disable color output
	Indent      string // indentation string
}

// Colors for different node types
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"  // Declarations
	ColorGreen   = "\033[32m"  // Statements
	ColorYellow  = "\033[33m"  // Expressions
	ColorBlue    = "\033[34m"  // Types
	ColorMagenta = "\033[35m"  // Literals
	ColorCyan    = "\033[36m"  // Identifiers
	ColorWhite   = "\033[37m"  // Others
)

// NodeCategory represents different types of AST nodes
type NodeCategory int

const (
	CategoryDecl NodeCategory = iota
	CategoryStmt
	CategoryExpr
	CategoryType
	CategoryLit
	CategoryIdent
	CategoryOther
)

func main() {
	var (
		mode     = flag.String("mode", "tree", "Output mode: standard, tree, compact, json")
		showPos  = flag.Bool("pos", false, "Show position information")
		maxDepth = flag.Int("depth", -1, "Maximum depth (-1 for unlimited)")
		filter   = flag.String("filter", "", "Filter node types (comma-separated)")
		noColor  = flag.Bool("no-color", false, "Disable colored output")
		help     = flag.Bool("help", false, "Show help message")
	)
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "AST Viewer - Visualize Go Abstract Syntax Trees\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [options] <file.go>        # View AST of Go file\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  echo 'code' | %s [options]   # View AST of stdin\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nModes:\n")
		fmt.Fprintf(os.Stderr, "  standard  - Detailed AST output (like go/ast.Print)\n")
		fmt.Fprintf(os.Stderr, "  tree      - Simplified tree structure (default)\n")
		fmt.Fprintf(os.Stderr, "  compact   - Compact node type display\n")
		fmt.Fprintf(os.Stderr, "  json      - JSON structured output\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -mode=tree main.go\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -mode=json -depth=3 main.go\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -filter=FuncDecl,TypeSpec main.go\n", os.Args[0])
	}
	
	flag.Parse()
	
	if *help {
		flag.Usage()
		return
	}

	config := &ViewerConfig{
		Mode:     *mode,
		ShowPos:  *showPos,
		MaxDepth: *maxDepth,
		Filter:   *filter,
		NoColor:  *noColor,
		Indent:   "  ",
	}

	var source string
	var filename string

	if flag.NArg() > 0 {
		// Read from file
		filename = flag.Arg(0)
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	} else {
		// Read from stdin
		filename = "<stdin>"
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	}

	if strings.TrimSpace(source) == "" {
		fmt.Fprintf(os.Stderr, "Error: empty input\n")
		os.Exit(1)
	}

	// Parse the Go source code
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	// Output based on selected mode
	switch config.Mode {
	case "standard":
		_ = ast.Fprint(os.Stdout, fset, node, nil)
	case "tree":
		printTreeAST(node, fset, config, 0)
	case "compact":
		printCompactAST(node, fset, config, 0)
	case "json":
		astNode := convertToJSON(node, fset, config, 0)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(astNode)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", config.Mode)
		os.Exit(1)
	}
}

func printTreeAST(node ast.Node, fset *token.FileSet, config *ViewerConfig, depth int) {
	if config.MaxDepth >= 0 && depth > config.MaxDepth {
		return
	}

	if node == nil {
		return
	}

	// Check filter
	nodeType := getNodeType(node)
	if config.Filter != "" && !shouldShowNode(nodeType, config.Filter) {
		return
	}

	// Print current node
	indent := strings.Repeat(config.Indent, depth)
	color := getNodeColor(node, config.NoColor)
	pos := ""
	if config.ShowPos && fset != nil {
		pos = fmt.Sprintf(" @%s", fset.Position(node.Pos()))
	}
	
	fmt.Printf("%s%s%s%s%s\n", indent, color, nodeType, pos, ColorReset)
	
	// Print node value if it's a simple value
	if value := getNodeValue(node); value != "" {
		fmt.Printf("%s%s└─ %s\n", indent, config.Indent, value)
	}

	// Recursively print children
	children := getNodeChildren(node)
	for _, child := range children {
		printTreeAST(child, fset, config, depth+1)
	}
}

func printCompactAST(node ast.Node, fset *token.FileSet, config *ViewerConfig, depth int) {
	if config.MaxDepth >= 0 && depth > config.MaxDepth {
		return
	}

	if node == nil {
		return
	}

	nodeType := getNodeType(node)
	if config.Filter != "" && !shouldShowNode(nodeType, config.Filter) {
		return
	}

	indent := strings.Repeat(config.Indent, depth)
	color := getNodeColor(node, config.NoColor)
	
	value := getNodeValue(node)
	if value != "" {
		fmt.Printf("%s%s%s%s: %s\n", indent, color, nodeType, ColorReset, value)
	} else {
		fmt.Printf("%s%s%s%s\n", indent, color, nodeType, ColorReset)
	}

	children := getNodeChildren(node)
	for _, child := range children {
		printCompactAST(child, fset, config, depth+1)
	}
}

func convertToJSON(node ast.Node, fset *token.FileSet, config *ViewerConfig, depth int) *ASTNode {
	if config.MaxDepth >= 0 && depth > config.MaxDepth {
		return nil
	}

	if node == nil {
		return nil
	}

	nodeType := getNodeType(node)
	if config.Filter != "" && !shouldShowNode(nodeType, config.Filter) {
		return nil
	}

	astNode := &ASTNode{
		Type: nodeType,
	}

	if config.ShowPos && fset != nil {
		astNode.Position = fset.Position(node.Pos()).String()
	}

	if value := getNodeValue(node); value != "" {
		astNode.Value = value
	}

	children := getNodeChildren(node)
	for _, child := range children {
		if childNode := convertToJSON(child, fset, config, depth+1); childNode != nil {
			astNode.Children = append(astNode.Children, childNode)
		}
	}

	return astNode
}

func getNodeType(node ast.Node) string {
	return strings.TrimPrefix(reflect.TypeOf(node).String(), "*ast.")
}

func getNodeValue(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Ident:
		return fmt.Sprintf("'%s'", n.Name)
	case *ast.BasicLit:
		return fmt.Sprintf("'%s'", n.Value)
	case *ast.Comment:
		return fmt.Sprintf("'%s'", n.Text)
	default:
		return ""
	}
}

func getNodeChildren(node ast.Node) []ast.Node {
	if node == nil {
		return nil
	}
	
	var children []ast.Node
	
	// Use ast.Inspect to safely traverse and collect children
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil || n == node {
			return true // Continue traversal, but skip self
		}
		children = append(children, n)
		return false // Don't traverse deeper for this child (we'll handle it recursively)
	})
	
	return children
}

func getNodeCategory(node ast.Node) NodeCategory {
	switch node.(type) {
	// Declarations
	case *ast.FuncDecl, *ast.GenDecl, *ast.ImportSpec, *ast.TypeSpec, *ast.ValueSpec:
		return CategoryDecl
	// Statements
	case *ast.AssignStmt, *ast.BlockStmt, *ast.ExprStmt, *ast.ForStmt, *ast.IfStmt, 
		 *ast.RangeStmt, *ast.ReturnStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return CategoryStmt
	// Expressions
	case *ast.BinaryExpr, *ast.CallExpr, *ast.IndexExpr, *ast.SelectorExpr, 
		 *ast.UnaryExpr, *ast.CompositeLit, *ast.FuncLit:
		return CategoryExpr
	// Types
	case *ast.ArrayType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType, 
		 *ast.MapType, *ast.StructType:
		return CategoryType
	// Literals
	case *ast.BasicLit:
		return CategoryLit
	// Identifiers
	case *ast.Ident:
		return CategoryIdent
	default:
		return CategoryOther
	}
}

func getNodeColor(node ast.Node, noColor bool) string {
	if noColor {
		return ""
	}
	
	category := getNodeCategory(node)
	switch category {
	case CategoryDecl:
		return ColorRed
	case CategoryStmt:
		return ColorGreen
	case CategoryExpr:
		return ColorYellow
	case CategoryType:
		return ColorBlue
	case CategoryLit:
		return ColorMagenta
	case CategoryIdent:
		return ColorCyan
	default:
		return ColorWhite
	}
}

func shouldShowNode(nodeType, filter string) bool {
	if filter == "" {
		return true
	}
	
	filters := strings.Split(filter, ",")
	for _, f := range filters {
		if strings.TrimSpace(f) == nodeType {
			return true
		}
	}
	return false
}