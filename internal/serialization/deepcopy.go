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
	_ "embed"
	"errors"
	"fmt"

	"github.com/crossplane-contrib/xrd-gen/internal/domain/metadata"
	"github.com/crossplane-contrib/xrd-gen/internal/parsing"
	"github.com/crossplane-contrib/xrd-gen/pkg/version"
)

//go:embed templates/deepcopy.tmpl
var deepcopyTemplate string

// Note: This implementation always generates DeepCopy methods with pointer receivers,
// which is the standard approach for Kubernetes types and Crossplane XRDs.
// Non-pointer receiver variants are not currently supported.
// For reference on non-pointer receiver support in controller-tools, see:
// https://github.com/kubernetes-sigs/controller-tools/blob/b0752da2e97c54bb4ab415d55e9a7c5c8dfd586e/pkg/deepcopy/traverse.go#L238

type templateData struct {
	Header      string
	Version     string
	PackageName string
	Imports     []importInfo
	Structs     []structInfo
}

type structInfo struct {
	Name        string
	HasTypeMeta bool
	Fields      []fieldCode
}

type fieldCode struct {
	Code string
}

func GenerateDeepCopy(structs []parsing.Struct, headerFile string) ([]byte, error) {
	if len(structs) == 0 {
		return nil, errors.New("no structs provided")
	}

	// Use shared helper to prepare structs
	header, packageName, targetStructs, err := prepareStructsForGeneration(structs, headerFile)
	if err != nil {
		return nil, err
	}

	structInfos := make([]structInfo, 0, len(targetStructs))
	needsRuntimeImport := false
	allImports := make(map[importInfo]bool)

	for _, s := range targetStructs {
		hasTypeMeta := hasInlineTypeMeta(s)
		if hasTypeMeta {
			// needed since we will be generating a method that returns a runtime.Object if there is a TypeMeta field
			needsRuntimeImport = true
		}

		var fields []fieldCode
		for _, field := range s.Fields {
			// Imports are only collected for code that names the field's type.
			// Fields without copy code are covered by *out = *in, and metadata
			// fields are copied by assignment or their own deep copy method.
			// Collecting more could pull in a second package under an alias already
			// in use.
			code := generateFieldCode(field, packageName)
			if code == "" {
				continue
			}
			fields = append(fields, fieldCode{Code: code})
			if _, isMetadata := field.Type.(parsing.MetadataType); !isMetadata {
				collectImports(field.Type, packageName, allImports)
			}
		}

		structInfos = append(structInfos, structInfo{
			Name:        s.Name,
			HasTypeMeta: hasTypeMeta,
			Fields:      fields,
		})
	}

	if needsRuntimeImport {
		allImports[importInfo{Alias: runtimeAlias, Path: runtimeImportPath}] = true
	}

	imports := sortImports(allImports)
	data := templateData{
		Header:      header,
		Version:     version.Version,
		PackageName: packageName,
		Imports:     imports,
		Structs:     structInfos,
	}

	return executeTemplate("deepcopy", deepcopyTemplate, data)
}

func hasInlineTypeMeta(s parsing.Struct) bool {
	for _, field := range s.Fields {
		if field.IsInline {
			if metaType, ok := field.Type.(parsing.MetadataType); ok {
				return metadata.IsTypeMeta(metaType.Name)
			}
		}
	}
	return false
}

func generateFieldCode(field parsing.Field, packageName string) string {
	// Handle inline/embedded metadata types first (TypeMeta, ObjectMeta, ListMeta)
	// These have empty names but we still need to generate code for them
	if metadataType, ok := field.Type.(parsing.MetadataType); ok {
		fieldName := field.Name
		if fieldName == "" {
			// Use the type name for inline/embedded fields
			fieldName = metadataType.Name
		}

		// TypeMeta is shallow copied
		if metadata.IsTypeMeta(metadataType.Name) {
			return fmt.Sprintf("\tout.%s = in.%s", fieldName, fieldName)
		}
		// ObjectMeta, ListMeta need DeepCopyInto
		return fmt.Sprintf("\tin.%s.DeepCopyInto(&out.%s)", fieldName, fieldName)
	}

	// Skip other inline/embedded fields - already handled by *out = *in
	if field.IsInline || field.Name == "" {
		return ""
	}

	switch fieldType := field.Type.(type) {
	case parsing.PrimitiveType:
		if fieldType.IsPointer {
			return generatePointerToPrimitive(field.Name, fieldType.Name)
		}
		// Non-pointer primitives handled by *out = *in
		return ""

	case parsing.StringType:
		if fieldType.IsPointer {
			return generatePointerToPrimitive(field.Name, fieldType.Name())
		}
		// Non-pointer strings handled by *out = *in
		return ""

	case parsing.NumericType:
		if fieldType.IsPointer {
			return generatePointerToPrimitive(field.Name, fieldType.Name)
		}
		// Non-pointer numeric types handled by *out = *in
		return ""

	case parsing.TimeType:
		if fieldType.IsPointer {
			return generatePointerToPrimitive(field.Name, fieldType.FullyQualifiedName())
		}
		// Non-pointer time types handled by *out = *in
		return ""

	case parsing.ArrayType:
		return generateSliceCode(field.Name, fieldType, packageName)

	case parsing.MapType:
		return generateMapCode(field.Name, fieldType, packageName)

	case parsing.StructType:
		if structNeedsDeepCopy(fieldType, packageName) {
			if fieldType.IsPointer {
				return generatePointerToStruct(field.Name, fieldType.Name)
			}
			return generateStructField(field.Name)
		}
		// External struct without DeepCopy handled by *out = *in
		return ""
	}
	return ""
}

