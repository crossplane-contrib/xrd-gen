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
	"go/parser"
	"go/token"
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
