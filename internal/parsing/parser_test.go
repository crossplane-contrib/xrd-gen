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

package parsing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParser_ParseFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// Should be ignored because it is an irrelevant import
import "fmt"

// Should be ignored becase it is an interface
type SomeInterface interface {
	Method() string
}

// Should be ignored because it is a constant
const something = "something"

// Should be ignored because it is a variable
var somethingElse = fmt.Sprintf("something else %s", something)

// +xrd-gen:object:root=true
// MyDB holds the specification of a database cluster.
type MyDB struct {
	// Version of the database engine.
	Version string ` + "`json:\"version\"`" + `

	// Image is the database Docker image to deploy.
	// +xrd-gen:validation:Optional
	Image string ` + "`json:\"image,omitempty\"`" + `

	// +xrd-gen:validation:MinItems=1
	// NodeSets allow specifying groups of database nodes.
	NodeSets []NodeSet ` + "`json:\"nodeSets\"`" + `
}

type NodeSet struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 2)

	db := structs[0]
	require.Equal(t, "MyDB", db.Name)
	require.True(t, db.IsRoot)
	require.Len(t, db.Fields, 3)

	nodeSet := structs[1]
	require.Equal(t, "NodeSet", nodeSet.Name)
	require.False(t, nodeSet.IsRoot)
	require.Len(t, nodeSet.Fields, 1)
	require.Equal(t, "Name", nodeSet.Fields[0].Name)
	require.Equal(t, "name", nodeSet.Fields[0].JSONName)

	versionField := db.Fields[0]
	require.Equal(t, "Version", versionField.Name)
	require.Equal(t, "version", versionField.JSONName)
	require.False(t, versionField.IsOptional)

	imageField := db.Fields[1]
	require.True(t, imageField.IsOptional)

	nodeSetsField := db.Fields[2]
	arrayType, ok := nodeSetsField.Type.(ArrayType)
	require.True(t, ok)
	require.NotNil(t, arrayType.MinItems)
	require.Equal(t, 1, *arrayType.MinItems)
}

func TestParser_ParseArrayWithMaxItems(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:MinItems=1
	// +xrd-gen:validation:MaxItems=10
	// Tags with constraints
	Tags []string ` + "`json:\"tags\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 1)

	tagsField := testStruct.Fields[0]
	require.Equal(t, "Tags", tagsField.Name)
	require.Equal(t, "tags", tagsField.JSONName)

	arrayType, ok := tagsField.Type.(ArrayType)
	require.True(t, ok, "Expected Tags field to be ArrayType")
	require.NotNil(t, arrayType.MinItems)
	require.Equal(t, 1, *arrayType.MinItems)
	require.NotNil(t, arrayType.MaxItems)
	require.Equal(t, 10, *arrayType.MaxItems)
}