func structNeedsDeepCopy(fieldType parsing.FieldType, packageName string) bool {
	switch t := fieldType.(type) {
	case parsing.StructType:
		// Same package structs always get DeepCopy (since we are in charge of generating them)
		return t.Package == packageName
	case parsing.MetadataType:
		// We know that k8s types already have these methods
		return !metadata.IsTypeMeta(t.Name)
	default:
		return false
	}
}

func generatePointerToPrimitive(fieldName, typeName string) string {
	return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = new(%s)
		**out = **in
	}`, fieldName, fieldName, fieldName, typeName)
}

func generatePointerToStruct(fieldName, typeName string) string {
	return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = new(%s)
		(*in).DeepCopyInto(*out)
	}`, fieldName, fieldName, fieldName, typeName)
}

func generateStructField(fieldName string) string {
	return fmt.Sprintf("\tin.%s.DeepCopyInto(&out.%s)", fieldName, fieldName)
}

func generateSliceCode(fieldName string, arrayType parsing.ArrayType, packageName string) string {
	elementTypeName := getTypeName(arrayType.ElementType, packageName)

	// Check if struct elements in the array need DeepCopy
	if structNeedsDeepCopy(arrayType.ElementType, packageName) {
		return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = make([]%s, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}`, fieldName, fieldName, fieldName, elementTypeName)
	}

	if isNestedCollection(arrayType.ElementType) {
		return generateNestedSliceCode(fieldName, arrayType, packageName)
	}

	// Slice of primitives does not need a DeepCopy for each element
	return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = make([]%s, len(*in))
		copy(*out, *in)
	}`, fieldName, fieldName, fieldName, elementTypeName)
}

func generateMapCode(fieldName string, mapType parsing.MapType, packageName string) string {
	keyTypeName := getTypeName(mapType.KeyType, packageName)
	valueTypeName := getTypeName(mapType.ValueType, packageName)
	mapTypeName := fmt.Sprintf("map[%s]%s", keyTypeName, valueTypeName)

	// Check if struct values in the map need DeepCopy
	if structNeedsDeepCopy(mapType.ValueType, packageName) {
		return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = make(%s, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}`, fieldName, fieldName, fieldName, mapTypeName)
	}

	if isNestedCollection(mapType.ValueType) {
		return generateNestedMapCode(fieldName, mapType, packageName)
	}

	// Map with primitive values does not need a DeepCopy for each value
	return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = make(%s, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}`, fieldName, fieldName, fieldName, mapTypeName)
}

func isNestedCollection(fieldType parsing.FieldType) bool {
	switch fieldType.(type) {
	case parsing.ArrayType, parsing.MapType:
		return true
	default:
		return false
	}
}

