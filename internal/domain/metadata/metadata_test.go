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

package metadata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsMetadataField(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "ObjectMeta",
			input:    "ObjectMeta",
			expected: true,
		},
		{
			name:     "ListMeta",
			input:    "ListMeta",
			expected: true,
		},
		{
			name:     "TypeMeta",
			input:    "TypeMeta",
			expected: true,
		},
		{
			name:     "qualified ObjectMeta",
			input:    "v1.ObjectMeta",
			expected: true,
		},
		{
			name:     "qualified ObjectMeta with alias",
			input:    "metav1.ObjectMeta",
			expected: true,
		},
		{
			name:     "qualified ListMeta",
			input:    "v1.ListMeta",
			expected: true,
		},
		{
			name:     "qualified ListMeta with alias",
			input:    "metav1.ListMeta",
			expected: true,
		},
		{
			name:     "qualified TypeMeta",
			input:    "v1.TypeMeta",
			expected: true,
		},
		{
			name:     "qualified TypeMeta with alias",
			input:    "metav1.TypeMeta",
			expected: true,
		},
		{
			name:     "Condition",
			input:    "Condition",
			expected: true,
		},
		{
			name:     "qualified Condition with alias",
			input:    "metav1.Condition",
			expected: true,
		},
		{
			name:     "regular field",
			input:    "Name",
			expected: false,
		},
		{
			name:     "another regular field",
			input:    "Count",
			expected: false,
		},
		{
			name:     "similar but not metadata",
			input:    "Metadata",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMetadataField(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTypeMeta(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "TypeMeta",
			input:    "TypeMeta",
			expected: true,
		},
		{
			name:     "qualified TypeMeta",
			input:    "v1.TypeMeta",
			expected: true,
		},
		{
			name:     "qualified TypeMeta with alias",
			input:    "metav1.TypeMeta",
			expected: true,
		},
		{
			name:     "ObjectMeta",
			input:    "ObjectMeta",
			expected: false,
		},
		{
			name:     "ListMeta",
			input:    "ListMeta",
			expected: false,
		},
		{
			name:     "qualified ObjectMeta",
			input:    "metav1.ObjectMeta",
			expected: false,
		},
		{
			name:     "regular field",
			input:    "Name",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTypeMeta(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
