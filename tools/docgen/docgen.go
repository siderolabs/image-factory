// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
)

// FieldDoc represents a single configuration parameter documentation entry.
type FieldDoc struct {
	Path        string
	Type        string
	Description string
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if len(os.Args) < 3 {
		return fmt.Errorf("usage: %s <file.go> <output.md>", os.Args[0])
	}

	inputFile := os.Args[1]
	outputFile := os.Args[2]

	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, inputFile, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse Go file: %w", err)
	}

	// Map of struct names to their AST definitions
	structDefs := collectStructs(node)

	root, ok := structDefs["Options"]
	if !ok {
		return fmt.Errorf("struct 'Options' not found in %s", inputFile)
	}

	var docs []FieldDoc
	walkStruct("", root, structDefs, &docs)

	help, err := collectUsage(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect CLI usage: %w", err)
	}

	dir := filepath.Dir(outputFile)

	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	f, err := os.OpenFile(outputFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	if err := printMarkdown(docs, help, f); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	return nil
}

// collectStructs finds all struct type declarations in the file.
func collectStructs(f *ast.File) map[string]*ast.StructType {
	out := make(map[string]*ast.StructType)

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}

		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			st, ok := ts.Type.(*ast.StructType)
			if ok {
				out[ts.Name.Name] = st
			}
		}
	}

	return out
}

// isBasicType returns true if the expression represents a "basic" leaf type.
func isBasicType(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string", "bool", "int", "int32", "int64", "uint", "uint32", "uint64", "float32", "float64":
			return true
		}
	case *ast.SelectorExpr:
		// Check for time.Duration
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name == "time" && t.Sel.Name == "Duration"
		}
	case *ast.StarExpr:
		return isBasicType(t.X)
	case *ast.ArrayType:
		return isBasicType(t.Elt)
	}

	return false
}

// walkStruct traverses struct fields recursively but only documents basic types.
func walkStruct(
	prefix string,
	st *ast.StructType,
	structDefs map[string]*ast.StructType,
	out *[]FieldDoc,
) {
	for _, field := range st.Fields.List {
		if field.Tag == nil || len(field.Names) == 0 {
			continue
		}

		tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))

		key := tag.Get("koanf")
		if key == "" || key == "-" {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		// Recurse into nested structs regardless of documenting current field
		if ident, ok := field.Type.(*ast.Ident); ok {
			if nested, exists := structDefs[ident.Name]; exists {
				walkStruct(path, nested, structDefs, out)

				continue // Do not document the struct itself
			}
		}

		// Only document if it's a basic type or time.Duration
		if isBasicType(field.Type) {
			typ := exprString(field.Type)

			desc := ""

			if field.Doc != nil {
				desc = strings.TrimSpace(field.Doc.Text())
			}

			*out = append(*out, FieldDoc{
				Path:        path,
				Type:        typ,
				Description: desc,
			})
		}
	}
}

// exprString converts an AST expression to a readable type string.
func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	default:
		return "unknown"
	}
}

// collectUsage runs the CLI help command and captures the output.
func collectUsage(ctx context.Context) (string, error) {
	cmdHelp := exec.CommandContext(ctx, "go", "run", "./cmd/image-factory", "-h")

	buf := new(bytes.Buffer)

	cmdHelp.Stderr = buf

	if err := cmdHelp.Run(); err != nil {
		return "", fmt.Errorf("help command failed: %w", err)
	}

	return buf.String(), nil
}

// printMarkdown generates the final documentation file.
func printMarkdown(docs []FieldDoc, help string, wr io.Writer) error {
	var b strings.Builder

	b.WriteString("# Configuration\n\n")
	b.WriteString("## CLI Usage\n\n")
	b.WriteString("```console\n")
	b.WriteString(help)
	b.WriteString("```\n\n")

	b.WriteString("## Configuration Reference\n\n")
	b.WriteString("Documentation for basic configuration parameters.\n\n")

	for _, d := range docs {
		fmt.Fprintf(&b, "### `%s`\n\n", d.Path)
		fmt.Fprintf(&b, "- **Type:** `%s`\n", d.Type)
		fmt.Fprintf(&b, "- **Env:** `%s`\n\n", strings.ReplaceAll(strings.ToUpper(d.Path), ".", "_"))

		if d.Description != "" {
			b.WriteString(d.Description)
			b.WriteString("\n\n")
		}

		b.WriteString("---\n\n")
	}

	// Default configuration handling via Koanf
	k := koanf.New(".")
	if err := k.Load(structs.Provider(cmd.DefaultOptions, "koanf"), nil); err != nil {
		return fmt.Errorf("failed to load default options: %w", err)
	}

	yb, err := k.Marshal(yaml.Parser())
	if err != nil {
		return fmt.Errorf("failed to marshal yaml: %w", err)
	}

	b.WriteString("## Default Configuration\n\n")
	b.WriteString("### YAML\n\n")
	b.WriteString("```yaml\n")
	b.Write(yb)
	b.WriteString("```\n\n")

	b.WriteString("### Environment Variables\n\n")
	b.WriteString("```env\n")

	flat := make(map[string]string)
	flatten("IF", k.All(), flat)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, flat[k])
	}

	b.WriteString("```\n")

	_, err = io.WriteString(wr, b.String())

	return err
}

// flatten recursively flattens a map for environment variable representation.
func flatten(prefix string, in map[string]any, out map[string]string) {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}

		switch val := v.(type) {
		case map[string]any:
			flatten(key, val, out)
		default:
			envKey := strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
			out[envKey] = fmt.Sprint(val)
		}
	}
}
