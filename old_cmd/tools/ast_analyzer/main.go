package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectAST represents the AST structure of the entire project
type ProjectAST struct {
	ProjectRoot string                `json:"project_root"`
	Modules     map[string]*ModuleAST `json:"modules"`
	Summary     *ProjectSummary       `json:"summary"`
}

// ModuleAST represents a Go module/package
type ModuleAST struct {
	ModulePath  string              `json:"module_path"`
	PackageName string              `json:"package_name"`
	Files       map[string]*FileAST `json:"files"`
}

// FileAST represents a single Go file's AST
type FileAST struct {
	FilePath    string          `json:"file_path"`
	PackageName string          `json:"package_name"`
	Imports     []ImportInfo    `json:"imports"`
	Functions   []FunctionInfo  `json:"functions"`
	Types       []TypeInfo      `json:"types"`
	Structs     []StructInfo    `json:"structs"`
	Interfaces  []InterfaceInfo `json:"interfaces"`
	Constants   []ConstInfo     `json:"constants"`
	Variables   []VarInfo       `json:"variables"`
}

// ImportInfo represents import declarations
type ImportInfo struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
	Line  int    `json:"line"`
}

// FunctionInfo represents function declarations
type FunctionInfo struct {
	Name       string      `json:"name"`
	Receiver   string      `json:"receiver,omitempty"`
	IsMethod   bool        `json:"is_method"`
	Parameters []ParamInfo `json:"parameters"`
	Returns    []ParamInfo `json:"returns"`
	Line       int         `json:"line"`
}

// TypeInfo represents type declarations
type TypeInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Line int    `json:"line"`
}

// StructInfo represents struct definitions
type StructInfo struct {
	Name   string      `json:"name"`
	Fields []FieldInfo `json:"fields"`
	Line   int         `json:"line"`
}

// InterfaceInfo represents interface definitions
type InterfaceInfo struct {
	Name    string         `json:"name"`
	Methods []FunctionInfo `json:"methods"`
	Line    int            `json:"line"`
}

// ConstInfo represents constant declarations
type ConstInfo struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
	Line  int    `json:"line"`
}

// VarInfo represents variable declarations
type VarInfo struct {
	Name  string `json:"name"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
	Line  int    `json:"line"`
}

// ParamInfo represents function parameters/returns
type ParamInfo struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

// FieldInfo represents struct fields
type FieldInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Tag  string `json:"tag,omitempty"`
}

// ProjectSummary provides overall statistics
type ProjectSummary struct {
	TotalFiles      int `json:"total_files"`
	TotalPackages   int `json:"total_packages"`
	TotalFunctions  int `json:"total_functions"`
	TotalMethods    int `json:"total_methods"`
	TotalStructs    int `json:"total_structs"`
	TotalInterfaces int `json:"total_interfaces"`
	TotalConstants  int `json:"total_constants"`
	TotalVariables  int `json:"total_variables"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <project-root> [output-format] [output-file]\n", os.Args[0])
		fmt.Printf("  output-format: json, tree, summary (default: tree)\n")
		fmt.Printf("  output-file: optional output file (default: stdout)\n")
		os.Exit(1)
	}

	projectRoot := os.Args[1]
	outputFormat := "tree"
	if len(os.Args) > 2 {
		outputFormat = os.Args[2]
	}

	var outputFile *os.File
	if len(os.Args) > 3 {
		var err error
		outputFile, err = os.Create(os.Args[3])
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer func() { _ = outputFile.Close() }()
	} else {
		outputFile = os.Stdout
	}

	// Analyze the project
	projectAST, err := analyzeProject(projectRoot)
	if err != nil {
		log.Fatalf("Failed to analyze project: %v", err)
	}

	// Output in requested format
	switch outputFormat {
	case "json":
		encoder := json.NewEncoder(outputFile)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(projectAST); err != nil {
			log.Fatalf("Failed to encode JSON: %v", err)
		}
	case "summary":
		printSummary(projectAST, outputFile)
	case "tree":
		printTreeView(projectAST, outputFile)
	default:
		log.Fatalf("Unknown output format: %s", outputFormat)
	}
}

