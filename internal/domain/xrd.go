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
	"fmt"
	"strings"

	"github.com/gertd/go-pluralize"
	"k8s.io/utils/ptr"

	"github.com/crossplane-contrib/xrd-gen/internal/parsing"
)

const defaultVersion = "v1alpha1"

type CompositeResourceDefinition struct {
	Name                     string
	Group                    string
	Version                  string
	Kind                     string
	PluralName               string
	Schema                   *OpenAPISchema
	AdditionalPrinterColumns []parsing.PrinterColumn
	Labels                   map[string]string
	Annotations              map[string]string
}

type OpenAPISchema struct {
	Type                 string
	Description          string
	Properties           map[string]*OpenAPISchema
	Items                *OpenAPISchema
	AdditionalProperties *OpenAPISchema
	Required             []string
	MinItems             *int
	MaxItems             *int
	MinLength            *int
	MaxLength            *int
	Minimum              *float64
	Maximum              *float64
	Pattern              *string
	Format               *string
	Enum                 []string
	Default              any
	XValidations         []parsing.XValidation
}

func GenerateXRD(structs []parsing.Struct) ([]*CompositeResourceDefinition, error) {
	xrds := make([]*CompositeResourceDefinition, 0)
	structMap := make(map[string]parsing.Struct)
	for _, s := range structs {
		structMap[s.FullyQualifiedName()] = s
	}

	for _, s := range structs {
		if !s.IsRoot {
			continue
		}

		xrd, err := generateXRDForStruct(s, structMap)
		if err != nil {
			return nil, fmt.Errorf("failed to generate XRD for struct %s: %w", s.Name, err)
		}
		xrds = append(xrds, xrd)
	}

	return xrds, nil
}

func generateXRDForStruct(s parsing.Struct, structMap map[string]parsing.Struct) (*CompositeResourceDefinition, error) {
	if s.Group == "" {
		return nil, fmt.Errorf("group annotation is required")
	}

	kind := s.Name
	plural := strings.ToLower(s.Plural)
	if plural == "" {
		pluralizeClient := pluralize.NewClient()
		plural = strings.ToLower(pluralizeClient.Pluralize(s.Name, 2, false))
	}

	version := s.Version
	if version == "" {
		version = defaultVersion
	}

	name := plural + "." + s.Group

	xrd := &CompositeResourceDefinition{
		Name:                     name,
		Group:                    s.Group,
		Version:                  version,
		Kind:                     kind,
		PluralName:               plural,
		Schema:                   buildSchema(s, structMap),
		AdditionalPrinterColumns: s.PrinterColumns,
		Labels:                   s.KubernetesLabels,
		Annotations:              s.KubernetesAnnotations,
	}

	return xrd, nil
}

func buildSchema(s parsing.Struct, structMap map[string]parsing.Struct) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:         "object",
		Properties:   make(map[string]*OpenAPISchema),
		Required:     []string{},
		Description:  s.Comments,
		XValidations: s.XValidations,
	}

	for _, field := range s.Fields {
		if field.IsInline || isMetadataField(field.Type) || isConditionsField(field) {
			continue
		}

		if field.JSONName == "" || field.JSONName == "-" {
			continue
		}

		fieldSchema := buildFieldSchema(field, structMap)
		schema.Properties[field.JSONName] = fieldSchema

		if !field.IsOptional {
			schema.Required = append(schema.Required, field.JSONName)
		}
	}

	return schema
}

func isMetadataField(fieldType parsing.FieldType) bool {
	_, ok := fieldType.(parsing.MetadataType)
	return ok
}

// isConditionsField checks if a field is the Conditions field of type []metav1.Condition
// Crossplane automatically adds this field, so we skip explicitly defining it in the XRD definition
func isConditionsField(field parsing.Field) bool {
	if field.Name != "Conditions" {
		return false
	}

	arrayType, ok := field.Type.(parsing.ArrayType)
	if !ok {
		return false
	}

	if metadataType, ok := arrayType.ElementType.(parsing.MetadataType); ok {
		return strings.Contains(metadataType.Name, "Condition")
	}

	return false
}

func buildFieldSchema(field parsing.Field, structMap map[string]parsing.Struct) *OpenAPISchema {
	schema := &OpenAPISchema{
		Description:  field.Comments,
		XValidations: field.XValidations,
	}

	// Regex pattern validation only applies to string fields
	if stringType, ok := field.Type.(parsing.StringType); ok && stringType.Pattern != "" {
		schema.Pattern = &stringType.Pattern
	}

	if len(field.EnumValues) > 0 {
		schema.Enum = field.EnumValues
	}

	return buildFieldTypeSchema(field.Type, structMap, schema)
}

func buildFieldTypeSchema(fieldType parsing.FieldType, structMap map[string]parsing.Struct, schema *OpenAPISchema) *OpenAPISchema {
	switch t := fieldType.(type) {
	case parsing.StringType:
		schema.Type = "string"
		if t.MinLength != nil {
			schema.MinLength = t.MinLength
		}
		if t.MaxLength != nil {
			schema.MaxLength = t.MaxLength
		}
		if t.Default != "" {
			schema.Default = t.Default
		}
		return schema

	case parsing.TimeType:
		schema.Type = "string"
		schema.Format = ptr.To("date-time")
		return schema

	case parsing.NumericType:
		openAPIType, format := mapNumericToOpenAPI(t.Name)
		schema.Type = openAPIType
		if format != "" {
			schema.Format = &format
		}
		if t.Minimum != nil {
			schema.Minimum = t.Minimum
		}
		if t.Maximum != nil {
			schema.Maximum = t.Maximum
		}
		if t.Default != nil {
			schema.Default = t.Default
		}
		return schema

	case parsing.PrimitiveType:
		openAPIType, format := mapPrimitiveToOpenAPI(t.Name)
		schema.Type = openAPIType
		if format != "" {
			schema.Format = &format
		}
		if t.Default != nil {
			schema.Default = t.Default
		}
		return schema

	case parsing.ArrayType:
		schema.Type = "array"
		if t.MinItems != nil {
			schema.MinItems = t.MinItems
		}
		if t.MaxItems != nil {
			schema.MaxItems = t.MaxItems
		}

		itemSchema := buildFieldTypeSchema(t.ElementType, structMap, &OpenAPISchema{})
		schema.Items = itemSchema
		return schema

	case parsing.MapType:
		schema.Type = "object"
		valueSchema := buildFieldTypeSchema(t.ValueType, structMap, &OpenAPISchema{})
		schema.AdditionalProperties = valueSchema
		return schema

	case parsing.StructType:
		if structDef, ok := structMap[t.FullyQualifiedName()]; ok {
			nestedSchema := buildSchema(structDef, structMap)
			if schema.Description != "" {
				nestedSchema.Description = schema.Description
			}
			return nestedSchema
		}
		schema.Type = "object"
		return schema

	default:
		schema.Type = "object"
		return schema
	}
}

func mapNumericToOpenAPI(numericType string) (string, string) {
	switch numericType {
	case "int", "int32", "int16", "int8":
		return "integer", "int32"
	case "int64":
		return "integer", "int64"
	case "uint", "uint32", "uint16", "uint8":
		return "integer", "int32"
	case "uint64":
		return "integer", "int64"
	case "float32":
		return "number", "float"
	case "float64":
		return "number", "double"
	default:
		return "integer", "int32"
	}
}

func mapPrimitiveToOpenAPI(primitiveType string) (string, string) {
	switch primitiveType {
	case "bool":
		return "boolean", ""
	case "byte":
		return "integer", "int32"
	case "rune":
		return "integer", "int32"
	default:
		return "object", ""
	}
}
