// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchitectureBoundaryMigrationLedger(t *testing.T) {
	t.Parallel()

	// These are the four packages containing the extracted application services,
	// not just their *_service.go files. Infrastructure subpackages are not
	// application packages. All frontend subpackages are checked separately.
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))

	var serviceBlockers []string

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(filename string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)

		if entry.IsDir() || filepath.Ext(filename) != ".go" || strings.HasSuffix(filename, "_test.go") {
			return nil
		}

		relative, err := filepath.Rel(root, filename)
		require.NoError(t, err)

		relative = filepath.ToSlash(relative)
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.SkipObjectResolution)
		require.NoError(t, err)

		for _, finding := range architectureFindings(path.Dir(relative), file) {
			if strings.HasPrefix(finding, "service imports ") {
				serviceBlockers = append(serviceBlockers, relative+": "+finding)
			} else {
				t.Errorf("%s: %s", relative, finding)
			}
		}

		return nil
	})
	require.NoError(t, err)
	// This is a migration ledger, NOT a clean-architecture assertion. Keep known
	// package-level blockers visible while preventing new debt; remove entries as
	// packages migrate and replace this ledger with require.Empty when complete.
	for _, blocker := range serviceBlockers {
		t.Logf("REMAINING TASK11 ACCEPTANCE BLOCKER: %s", blocker)
	}

	require.Empty(t, serviceBlockers)
}