func analyzeProject(projectRoot string) (*ProjectAST, error) {
	projectAST := &ProjectAST{
		ProjectRoot: projectRoot,
		Modules:     make(map[string]*ModuleAST),
		Summary:     &ProjectSummary{},
	}

	err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files and vendor/test directories
		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") ||
			strings.Contains(path, ".git/") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Analyze the Go file
		fileAST, err := analyzeGoFile(path)
		if err != nil {
			log.Printf("Warning: Failed to analyze %s: %v", path, err)
			return nil // Continue with other files
		}

		// Get relative path from project root
		relPath, _ := filepath.Rel(projectRoot, path)
		dirPath := filepath.Dir(relPath)

		// Group by package/module
		if projectAST.Modules[dirPath] == nil {
			projectAST.Modules[dirPath] = &ModuleAST{
				ModulePath:  dirPath,
				PackageName: fileAST.PackageName,
				Files:       make(map[string]*FileAST),
			}
		}

		projectAST.Modules[dirPath].Files[relPath] = fileAST

		// Update summary
		projectAST.Summary.TotalFiles++
		projectAST.Summary.TotalFunctions += len(fileAST.Functions)
		projectAST.Summary.TotalStructs += len(fileAST.Structs)
		projectAST.Summary.TotalInterfaces += len(fileAST.Interfaces)
		projectAST.Summary.TotalConstants += len(fileAST.Constants)
		projectAST.Summary.TotalVariables += len(fileAST.Variables)

		for _, fn := range fileAST.Functions {
			if fn.IsMethod {
				projectAST.Summary.TotalMethods++
			}
		}

		return nil
	})

	projectAST.Summary.TotalPackages = len(projectAST.Modules)
	return projectAST, err
}

func analyzeGoFile(filePath string) (*FileAST, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	fileAST := &FileAST{
		FilePath:    filePath,
		PackageName: node.Name.Name,
		Imports:     []ImportInfo{},
		Functions:   []FunctionInfo{},
		Types:       []TypeInfo{},
		Structs:     []StructInfo{},
		Interfaces:  []InterfaceInfo{},
		Constants:   []ConstInfo{},
		Variables:   []VarInfo{},
	}

	// Extract imports
	for _, imp := range node.Imports {
		importInfo := ImportInfo{
			Path: strings.Trim(imp.Path.Value, `"`),
			Line: fset.Position(imp.Pos()).Line,
		}
		if imp.Name != nil {
			importInfo.Alias = imp.Name.Name
		}
		fileAST.Imports = append(fileAST.Imports, importInfo)
	}

	// Walk AST to extract symbols
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			function := FunctionInfo{
				Name:       x.Name.Name,
				Parameters: extractParams(x.Type.Params),
				Returns:    extractParams(x.Type.Results),
				Line:       fset.Position(x.Pos()).Line,
				IsMethod:   x.Recv != nil,
			}

			if x.Recv != nil && len(x.Recv.List) > 0 {
				if recv := x.Recv.List[0]; recv.Type != nil {
					function.Receiver = extractTypeString(recv.Type)
				}
			}

			fileAST.Functions = append(fileAST.Functions, function)

		case *ast.GenDecl:
			for _, spec := range x.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					switch typeExpr := s.Type.(type) {
					case *ast.StructType:
						structInfo := StructInfo{
							Name:   s.Name.Name,
							Fields: extractFields(typeExpr.Fields),
							Line:   fset.Position(s.Pos()).Line,
						}
						fileAST.Structs = append(fileAST.Structs, structInfo)

					case *ast.InterfaceType:
						interfaceInfo := InterfaceInfo{
							Name:    s.Name.Name,
							Methods: extractInterfaceMethods(typeExpr.Methods, fset),
							Line:    fset.Position(s.Pos()).Line,
						}
						fileAST.Interfaces = append(fileAST.Interfaces, interfaceInfo)

					default:
						typeInfo := TypeInfo{
							Name: s.Name.Name,
							Type: extractTypeString(typeExpr),
							Line: fset.Position(s.Pos()).Line,
						}
						fileAST.Types = append(fileAST.Types, typeInfo)
					}

				case *ast.ValueSpec:
					for i, name := range s.Names {
						switch x.Tok {
						case token.CONST:
							constant := ConstInfo{
								Name: name.Name,
								Line: fset.Position(name.Pos()).Line,
							}
							if s.Type != nil {
								constant.Type = extractTypeString(s.Type)
							}
							if s.Values != nil && i < len(s.Values) {
								constant.Value = extractValueString(s.Values[i])
							}
							fileAST.Constants = append(fileAST.Constants, constant)

						case token.VAR:
							variable := VarInfo{
								Name: name.Name,
								Line: fset.Position(name.Pos()).Line,
							}
							if s.Type != nil {
								variable.Type = extractTypeString(s.Type)
							}
							if s.Values != nil && i < len(s.Values) {
								variable.Value = extractValueString(s.Values[i])
							}
							fileAST.Variables = append(fileAST.Variables, variable)
						}
					}
				}
			}
		}
		return true
	})

	return fileAST, nil
}

