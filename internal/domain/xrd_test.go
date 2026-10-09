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

package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/crossplane-contrib/xrd-gen/internal/parsing"
)

func TestXRDGenerator_GenerateXRD(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:     "MyDB",
			Package:  "v1",
			Comments: "MyDB holds the specification of a database cluster.",
			IsRoot:   true,
			Group:    "example.com",
			Version:  "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Version",
					Type:     parsing.StringType{},
					JSONName: "version",
					Comments: "Version of the database engine.",
				},
				{
					Name:       "Image",
					Type:       parsing.StringType{},
					JSONName:   "image",
					Comments:   "Image is the database Docker image to deploy.",
					IsOptional: true,
				},
				{
					Name:     "NodeSets",
					Type:     parsing.ArrayType{ElementType: parsing.StructType{Name: "NodeSet", Package: "v1"}},
					JSONName: "nodeSets",
					Comments: "NodeSets allow specifying groups of database nodes.",
				},
			},
		},
		{
			Name:     "NodeSet",
			Package:  "v1",
			Comments: "NodeSet represents a group of nodes.",
			IsRoot:   false,
			Group:    "example.com",
			Version:  "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{},
					JSONName: "name",
					Comments: "Name of the NodeSet.",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.Equal(t, "mydbs.example.com", xrd.Name)
	require.Equal(t, "example.com", xrd.Group)
	require.Equal(t, "v1alpha1", xrd.Version)
	require.Equal(t, "MyDB", xrd.Kind)
	require.Equal(t, "mydbs", xrd.PluralName)
	require.NotNil(t, xrd.Schema)
	require.Equal(t, "object", xrd.Schema.Type)
	require.Contains(t, xrd.Schema.Properties, "version")
	require.Contains(t, xrd.Schema.Properties, "image")

	require.Contains(t, xrd.Schema.Required, "version")
	require.Contains(t, xrd.Schema.Required, "nodeSets")
	require.NotContains(t, xrd.Schema.Required, "image")
	require.Equal(t, "string", xrd.Schema.Properties["version"].Type)
	require.Equal(t, "string", xrd.Schema.Properties["image"].Type)

	require.Equal(t, "array", xrd.Schema.Properties["nodeSets"].Type)
	require.Equal(t, "object", xrd.Schema.Properties["nodeSets"].Items.Type)
	require.Equal(t, "string", xrd.Schema.Properties["nodeSets"].Items.Properties["name"].Type)
	require.Len(t, xrd.Schema.Properties["nodeSets"].Items.Required, 1)
	require.Equal(t, "name", xrd.Schema.Properties["nodeSets"].Items.Required[0])

	require.Equal(t, "MyDB holds the specification of a database cluster.", xrd.Schema.Description)
	require.Equal(t, "Version of the database engine.", xrd.Schema.Properties["version"].Description)
	require.Equal(t, "Image is the database Docker image to deploy.", xrd.Schema.Properties["image"].Description)
	require.Equal(t, "NodeSets allow specifying groups of database nodes.", xrd.Schema.Properties["nodeSets"].Description)
	require.Equal(t, "Name of the NodeSet.", xrd.Schema.Properties["nodeSets"].Items.Properties["name"].Description)
}