func TestArchitectureBoundaryCounterexamples(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		directory string
		source    string
		want      []string
	}{
		{
			name:      "service HTTP alias",
			directory: "internal/artifacts",
			source:    `package artifacts; import web "net/http"`,
			want:      []string{"service imports net/http"},
		},
		{
			name:      "service router dot import",
			directory: "internal/registry",
			source:    `package registry; import . "github.com/julienschmidt/httprouter"`,
			want:      []string{"service imports github.com/julienschmidt/httprouter"},
		},
		{
			name:      "endpoint parent",
			directory: "internal/frontend/http/api",
			source:    `package api; import parent "github.com/siderolabs/image-factory/internal/frontend/http"`,
			want:      []string{"endpoint imports internal/frontend/http"},
		},
		{
			name:      "endpoint sibling",
			directory: "internal/frontend/http/api",
			source:    `package api; import "github.com/siderolabs/image-factory/internal/frontend/http/ui"`,
			want:      []string{"endpoint imports internal/frontend/http/ui"},
		},
		{
			name:      "builder constructor alias",
			directory: "internal/frontend/http/api",
			source:    `package api; import infra "github.com/siderolabs/image-factory/internal/asset"; var build = infra.NewBuilder`,
			want:      []string{"endpoint constructs internal/asset.NewBuilder"},
		},
		{
			name:      "storage literal",
			directory: "internal/frontend/http/api",
			source:    `package api; import infra "github.com/siderolabs/image-factory/internal/schematic/storage/registry"; var storage = &infra.Storage{}`,
			want:      []string{"endpoint constructs internal/schematic/storage/registry.Storage"},
		},
		{
			name:      "puller constructor",
			directory: "internal/frontend/http/oci",
			source:    `package oci; import remote "github.com/siderolabs/image-factory/internal/remotewrap"; var puller = remote.NewPuller()`,
			want:      []string{"endpoint constructs internal/remotewrap.NewPuller"},
		},
		{
			name:      "pusher constructor",
			directory: "internal/frontend/http/oci",
			source:    `package oci; import remote "github.com/siderolabs/image-factory/internal/remotewrap"; var pusher = remote.NewPusher()`,
			want:      []string{"endpoint constructs internal/remotewrap.NewPusher"},
		},
		{
			name:      "signer constructor",
			directory: "internal/frontend/http/oci",
			source:    `package oci; import signing "github.com/siderolabs/image-factory/internal/image/signer"; var signer = signing.NewGSASigner()`,
			want:      []string{"endpoint constructs internal/image/signer.NewGSASigner"},
		},
		{
			name:      "concrete manager port",
			directory: "internal/frontend/http/api",
			source:    `package api; import infra "github.com/siderolabs/image-factory/internal/artifacts"; func New(manager *infra.Manager) {}`,
			want:      []string{"endpoint constructs internal/artifacts.Manager"},
		},
		{
			name:      "dot imported infrastructure",
			directory: "internal/frontend/http/api",
			source:    `package api; import . "github.com/siderolabs/image-factory/internal/asset"; var builder = NewBuilder()`,
			want:      []string{"endpoint dot imports internal/asset"},
		},
		{
			name:      "inactive enterprise file",
			directory: "internal/asset",
			source:    "//go:build enterprise\n\npackage asset\nimport _ \"net/http\"",
			want:      []string{"service imports net/http"},
		},
		{
			name:      "allowed service",
			directory: "internal/registry",
			source:    `package registry; import "context"`,
		},
		{
			name:      "allowed narrow application",
			directory: "internal/frontend/http/api",
			source:    `package api; import "github.com/siderolabs/image-factory/internal/asset"; type Application interface { Get() asset.ImageResult }`,
		},
		{
			name:      "allowed transport",
			directory: "internal/frontend/http/ui",
			source:    `package ui; import "github.com/siderolabs/image-factory/internal/frontend/http/transport"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", test.source, parser.SkipObjectResolution)
			require.NoError(t, err)
			require.Equal(t, test.want, architectureFindings(test.directory, file))
		})
	}
}

// architectureFindings checks direct source dependencies, including inactive build-tag
// files. It is deliberately not a transitive dependency or interface-width proof.
func architectureFindings(directory string, file *ast.File) []string {
	const frontend = "internal/frontend/http"

	service := slices.Contains([]string{"internal/artifacts", "internal/asset", "internal/schematic", "internal/registry"}, directory)
	endpoint := strings.HasPrefix(directory, frontend+"/")
	aliases := make(map[string]string, len(file.Imports))

	var findings []string

	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			panic(err)
		}

		imported = strings.TrimPrefix(imported, "github.com/siderolabs/image-factory/")

		name := path.Base(imported)
		if spec.Name != nil {
			name = spec.Name.Name
		}

		aliases[name] = imported
		if service && (imported == "net/http" || strings.HasPrefix(imported, "net/http/") || imported == "github.com/julienschmidt/httprouter") {
			findings = append(findings, "service imports "+imported)
		}

		if endpoint && forbiddenEndpointImport(directory, imported) {
			findings = append(findings, "endpoint imports "+imported)
		}
		// Dot imports hide the provenance of constructors and types from this AST
		// check; reject them rather than silently accepting an unanalyzed dependency.
		if endpoint && name == "." && strings.HasPrefix(imported, "internal/") {
			findings = append(findings, "endpoint dot imports "+imported)
		}
	}

	if !endpoint {
		return findings
	}

	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		identifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}

		imported := aliases[identifier.Name]
		if infrastructureSymbol(imported, selector.Sel.Name) {
			findings = append(findings, "endpoint constructs "+imported+"."+selector.Sel.Name)
		}

		return true
	})

	return findings
}

func forbiddenEndpointImport(directory, imported string) bool {
	const frontend = "internal/frontend/http"

	return (imported == frontend || strings.HasPrefix(imported, frontend+"/")) &&
		!slices.Contains([]string{directory, frontend + "/transport", frontend + "/authentication"}, imported)
}

// Infrastructure constructors are forbidden even as function values. Concrete
// infrastructure types are forbidden too, preventing literals/new and broad ports.
// Application result types and consumer interfaces remain legal dependencies.
func infrastructureSymbol(imported, symbol string) bool {
	switch imported {
	case "internal/asset":
		return symbol == "NewBuilder" || symbol == "Builder"
	case "internal/artifacts":
		return symbol == "NewManager" || symbol == "Manager"
	case "internal/schematic":
		return symbol == "NewFactory" || symbol == "Factory"
	case "internal/remotewrap":
		return symbol == "NewPuller" || symbol == "NewPusher"
	case "internal/image/signer":
		return strings.HasPrefix(symbol, "New") || symbol == "KeySigner" || symbol == "GSASigner"
	default:
		return (strings.HasPrefix(imported, "internal/schematic/storage/") || strings.HasPrefix(imported, "internal/asset/cache/")) &&
			(strings.HasPrefix(symbol, "New") || symbol == "Storage" || symbol == "Cache")
	}
}
