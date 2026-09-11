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
	"strconv"
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

		for _, finding := range findRawRouterFields(file) {
			t.Errorf("%s remains in %s", finding, filename)
		}

		return nil
	})
	require.NoError(t, err)
}

func TestRawRouterFieldsCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
import mux "github.com/julienschmidt/httprouter"
type server struct { router *mux.Router }
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Equal(t, []string{"raw router field router"}, findRawRouterFields(file))
}

func TestEmbeddedRawRouterCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
import mux "github.com/julienschmidt/httprouter"
type server struct { *mux.Router }
func register(server server) { server.Router.GET("/bypass", nil) }
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Equal(t, []string{"raw router field Router"}, findRawRouterFields(file))
	require.Equal(t, []string{"raw router registration through GET"}, findRawRegistrations(file))
}

func TestNestedRouterFieldCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
type bypass struct {
 nested struct {
  router *httprouter.Router
 }
}
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Equal(t, []string{"raw router field router"}, findRawRouterFields(file))
}

func TestVarRouterDeclarationCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
import mux "github.com/julienschmidt/httprouter"
func register() {
 var router = mux.New()
 router.GET("/bypass", nil)
}
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Equal(t, []string{"raw router registration through GET"}, findRawRegistrations(file))
}

func TestDotImportedRouterCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
import . "github.com/julienschmidt/httprouter"
type rawRouter = Router
func register() {
 router := New()
 router.GET("/bypass", nil)
}
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Equal(t, []string{"raw router registration through GET"}, findRawRegistrations(file))
	require.Equal(t, []string{"raw router type alias rawRouter"}, findRawRouterFields(file))
}

func TestRouterReturningCallCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
import "github.com/julienschmidt/httprouter"
var handler httprouter.Handle
func acquireCanary() *httprouter.Router { return httprouter.New() }
func register() {
 acquireCanary().GET("/direct", handler)
 register := acquireCanary().POST
 register("/method-value", handler)
}
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Equal(t, []string{
		"raw router registration through GET",
		"raw router registration through POST",
	}, findRawRegistrations(file))
}

func TestNonRouterFluentHandlerIsAllowedByArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "allowed.go", `package http
import "github.com/rs/cors"
func assemble(next Handler) {
 cors.New(cors.Options{}).Handler(next)
}
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Empty(t, findRawRegistrations(file))
}

func TestPackageAliasesCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", `package http
import mux "github.com/julienschmidt/httprouter"
var newRouter = mux.New
type routerAlias = mux.Router
`, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Contains(t, findRawRegistrations(file), "raw router constructor alias")
	require.Contains(t, findRawRouterFields(file), "raw router type alias routerAlias")
}

func TestRouteRegistrationCannotBypassArchitectureGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		source   string
		expected []string
	}{
		{
			name: "local alias",
			source: `package http
+func bypass(handler Handler) {
+	router := httprouter.New()
+	r := router
+	register := r.GET
+	register("/bypass", handler)
+}`,
			expected: []string{"raw router registration through GET"},
		},
		{
			name: "package and constructor aliases",
			source: `package http
+import routerlib "github.com/julienschmidt/httprouter"
+func bypass(handler Handler) {
+	newRouter := routerlib.New
+	router := newRouter()
+	register := router.PUT
+	register("/bypass", handler)
+}`,
			expected: []string{"raw router registration through PUT"},
		},
		{
			name: "router-valued struct field",
			source: `package http
+import routerlib "github.com/julienschmidt/httprouter"
+type server struct { mux *routerlib.Router }
+func bypass(s server, handler Handler) {
+	register := s.mux.DELETE
+	register("/bypass", handler)
+}`,
			expected: []string{"raw router registration through DELETE"},
		},
		{
			name: "all registration methods",
			source: `package http