func TestXRDGenerator_GenerateXRD_DefaultVersion(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:     "MyDB",
			Package:  "v1",
			Comments: "MyDB holds the specification of a database cluster.",
			IsRoot:   true,
			Group:    "example.com",
			// No version specified
			Fields: []parsing.Field{
				{
					Name:     "Version",
					Type:     parsing.StringType{},
					JSONName: "version",
					Comments: "Version of the database engine.",
				},
				{
					Name:       "Image",
					Type:       parsing.StringType{},
					JSONName:   "image",
					Comments:   "Image is the database Docker image to deploy.",
					IsOptional: true,
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.Equal(t, defaultVersion, xrd.Version)
}

func TestXRDGenerator_GenerateXRD_ExternalPackage(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:     "MyDB",
			Package:  "v1",
			Comments: "MyDB holds the specification of a database cluster.",
			IsRoot:   true,
			Group:    "example.com",
			Version:  "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Version",
					Type:     parsing.StringType{},
					JSONName: "version",
					Comments: "Version of the database engine.",
				},
				{
					Name:       "Image",
					Type:       parsing.StructType{Name: "Image", Package: "externalpackage"},
					JSONName:   "image",
					Comments:   "Image is the database Docker image to deploy.",
					IsOptional: true,
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.Equal(t, "object", xrd.Schema.Properties["image"].Type)
	require.Empty(t, xrd.Schema.Properties["image"].Properties)
}

func TestXRDGenerator_GenerateXRD_ExternalPackageWithoutFullyQualifiedNameConflictWithLocalType(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:     "MyDB",
			Package:  "v1",
			Comments: "MyDB holds the specification of a database cluster.",
			IsRoot:   true,
			Group:    "example.com",
			Version:  "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Version",
					Type:     parsing.StructType{Name: "Image", Package: "v1"}, // name is a conflict with the local type
					JSONName: "version",
					Comments: "Version of the database engine.",
				},
				{
					Name:       "Image",
					Type:       parsing.StructType{Name: "Image", Package: "externalpackage"}, // name is not fully qualified
					JSONName:   "image",
					Comments:   "Image is the database Docker image to deploy.",
					IsOptional: true,
				},
			},
		},
		{
			Name:    "Image",
			Package: "v1", // same package as the root struct
			IsRoot:  false,
			Fields: []parsing.Field{ // local type has fields, since we've defined it ourselves
				{
					Name:     "Tag",
					Type:     parsing.StringType{},
					JSONName: "tag",
					Comments: "Tag of the image.",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.Equal(t, "object", xrd.Schema.Properties["image"].Type)
	require.Empty(t, xrd.Schema.Properties["image"].Properties)

	require.Equal(t, "object", xrd.Schema.Properties["version"].Type)
	require.NotEmpty(t, xrd.Schema.Properties["version"].Properties)
	require.Equal(t, "string", xrd.Schema.Properties["version"].Properties["tag"].Type)
	require.Equal(t, "Tag of the image.", xrd.Schema.Properties["version"].Properties["tag"].Description)
}

func TestXRDGenerator_BuildSchema(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:     "NodeSet",
			Package:  "v1",
			Comments: "NodeSet represents a group of nodes.",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{},
					JSONName: "name",
					Comments: "Name of the NodeSet.",
				},
				{
					Name:     "Count",
					Type:     parsing.NumericType{Name: "int32"},
					JSONName: "count",
					Comments: "Number of nodes.",
				},
			},
		},
	}

	structMap := make(map[string]parsing.Struct)
	for _, s := range structs {
		structMap[s.Name] = s
	}

	schema := buildSchema(structs[0], structMap)
	require.NotNil(t, schema)
	require.Equal(t, "object", schema.Type)
	require.Len(t, schema.Properties, 2)
	require.Contains(t, schema.Properties, "name")
	require.Contains(t, schema.Properties, "count")

	nameSchema := schema.Properties["name"]
	require.Equal(t, "string", nameSchema.Type)
	require.Equal(t, "Name of the NodeSet.", nameSchema.Description)

	countSchema := schema.Properties["count"]
	require.Equal(t, "integer", countSchema.Type)
	require.NotNil(t, countSchema.Format)
	require.Equal(t, "int32", *countSchema.Format)
}

func TestXRDGenerator_ArrayFields(t *testing.T) {
	minItems := 1
	structs := []parsing.Struct{
		{
			Name:    "TestStruct",
			Package: "v1",
			IsRoot:  true,
			Group:   "test.co",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Tags",
					Type:     parsing.ArrayType{ElementType: parsing.StringType{}, MinItems: &minItems},
					JSONName: "tags",
					Comments: "List of tags.",
				},
			},
		},
	}

	structMap := make(map[string]parsing.Struct)
	for _, s := range structs {
		structMap[s.Name] = s
	}

	schema := buildSchema(structs[0], structMap)

	tagsSchema := schema.Properties["tags"]
	require.Equal(t, "array", tagsSchema.Type)
	require.NotNil(t, tagsSchema.Items)
	require.Equal(t, "string", tagsSchema.Items.Type)
	require.NotNil(t, tagsSchema.MinItems)
	require.Equal(t, minItems, *tagsSchema.MinItems)
}

func TestXRDGenerator_ArrayFieldsWithMaxItems(t *testing.T) {
	minItems := 1
	maxItems := 10
	structs := []parsing.Struct{
		{
			Name:    "TestStruct",
			Package: "v1",
			IsRoot:  true,
			Group:   "test.co",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Tags",
					Type:     parsing.ArrayType{ElementType: parsing.StringType{}, MinItems: &minItems, MaxItems: &maxItems},
					JSONName: "tags",
					Comments: "List of tags with constraints.",
				},
			},
		},
	}

	structMap := make(map[string]parsing.Struct)
	for _, s := range structs {
		structMap[s.Name] = s
	}

	schema := buildSchema(structs[0], structMap)

	tagsSchema := schema.Properties["tags"]
	require.Equal(t, "array", tagsSchema.Type)
	require.NotNil(t, tagsSchema.Items)
	require.Equal(t, "string", tagsSchema.Items.Type)
	require.NotNil(t, tagsSchema.MinItems)
	require.Equal(t, minItems, *tagsSchema.MinItems)
	require.NotNil(t, tagsSchema.MaxItems)
	require.Equal(t, maxItems, *tagsSchema.MaxItems)
}