func generateNestedSliceCode(fieldName string, arrayType parsing.ArrayType, packageName string) string {
	elementTypeName := getTypeName(arrayType.ElementType, packageName)
	innerCopyCode := generateNestedElementCopyCode(arrayType.ElementType, packageName)

	return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = make([]%s, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				%s
			}
		}
	}`, fieldName, fieldName, fieldName, elementTypeName, innerCopyCode)
}

func generateNestedElementCopyCode(elementType parsing.FieldType, packageName string) string {
	switch t := elementType.(type) {
	case parsing.ArrayType:
		// Check if this is a nested collection (array of arrays, etc) or just array of primitives/structs
		if isNestedCollection(t.ElementType) {
			// Recursively handle deeply nested arrays (e.g., array of arrays of arrays)
			elementTypeName := getTypeName(t.ElementType, packageName)
			innerCopyCode := generateNestedElementCopyCode(t.ElementType, packageName)
			return fmt.Sprintf(`*out = make([]%s, len(*in))
				for i := range *in {
					if (*in)[i] != nil {
						in, out := &(*in)[i], &(*out)[i]
						%s
					}
				}`, elementTypeName, innerCopyCode)
		}
		// Check if elements need DeepCopy
		if structNeedsDeepCopy(t.ElementType, packageName) {
			elementTypeName := getTypeName(t.ElementType, packageName)
			return fmt.Sprintf(`*out = make([]%s, len(*in))
				for i := range *in {
					(*in)[i].DeepCopyInto(&(*out)[i])
				}`, elementTypeName)
		}
		// Array of primitives - just use copy
		elementTypeName := getTypeName(t.ElementType, packageName)
		return fmt.Sprintf(`*out = make([]%s, len(*in))
				copy(*out, *in)`, elementTypeName)
	case parsing.MapType:
		// Check if this is a nested collection (map with array/map values) or just map of primitives/structs
		if isNestedCollection(t.ValueType) {
			// Recursively handle maps with nested collection values
			keyTypeName := getTypeName(t.KeyType, packageName)
			valueTypeName := getTypeName(t.ValueType, packageName)
			innerCopyCode := generateNestedMapValueCopyCode(t.ValueType, packageName)
			return fmt.Sprintf(`*out = make(map[%s]%s, len(*in))
				for key, val := range *in {
					%s
				}`, keyTypeName, valueTypeName, innerCopyCode)
		}
		// Check if values need DeepCopy
		if structNeedsDeepCopy(t.ValueType, packageName) {
			keyTypeName := getTypeName(t.KeyType, packageName)
			valueTypeName := getTypeName(t.ValueType, packageName)
			return fmt.Sprintf(`*out = make(map[%s]%s, len(*in))
				for key, val := range *in {
					(*out)[key] = *val.DeepCopy()
				}`, keyTypeName, valueTypeName)
		}
		// Map of primitives - just copy each value
		keyTypeName := getTypeName(t.KeyType, packageName)
		valueTypeName := getTypeName(t.ValueType, packageName)
		return fmt.Sprintf(`*out = make(map[%s]%s, len(*in))
				for key, val := range *in {
					(*out)[key] = val
				}`, keyTypeName, valueTypeName)
	default:
		// Base case: primitive type (should not reach here in normal flow)
		return "// ERROR: unexpected primitive type in nested element copy"
	}
}

func generateNestedMapCode(fieldName string, mapType parsing.MapType, packageName string) string {
	keyTypeName := getTypeName(mapType.KeyType, packageName)
	valueTypeName := getTypeName(mapType.ValueType, packageName)
	mapTypeName := fmt.Sprintf("map[%s]%s", keyTypeName, valueTypeName)

	innerCopyCode := generateNestedMapValueCopyCode(mapType.ValueType, packageName)

	return fmt.Sprintf(`if in.%s != nil {
		in, out := &in.%s, &out.%s
		*out = make(%s, len(*in))
		for key, val := range *in {
			%s
		}
	}`, fieldName, fieldName, fieldName, mapTypeName, innerCopyCode)
}

func generateNestedMapValueCopyCode(valueType parsing.FieldType, packageName string) string {
	valueTypeName := getTypeName(valueType, packageName)

	switch t := valueType.(type) {
	case parsing.ArrayType:
		// Map value is an array - need to deep copy it
		innerCopyCode := generateNestedElementCopyCode(t, packageName)
		return fmt.Sprintf(`var outVal %s
			if val == nil {
				(*out)[key] = nil
			} else {
				inVal := (*in)[key]
				in, out := &inVal, &outVal
				%s
			}
			(*out)[key] = outVal`, valueTypeName, innerCopyCode)
	case parsing.MapType:
		// Map value is a map - need to deep copy it
		innerCopyCode := generateNestedMapValueCopyCode(t.ValueType, packageName)
		return fmt.Sprintf(`var outVal %s
			if val == nil {
				(*out)[key] = nil
			} else {
				inVal := (*in)[key]
				in, out := &inVal, &outVal
				*out = make(%s, len(*in))
				for key, val := range *in {
					%s
				}
			}
			(*out)[key] = outVal`, valueTypeName, valueTypeName, innerCopyCode)
	default:
		if structNeedsDeepCopy(valueType, packageName) {
			return "(*out)[key] = *val.DeepCopy()"
		}
		return "(*out)[key] = val"
	}
}
