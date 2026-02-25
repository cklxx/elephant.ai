package di_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestExternalAgentWiring_HasOnlyExpectedManagedExecutorConstructors(t *testing.T) {
	moduleRoot := findModuleRootForGuard(t)
	sites, err := collectCallSites(moduleRoot, func(call *ast.CallExpr) bool {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel != nil && sel.Sel.Name == "NewManagedExternalExecutor"
	})
	if err != nil {
		t.Fatalf("collect call sites failed: %v", err)
	}

	got := toPathSet(sites)
	want := []string{
		"internal/app/di/builder_hooks.go",
		"internal/app/di/container_builder.go",
	}
	if !equalStringSlices(got, want) {
		t.Fatalf("managed executor constructor call sites changed.\nwant: %v\ngot: %v\nsites: %v", want, got, formatSites(sites))
	}
}

func TestExternalAgentDispatch_HasSingleRuntimeExecuteEntryPoint(t *testing.T) {
	moduleRoot := findModuleRootForGuard(t)
	sites, err := collectCallSites(moduleRoot, func(call *ast.CallExpr) bool {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "Execute" {
			return false
		}
		owner, ok := sel.X.(*ast.SelectorExpr)
		return ok && owner.Sel != nil && owner.Sel.Name == "externalExecutor"
	})
	if err != nil {
		t.Fatalf("collect call sites failed: %v", err)
	}

	if len(sites) != 1 {
		t.Fatalf("expected exactly one externalExecutor.Execute runtime call site, got %d: %v", len(sites), formatSites(sites))
	}
	if sites[0].Path != "internal/domain/agent/react/background.go" {
		t.Fatalf("unexpected call site %q:%d", sites[0].Path, sites[0].Line)
	}
}

type callSite struct {
	Path string
	Line int
}

func collectCallSites(moduleRoot string, matcher func(*ast.CallExpr) bool) ([]callSite, error) {
	internalRoot := filepath.Join(moduleRoot, "internal")
	fset := token.NewFileSet()
	var sites []callSite

	err := filepath.WalkDir(internalRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileNode, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		ast.Inspect(fileNode, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !matcher(call) {
				return true
			}
			pos := fset.Position(call.Pos())
			sites = append(sites, callSite{Path: rel, Line: pos.Line})
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(sites, func(i, j int) bool {
		if sites[i].Path == sites[j].Path {
			return sites[i].Line < sites[j].Line
		}
		return sites[i].Path < sites[j].Path
	})
	return sites, nil
}

func toPathSet(sites []callSite) []string {
	set := make(map[string]struct{}, len(sites))
	for _, site := range sites {
		set[site.Path] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func formatSites(sites []callSite) []string {
	out := make([]string, 0, len(sites))
	for _, site := range sites {
		out = append(out, site.Path+":"+strconv.Itoa(site.Line))
	}
	return out
}

func findModuleRootForGuard(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