func TestMapNumericToOpenAPI(t *testing.T) {
	tests := []struct {
		name           string
		numericType    string
		expectedType   string
		expectedFormat string
	}{
		{"int32", "int32", "integer", "int32"},
		{"int64", "int64", "integer", "int64"},
		{"uint", "uint", "integer", "int32"},
		{"uint8", "uint8", "integer", "int32"},
		{"uint16", "uint16", "integer", "int32"},
		{"uint32", "uint32", "integer", "int32"},
		{"uint64", "uint64", "integer", "int64"},
		{"float32", "float32", "number", "float"},
		{"float64", "float64", "number", "double"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotFormat := mapNumericToOpenAPI(tt.numericType)
			require.Equal(t, tt.expectedType, gotType)
			require.Equal(t, tt.expectedFormat, gotFormat)
		})
	}
}

func TestMapPrimitiveToOpenAPI(t *testing.T) {
	tests := []struct {
		name           string
		primitiveType  string
		expectedType   string
		expectedFormat string
	}{
		{"byte", "byte", "integer", "int32"},
		{"rune", "rune", "integer", "int32"},
		{"bool", "bool", "boolean", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotFormat := mapPrimitiveToOpenAPI(tt.primitiveType)
			require.Equal(t, tt.expectedType, gotType)
			require.Equal(t, tt.expectedFormat, gotFormat)
		})
	}
}

func TestBuildFieldSchema_Map(t *testing.T) {
	tests := []struct {
		name              string
		field             parsing.Field
		expectedValueType string
	}{
		{
			name: "map[string]string",
			field: parsing.Field{
				Name:     "labels",
				Type:     parsing.MapType{KeyType: parsing.StringType{}, ValueType: parsing.StringType{}},
				Comments: "Labels is a map",
			},
			expectedValueType: "string",
		},
		{
			name: "map[string]int32",
			field: parsing.Field{
				Name:     "counts",
				Type:     parsing.MapType{KeyType: parsing.StringType{}, ValueType: parsing.NumericType{Name: "int32"}},
				Comments: "Counts is a map of integers",
			},
			expectedValueType: "integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			structMap := make(map[string]parsing.Struct)
			schema := buildFieldSchema(tt.field, structMap)
			require.Equal(t, "object", schema.Type)
			require.NotNil(t, schema.AdditionalProperties)
			require.Equal(t, tt.expectedValueType, schema.AdditionalProperties.Type)
		})
	}
}