func printSummary(projectAST *ProjectAST, output *os.File) {
	_, _ = fmt.Fprintf(output, "Project AST Summary\n")
	_, _ = fmt.Fprintf(output, "==================\n\n")
	_, _ = fmt.Fprintf(output, "Project Root: %s\n\n", projectAST.ProjectRoot)

	summary := projectAST.Summary
	_, _ = fmt.Fprintf(output, "Overall Statistics:\n")
	_, _ = fmt.Fprintf(output, "- Total Packages:  %d\n", summary.TotalPackages)
	_, _ = fmt.Fprintf(output, "- Total Files:     %d\n", summary.TotalFiles)
	_, _ = fmt.Fprintf(output, "- Total Functions: %d\n", summary.TotalFunctions)
	_, _ = fmt.Fprintf(output, "- Total Methods:   %d\n", summary.TotalMethods)
	_, _ = fmt.Fprintf(output, "- Total Structs:   %d\n", summary.TotalStructs)
	_, _ = fmt.Fprintf(output, "- Total Interfaces:%d\n", summary.TotalInterfaces)
	_, _ = fmt.Fprintf(output, "- Total Constants: %d\n", summary.TotalConstants)
	_, _ = fmt.Fprintf(output, "- Total Variables: %d\n\n", summary.TotalVariables)

	_, _ = fmt.Fprintf(output, "Packages:\n")
	var sortedModules []string
	for modulePath := range projectAST.Modules {
		sortedModules = append(sortedModules, modulePath)
	}
	sort.Strings(sortedModules)

	for _, modulePath := range sortedModules {
		module := projectAST.Modules[modulePath]
		_, _ = fmt.Fprintf(output, "- %s (%s) - %d files\n", modulePath, module.PackageName, len(module.Files))
	}
}

