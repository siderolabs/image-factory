// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouteRegistrationIsOwnedByTransportRegistrar(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))

	err := filepath.WalkDir(repositoryRoot, func(filename string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)

		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if filepath.Ext(filename) != ".go" || strings.HasSuffix(filename, "_test.go") {
			return nil
		}

		relative, relativeErr := filepath.Rel(repositoryRoot, filename)
		require.NoError(t, relativeErr)

		if filepath.ToSlash(relative) == "internal/frontend/http/transport/registrar.go" {
			return nil
		}

		file, parseErr := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		require.NoError(t, parseErr)

		for _, finding := range findRawRegistrations(file) {
			t.Errorf("%s remains in %s", finding, filename)
		}

		return nil
	})
	require.NoError(t, err)
}

func TestRouteRegistrationMethodValuesCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
func bypass(handler Handler) {
	router := httprouter.New()
	r := router
	register := r.GET
	register("/bypass", handler)
}`, 0)
	require.NoError(t, err)

	require.Equal(t, []string{"raw router registration through GET"}, findRawRegistrations(file))
}

func findRawRegistrations(file *ast.File) []string {
	forbiddenSelectors := map[string]struct{}{
		"GET":    {},
		"HEAD":   {},
		"POST":   {},
		"Handle": {},
	}

	var findings []string

	routerIdentifiers := findRouterIdentifiers(file)

	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncDecl:
			if value.Name.Name == "registerRuntimeRoute" {
				findings = append(findings, "legacy registerRuntimeRoute function")
			}
		case *ast.SelectorExpr:
			if _, forbidden := forbiddenSelectors[value.Sel.Name]; forbidden && selectsRouter(value.X, routerIdentifiers) {
				findings = append(findings, "raw router registration through "+value.Sel.Name)
			}
		}

		return true
	})

	return findings
}

func findRouterIdentifiers(file *ast.File) map[string]struct{} {
	identifiers := map[string]struct{}{}

	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok || !isHTTPRouterType(field.Type) {
			return true
		}

		for _, name := range field.Names {
			identifiers[name.Name] = struct{}{}
		}

		return true
	})

	changed := true
	for changed {
		changed = false

		ast.Inspect(file, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}

			for index, right := range assignment.Rhs {
				if index >= len(assignment.Lhs) || !selectsRouter(right, identifiers) {
					continue
				}

				left, ok := assignment.Lhs[index].(*ast.Ident)
				if !ok {
					continue
				}

				if _, exists := identifiers[left.Name]; !exists {
					identifiers[left.Name] = struct{}{}
					changed = true
				}
			}

			return true
		})
	}

	return identifiers
}

func isHTTPRouterType(expression ast.Expr) bool {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}

	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name == "Router"
	case *ast.SelectorExpr:
		packageName, ok := value.X.(*ast.Ident)

		return ok && packageName.Name == "httprouter" && value.Sel.Name == "Router"
	default:
		return false
	}
}

func selectsRouter(expression ast.Expr, identifiers map[string]struct{}) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		_, ok := identifiers[value.Name]

		return ok
	case *ast.CallExpr:
		selector, ok := value.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "New" {
			return false
		}

		packageName, ok := selector.X.(*ast.Ident)

		return ok && packageName.Name == "httprouter"
	case *ast.SelectorExpr:
		return value.Sel.Name == "router" || selectsRouter(value.X, identifiers)
	default:
		return false
	}
}