func TestXRDGenerator_CustomPluralName(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyCustomDB",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Plural:  "myplurals",
			Fields: []parsing.Field{
				{
					Name:     "Version",
					Type:     parsing.StringType{},
					JSONName: "version",
					Comments: "Version is the database engine version",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.Equal(t, "myplurals", xrd.PluralName)
	require.Equal(t, "myplurals.example.com", xrd.Name)
	require.Equal(t, "MyCustomDB", xrd.Kind)
}

func TestXRDGenerator_DefaultPluralName(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyXRD",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			// No custom plural set
			Fields: []parsing.Field{
				{
					Name:     "Version",
					Type:     parsing.StringType{},
					JSONName: "version",
					Comments: "Version is the database engine version",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.Equal(t, "myxrds", xrd.PluralName)
	require.Equal(t, "myxrds.example.com", xrd.Name)
	require.Equal(t, "MyXRD", xrd.Kind)
}

func TestXRDGenerator_DefaultPluralNameEndingInSibilant(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "ServiceMesh",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{},
					JSONName: "name",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.Equal(t, "servicemeshes", xrd.PluralName)
	require.Equal(t, "servicemeshes.example.com", xrd.Name)
}

func TestXRDGenerator_PatternValidation(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{Pattern: "^[a-z0-9-]+$"},
					JSONName: "name",
					Comments: "Name with pattern validation",
				},
				{
					Name:     "Count",
					Type:     parsing.NumericType{Name: "int32"},
					JSONName: "count",
					Comments: "Count is a number",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check that pattern is applied to string field
	nameSchema := xrd.Schema.Properties["name"]
	require.NotNil(t, nameSchema)
	require.Equal(t, "string", nameSchema.Type)
	require.NotNil(t, nameSchema.Pattern)
	require.Equal(t, "^[a-z0-9-]+$", *nameSchema.Pattern)

	// Check that non-string fields don't have pattern
	countSchema := xrd.Schema.Properties["count"]
	require.NotNil(t, countSchema)
	require.Equal(t, "integer", countSchema.Type)
	require.Nil(t, countSchema.Pattern)
}

func TestXRDGenerator_StringLengthValidation(t *testing.T) {
	minLen := 3
	maxLen := 50
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Username",
					Type:     parsing.StringType{MinLength: &minLen, MaxLength: &maxLen},
					JSONName: "username",
					Comments: "Username with length constraints",
				},
				{
					Name:     "Count",
					Type:     parsing.NumericType{Name: "int32"},
					JSONName: "count",
					Comments: "Count is a number",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check that minLength and maxLength are applied to string field
	usernameSchema := xrd.Schema.Properties["username"]
	require.NotNil(t, usernameSchema)
	require.Equal(t, "string", usernameSchema.Type)
	require.NotNil(t, usernameSchema.MinLength)
	require.Equal(t, 3, *usernameSchema.MinLength)
	require.NotNil(t, usernameSchema.MaxLength)
	require.Equal(t, 50, *usernameSchema.MaxLength)

	// Check that length constraints are NOT applied to non-string field
	countSchema := xrd.Schema.Properties["count"]
	require.NotNil(t, countSchema)
	require.Equal(t, "integer", countSchema.Type)
	require.Nil(t, countSchema.MinLength)
	require.Nil(t, countSchema.MaxLength)
}

func TestXRDGenerator_DefaultValues(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{Default: "default-name"},
					JSONName: "name",
					Comments: "Name with default value",
				},
				{
					Name:     "Count",
					Type:     parsing.NumericType{Name: "int32", Default: 10},
					JSONName: "count",
					Comments: "Count with default value",
				},
				{
					Name:     "Enabled",
					Type:     parsing.PrimitiveType{Name: "bool", Default: true},
					JSONName: "enabled",
					Comments: "Enabled with default value",
				},
				{
					Name:     "Score",
					Type:     parsing.NumericType{Name: "float64", Default: 3.14},
					JSONName: "score",
					Comments: "Score with default value",
				},
				{
					Name:     "Description",
					Type:     parsing.StringType{},
					JSONName: "description",
					Comments: "Description without default",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	nameSchema := xrd.Schema.Properties["name"]
	require.NotNil(t, nameSchema)
	require.Equal(t, "string", nameSchema.Type)
	require.NotNil(t, nameSchema.Default)
	require.Equal(t, "default-name", nameSchema.Default)

	countSchema := xrd.Schema.Properties["count"]
	require.NotNil(t, countSchema)
	require.Equal(t, "integer", countSchema.Type)
	require.NotNil(t, countSchema.Default)
	require.Equal(t, 10, countSchema.Default)

	enabledSchema := xrd.Schema.Properties["enabled"]
	require.NotNil(t, enabledSchema)
	require.Equal(t, "boolean", enabledSchema.Type)
	require.NotNil(t, enabledSchema.Default)
	require.Equal(t, true, enabledSchema.Default)

	scoreSchema := xrd.Schema.Properties["score"]
	require.NotNil(t, scoreSchema)
	require.Equal(t, "number", scoreSchema.Type)
	require.NotNil(t, scoreSchema.Default)
	require.Equal(t, 3.14, scoreSchema.Default)

	descSchema := xrd.Schema.Properties["description"]
	require.NotNil(t, descSchema)
	require.Equal(t, "string", descSchema.Type)
	require.Nil(t, descSchema.Default)
}

func TestXRDGenerator_EnumWithDefault(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:       "Environment",
					Type:       parsing.StringType{Default: "dev"},
					JSONName:   "environment",
					Comments:   "Environment with enum and default",
					EnumValues: []string{"dev", "qa", "staging", "production"},
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	envSchema := xrd.Schema.Properties["environment"]
	require.NotNil(t, envSchema)
	require.Equal(t, "string", envSchema.Type)

	require.Len(t, envSchema.Enum, 4)
	require.Equal(t, []string{"dev", "qa", "staging", "production"}, envSchema.Enum)
	require.NotNil(t, envSchema.Default)
	require.Equal(t, "dev", envSchema.Default)
}

func TestXRDGenerator_EnumValidation(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:       "Status",
					Type:       parsing.StringType{},
					JSONName:   "status",
					Comments:   "Status with enum values",
					EnumValues: []string{"active", "inactive", "pending"},
				},
				{
					Name:     "Count",
					Type:     parsing.NumericType{Name: "int32"},
					JSONName: "count",
					Comments: "Count is a number",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check that enum is applied to string field
	statusSchema := xrd.Schema.Properties["status"]
	require.NotNil(t, statusSchema)
	require.Equal(t, "string", statusSchema.Type)
	require.Len(t, statusSchema.Enum, 3)
	require.Equal(t, []string{"active", "inactive", "pending"}, statusSchema.Enum)

	// Check that enum is not present on field without it
	countSchema := xrd.Schema.Properties["count"]
	require.NotNil(t, countSchema)
	require.Equal(t, "integer", countSchema.Type)
	require.Len(t, countSchema.Enum, 0)
}

func TestIsConditionsField(t *testing.T) {
	// Test that it correctly identifies Conditions field with MetadataType
	conditionsField := parsing.Field{
		Name: "Conditions",
		Type: parsing.ArrayType{
			ElementType: parsing.MetadataType{Name: "Condition", Package: "metav1"},
		},
		JSONName: "conditions",
	}
	require.True(t, isConditionsField(conditionsField))

	// Test with different field name
	otherField := parsing.Field{
		Name: "OtherField",
		Type: parsing.ArrayType{
			ElementType: parsing.MetadataType{Name: "Condition", Package: "metav1"},
		},
		JSONName: "otherField",
	}
	require.False(t, isConditionsField(otherField))

	// Test with conditions but wrong type
	wrongTypeField := parsing.Field{
		Name:     "Conditions",
		Type:     parsing.ArrayType{ElementType: parsing.StringType{}},
		JSONName: "conditions",
	}
	require.False(t, isConditionsField(wrongTypeField))
}

func TestXRDGenerator_SkipsConditionsField(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResourceStatus",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Ready",
					Type:     parsing.PrimitiveType{Name: "bool"},
					JSONName: "ready",
					Comments: "Ready indicates if the resource is ready",
				},
				{
					Name: "Conditions",
					Type: parsing.ArrayType{
						ElementType: parsing.MetadataType{Name: "Condition", Package: "metav1"},
					},
					JSONName: "conditions",
					Comments: "Shows the current state of the resource",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check that ready field is included
	readySchema := xrd.Schema.Properties["ready"]
	require.NotNil(t, readySchema)
	require.Equal(t, "boolean", readySchema.Type)

	// Check that conditions field is NOT included
	_, hasConditions := xrd.Schema.Properties["conditions"]
	require.False(t, hasConditions, "Conditions field should be skipped")
	require.NotContains(t, xrd.Schema.Required, "conditions")
}

func TestXRDGenerator_OnlySkipsConditionsArrayOfCondition(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Conditions",
					Type:     parsing.ArrayType{ElementType: parsing.StringType{}},
					JSONName: "conditions",
					Comments: "Conditions as array of strings (not metav1.Condition)",
				},
				{
					Name:     "OtherField",
					Type:     parsing.ArrayType{ElementType: parsing.MetadataType{Name: "Condition", Package: "metav1"}},
					JSONName: "otherField",
					Comments: "Array of Condition but different field name",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check that conditions field with wrong type IS included
	conditionsSchema := xrd.Schema.Properties["conditions"]
	require.NotNil(t, conditionsSchema, "Conditions field with non-Condition type should be included")
	require.Equal(t, "array", conditionsSchema.Type)

	// Check that other field name with Condition type IS still included
	otherFieldSchema := xrd.Schema.Properties["otherField"]
	require.NotNil(t, otherFieldSchema, "Field with Condition type but different name should be included")
	require.Equal(t, "array", otherFieldSchema.Type)
}

func TestXRDGenerator_UnsignedAndSpecialTypes(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Count",
					Type:     parsing.NumericType{Name: "uint"},
					JSONName: "count",
					Comments: "Count as unsigned int",
				},
				{
					Name:     "Count8",
					Type:     parsing.NumericType{Name: "uint8"},
					JSONName: "count8",
					Comments: "Count as uint8",
				},
				{
					Name:     "Count16",
					Type:     parsing.NumericType{Name: "uint16"},
					JSONName: "count16",
					Comments: "Count as uint16",
				},
				{
					Name:     "Count32",
					Type:     parsing.NumericType{Name: "uint32"},
					JSONName: "count32",
					Comments: "Count as uint32",
				},
				{
					Name:     "Count64",
					Type:     parsing.NumericType{Name: "uint64"},
					JSONName: "count64",
					Comments: "Count as uint64",
				},
				{
					Name:     "Data",
					Type:     parsing.PrimitiveType{Name: "byte"},
					JSONName: "data",
					Comments: "Data as byte",
				},
				{
					Name:     "Char",
					Type:     parsing.PrimitiveType{Name: "rune"},
					JSONName: "char",
					Comments: "Char as rune",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check uint types
	countSchema := xrd.Schema.Properties["count"]
	require.NotNil(t, countSchema)
	require.Equal(t, "integer", countSchema.Type)
	require.NotNil(t, countSchema.Format)
	require.Equal(t, "int32", *countSchema.Format)

	count8Schema := xrd.Schema.Properties["count8"]
	require.NotNil(t, count8Schema)
	require.Equal(t, "integer", count8Schema.Type)
	require.NotNil(t, count8Schema.Format)
	require.Equal(t, "int32", *count8Schema.Format)

	count16Schema := xrd.Schema.Properties["count16"]
	require.NotNil(t, count16Schema)
	require.Equal(t, "integer", count16Schema.Type)
	require.NotNil(t, count16Schema.Format)
	require.Equal(t, "int32", *count16Schema.Format)

	count32Schema := xrd.Schema.Properties["count32"]
	require.NotNil(t, count32Schema)
	require.Equal(t, "integer", count32Schema.Type)
	require.NotNil(t, count32Schema.Format)
	require.Equal(t, "int32", *count32Schema.Format)

	count64Schema := xrd.Schema.Properties["count64"]
	require.NotNil(t, count64Schema)
	require.Equal(t, "integer", count64Schema.Type)
	require.NotNil(t, count64Schema.Format)
	require.Equal(t, "int64", *count64Schema.Format)

	// Check byte type
	dataSchema := xrd.Schema.Properties["data"]
	require.NotNil(t, dataSchema)
	require.Equal(t, "integer", dataSchema.Type)
	require.NotNil(t, dataSchema.Format)
	require.Equal(t, "int32", *dataSchema.Format)

	// Check rune type
	charSchema := xrd.Schema.Properties["char"]
	require.NotNil(t, charSchema)
	require.Equal(t, "integer", charSchema.Type)
	require.NotNil(t, charSchema.Format)
	require.Equal(t, "int32", *charSchema.Format)
}

func TestXRDGenerator_MissingGroupReturnsError(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "", // Missing group annotation
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{},
					JSONName: "name",
					Comments: "Name of the resource",
				},
			},
		},
	}

	_, err := GenerateXRD(structs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "group annotation is required")
}

