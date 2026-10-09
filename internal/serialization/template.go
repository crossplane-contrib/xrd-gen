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
	"go/format"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/crossplane-contrib/xrd-gen/internal/parsing"
)

const (
	runtimeImportPath = "k8s.io/apimachinery/pkg/runtime"
	runtimeAlias      = "runtime"
	metav1ImportPath  = "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1Alias       = "metav1"
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

	return formatted, nil
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
// imports: map to collect import path -> alias
func collectImports(fieldType parsing.FieldType, packageName string, imports map[string]string) {
	switch t := fieldType.(type) {
	case parsing.MetadataType:
		if t.ImportPath == "" {
			return
		}
		// Always use metav1 alias for consistency with Kubernetes conventions
		if t.ImportPath == metav1ImportPath {
			imports[t.ImportPath] = metav1Alias
		} else {
			imports[t.ImportPath] = t.Package
		}
	case parsing.TimeType:
		if t.ImportPath != "" {
			imports[t.ImportPath] = t.Package
		}
	case parsing.StructType:
		if t.Package == packageName || t.ImportPath == "" {
			return // Skip local types
		}
		imports[t.ImportPath] = t.Package
	case parsing.ArrayType:
		collectImports(t.ElementType, packageName, imports)
	case parsing.MapType:
		collectImports(t.KeyType, packageName, imports)
		collectImports(t.ValueType, packageName, imports)
	}
}

// sortImports converts a map of imports to a sorted slice of importInfo
// for deterministic output
func sortImports(importsMap map[string]string) []importInfo {
	importPaths := make([]string, 0, len(importsMap))
	for path := range importsMap {
		importPaths = append(importPaths, path)
	}
	sort.Strings(importPaths)

	imports := make([]importInfo, 0, len(importPaths))
	for _, path := range importPaths {
		imports = append(imports, importInfo{
			Alias: importsMap[path],
			Path:  path,
		})
	}
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