func TestExtractJSONName(t *testing.T) {
	parser := NewParser()
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{
			name: "simple tag",
			tag:  `json:"fieldName"`,
			want: "fieldName",
		},
		{
			name: "tag with omitempty",
			tag:  `json:"fieldName,omitempty"`,
			want: "fieldName",
		},
		{
			name: "tag with inline",
			tag:  `json:",inline"`,
			want: "",
		},
		{
			name: "multiple tags",
			tag:  `yaml:"field" json:"fieldName"`,
			want: "fieldName",
		},
		{
			name: "no json tag",
			tag:  `yaml:"field"`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractJSONName(tt.tag)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseAnnotations(t *testing.T) {
	tests := []struct {
		name string
		text string
		want map[string]string
	}{
		{
			name: "single annotation",
			text: "// +xrd-gen:validation:MinItems=1",
			want: map[string]string{"validation:MinItems": "1"},
		},
		{
			name: "boolean annotation",
			text: "// +xrd-gen:object:root=true",
			want: map[string]string{"object:root": "true"},
		},
		{
			name: "flag annotation",
			text: "// +xrd-gen:validation:Optional",
			want: map[string]string{"validation:Optional": "true"},
		},
		{
			name: "multiple annotations",
			text: "// +xrd-gen:validation:MinItems=1\n// +xrd-gen:validation:MaxItems=10",
			want: map[string]string{
				"validation:MinItems": "1",
				"validation:MaxItems": "10",
			},
		},
		{
			name: "mixed with regular comments",
			text: "// This is a regular comment\n// +xrd-gen:validation:Optional\n// Another comment",
			want: map[string]string{"validation:Optional": "true"},
		},
		{
			name: "mix of spaces and newlines",
			text: "\n//+xrd-gen:validation:MinItems=2\n\n// +xrd-gen:validation:MaxItems=5\n\t",
			want: map[string]string{
				"validation:MinItems": "2",
				"validation:MaxItems": "5",
			},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.parseCommentAnnotations(tt.text)
			require.Len(t, got, len(tt.want))
			for key, wantVal := range tt.want {
				require.Equal(t, wantVal, got[key])
			}
		})
	}
}

func TestParser_SkipsTestFilesAndDeepCopyFiles(t *testing.T) {
	tempDir := t.TempDir()

	testFile1 := filepath.Join(tempDir, "types.go")
	testCode1 := `package test

type MyStruct struct {
	Field string ` + "`json:\"field\"`" + `
}
`

	testFile2 := filepath.Join(tempDir, "types2.go")
	testCode2 := `package test

type AnotherStruct struct {
	Value int ` + "`json:\"value\"`" + `
}
`

	testFileTest := filepath.Join(tempDir, "types_test.go")
	testCodeTest := `package test

type TestStruct struct {
	Test string ` + "`json:\"test\"`" + `s
}
`

	deepCopyFile := filepath.Join(tempDir, "zz_generated.deepcopy.go")
	deepCopyCode := `package test

type DeepCopyStruct struct {
	Field string ` + "`json:\"field\"`" + `
}
`

	err := os.WriteFile(testFile1, []byte(testCode1), 0644)
	require.NoError(t, err)
	err = os.WriteFile(testFile2, []byte(testCode2), 0644)
	require.NoError(t, err)
	err = os.WriteFile(testFileTest, []byte(testCodeTest), 0644)
	require.NoError(t, err)
	err = os.WriteFile(deepCopyFile, []byte(deepCopyCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.ParseDirectory(tempDir)
	require.NoError(t, err)
	require.Len(t, structs, 2, "test and deep copy files should be skipped")
}

func TestParseComments(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		expected string
	}{
		{
			name:     "simple description",
			doc:      "This is a simple description.",
			expected: "This is a simple description.",
		},
		{
			name:     "description with annotation",
			doc:      "This is a description.\n+xrd-gen:validation:Optional",
			expected: "This is a description.",
		},
		{
			name:     "multiline description",
			doc:      "This is line one.\nThis is line two.",
			expected: "This is line one. This is line two.",
		},
		{
			name:     "multiline description with spaces",
			doc:      "   This is line one.\nThis is line two. \n \t",
			expected: "This is line one. This is line two.",
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.parseComments(tt.doc)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestParser_ParsePluralAnnotation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// +xrd-gen:object:root
// +xrd-gen:groupName=example.com
// +xrd-gen:version=v1
// +xrd-gen:plural=mycustomplural
// MyResource is a test resource
type MyResource struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	myResource := structs[0]
	require.Equal(t, "MyResource", myResource.Name)
	require.Equal(t, "mycustomplural", myResource.Plural)
	require.Equal(t, "example.com", myResource.Group)
	require.Equal(t, "v1", myResource.Version)
}

func TestParser_ParseNoPluralAnnotation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// +xrd-gen:object:root
// +xrd-gen:groupName=example.com
// +xrd-gen:version=v1alpha1
// MyResource is a test resource
type MyResource struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	myResource := structs[0]
	require.Equal(t, "MyResource", myResource.Name)
	require.Equal(t, "", myResource.Plural) // Should be empty if not provided
	require.Equal(t, "example.com", myResource.Group)
	require.Equal(t, "v1alpha1", myResource.Version)
}

func TestParser_ParsePatternAnnotation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:Pattern=^[a-z0-9-]+$
	// Name must be lowercase alphanumeric with hyphens
	Name string ` + "`json:\"name\"`" + `
	
	// Age without pattern
	Age int32 ` + "`json:\"age\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 2)

	nameField := testStruct.Fields[0]
	require.Equal(t, "Name", nameField.Name)
	stringType, ok := nameField.Type.(StringType)
	require.True(t, ok, "Name should be StringType")
	require.Equal(t, "^[a-z0-9-]+$", stringType.Pattern)

	ageField := testStruct.Fields[1]
	require.Equal(t, "Age", ageField.Name)
	numericType, ok := ageField.Type.(NumericType)
	require.True(t, ok, "Age should be NumericType")
	require.Equal(t, "int32", numericType.Name)
}

func TestParser_ParseStringLengthAnnotations(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:MinLength=3
	// +xrd-gen:validation:MaxLength=50
	// Username with length constraints
	Username string ` + "`json:\"username\"`" + `
	
	// +xrd-gen:validation:MinLength=10
	// Password with min length only
	Password string ` + "`json:\"password\"`" + `
	
	// Name without constraints
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 3)

	// Check Username with both min and max length
	usernameField := testStruct.Fields[0]
	require.Equal(t, "Username", usernameField.Name)
	stringType, ok := usernameField.Type.(StringType)
	require.True(t, ok, "Username should be StringType")
	require.NotNil(t, stringType.MinLength)
	require.Equal(t, 3, *stringType.MinLength)
	require.NotNil(t, stringType.MaxLength)
	require.Equal(t, 50, *stringType.MaxLength)

	// Check Password with min length only
	passwordField := testStruct.Fields[1]
	require.Equal(t, "Password", passwordField.Name)
	stringType, ok = passwordField.Type.(StringType)
	require.True(t, ok, "Password should be StringType")
	require.NotNil(t, stringType.MinLength)
	require.Equal(t, 10, *stringType.MinLength)
	require.Nil(t, stringType.MaxLength)

	// Check Name without constraints
	nameField := testStruct.Fields[2]
	require.Equal(t, "Name", nameField.Name)
	stringType, ok = nameField.Type.(StringType)
	require.True(t, ok, "Name should be StringType")
	require.Nil(t, stringType.MinLength)
	require.Nil(t, stringType.MaxLength)
}

func TestParser_ParseDefaultAnnotation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:Default=default-name
	// Name with default value
	Name string ` + "`json:\"name\"`" + `
	
	// +xrd-gen:validation:Default=5
	// Count with default numeric value
	Count int32 ` + "`json:\"count\"`" + `
	
	// +xrd-gen:validation:Default=true
	// Enabled with default bool value
	Enabled bool ` + "`json:\"enabled\"`" + `
	
	// +xrd-gen:validation:Default=3.14
	// Score with default float value
	Score float64 ` + "`json:\"score\"`" + `
	
	// Description without default
	Description string ` + "`json:\"description\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 5)

	nameField := testStruct.Fields[0]
	require.Equal(t, "Name", nameField.Name)
	stringType, ok := nameField.Type.(StringType)
	require.True(t, ok, "Name should be StringType")
	require.NotNil(t, stringType.Default)
	require.Equal(t, "default-name", stringType.Default)

	countField := testStruct.Fields[1]
	require.Equal(t, "Count", countField.Name)
	numericType, ok := countField.Type.(NumericType)
	require.True(t, ok, "Count should be NumericType")
	require.NotNil(t, numericType.Default)
	require.Equal(t, 5, numericType.Default)

	enabledField := testStruct.Fields[2]
	require.Equal(t, "Enabled", enabledField.Name)
	primitiveType, ok := enabledField.Type.(PrimitiveType)
	require.True(t, ok, "Enabled should be PrimitiveType")
	require.NotNil(t, primitiveType.Default)
	require.Equal(t, true, primitiveType.Default)

	scoreField := testStruct.Fields[3]
	require.Equal(t, "Score", scoreField.Name)
	numericType, ok = scoreField.Type.(NumericType)
	require.True(t, ok, "Score should be NumericType")
	require.NotNil(t, numericType.Default)
	require.Equal(t, 3.14, numericType.Default)

	descField := testStruct.Fields[4]
	require.Equal(t, "Description", descField.Name)
	stringType, ok = descField.Type.(StringType)
	require.True(t, ok, "Description should be StringType")
	require.Equal(t, "", stringType.Default)
}

func TestParser_ParseEnumAnnotation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:Enum=active,inactive,pending
	// Status with enum values
	Status string ` + "`json:\"status\"`" + `
	
	// +xrd-gen:validation:Enum=small, medium, large
	// Size with enum values (with spaces)
	Size string ` + "`json:\"size\"`" + `
	
	// Name without enum
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 3)

	// Check Status with enum values
	statusField := testStruct.Fields[0]
	require.Equal(t, "Status", statusField.Name)
	require.Len(t, statusField.EnumValues, 3)
	require.Equal(t, []string{"active", "inactive", "pending"}, statusField.EnumValues)

	// Check Size with enum values (should trim spaces)
	sizeField := testStruct.Fields[1]
	require.Equal(t, "Size", sizeField.Name)
	require.Len(t, sizeField.EnumValues, 3)
	require.Equal(t, []string{"small", "medium", "large"}, sizeField.EnumValues)

	// Check Name without enum
	nameField := testStruct.Fields[2]
	require.Equal(t, "Name", nameField.Name)
	require.Len(t, nameField.EnumValues, 0)
}

func TestParser_ParseMapFields(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// Labels is a map of string to string
	Labels map[string]string ` + "`json:\"labels\"`" + `
	
	// Config is a map with int32 values
	Config map[string]int32 ` + "`json:\"config\"`" + `
	
	// Metadata is a map with struct values
	Metadata map[string]MetadataValue ` + "`json:\"metadata\"`" + `

	// Other is a map of string to struct from another package
	Other map[string]otherpackage.OtherType ` + "`json:\"other\"`" + `
}

type MetadataValue struct {
	Value string ` + "`json:\"value\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 2)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 4)

	// Check Labels map[string]string
	labelsField := testStruct.Fields[0]
	require.Equal(t, "Labels", labelsField.Name)
	mapType, ok := labelsField.Type.(MapType)
	require.True(t, ok, "Labels should be MapType")
	stringKeyType, ok := mapType.KeyType.(StringType)
	require.True(t, ok, "Labels key should be StringType")
	require.False(t, stringKeyType.IsPointer)
	stringValueType, ok := mapType.ValueType.(StringType)
	require.True(t, ok, "Labels value should be StringType")
	require.False(t, stringValueType.IsPointer)

	// Check Config map[string]int32
	configField := testStruct.Fields[1]
	require.Equal(t, "Config", configField.Name)
	mapType, ok = configField.Type.(MapType)
	require.True(t, ok, "Config should be MapType")
	_, ok = mapType.KeyType.(StringType)
	require.True(t, ok, "Config key should be StringType")
	numericValueType, ok := mapType.ValueType.(NumericType)
	require.True(t, ok, "Config value should be NumericType")
	require.Equal(t, "int32", numericValueType.Name)

	// Check Metadata map[string]MetadataValue
	metadataField := testStruct.Fields[2]
	require.Equal(t, "Metadata", metadataField.Name)
	mapType, ok = metadataField.Type.(MapType)
	require.True(t, ok, "Metadata should be MapType")
	_, ok = mapType.KeyType.(StringType)
	require.True(t, ok, "Metadata key should be StringType")
	structValueType, ok := mapType.ValueType.(StructType)
	require.True(t, ok, "Metadata value should be StructType")
	require.Equal(t, "MetadataValue", structValueType.Name)
	require.Equal(t, "test", structValueType.Package)

	// Check Other map[string]otherpackage.OtherType
	otherField := testStruct.Fields[3]
	require.Equal(t, "Other", otherField.Name)
	mapType, ok = otherField.Type.(MapType)
	require.True(t, ok, "Other should be MapType")
	_, ok = mapType.KeyType.(StringType)
	require.True(t, ok, "Other key should be StringType")
	structValueType, ok = mapType.ValueType.(StructType)
	require.True(t, ok, "Other value should be StructType")
	require.Equal(t, "OtherType", structValueType.Name)
	require.Equal(t, "otherpackage", structValueType.Package)
}

func TestParser_ParsePointerFields(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// Name is a pointer to string
	Name *string ` + "`json:\"name,omitempty\"`" + `
	
	// Count is a pointer to int32
	Count *int32 ` + "`json:\"count,omitempty\"`" + `
	
	// Enabled is a pointer to bool
	Enabled *bool ` + "`json:\"enabled,omitempty\"`" + `
	
	// Replicas is a pointer to int
	Replicas *int ` + "`json:\"replicas,omitempty\"`" + `
	
	// Config is a pointer to a local struct
	Config *ConfigSpec ` + "`json:\"config,omitempty\"`" + `

	// Other is a pointer to a struct from another package
	Other *otherpackage.OtherType ` + "`json:\"other,omitempty\"`" + `
}

type ConfigSpec struct {
	Value string ` + "`json:\"value\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 2)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 6)

	// Check pointer to string
	nameField := testStruct.Fields[0]
	require.Equal(t, "Name", nameField.Name)
	stringType, ok := nameField.Type.(StringType)
	require.True(t, ok, "Name should be StringType")
	require.True(t, stringType.IsPointer)

	// Check pointer to int32
	countField := testStruct.Fields[1]
	require.Equal(t, "Count", countField.Name)
	numericType, ok := countField.Type.(NumericType)
	require.True(t, ok, "Count should be NumericType")
	require.Equal(t, "int32", numericType.Name)
	require.True(t, numericType.IsPointer)

	// Check pointer to bool
	enabledField := testStruct.Fields[2]
	require.Equal(t, "Enabled", enabledField.Name)
	primitiveType, ok := enabledField.Type.(PrimitiveType)
	require.True(t, ok, "Enabled should be PrimitiveType")
	require.Equal(t, "bool", primitiveType.Name)
	require.True(t, primitiveType.IsPointer)

	// Check pointer to int
	replicasField := testStruct.Fields[3]
	require.Equal(t, "Replicas", replicasField.Name)
	numericType, ok = replicasField.Type.(NumericType)
	require.True(t, ok, "Replicas should be NumericType")
	require.Equal(t, "int", numericType.Name)
	require.True(t, numericType.IsPointer)

	// Check pointer to local struct
	configField := testStruct.Fields[4]
	require.Equal(t, "Config", configField.Name)
	structType, ok := configField.Type.(StructType)
	require.True(t, ok, "Config should be StructType")
	require.Equal(t, "ConfigSpec", structType.Name)
	require.Equal(t, "test", structType.Package)
	require.True(t, structType.IsPointer)

	// Check pointer to struct from another package
	otherField := testStruct.Fields[5]
	require.Equal(t, "Other", otherField.Name)
	structType, ok = otherField.Type.(StructType)
	require.True(t, ok, "Other should be StructType")
	require.Equal(t, "OtherType", structType.Name)
	require.Equal(t, "otherpackage", structType.Package)
	require.True(t, structType.IsPointer)
}

func TestParser_ParseQualifiedMetadataTypes(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type TestResource struct {
	// TypeMeta is the K8s type metadata
	metav1.TypeMeta ` + "`json:\",inline\"`" + `
	
	// ObjectMeta is the K8s object metadata
	metav1.ObjectMeta ` + "`json:\"metadata,omitempty\"`" + `
	
	// Name is a regular field
	Name string ` + "`json:\"name\"`" + `
}

type TestResourceList struct {
	metav1.TypeMeta ` + "`json:\",inline\"`" + `
	metav1.ListMeta ` + "`json:\"metadata,omitempty\"`" + `
	Items []TestResource ` + "`json:\"items\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 2)

	testResource := structs[0]
	require.Equal(t, "TestResource", testResource.Name)
	require.Len(t, testResource.Fields, 3)

	typeMetaField := testResource.Fields[0]
	require.Equal(t, "", typeMetaField.Name) // embedded field has no name
	require.True(t, typeMetaField.IsInline)
	metadataType, ok := typeMetaField.Type.(MetadataType)
	require.True(t, ok, "TypeMeta should be MetadataType")
	require.Equal(t, "TypeMeta", metadataType.Name)
	require.Equal(t, "metav1", metadataType.Package)
	require.Equal(t, "k8s.io/apimachinery/pkg/apis/meta/v1", metadataType.ImportPath)

	objectMetaField := testResource.Fields[1]
	metadataType, ok = objectMetaField.Type.(MetadataType)
	require.True(t, ok, "ObjectMeta should be MetadataType")
	require.Equal(t, "ObjectMeta", metadataType.Name)
	require.Equal(t, "metav1", metadataType.Package)
	require.Equal(t, "k8s.io/apimachinery/pkg/apis/meta/v1", metadataType.ImportPath)

	nameField := testResource.Fields[2]
	require.Equal(t, "Name", nameField.Name)
	_, ok = nameField.Type.(StringType)
	require.True(t, ok, "Name should be StringType")

	// Check TestResourceList
	testResourceList := structs[1]
	require.Equal(t, "TestResourceList", testResourceList.Name)
	require.Len(t, testResourceList.Fields, 3)

	listMetaField := testResourceList.Fields[1]
	metadataType, ok = listMetaField.Type.(MetadataType)
	require.True(t, ok, "ListMeta should be MetadataType")
	require.Equal(t, "ListMeta", metadataType.Name)
	require.Equal(t, "metav1", metadataType.Package)
	require.Equal(t, "k8s.io/apimachinery/pkg/apis/meta/v1", metadataType.ImportPath)
}

func TestParser_ParseComplexNestedTypes(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// Array of pointers to strings
	Tags []*string ` + "`json:\"tags,omitempty\"`" + `
	
	// Array of pointers to structs
	Configs []*Config ` + "`json:\"configs,omitempty\"`" + `
	
	// Map with pointer values
	Settings map[string]*int32 ` + "`json:\"settings,omitempty\"`" + `

	// Type from other packages (that are not metadata)
	Something somepackage.SomeType ` + "`json:\"something,omitempty\"`" + `
}

type Config struct {
	Key string ` + "`json:\"key\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 2)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 4)

	// Check array of pointer to string
	tagsField := testStruct.Fields[0]
	require.Equal(t, "Tags", tagsField.Name)
	arrayType, ok := tagsField.Type.(ArrayType)
	require.True(t, ok, "Tags should be ArrayType")
	stringType, ok := arrayType.ElementType.(StringType)
	require.True(t, ok, "Tags element should be StringType")
	require.True(t, stringType.IsPointer)

	// Check array of pointer to struct
	configsField := testStruct.Fields[1]
	require.Equal(t, "Configs", configsField.Name)
	arrayType, ok = configsField.Type.(ArrayType)
	require.True(t, ok, "Configs should be ArrayType")
	structType, ok := arrayType.ElementType.(StructType)
	require.True(t, ok, "Configs element should be StructType")
	require.Equal(t, "Config", structType.Name)
	require.Equal(t, "test", structType.Package)
	require.True(t, structType.IsPointer)
	require.Equal(t, "", structType.ImportPath, "Local struct should have empty ImportPath")

	// Check map with pointer values
	settingsField := testStruct.Fields[2]
	require.Equal(t, "Settings", settingsField.Name)
	mapType, ok := settingsField.Type.(MapType)
	require.True(t, ok, "Settings should be MapType")
	_, ok = mapType.KeyType.(StringType)
	require.True(t, ok, "Settings key should be StringType")
	numericPointerType, ok := mapType.ValueType.(NumericType)
	require.True(t, ok, "Settings value should be NumericType")
	require.Equal(t, "int32", numericPointerType.Name)
	require.True(t, numericPointerType.IsPointer)

	// Check type from other package
	structFromOtherPackage := testStruct.Fields[3]
	require.Equal(t, "Something", structFromOtherPackage.Name)
	structType, ok = structFromOtherPackage.Type.(StructType)
	require.True(t, ok, "Something should be StructType")
	require.Equal(t, "SomeType", structType.Name)
	require.Equal(t, "somepackage", structType.Package)
	require.False(t, structType.IsPointer)
	require.Equal(t, "", structType.ImportPath, "ImportPath is empty because import is missing in test code")
}

func TestParser_ParsePointerWithStringLengthValidation(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:MinLength=3
	// +xrd-gen:validation:MaxLength=50
	// Username with length constraints
	Username *string ` + "`json:\"username,omitempty\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 1)

	usernameField := testStruct.Fields[0]
	require.Equal(t, "Username", usernameField.Name)
	stringType, ok := usernameField.Type.(StringType)
	require.True(t, ok, "Username should be StringType")
	require.True(t, stringType.IsPointer)
	require.NotNil(t, stringType.MinLength)
	require.Equal(t, 3, *stringType.MinLength)
	require.NotNil(t, stringType.MaxLength)
	require.Equal(t, 50, *stringType.MaxLength)
}

func TestParser_ParseFileWithInvalidSyntax(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "invalid.go")

	// Create a file with invalid Go syntax
	invalidCode := `package test

type BrokenStruct struct {
	Field string
// missing closing brace
`

	err := os.WriteFile(testFile, []byte(invalidCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	_, err = parser.parseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse file")
}

func TestParser_ParseFileWithInvalidStructSyntax(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "invalid.go")

	// Create a file with syntactically valid Go but malformed struct
	invalidCode := `package test

type BrokenStruct struct {
	Field string json:"field"  // missing backticks around tag
}
`

	err := os.WriteFile(testFile, []byte(invalidCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	_, err = parser.parseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse file")
}

func TestParser_ParseFileNotFound(t *testing.T) {
	parser := NewParser()
	_, err := parser.parseFile("/nonexistent/path/to/file.go")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse file")
}

func TestParser_ParseDirectoryNotFound(t *testing.T) {
	parser := NewParser()
	_, err := parser.ParseDirectory("/nonexistent/path/to/directory")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to walk directory")
}

func TestParser_ParseDirectoryWithInvalidFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a valid Go file
	validFile := filepath.Join(tempDir, "valid.go")
	validCode := `package test

type ValidStruct struct {
	Field string ` + "`json:\"field\"`" + `
}
`
	err := os.WriteFile(validFile, []byte(validCode), 0o644)
	require.NoError(t, err)

	// Create an invalid Go file
	invalidFile := filepath.Join(tempDir, "invalid.go")
	invalidCode := `package test

type BrokenStruct struct {
	Field string
// missing closing brace
`
	err = os.WriteFile(invalidFile, []byte(invalidCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	_, err = parser.ParseDirectory(tempDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse file")
	require.Contains(t, err.Error(), "invalid.go")
}

func TestParser_ParseDirectoryWithNestedDirectories(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file in the root
	rootFile := filepath.Join(tempDir, "root.go")
	rootCode := `package test

type RootStruct struct {
	Field string ` + "`json:\"field\"`" + `
}
`
	err := os.WriteFile(rootFile, []byte(rootCode), 0o644)
	require.NoError(t, err)

	// Create a subdirectory with a file
	subDir := filepath.Join(tempDir, "subpackage")
	err = os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	subFile := filepath.Join(subDir, "sub.go")
	subCode := `package subpackage

type SubStruct struct {
	Field string ` + "`json:\"field\"`" + `
}
`
	err = os.WriteFile(subFile, []byte(subCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.ParseDirectory(tempDir)
	require.NoError(t, err)
	require.Len(t, structs, 2)

	// Verify both structs were found
	structNames := make(map[string]bool)
	for _, s := range structs {
		structNames[s.Name] = true
	}
	require.True(t, structNames["RootStruct"])
	require.True(t, structNames["SubStruct"])
}

func TestStringType_Name(t *testing.T) {
	require.Equal(t, stringTypeName, StringType{}.Name())
}

func Test_Struct_FullyQualifiedName(t *testing.T) {
	s := Struct{
		Name:    "TestStruct",
		Package: "test",
	}
	require.Equal(t, "test.TestStruct", s.FullyQualifiedName())
}

func Test_StructType_FullyQualifiedName(t *testing.T) {
	s := StructType{
		Name:    "TestStruct",
		Package: "test",
	}
	require.Equal(t, "test.TestStruct", s.FullyQualifiedName())
}

func Test_MetadataType_FullyQualifiedName(t *testing.T) {
	m := MetadataType{
		Name:    "TypeMeta",
		Package: "metav1",
	}
	require.Equal(t, "metav1.TypeMeta", m.FullyQualifiedName())
}

func TestParser_ParseImportPaths(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	myalias "github.com/example/mypackage"
)

type TestStruct struct {
	// Local struct - should have empty ImportPath
	LocalField LocalType ` + "`json:\"local\"`" + `
	
	// External type with default alias (v1) - should have ImportPath
	PodSpec v1.PodSpec ` + "`json:\"podSpec\"`" + `
	
	// External type with explicit alias (metav1) - should have ImportPath
	ObjectMeta metav1.ObjectMeta ` + "`json:\"metadata\"`" + `
	
	// External type with custom alias (myalias) - should have ImportPath
	CustomType myalias.CustomType ` + "`json:\"custom\"`" + `
	
	// Array of external types - should have ImportPath
	Pods []v1.Pod ` + "`json:\"pods\"`" + `
	
	// Map with external types - should have ImportPath
	ConfigMaps map[string]v1.ConfigMap ` + "`json:\"configMaps\"`" + `
}

type LocalType struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 2)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 6)

	// Check local struct field - should have empty ImportPath
	localField := testStruct.Fields[0]
	require.Equal(t, "LocalField", localField.Name)
	localStructType, ok := localField.Type.(StructType)
	require.True(t, ok, "LocalField should be StructType")
	require.Equal(t, "LocalType", localStructType.Name)
	require.Equal(t, "test", localStructType.Package)
	require.Equal(t, "", localStructType.ImportPath, "Local struct should have empty ImportPath")

	// Check external type with default alias (v1)
	podSpecField := testStruct.Fields[1]
	require.Equal(t, "PodSpec", podSpecField.Name)
	podSpecType, ok := podSpecField.Type.(StructType)
	require.True(t, ok, "PodSpec should be StructType")
	require.Equal(t, "PodSpec", podSpecType.Name)
	require.Equal(t, "v1", podSpecType.Package)
	require.Equal(t, "k8s.io/api/core/v1", podSpecType.ImportPath, "External type should have full import path")

	// Check external type with explicit alias (metav1)
	objectMetaField := testStruct.Fields[2]
	require.Equal(t, "ObjectMeta", objectMetaField.Name)
	objectMetaType, ok := objectMetaField.Type.(MetadataType)
	require.True(t, ok, "ObjectMeta should be MetadataType")
	require.Equal(t, "ObjectMeta", objectMetaType.Name)
	require.Equal(t, "metav1", objectMetaType.Package)
	require.Equal(t, "k8s.io/apimachinery/pkg/apis/meta/v1", objectMetaType.ImportPath, "Metadata type with alias should have full import path")

	// Check external type with custom alias (myalias)
	customField := testStruct.Fields[3]
	require.Equal(t, "CustomType", customField.Name)
	customType, ok := customField.Type.(StructType)
	require.True(t, ok, "CustomType should be StructType")
	require.Equal(t, "CustomType", customType.Name)
	require.Equal(t, "myalias", customType.Package)
	require.Equal(t, "github.com/example/mypackage", customType.ImportPath, "Custom alias should have full import path")

	// Check array of external types
	podsField := testStruct.Fields[4]
	require.Equal(t, "Pods", podsField.Name)
	podsArrayType, ok := podsField.Type.(ArrayType)
	require.True(t, ok, "Pods should be ArrayType")
	podType, ok := podsArrayType.ElementType.(StructType)
	require.True(t, ok, "Pods element should be StructType")
	require.Equal(t, "Pod", podType.Name)
	require.Equal(t, "v1", podType.Package)
	require.Equal(t, "k8s.io/api/core/v1", podType.ImportPath, "External type in array should have full import path")

	// Check map with external types
	configMapsField := testStruct.Fields[5]
	require.Equal(t, "ConfigMaps", configMapsField.Name)
	configMapsMapType, ok := configMapsField.Type.(MapType)
	require.True(t, ok, "ConfigMaps should be MapType")
	configMapType, ok := configMapsMapType.ValueType.(StructType)
	require.True(t, ok, "ConfigMaps value should be StructType")
	require.Equal(t, "ConfigMap", configMapType.Name)
	require.Equal(t, "v1", configMapType.Package)
	require.Equal(t, "k8s.io/api/core/v1", configMapType.ImportPath, "External type in map should have full import path")

	// Check LocalType struct
	localTypeStruct := structs[1]
	require.Equal(t, "LocalType", localTypeStruct.Name)
	require.Equal(t, "test", localTypeStruct.Package)
}

func TestParser_TimeTypes(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

import (
	"time"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Event struct {
	// Timestamp is a standard Go time
	Timestamp time.Time ` + "`json:\"timestamp\"`" + `
	
	// LastUpdated is a Kubernetes time
	LastUpdated metav1.Time ` + "`json:\"lastUpdated\"`" + `
	
	// OptionalTime is a pointer to Go time
	OptionalTime *time.Time ` + "`json:\"optionalTime,omitempty\"`" + `
	
	// OptionalK8sTime is a pointer to K8s time
	OptionalK8sTime *metav1.Time ` + "`json:\"optionalK8sTime,omitempty\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.ParseDirectory(tempDir)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "Event", testStruct.Name)
	require.Len(t, testStruct.Fields, 4)

	timestampField := testStruct.Fields[0]
	require.Equal(t, "Timestamp", timestampField.Name)
	timeType, ok := timestampField.Type.(TimeType)
	require.True(t, ok, "Timestamp should be TimeType")
	require.False(t, timeType.IsPointer)
	require.Equal(t, "time", timeType.Package)
	require.Equal(t, "time", timeType.ImportPath)

	lastUpdatedField := testStruct.Fields[1]
	require.Equal(t, "LastUpdated", lastUpdatedField.Name)
	k8sTimeType, ok := lastUpdatedField.Type.(TimeType)
	require.True(t, ok, "LastUpdated should be TimeType")
	require.False(t, k8sTimeType.IsPointer)
	require.Equal(t, "metav1", k8sTimeType.Package)
	require.Equal(t, "k8s.io/apimachinery/pkg/apis/meta/v1", k8sTimeType.ImportPath)

	optionalTimeField := testStruct.Fields[2]
	require.Equal(t, "OptionalTime", optionalTimeField.Name)
	optionalTimeType, ok := optionalTimeField.Type.(TimeType)
	require.True(t, ok, "OptionalTime should be TimeType")
	require.True(t, optionalTimeType.IsPointer)
	require.Equal(t, "time", optionalTimeType.Package)
	require.Equal(t, "time", optionalTimeType.ImportPath)

	optionalK8sTimeField := testStruct.Fields[3]
	require.Equal(t, "OptionalK8sTime", optionalK8sTimeField.Name)
	optionalK8sTimeType, ok := optionalK8sTimeField.Type.(TimeType)
	require.True(t, ok, "OptionalK8sTime should be TimeType")
	require.True(t, optionalK8sTimeType.IsPointer)
	require.Equal(t, "metav1", optionalK8sTimeType.Package)
	require.Equal(t, "k8s.io/apimachinery/pkg/apis/meta/v1", optionalK8sTimeType.ImportPath)
}

func TestTimeType_FullyQualifiedName(t *testing.T) {
	tests := []struct {
		name     string
		timeType TimeType
		expected string
	}{
		{
			name: "time.Time",
			timeType: TimeType{
				Package:    "time",
				ImportPath: "time",
			},
			expected: "time.Time",
		},
		{
			name: "metav1.Time",
			timeType: TimeType{
				Package:    "metav1",
				ImportPath: "k8s.io/apimachinery/pkg/apis/meta/v1",
			},
			expected: "metav1.Time",
		},
		{
			name: "pointer to time.Time",
			timeType: TimeType{
				Package:    "time",
				ImportPath: "time",
				IsPointer:  true,
			},
			expected: "time.Time",
		},
		{
			name: "pointer to metav1.Time",
			timeType: TimeType{
				Package:    "metav1",
				ImportPath: "k8s.io/apimachinery/pkg/apis/meta/v1",
				IsPointer:  true,
			},
			expected: "metav1.Time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.timeType.FullyQualifiedName()
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestParser_NumericTypes(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type Config struct {
	// +xrd-gen:validation:Minimum=0
	// +xrd-gen:validation:Maximum=100
	// Score must be between 0 and 100
	Score int32 ` + "`json:\"score\"`" + `
	
	// +xrd-gen:validation:Minimum=0.0
	// Price must be positive
	Price float64 ` + "`json:\"price\"`" + `
	
	// +xrd-gen:validation:Maximum=10000
	// Max count is 10000
	Count int ` + "`json:\"count\"`" + `
	
	// Optional numeric with validation
	// +xrd-gen:validation:Optional
	// +xrd-gen:validation:Minimum=-1
	// +xrd-gen:validation:Maximum=1
	OptionalRatio *float32 ` + "`json:\"optionalRatio,omitempty\"`" + `
	
	// No validation
	PlainNumber uint64 ` + "`json:\"plainNumber\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.ParseDirectory(tempDir)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "Config", testStruct.Name)
	require.Len(t, testStruct.Fields, 5)

	scoreField := testStruct.Fields[0]
	require.Equal(t, "Score", scoreField.Name)
	numericType, ok := scoreField.Type.(NumericType)
	require.True(t, ok, "Score should be NumericType")
	require.False(t, numericType.IsPointer)
	require.Equal(t, "int32", numericType.Name)
	require.NotNil(t, numericType.Minimum)
	require.Equal(t, 0.0, *numericType.Minimum)
	require.NotNil(t, numericType.Maximum)
	require.Equal(t, 100.0, *numericType.Maximum)

	priceField := testStruct.Fields[1]
	require.Equal(t, "Price", priceField.Name)
	priceType, ok := priceField.Type.(NumericType)
	require.True(t, ok, "Price should be NumericType")
	require.False(t, priceType.IsPointer)
	require.Equal(t, "float64", priceType.Name)
	require.NotNil(t, priceType.Minimum)
	require.Equal(t, 0.0, *priceType.Minimum)
	require.Nil(t, priceType.Maximum)

	countField := testStruct.Fields[2]
	require.Equal(t, "Count", countField.Name)
	countType, ok := countField.Type.(NumericType)
	require.True(t, ok, "Count should be NumericType")
	require.False(t, countType.IsPointer)
	require.Equal(t, "int", countType.Name)
	require.Nil(t, countType.Minimum)
	require.NotNil(t, countType.Maximum)
	require.Equal(t, 10000.0, *countType.Maximum)

	optionalRatioField := testStruct.Fields[3]
	require.Equal(t, "OptionalRatio", optionalRatioField.Name)
	optionalRatioType, ok := optionalRatioField.Type.(NumericType)
	require.True(t, ok, "OptionalRatio should be NumericType")
	require.True(t, optionalRatioType.IsPointer)
	require.Equal(t, "float32", optionalRatioType.Name)
	require.NotNil(t, optionalRatioType.Minimum)
	require.Equal(t, -1.0, *optionalRatioType.Minimum)
	require.NotNil(t, optionalRatioType.Maximum)
	require.Equal(t, 1.0, *optionalRatioType.Maximum)

	plainNumberField := testStruct.Fields[4]
	require.Equal(t, "PlainNumber", plainNumberField.Name)
	plainNumberType, ok := plainNumberField.Type.(NumericType)
	require.True(t, ok, "PlainNumber should be NumericType")
	require.False(t, plainNumberType.IsPointer)
	require.Equal(t, "uint64", plainNumberType.Name)
	require.Nil(t, plainNumberType.Minimum)
	require.Nil(t, plainNumberType.Maximum)
}

func TestParser_ParsePrinterColumns(t *testing.T) {
	parser := NewParser()

	docText := `// +xrd-gen:object:root=true
// +xrd-gen:groupName=example.com
// +xrd-gen:printcolumn:name="health",type="string",JSONPath=".status.health"
// +xrd-gen:printcolumn:name="nodes",type="integer",JSONPath=".status.availableNodes",description="Available nodes"
`

	columns, err := parser.parsePrinterColumns(docText)
	require.NoError(t, err)
	require.Len(t, columns, 2)

	require.Equal(t, "health", columns[0].Name)
	require.Equal(t, "string", columns[0].Type)
	require.Equal(t, ".status.health", columns[0].JSONPath)

	require.Equal(t, "nodes", columns[1].Name)
	require.Equal(t, "integer", columns[1].Type)
	require.Equal(t, ".status.availableNodes", columns[1].JSONPath)
	require.Equal(t, "Available nodes", columns[1].Description)
}

func TestParser_ParsePrinterColumn(t *testing.T) {
	parser := NewParser()

	col1, err := parser.parsePrinterColumn(`name="health",type="string",JSONPath=".status.health"`)
	require.NoError(t, err)
	require.Equal(t, "health", col1.Name)
	require.Equal(t, "string", col1.Type)
	require.Equal(t, ".status.health", col1.JSONPath)
	require.Equal(t, "", col1.Description)
	require.Equal(t, int32(0), col1.Priority)

	col2, err := parser.parsePrinterColumn(`name="nodes",type="integer",JSONPath=".status.availableNodes",description="Available nodes"`)
	require.NoError(t, err)
	require.Equal(t, "nodes", col2.Name)
	require.Equal(t, "integer", col2.Type)
	require.Equal(t, ".status.availableNodes", col2.JSONPath)
	require.Equal(t, "Available nodes", col2.Description)
	require.Equal(t, int32(0), col2.Priority)

	col3, err := parser.parsePrinterColumn(`name="version",type="string",JSONPath=".status.version",description="Version",priority=1`)
	require.NoError(t, err)
	require.Equal(t, "version", col3.Name)
	require.Equal(t, "string", col3.Type)
	require.Equal(t, ".status.version", col3.JSONPath)
	require.Equal(t, "Version", col3.Description)
	require.Equal(t, int32(1), col3.Priority)
}

func TestParser_PrinterColumns(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// +xrd-gen:object:root=true
// +xrd-gen:groupName=example.com
// +xrd-gen:version=v1
// +xrd-gen:printcolumn:name="health",type="string",JSONPath=".status.health"
// +xrd-gen:printcolumn:name="nodes",type="integer",JSONPath=".status.availableNodes",description="Available nodes"
// +xrd-gen:printcolumn:name="version",type="string",JSONPath=".status.version",description="Version",priority=1
// +xrd-gen:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"
// MyResource represents a custom resource
type MyResource struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.ParseDirectory(tempDir)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "MyResource", testStruct.Name)
	require.True(t, testStruct.IsRoot)
	require.Len(t, testStruct.PrinterColumns, 4)

	healthCol := testStruct.PrinterColumns[0]
	require.Equal(t, "health", healthCol.Name)
	require.Equal(t, "string", healthCol.Type)
	require.Equal(t, ".status.health", healthCol.JSONPath)
	require.Equal(t, "", healthCol.Description)
	require.Equal(t, int32(0), healthCol.Priority)

	nodesCol := testStruct.PrinterColumns[1]
	require.Equal(t, "nodes", nodesCol.Name)
	require.Equal(t, "integer", nodesCol.Type)
	require.Equal(t, ".status.availableNodes", nodesCol.JSONPath)
	require.Equal(t, "Available nodes", nodesCol.Description)
	require.Equal(t, int32(0), nodesCol.Priority)

	versionCol := testStruct.PrinterColumns[2]
	require.Equal(t, "version", versionCol.Name)
	require.Equal(t, "string", versionCol.Type)
	require.Equal(t, ".status.version", versionCol.JSONPath)
	require.Equal(t, "Version", versionCol.Description)
	require.Equal(t, int32(1), versionCol.Priority)

	ageCol := testStruct.PrinterColumns[3]
	require.Equal(t, "age", ageCol.Name)
	require.Equal(t, "date", ageCol.Type)
	require.Equal(t, ".metadata.creationTimestamp", ageCol.JSONPath)
	require.Equal(t, "", ageCol.Description)
	require.Equal(t, int32(0), ageCol.Priority)
}

func TestParser_LabelsAndAnnotations(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// +xrd-gen:object:root=true
// +xrd-gen:groupName=example.com
// +xrd-gen:version=v1
// +xrd-gen:label:app.kubernetes.io/name=myapp
// +xrd-gen:label:environment=production
// +xrd-gen:annotation:app.kubernetes.io/managed-by=xrd-gen
// +xrd-gen:annotation:description=My custom resource
// MyResource represents a custom resource
type MyResource struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.ParseDirectory(tempDir)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "MyResource", testStruct.Name)
	require.True(t, testStruct.IsRoot)

	require.Len(t, testStruct.KubernetesLabels, 2)
	require.Equal(t, "myapp", testStruct.KubernetesLabels["app.kubernetes.io/name"])
	require.Equal(t, "production", testStruct.KubernetesLabels["environment"])

	require.Len(t, testStruct.KubernetesAnnotations, 2)
	require.Equal(t, "xrd-gen", testStruct.KubernetesAnnotations["app.kubernetes.io/managed-by"])
	require.Equal(t, "My custom resource", testStruct.KubernetesAnnotations["description"])
}

func TestParser_LabelsAndAnnotations_EmptyWhenNotProvided(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// +xrd-gen:object:root=true
// +xrd-gen:groupName=example.com
// MyResource represents a custom resource without labels or annotations
type MyResource struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.ParseDirectory(tempDir)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "MyResource", testStruct.Name)
	require.True(t, testStruct.IsRoot)

	require.Empty(t, testStruct.KubernetesLabels)
	require.Empty(t, testStruct.KubernetesAnnotations)
}

func TestParseKeyValues(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		annotationType string
		want           map[string]string
	}{
		{
			name:           "single label",
			text:           "// +xrd-gen:label:app=myapp",
			annotationType: annotationLabel,
			want:           map[string]string{"app": "myapp"},
		},
		{
			name:           "multiple labels",
			text:           "// +xrd-gen:label:app=myapp\n// +xrd-gen:label:env=prod",
			annotationType: annotationLabel,
			want:           map[string]string{"app": "myapp", "env": "prod"},
		},
		{
			name:           "label with kubernetes-style key",
			text:           "// +xrd-gen:label:app.kubernetes.io/name=myapp",
			annotationType: annotationLabel,
			want:           map[string]string{"app.kubernetes.io/name": "myapp"},
		},
		{
			name:           "annotation with spaces in value",
			text:           "// +xrd-gen:annotation:description=My custom resource",
			annotationType: annotationAnnotation,
			want:           map[string]string{"description": "My custom resource"},
		},
		{
			name:           "annotation with equals sign in value",
			text:           "// +xrd-gen:annotation:formula=x=y+z",
			annotationType: annotationAnnotation,
			want:           map[string]string{"formula": "x=y+z"},
		},
		{
			name:           "mixed with regular comments",
			text:           "// This is a comment\n// +xrd-gen:label:app=myapp\n// Another comment",
			annotationType: annotationLabel,
			want:           map[string]string{"app": "myapp"},
		},
		{
			name:           "empty when none provided",
			text:           "// Just a regular comment\n// +xrd-gen:groupName=example.com",
			annotationType: annotationLabel,
			want:           map[string]string{},
		},
		{
			name:           "spaces around key and value",
			text:           "// +xrd-gen:label: app = myapp ",
			annotationType: annotationLabel,
			want:           map[string]string{"app": "myapp"},
		},
		{
			name:           "value with double quotes",
			text:           `// +xrd-gen:label:app="myapp"`,
			annotationType: annotationLabel,
			want:           map[string]string{"app": "myapp"},
		},
		{
			name:           "annotation value with spaces and quotes",
			text:           `// +xrd-gen:annotation:description="My custom resource"`,
			annotationType: annotationAnnotation,
			want:           map[string]string{"description": "My custom resource"},
		},
		{
			name:           "mixed quoted and unquoted",
			text:           "// +xrd-gen:label:app=\"myapp\"\n// +xrd-gen:label:env=prod",
			annotationType: annotationLabel,
			want:           map[string]string{"app": "myapp", "env": "prod"},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.parseKeyValues(tt.text, tt.annotationType)
			require.Len(t, got, len(tt.want))
			for key, wantVal := range tt.want {
				require.Equal(t, wantVal, got[key], "mismatch for key %s", key)
			}
		})
	}
}

func TestParser_ParseEnumWithDefault(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:Enum=dev,qa,staging,production
	// +xrd-gen:validation:Default=dev
	// Environment with enum and default
	Environment string ` + "`json:\"environment\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 1)

	envField := testStruct.Fields[0]
	require.Equal(t, "Environment", envField.Name)

	require.Len(t, envField.EnumValues, 4)
	require.Equal(t, []string{"dev", "qa", "staging", "production"}, envField.EnumValues)

	stringType, ok := envField.Type.(StringType)
	require.True(t, ok, "Environment should be StringType")
	require.NotNil(t, stringType.Default)
	require.Equal(t, "dev", stringType.Default)
}

func TestParser_ParseNumericDefaults(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:Default=42
	Int32Field int32 ` + "`json:\"int32Field\"`" + `
	
	// +xrd-gen:validation:Default=9223372036854775807
	Int64Field int64 ` + "`json:\"int64Field\"`" + `
	
	// +xrd-gen:validation:Default=3.14159
	Float32Field float32 ` + "`json:\"float32Field\"`" + `
	
	// +xrd-gen:validation:Default=2.718281828
	Float64Field float64 ` + "`json:\"float64Field\"`" + `
	
	// +xrd-gen:validation:Default=255
	UintField uint ` + "`json:\"uintField\"`" + `
	
	// +xrd-gen:validation:Default=100
	Uint32Field uint32 ` + "`json:\"uint32Field\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 6)

	int32Field := testStruct.Fields[0]
	numericType, ok := int32Field.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, 42, numericType.Default)

	int64Field := testStruct.Fields[1]
	numericType, ok = int64Field.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, int64(9223372036854775807), numericType.Default)

	float32Field := testStruct.Fields[2]
	numericType, ok = float32Field.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, 3.14159, numericType.Default)

	float64Field := testStruct.Fields[3]
	numericType, ok = float64Field.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, 2.718281828, numericType.Default)

	uintField := testStruct.Fields[4]
	numericType, ok = uintField.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, 255, numericType.Default)

	uint32Field := testStruct.Fields[5]
	numericType, ok = uint32Field.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, 100, numericType.Default)
}

func TestParser_ParsePrimitiveDefaults(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:Default=true
	BoolFieldTrue bool ` + "`json:\"boolFieldTrue\"`" + `
	
	// +xrd-gen:validation:Default=false
	BoolFieldFalse bool ` + "`json:\"boolFieldFalse\"`" + `
	
	// +xrd-gen:validation:Default=65
	ByteField byte ` + "`json:\"byteField\"`" + `
	
	// +xrd-gen:validation:Default=97
	RuneField rune ` + "`json:\"runeField\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 4)

	boolTrueField := testStruct.Fields[0]
	primitiveType, ok := boolTrueField.Type.(PrimitiveType)
	require.True(t, ok)
	require.Equal(t, true, primitiveType.Default)

	boolFalseField := testStruct.Fields[1]
	primitiveType, ok = boolFalseField.Type.(PrimitiveType)
	require.True(t, ok)
	require.Equal(t, false, primitiveType.Default)

	byteField := testStruct.Fields[2]
	primitiveType, ok = byteField.Type.(PrimitiveType)
	require.True(t, ok)
	require.Equal(t, 65, primitiveType.Default)

	runeField := testStruct.Fields[3]
	primitiveType, ok = runeField.Type.(PrimitiveType)
	require.True(t, ok)
	require.Equal(t, 97, primitiveType.Default)
}

func TestParser_ParseInvalidDefaults(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:Default=not-a-number
	IntField int32 ` + "`json:\"intField\"`" + `
	
	// +xrd-gen:validation:Default=not-a-bool
	BoolField bool ` + "`json:\"boolField\"`" + `
	
	// +xrd-gen:validation:Default=12.34.56
	FloatField float64 ` + "`json:\"floatField\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Equal(t, "TestStruct", testStruct.Name)
	require.Len(t, testStruct.Fields, 3)

	// When parsing fails, defaults should fall back to the string value
	intField := testStruct.Fields[0]
	numericType, ok := intField.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, "not-a-number", numericType.Default)

	boolField := testStruct.Fields[1]
	primitiveType, ok := boolField.Type.(PrimitiveType)
	require.True(t, ok)
	require.Equal(t, "not-a-bool", primitiveType.Default)

	floatField := testStruct.Fields[2]
	numericType, ok = floatField.Type.(NumericType)
	require.True(t, ok)
	require.Equal(t, "12.34.56", numericType.Default)
}