func TestXRDGenerator_MissingGroupMultipleStructsReturnsError(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "ValidResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{},
					JSONName: "name",
				},
			},
		},
		{
			Name:    "InvalidResource",
			Package: "test",
			IsRoot:  true,
			Group:   "", // Missing group annotation
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Status",
					Type:     parsing.StringType{},
					JSONName: "status",
				},
			},
		},
	}

	_, err := GenerateXRD(structs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "group annotation is required")
	require.Contains(t, err.Error(), "InvalidResource")
}

func TestXRDGenerator_MissingGroupNonRootStruct(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Config",
					Type:     parsing.StructType{Name: "Config"},
					JSONName: "config",
				},
			},
		},
		{
			Name:    "Config",
			Package: "test",
			IsRoot:  false,
			Group:   "", // Non-root structs don't need group annotation
			Fields: []parsing.Field{
				{
					Name:     "Value",
					Type:     parsing.StringType{},
					JSONName: "value",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)
	require.Equal(t, "example.com", xrds[0].Group)
}

func TestXRDGenerator_TimeTypes(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "XEvent",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1",
			Fields: []parsing.Field{
				{
					Name:     "Timestamp",
					JSONName: "timestamp",
					Type: parsing.TimeType{
						Package:    "time",
						ImportPath: "time",
					},
					Comments: "Timestamp of the event",
				},
				{
					Name:     "LastUpdated",
					JSONName: "lastUpdated",
					Type: parsing.TimeType{
						Package:    "metav1",
						ImportPath: "k8s.io/apimachinery/pkg/apis/meta/v1",
					},
					Comments:   "Last time this event was updated",
					IsOptional: true,
				},
				{
					Name:     "OptionalTime",
					JSONName: "optionalTime",
					Type: parsing.TimeType{
						Package:    "time",
						ImportPath: "time",
						IsPointer:  true,
					},
					Comments:   "Optional timestamp",
					IsOptional: true,
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check timestamp field
	timestampSchema := xrd.Schema.Properties["timestamp"]
	require.NotNil(t, timestampSchema)
	require.Equal(t, "string", timestampSchema.Type)
	require.NotNil(t, timestampSchema.Format)
	require.Equal(t, "date-time", *timestampSchema.Format)
	require.Equal(t, "Timestamp of the event", timestampSchema.Description)

	// Check lastUpdated field
	lastUpdatedSchema := xrd.Schema.Properties["lastUpdated"]
	require.NotNil(t, lastUpdatedSchema)
	require.Equal(t, "string", lastUpdatedSchema.Type)
	require.NotNil(t, lastUpdatedSchema.Format)
	require.Equal(t, "date-time", *lastUpdatedSchema.Format)
	require.Equal(t, "Last time this event was updated", lastUpdatedSchema.Description)

	// Check optionalTime field
	optionalTimeSchema := xrd.Schema.Properties["optionalTime"]
	require.NotNil(t, optionalTimeSchema)
	require.Equal(t, "string", optionalTimeSchema.Type)
	require.NotNil(t, optionalTimeSchema.Format)
	require.Equal(t, "date-time", *optionalTimeSchema.Format)
	require.Equal(t, "Optional timestamp", optionalTimeSchema.Description)

	// Check required fields (timestamp should be required, others optional)
	require.Contains(t, xrd.Schema.Required, "timestamp")
	require.NotContains(t, xrd.Schema.Required, "lastUpdated")
	require.NotContains(t, xrd.Schema.Required, "optionalTime")
}

func TestXRDGenerator_NumericTypes(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "XConfig",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1",
			Fields: []parsing.Field{
				{
					Name:     "Score",
					JSONName: "score",
					Type: parsing.NumericType{
						Name:    "int32",
						Minimum: ptr.To(0.0),
						Maximum: ptr.To(100.0),
					},
					Comments: "Score must be between 0 and 100",
				},
				{
					Name:     "Price",
					JSONName: "price",
					Type: parsing.NumericType{
						Name:    "float64",
						Minimum: ptr.To(1.0),
					},
					Comments: "Price must be positive",
				},
				{
					Name:     "Count",
					JSONName: "count",
					Type: parsing.NumericType{
						Name:    "int",
						Maximum: ptr.To(10000.0),
					},
					Comments:   "Max count is 10000",
					IsOptional: true,
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.Schema)

	// Check score field with min/max
	scoreSchema := xrd.Schema.Properties["score"]
	require.NotNil(t, scoreSchema)
	require.Equal(t, "integer", scoreSchema.Type)
	require.NotNil(t, scoreSchema.Format)
	require.Equal(t, "int32", *scoreSchema.Format)
	require.NotNil(t, scoreSchema.Minimum)
	require.Equal(t, 0.0, *scoreSchema.Minimum)
	require.NotNil(t, scoreSchema.Maximum)
	require.Equal(t, 100.0, *scoreSchema.Maximum)
	require.Equal(t, "Score must be between 0 and 100", scoreSchema.Description)

	// Check price field with minimum only
	priceSchema := xrd.Schema.Properties["price"]
	require.NotNil(t, priceSchema)
	require.Equal(t, "number", priceSchema.Type)
	require.NotNil(t, priceSchema.Format)
	require.Equal(t, "double", *priceSchema.Format)
	require.NotNil(t, priceSchema.Minimum)
	require.Equal(t, 1.0, *priceSchema.Minimum)
	require.Nil(t, priceSchema.Maximum)
	require.Equal(t, "Price must be positive", priceSchema.Description)

	// Check count field with maximum only
	countSchema := xrd.Schema.Properties["count"]
	require.NotNil(t, countSchema)
	require.Equal(t, "integer", countSchema.Type)
	require.NotNil(t, countSchema.Format)
	require.Equal(t, "int32", *countSchema.Format)
	require.Nil(t, countSchema.Minimum)
	require.NotNil(t, countSchema.Maximum)
	require.Equal(t, 10000.0, *countSchema.Maximum)
	require.Equal(t, "Max count is 10000", countSchema.Description)

	// Check required fields
	require.Contains(t, xrd.Schema.Required, "score")
	require.Contains(t, xrd.Schema.Required, "price")
	require.NotContains(t, xrd.Schema.Required, "count")
}

func TestXRDGenerator_PrinterColumns(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "XMyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					JSONName: "name",
					Type:     parsing.StringType{},
					Comments: "Name of the resource",
				},
			},
			PrinterColumns: []parsing.PrinterColumn{
				{
					Name:        "health",
					Type:        "string",
					JSONPath:    ".status.health",
					Description: "Health status",
					Priority:    0,
				},
				{
					Name:        "nodes",
					Type:        "integer",
					JSONPath:    ".status.availableNodes",
					Description: "Available nodes",
					Priority:    0,
				},
				{
					Name:        "version",
					Type:        "string",
					JSONPath:    ".status.version",
					Description: "Version",
					Priority:    1,
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]
	require.NotNil(t, xrd.AdditionalPrinterColumns)
	require.Len(t, xrd.AdditionalPrinterColumns, 3)

	healthCol := xrd.AdditionalPrinterColumns[0]
	require.Equal(t, "health", healthCol.Name)
	require.Equal(t, "string", healthCol.Type)
	require.Equal(t, ".status.health", healthCol.JSONPath)
	require.Equal(t, "Health status", healthCol.Description)
	require.Equal(t, int32(0), healthCol.Priority)

	nodesCol := xrd.AdditionalPrinterColumns[1]
	require.Equal(t, "nodes", nodesCol.Name)
	require.Equal(t, "integer", nodesCol.Type)
	require.Equal(t, ".status.availableNodes", nodesCol.JSONPath)
	require.Equal(t, "Available nodes", nodesCol.Description)
	require.Equal(t, int32(0), nodesCol.Priority)

	versionCol := xrd.AdditionalPrinterColumns[2]
	require.Equal(t, "version", versionCol.Name)
	require.Equal(t, "string", versionCol.Type)
	require.Equal(t, ".status.version", versionCol.JSONPath)
	require.Equal(t, "Version", versionCol.Description)
	require.Equal(t, int32(1), versionCol.Priority)
}

func TestXRDGenerator_LabelsAndAnnotations(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "XMyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					JSONName: "name",
					Type:     parsing.StringType{},
					Comments: "Name of the resource",
				},
			},
			KubernetesLabels: map[string]string{
				"app.kubernetes.io/name":       "myapp",
				"app.kubernetes.io/managed-by": "xrd-gen",
				"environment":                  "production",
			},
			KubernetesAnnotations: map[string]string{
				"description":         "My custom resource",
				"owner":               "platform-team",
				"example.com/version": "v1.2.3",
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]

	require.NotNil(t, xrd.Labels)
	require.Len(t, xrd.Labels, 3)
	require.Equal(t, "myapp", xrd.Labels["app.kubernetes.io/name"])
	require.Equal(t, "xrd-gen", xrd.Labels["app.kubernetes.io/managed-by"])
	require.Equal(t, "production", xrd.Labels["environment"])

	require.NotNil(t, xrd.Annotations)
	require.Len(t, xrd.Annotations, 3)
	require.Equal(t, "My custom resource", xrd.Annotations["description"])
	require.Equal(t, "platform-team", xrd.Annotations["owner"])
	require.Equal(t, "v1.2.3", xrd.Annotations["example.com/version"])
}

func TestXRDGenerator_EmptyLabelsAndAnnotations(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "XMyResource",
			Package: "test",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					JSONName: "name",
					Type:     parsing.StringType{},
					Comments: "Name of the resource",
				},
			},
			KubernetesLabels:      map[string]string{},
			KubernetesAnnotations: map[string]string{},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	xrd := xrds[0]

	require.NotNil(t, xrd.Labels)
	require.Empty(t, xrd.Labels)
	require.NotNil(t, xrd.Annotations)
	require.Empty(t, xrd.Annotations)
}