+func bypass(router *httprouter.Router, handler Handler) {
+	_ = router.HEAD
+	_ = router.POST
+	_ = router.PUT
+	_ = router.PATCH
+	_ = router.DELETE
+	_ = router.OPTIONS
+	_ = router.Handle
+	_ = router.Handler
+	_ = router.HandlerFunc
+	_ = router.ServeFiles
+}`,
			expected: []string{
				"raw router registration through HEAD",
				"raw router registration through POST",
				"raw router registration through PUT",
				"raw router registration through PATCH",
				"raw router registration through DELETE",
				"raw router registration through OPTIONS",
				"raw router registration through Handle",
				"raw router registration through Handler",
				"raw router registration through HandlerFunc",
				"raw router registration through ServeFiles",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "bypass.go", strings.ReplaceAll(test.source, "\n+", "\n"), parser.SkipObjectResolution)
			require.NoError(t, err)

			require.Equal(t, test.expected, findRawRegistrations(file))
		})
	}
}

func findRawRegistrations(file *ast.File) []string {
	forbiddenSelectors := map[string]struct{}{
		"GET": {}, "HEAD": {}, "POST": {}, "PUT": {}, "PATCH": {}, "DELETE": {}, "OPTIONS": {},
		"Handle": {}, "Handler": {}, "HandlerFunc": {}, "ServeFiles": {},
	}

	aliases := findRouterPackageAliases(file)
	identifiers, fieldNames, constructors := findRouterIdentifiers(file, aliases)
	providerFunctions := findRouterProviderFunctions(file, aliases)

	var findings []string

	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.FuncDecl:
			if value.Name.Name == "registerRuntimeRoute" {
				findings = append(findings, "legacy registerRuntimeRoute function")
			}
		case *ast.ValueSpec:
			for _, expression := range value.Values {
				if selectsRouterConstructor(expression, aliases) {
					findings = append(findings, "raw router constructor alias")
				}
			}
		case *ast.SelectorExpr:
			directRouterCall := callsRouterProvider(value.X, providerFunctions)

			if _, forbidden := forbiddenSelectors[value.Sel.Name]; forbidden && (directRouterCall || selectsRouter(value.X, identifiers, fieldNames, constructors, aliases)) {
				findings = append(findings, "raw router registration through "+value.Sel.Name)
			}
		}

		return true
	})

	return findings
}

func findRawRouterFields(file *ast.File) []string {
	aliases := findRouterPackageAliases(file)

	var findings []string

	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.TypeSpec:
			if isHTTPRouterType(value.Type, aliases) {
				findings = append(findings, "raw router type alias "+value.Name.Name)
			}

			return true
		case *ast.StructType:
			for _, field := range value.Fields.List {
				if !isHTTPRouterType(field.Type, aliases) {
					continue
				}

				for _, name := range routerFieldNames(field) {
					findings = append(findings, "raw router field "+name.Name)
				}
			}

			return true
		default:
			return true
		}
	})

	return findings
}

func findRouterPackageAliases(file *ast.File) map[string]struct{} {
	aliases := map[string]struct{}{}

	for _, importSpec := range file.Imports {
		path, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || path != "github.com/julienschmidt/httprouter" {
			continue
		}

		name := "httprouter"
		if importSpec.Name != nil {
			name = importSpec.Name.Name
		}

		aliases[name] = struct{}{}
	}

	// Test snippets may intentionally omit imports to focus on data-flow behavior.
	aliases["httprouter"] = struct{}{}

	return aliases
}

func findRouterIdentifiers(file *ast.File, aliases map[string]struct{}) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	identifiers := map[string]struct{}{}
	fieldNames := map[string]struct{}{}
	constructors := map[string]struct{}{}

	ast.Inspect(file, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if !ok || !isHTTPRouterType(field.Type, aliases) {
			return true
		}

		for _, name := range routerFieldNames(field) {
			identifiers[name.Name] = struct{}{}
			fieldNames[name.Name] = struct{}{}
		}

		return true
	})

	for propagateRouterAssignments(file, identifiers, fieldNames, constructors, aliases) {
	}

	return identifiers, fieldNames, constructors
}

func propagateRouterAssignments(
	file *ast.File,
	identifiers map[string]struct{},
	fieldNames map[string]struct{},
	constructors map[string]struct{},
	aliases map[string]struct{},
) bool {
	changed := false

	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.AssignStmt:
			for index, right := range value.Rhs {
				if index < len(value.Lhs) && recordRouterAssignment(value.Lhs[index], right, identifiers, fieldNames, constructors, aliases) {
					changed = true
				}
			}
		case *ast.ValueSpec:
			for index, right := range value.Values {
				if index < len(value.Names) && recordRouterAssignment(value.Names[index], right, identifiers, fieldNames, constructors, aliases) {
					changed = true
				}
			}
		}

		return true
	})

	return changed
}

func routerFieldNames(field *ast.Field) []*ast.Ident {
	if len(field.Names) > 0 {
		return field.Names
	}

	expression := field.Type
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}

	if selector, ok := expression.(*ast.SelectorExpr); ok {
		return []*ast.Ident{selector.Sel}
	}

	return nil
}

func recordRouterAssignment(
	leftExpression ast.Expr,
	right ast.Expr,
	identifiers map[string]struct{},
	fieldNames map[string]struct{},
	constructors map[string]struct{},
	aliases map[string]struct{},
) bool {
	left, ok := leftExpression.(*ast.Ident)
	if !ok {
		return false
	}

	changed := false

	if selectsRouterConstructor(right, aliases) {
		if _, exists := constructors[left.Name]; !exists {
			constructors[left.Name] = struct{}{}
			changed = true
		}
	}

	if selectsRouter(right, identifiers, fieldNames, constructors, aliases) {
		if _, exists := identifiers[left.Name]; !exists {
			identifiers[left.Name] = struct{}{}
			changed = true
		}
	}

	return changed
}

func isHTTPRouterType(expression ast.Expr, aliases map[string]struct{}) bool {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}

	if identifier, ok := expression.(*ast.Ident); ok && identifier.Name == "Router" {
		_, dotImported := aliases["."]

		return dotImported
	}

	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Router" {
		return false
	}

	packageName, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	_, ok = aliases[packageName.Name]

	return ok
}

func selectsRouterConstructor(expression ast.Expr, aliases map[string]struct{}) bool {
	if identifier, ok := expression.(*ast.Ident); ok && identifier.Name == "New" {
		_, dotImported := aliases["."]

		return dotImported
	}

	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "New" {
		return false
	}

	packageName, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	_, ok = aliases[packageName.Name]

	return ok
}

func findRouterProviderFunctions(file *ast.File, aliases map[string]struct{}) map[string]struct{} {
	providers := map[string]struct{}{}

	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Type.Results == nil {
			return true
		}

		for _, result := range function.Type.Results.List {
			if isHTTPRouterType(result.Type, aliases) {
				providers[function.Name.Name] = struct{}{}

				break
			}
		}

		return true
	})

	return providers
}

func callsRouterProvider(expression ast.Expr, providers map[string]struct{}) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}

	switch function := call.Fun.(type) {
	case *ast.Ident:
		_, ok = providers[function.Name]

		return ok
	case *ast.SelectorExpr:
		return function.Sel.Name == "Router"
	default:
		return false
	}
}

func selectsRouter(
	expression ast.Expr,
	identifiers map[string]struct{},
	fieldNames map[string]struct{},
	constructors map[string]struct{},
	aliases map[string]struct{},
) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		_, ok := identifiers[value.Name]

		return ok
	case *ast.CallExpr:
		if identifier, ok := value.Fun.(*ast.Ident); ok {
			_, ok = constructors[identifier.Name]
			if ok {
				return true
			}

			return selectsRouterConstructor(value.Fun, aliases)
		}

		return selectsRouterConstructor(value.Fun, aliases)
	case *ast.SelectorExpr:
		if _, ok := fieldNames[value.Sel.Name]; ok {
			return true
		}

		return selectsRouter(value.X, identifiers, fieldNames, constructors, aliases)
	default:
		return false
	}
}