func TestParser_ParseXValidation_FieldLevel(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	Name string ` + "`json:\"name\"`" + `

	// +xrd-gen:validation:XValidation:rule="self.startsWith('kube-')",message="must start with kube-"
	// +xrd-gen:validation:XValidation:rule="size(self) <= 63",message="must be at most 63 characters"
	Prefix string ` + "`json:\"prefix\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]

	nameField := testStruct.Fields[0]
	require.Len(t, nameField.XValidations, 1)
	require.Equal(t, "self == oldSelf", nameField.XValidations[0].Rule)
	require.Equal(t, "name is immutable", nameField.XValidations[0].Message)

	prefixField := testStruct.Fields[1]
	require.Len(t, prefixField.XValidations, 2)
	require.Equal(t, "self.startsWith('kube-')", prefixField.XValidations[0].Rule)
	require.Equal(t, "must start with kube-", prefixField.XValidations[0].Message)
	require.Equal(t, "size(self) <= 63", prefixField.XValidations[1].Rule)
	require.Equal(t, "must be at most 63 characters", prefixField.XValidations[1].Message)
}

func TestParser_ParseXValidation_StructLevel(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// +xrd-gen:validation:XValidation:rule="self.minReplicas <= self.maxReplicas",message="minReplicas must not exceed maxReplicas"
type ReplicaSpec struct {
	MinReplicas int32 ` + "`json:\"minReplicas\"`" + `
	MaxReplicas int32 ` + "`json:\"maxReplicas\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	testStruct := structs[0]
	require.Len(t, testStruct.XValidations, 1)
	require.Equal(t, "self.minReplicas <= self.maxReplicas", testStruct.XValidations[0].Rule)
	require.Equal(t, "minReplicas must not exceed maxReplicas", testStruct.XValidations[0].Message)
}

func TestParser_ParseXValidation_AllParameters(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:XValidation:rule="!oldSelf.hasValue() || self == oldSelf.value()",message="name is immutable once set",messageExpression="'name cannot change from ' + string(oldSelf.value())",reason=FieldValueForbidden,fieldPath=".name",optionalOldSelf=true
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	nameField := structs[0].Fields[0]
	require.Len(t, nameField.XValidations, 1)
	xv := nameField.XValidations[0]
	require.Equal(t, "!oldSelf.hasValue() || self == oldSelf.value()", xv.Rule)
	require.Equal(t, "name is immutable once set", xv.Message)
	require.Equal(t, "'name cannot change from ' + string(oldSelf.value())", xv.MessageExpression)
	require.Equal(t, "FieldValueForbidden", xv.Reason)
	require.Equal(t, ".name", xv.FieldPath)
	require.NotNil(t, xv.OptionalOldSelf)
	require.True(t, *xv.OptionalOldSelf)
}

func TestParser_ParseXValidation_RuleWithCommas(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:XValidation:rule="self.all(x, x > 0 && x < 100)",message="all values must be between 0 and 100"
	Values []int32 ` + "`json:\"values\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	structs, err := parser.parseFile(testFile)
	require.NoError(t, err)
	require.Len(t, structs, 1)

	valuesField := structs[0].Fields[0]
	require.Len(t, valuesField.XValidations, 1)
	require.Equal(t, "self.all(x, x > 0 && x < 100)", valuesField.XValidations[0].Rule)
	require.Equal(t, "all values must be between 0 and 100", valuesField.XValidations[0].Message)
}

func TestParser_ParseXValidation_QuotedKeys(t *testing.T) {
	parser := NewParser()

	xv, err := parser.parseXValidation(`"rule"="self == oldSelf","message"="name is immutable"`)
	require.NoError(t, err)
	require.Equal(t, "self == oldSelf", xv.Rule)
	require.Equal(t, "name is immutable", xv.Message)
}

func TestParser_ParseXValidation_InvalidKey(t *testing.T) {
	parser := NewParser()

	_, err := parser.parseXValidation(`rule="self == oldSelf",bogus="oops"`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown XValidation key")
	require.Contains(t, err.Error(), "bogus")
}

func TestParser_ParseXValidations_InvalidKeyFailsFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

type TestStruct struct {
	// +xrd-gen:validation:XValidation:rule="self == oldSelf",bogus="oops"
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	_, err = parser.parseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown XValidation key")
}

func TestParser_ParsePrinterColumn_InvalidKey(t *testing.T) {
	parser := NewParser()

	_, err := parser.parsePrinterColumn(`name="health",type="string",JSONPath=".status.health",bogus="oops"`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown printcolumn key")
	require.Contains(t, err.Error(), "bogus")
}

func TestParser_ParsePrinterColumns_InvalidKeyFailsFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	testCode := `package test

// +xrd-gen:object:root=true
// +xrd-gen:groupName=example.com
// +xrd-gen:version=v1
// +xrd-gen:printcolumn:name="health",type="string",JSONPath=".status.health",bogus="oops"
type MyResource struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	require.NoError(t, err)

	parser := NewParser()
	_, err = parser.parseFile(testFile)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown printcolumn key")
}