func TestXRDGenerator_GenerateXRD_XValidations_FieldLevel(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "v1",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{},
					JSONName: "name",
					XValidations: []parsing.XValidation{
						{Rule: "self == oldSelf", Message: "name is immutable"},
					},
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	nameSchema := xrds[0].Schema.Properties["name"]
	require.Len(t, nameSchema.XValidations, 1)
	require.Equal(t, "self == oldSelf", nameSchema.XValidations[0].Rule)
	require.Equal(t, "name is immutable", nameSchema.XValidations[0].Message)
}

func TestXRDGenerator_GenerateXRD_XValidations_StructLevel(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "v1",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			XValidations: []parsing.XValidation{
				{Rule: "self.minReplicas <= self.maxReplicas", Message: "minReplicas must not exceed maxReplicas"},
			},
			Fields: []parsing.Field{
				{
					Name:     "MinReplicas",
					Type:     parsing.NumericType{Name: "int32"},
					JSONName: "minReplicas",
				},
				{
					Name:     "MaxReplicas",
					Type:     parsing.NumericType{Name: "int32"},
					JSONName: "maxReplicas",
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	schema := xrds[0].Schema
	require.Len(t, schema.XValidations, 1)
	require.Equal(t, "self.minReplicas <= self.maxReplicas", schema.XValidations[0].Rule)
	require.Equal(t, "minReplicas must not exceed maxReplicas", schema.XValidations[0].Message)
}

func TestXRDGenerator_GenerateXRD_XValidations_MultipleRules(t *testing.T) {
	structs := []parsing.Struct{
		{
			Name:    "MyResource",
			Package: "v1",
			IsRoot:  true,
			Group:   "example.com",
			Version: "v1alpha1",
			Fields: []parsing.Field{
				{
					Name:     "Name",
					Type:     parsing.StringType{},
					JSONName: "name",
					XValidations: []parsing.XValidation{
						{Rule: "self.startsWith('kube-')", Message: "must start with kube-"},
						{Rule: "size(self) <= 63", Message: "must be at most 63 characters"},
					},
				},
			},
		},
	}

	xrds, err := GenerateXRD(structs)
	require.NoError(t, err)
	require.Len(t, xrds, 1)

	nameSchema := xrds[0].Schema.Properties["name"]
	require.Len(t, nameSchema.XValidations, 2)
	require.Equal(t, "self.startsWith('kube-')", nameSchema.XValidations[0].Rule)
	require.Equal(t, "size(self) <= 63", nameSchema.XValidations[1].Rule)
}
