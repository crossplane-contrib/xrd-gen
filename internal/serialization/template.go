/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package serialization

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/crossplane-contrib/xrd-gen/internal/parsing"
)

const (
	runtimeImportPath = "k8s.io/apimachinery/pkg/runtime"
	runtimeAlias      = "runtime"
)

type importInfo struct {
	Alias string
	Path  string
}

func prepareStructsForGeneration(structs []parsing.Struct, headerFile string) (header, packageName string, targetStructs []parsing.Struct, err error) {
	if headerFile != "" {
		headerBytes, err := os.ReadFile(headerFile)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to read header file: %w", err)
		}
		header = formatHeaderAsComment(string(headerBytes))
	}

	packageName = getPackageName(structs)
	targetStructs = filterByPackage(structs, packageName)

	sort.Slice(targetStructs, func(i, j int) bool {
		return targetStructs[i].Name < targetStructs[j].Name
	})

	return header, packageName, targetStructs, nil
}

func executeTemplate(tmplName, tmplContent string, data interface{}) ([]byte, error) {
	tmpl, err := template.New(tmplName).Parse(tmplContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to format generated code: %w", err)
	}

	pruned, err := removeUnusedImports(formatted)
	if err != nil {
		return nil, fmt.Errorf("failed to remove unused imports from generated code: %w", err)
	}

	return pruned, nil
}

// Imports are collected from every field with copy code, but some of that code
// never names the field's type, so imports it never references are dropped.
// Usage is checked per alias, since the same package can be imported under
// more than one alias.
func removeUnusedImports(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	referenced := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Obj == nil {
				referenced[ident.Name] = true
			}
		}
		return true
	})

	type namedImport struct{ name, path string }
	var unused []namedImport
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		name := alias
		if name == "" {
			name = path[strings.LastIndex(path, "/")+1:]
		}
		if name == "_" || name == "." || referenced[name] {
			continue
		}
		unused = append(unused, namedImport{name: alias, path: path})
	}
	if len(unused) == 0 {
		return src, nil
	}

	for _, imp := range unused {
		astutil.DeleteNamedImport(fset, file, imp.name, imp.path)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatHeaderAsComment(header string) string {
	if header == "" {
		return ""
	}

	lines := strings.Split(strings.TrimRight(header, "\n"), "\n")
	var commentedLines []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			commentedLines = append(commentedLines, line)
		} else {
			commentedLines = append(commentedLines, "// "+line)
		}
	}
	return strings.Join(commentedLines, "\n")
}

func getPackageName(structs []parsing.Struct) string {
	for _, s := range structs {
		if s.IsRoot {
			return s.Package
		}
	}
	if len(structs) > 0 {
		return structs[0].Package
	}
	return ""
}

func filterByPackage(structs []parsing.Struct, packageName string) []parsing.Struct {
	var filtered []parsing.Struct
	for _, s := range structs {
		if s.Package == packageName {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// collectImports recursively collects all imports needed for a field type
// packageName: the current package name (to filter out local types)
// imports: set of alias and path pairs. Source files can import the same
// package under different aliases, and the copy code keeps each field's alias.
func collectImports(fieldType parsing.FieldType, packageName string, imports map[importInfo]bool) {
	switch t := fieldType.(type) {
	case parsing.MetadataType:
		if t.ImportPath != "" {
			imports[importInfo{Alias: t.Package, Path: t.ImportPath}] = true
		}
	case parsing.TimeType:
		if t.ImportPath != "" {
			imports[importInfo{Alias: t.Package, Path: t.ImportPath}] = true
		}
	case parsing.StructType:
		if t.Package == packageName || t.ImportPath == "" {
			return // Skip local types
		}
		imports[importInfo{Alias: t.Package, Path: t.ImportPath}] = true
	case parsing.ArrayType:
		collectImports(t.ElementType, packageName, imports)
	case parsing.MapType:
		collectImports(t.KeyType, packageName, imports)
		collectImports(t.ValueType, packageName, imports)
	}
}

// sortImports converts a set of imports to a slice sorted by path, then alias
func sortImports(importSet map[importInfo]bool) []importInfo {
	imports := make([]importInfo, 0, len(importSet))
	for imp := range importSet {
		imports = append(imports, imp)
	}
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Path != imports[j].Path {
			return imports[i].Path < imports[j].Path
		}
		return imports[i].Alias < imports[j].Alias
	})
	return imports
}

// getTypeName returns the Go type name for a field type, with proper package qualification
func getTypeName(fieldType parsing.FieldType, packageName string) string {
	switch t := fieldType.(type) {
	case parsing.PrimitiveType:
		return t.Name
	case parsing.NumericType:
		return t.Name
	case parsing.StringType:
		return t.Name()
	case parsing.TimeType:
		return t.FullyQualifiedName()
	case parsing.MetadataType:
		return t.FullyQualifiedName()
	case parsing.StructType:
		if t.Package == packageName { // if we are in the same package, we need to omit the package prefix
			return t.Name
		}
		return t.FullyQualifiedName()
	case parsing.ArrayType:
		return "[]" + getTypeName(t.ElementType, packageName)
	case parsing.MapType:
		return fmt.Sprintf("map[%s]%s", getTypeName(t.KeyType, packageName), getTypeName(t.ValueType, packageName))
	default:
		return "unknown"
	}
}