func printTreeView(projectAST *ProjectAST, output *os.File) {
	_, _ = fmt.Fprintf(output, "Project AST Tree View\n")
	_, _ = fmt.Fprintf(output, "=====================\n\n")
	_, _ = fmt.Fprintf(output, "📁 %s\n", filepath.Base(projectAST.ProjectRoot))

	var sortedModules []string
	for modulePath := range projectAST.Modules {
		sortedModules = append(sortedModules, modulePath)
	}
	sort.Strings(sortedModules)

	for i, modulePath := range sortedModules {
		isLastModule := i == len(sortedModules)-1
		module := projectAST.Modules[modulePath]

		modulePrefix := "├── "
		filePrefix := "│   "
		if isLastModule {
			modulePrefix = "└── "
			filePrefix = "    "
		}

		_, _ = fmt.Fprintf(output, "%s📦 %s (%s)\n", modulePrefix, modulePath, module.PackageName)

		var sortedFiles []string
		for filePath := range module.Files {
			sortedFiles = append(sortedFiles, filePath)
		}
		sort.Strings(sortedFiles)

		for j, filePath := range sortedFiles {
			isLastFile := j == len(sortedFiles)-1
			file := module.Files[filePath]

			fileSymbol := filePrefix + "├── "
			itemPrefix := filePrefix + "│   "
			if isLastFile {
				fileSymbol = filePrefix + "└── "
				itemPrefix = filePrefix + "    "
			}

			fileName := filepath.Base(filePath)
			_, _ = fmt.Fprintf(output, "%s📄 %s\n", fileSymbol, fileName)

			// Show key symbols in each file
			if len(file.Functions) > 0 {
				_, _ = fmt.Fprintf(output, "%s🔧 %d functions\n", itemPrefix, len(file.Functions))
			}
			if len(file.Structs) > 0 {
				_, _ = fmt.Fprintf(output, "%s🏗️  %d structs\n", itemPrefix, len(file.Structs))
			}
			if len(file.Interfaces) > 0 {
				_, _ = fmt.Fprintf(output, "%s🔌 %d interfaces\n", itemPrefix, len(file.Interfaces))
			}
		}
	}

	_, _ = fmt.Fprintf(output, "\n")
	printSummary(projectAST, output)
}

// Helper functions (copied from file_read.go structure)
func extractParams(fieldList *ast.FieldList) []ParamInfo {
	if fieldList == nil {
		return nil
	}

	var params []ParamInfo
	for _, field := range fieldList.List {
		typeStr := extractTypeString(field.Type)
		if len(field.Names) == 0 {
			params = append(params, ParamInfo{Type: typeStr})
		} else {
			for _, name := range field.Names {
				params = append(params, ParamInfo{Name: name.Name, Type: typeStr})
			}
		}
	}
	return params
}

func extractFields(fieldList *ast.FieldList) []FieldInfo {
	if fieldList == nil {
		return nil
	}

	var fields []FieldInfo
	for _, field := range fieldList.List {
		typeStr := extractTypeString(field.Type)
		tag := ""
		if field.Tag != nil {
			tag = field.Tag.Value
		}

		if len(field.Names) == 0 {
			// Embedded field
			fields = append(fields, FieldInfo{Type: typeStr, Tag: tag})
		} else {
			for _, name := range field.Names {
				fields = append(fields, FieldInfo{Name: name.Name, Type: typeStr, Tag: tag})
			}
		}
	}
	return fields
}

func extractInterfaceMethods(fieldList *ast.FieldList, fset *token.FileSet) []FunctionInfo {
	if fieldList == nil {
		return nil
	}

	var methods []FunctionInfo
	for _, field := range fieldList.List {
		if len(field.Names) > 0 {
			for _, name := range field.Names {
				if funcType, ok := field.Type.(*ast.FuncType); ok {
					method := FunctionInfo{
						Name:       name.Name,
						Parameters: extractParams(funcType.Params),
						Returns:    extractParams(funcType.Results),
						Line:       fset.Position(field.Pos()).Line,
						IsMethod:   false, // Interface methods are not receiver methods
					}
					methods = append(methods, method)
				}
			}
		}
	}
	return methods
}

func extractTypeString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return extractTypeString(x.X) + "." + x.Sel.Name
	case *ast.StarExpr:
		return "*" + extractTypeString(x.X)
	case *ast.ArrayType:
		if x.Len == nil {
			return "[]" + extractTypeString(x.Elt)
		}
		return fmt.Sprintf("[%s]%s", extractValueString(x.Len), extractTypeString(x.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", extractTypeString(x.Key), extractTypeString(x.Value))
	case *ast.ChanType:
		switch x.Dir {
		case ast.RECV:
			return "<-chan " + extractTypeString(x.Value)
		case ast.SEND:
			return "chan<- " + extractTypeString(x.Value)
		default:
			return "chan " + extractTypeString(x.Value)
		}
	case *ast.FuncType:
		return "func"
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	default:
		return "unknown"
	}
}

func extractValueString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.BasicLit:
		return x.Value
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return extractValueString(x.X) + "." + x.Sel.Name
	default:
		return "..."
	}
}
