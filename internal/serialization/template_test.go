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
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func requireCodeEquals(t *testing.T, expected, actual string) {
	t.Helper()
	// Verify the actual code is syntactically correct
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "unused", actual, parser.AllErrors)
	require.NoError(t, err, "Generated code has syntax errors")

	expectedNorm := normalizeCode(expected)
	actualNorm := normalizeCode(actual)

	if expectedNorm != actualNorm {
		t.Errorf("Code mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}

func normalizeCode(code string) string {
	lines := strings.Split(code, "\n")
	var normalized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, "\n")
}

// Expected imports are given as `alias "path"`. Every imported package must
// also be referenced, otherwise the generated code does not compile.
func requireImports(t *testing.T, expected []string, code string) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "", code, 0)
	require.NoError(t, err, "Generated code has syntax errors")

	usedQualifiers := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				usedQualifiers[ident.Name] = true
			}
		}
		return true
	})

	actual := make([]string, 0, len(file.Imports))
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)
		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		require.Truef(t, usedQualifiers[name], "%q is imported as %s and not used", path, name)
		actual = append(actual, name+" "+imp.Path.Value)
	}
	require.ElementsMatch(t, expected, actual)
}

func TestRemoveUnusedImports(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"keeps used imports and drops unused ones": {
			input: `package example

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	otherpkg "github.com/example/otherpkg"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"time"
)

var _ runtime.Object
var _ time.Time
`,
			expected: `package example

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	"time"
)

var _ runtime.Object
var _ time.Time
`,
		},
		"drops the import block when no import is used": {
			input: `package example

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Example struct{}
`,
			expected: `package example

type Example struct{}
`,
		},
		"keeps blank and dot imports": {
			input: `package example

import (
	_ "embed"
	. "github.com/example/dotpkg"
	unused "github.com/example/unused"
)
`,
			expected: `package example

import (
	_ "embed"
	. "github.com/example/dotpkg"
)
`,
		},
		"names unaliased imports after the last path element": {
			input: `package example

import (
	"github.com/example/unused"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ sets.Empty
`,
			expected: `package example

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ sets.Empty
`,
		},
		"checks each alias of the same package separately": {
			input: `package example

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ metav1.Condition
`,
			expected: `package example

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ metav1.Condition
`,
		},
		"leaves code without unused imports unchanged": {
			input: `package example

import "testing"

func TestExample(t *testing.T) {}
`,
			expected: `package example

import "testing"

func TestExample(t *testing.T) {}
`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := removeUnusedImports([]byte(tc.input))
			require.NoError(t, err)
			require.Equal(t, tc.expected, string(actual))
		})
	}
}

func TestRemoveUnusedImports_InvalidSource(t *testing.T) {
	_, err := removeUnusedImports([]byte("package example\n\nfunc {"))
	require.Error(t, err)
}
