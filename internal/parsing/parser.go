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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/crossplane-contrib/xrd-gen/internal"
	"github.com/crossplane-contrib/xrd-gen/internal/domain/metadata"
)

const (
	annotationPrefix = "+xrd-gen:"
	stringTypeName   = "string"

	// Struct-level annotation keys
	annotationObjectRoot     = "object:root"
	annotationGroupName      = "groupName"
	annotationVersion        = "version"
	annotationPlural         = "plural"
	annotationPrinterColumns = "printcolumn"
	annotationLabel          = "label"
	annotationAnnotation     = "annotation"

	// Field validation annotation keys
	annotationValidationOptional    = "validation:Optional"
	annotationValidationPattern     = "validation:Pattern"
	annotationValidationEnum        = "validation:Enum"
	annotationValidationMinLength   = "validation:MinLength"
	annotationValidationMaxLength   = "validation:MaxLength"
	annotationValidationMinItems    = "validation:MinItems"
	annotationValidationMaxItems    = "validation:MaxItems"
	annotationValidationMinimum     = "validation:Minimum"
	annotationValidationMaximum     = "validation:Maximum"
	annotationValidationDefault     = "validation:Default"
	annotationValidationXValidation = "validation:XValidation:"
)

// XValidation represents a CEL validation rule, following the same structure
// as the Kubernetes x-kubernetes-validations field.
type XValidation struct {
	Rule              string
	Message           string
	MessageExpression string
	Reason            string
	FieldPath         string
	OptionalOldSelf   *bool
}

type Parser struct {
	fset *token.FileSet
}

type Struct struct {
	Name                  string
	Package               string
	Comments              string
	Fields                []Field
	IsRoot                bool
	Group                 string
	Version               string
	Plural                string
	PrinterColumns        []PrinterColumn
	KubernetesLabels      map[string]string
	KubernetesAnnotations map[string]string
	XValidations          []XValidation
	commentAnnotations    map[string]string
}

type PrinterColumn struct {
	Name        string
	Type        string
	JSONPath    string
	Description string
	Priority    int32
}

func (s Struct) FullyQualifiedName() string {
	return fullyQualifiedName(s.Package, s.Name)
}

type FieldType interface {
	isFieldType()
}

type PrimitiveType struct {
	Name      string
	IsPointer bool
	Default   any
}

func (PrimitiveType) isFieldType() {}

type StringType struct {
	IsPointer bool
	MinLength *int
	MaxLength *int
	Pattern   string
	Default   string
}

func (StringType) isFieldType() {}

func (StringType) Name() string {
	return stringTypeName
}

type TimeType struct {
	IsPointer  bool
	Package    string // "time" for time.Time, or "v1" for metav1.Time
	ImportPath string // "time" for time.Time, or "k8s.io/apimachinery/pkg/apis/meta/v1" for metav1.Time
}

func (t TimeType) FullyQualifiedName() string {
	return fmt.Sprintf("%s.Time", t.Package)
}

func (TimeType) isFieldType() {}

type NumericType struct {
	Name      string
	IsPointer bool
	Minimum   *float64
	Maximum   *float64
	Default   any
}

func (NumericType) isFieldType() {}

type ArrayType struct {
	ElementType FieldType
	MinItems    *int
	MaxItems    *int
}

func (ArrayType) isFieldType() {}

type MapType struct {
	KeyType   FieldType
	ValueType FieldType
}

func (MapType) isFieldType() {}

type StructType struct {
	Name       string
	IsPointer  bool
	Package    string // Package name, e.g. "v1" if the package is "k8s.io/apimachinery/pkg/apis/meta/v1", or an alias, or empty if it is a local struct
	ImportPath string // Full import path, e.g. "k8s.io/api/core/v1"
}

func (s StructType) FullyQualifiedName() string {
	return fullyQualifiedName(s.Package, s.Name)
}

func (StructType) isFieldType() {}

type MetadataType struct {
	Name       string
	Package    string // Package name, e.g. "v1" if the package is "k8s.io/apimachinery/pkg/apis/meta/v1", or an alias
	ImportPath string // Full import path, e.g. "k8s.io/apimachinery/pkg/apis/meta/v1"
}

func (m MetadataType) FullyQualifiedName() string {
	return fullyQualifiedName(m.Package, m.Name)
}

func (MetadataType) isFieldType() {}

func fullyQualifiedName(packageName, structName string) string {
	return fmt.Sprintf("%s.%s", packageName, structName)
}

type Field struct {
	Name         string
	Type         FieldType
	JSONName     string
	Comments     string
	IsOptional   bool
	IsInline     bool
	EnumValues   []string
	XValidations []XValidation
}

func NewParser() *Parser {
	return &Parser{
		fset: token.NewFileSet(),
	}
}

func (p *Parser) ParseDirectory(path string) ([]Struct, error) {
	var structs []Struct

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(filePath, ".go") {
			return nil
		}

		if strings.HasSuffix(filePath, "_test.go") || strings.HasSuffix(filePath, internal.DeepCopyFileName) {
			return nil
		}

		fileStructs, parseErr := p.parseFile(filePath)
		if parseErr != nil {
			return fmt.Errorf("failed to parse file %s: %w", filePath, parseErr)
		}

		structs = append(structs, fileStructs...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", path, err)
	}

	return structs, nil
}

func (p *Parser) parseFile(filePath string) ([]Struct, error) {
	file, err := parser.ParseFile(p.fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	imports := p.extractImports(file)

	var structs []Struct

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			parsedStruct, err := p.parseStruct(file.Name.Name, typeSpec, structType, genDecl.Doc, imports)
			if err != nil {
				return nil, fmt.Errorf("failed to parse struct %s: %w", typeSpec.Name.Name, err)
			}
			structs = append(structs, parsedStruct)
		}
	}

	return structs, nil
}

// extractImports builds a map of package alias -> full import path from the AST file
// This is necessary since the imports are only available on the file level, not on the struct/type/field level.
func (p *Parser) extractImports(file *ast.File) map[string]string {
	imports := make(map[string]string)
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		var alias string
		if imp.Name != nil {
			// Explicit alias (import foo "bar")
			alias = imp.Name.Name
		} else {
			// Default alias is the last part of the import path
			parts := strings.Split(importPath, "/")
			alias = parts[len(parts)-1]
		}

		imports[alias] = importPath
	}
	return imports
}

func (p *Parser) parseStruct(packageName string, typeSpec *ast.TypeSpec, structType *ast.StructType, doc *ast.CommentGroup, imports map[string]string) (Struct, error) {
	ps := Struct{
		Name:                  typeSpec.Name.Name,
		Package:               packageName,
		Fields:                []Field{},
		PrinterColumns:        []PrinterColumn{},
		KubernetesLabels:      make(map[string]string),
		KubernetesAnnotations: make(map[string]string),
		commentAnnotations:    make(map[string]string),
	}

	if doc != nil {
		text := doc.Text()
		ps.Comments = p.parseComments(text)
		ps.commentAnnotations = p.parseCommentAnnotations(text)
		printerColumns, err := p.parsePrinterColumns(text)
		if err != nil {
			return Struct{}, err
		}
		ps.PrinterColumns = printerColumns
		ps.KubernetesLabels = p.parseKeyValues(text, annotationLabel)
		ps.KubernetesAnnotations = p.parseKeyValues(text, annotationAnnotation)
		validations, err := p.parseXValidations(text)
		if err != nil {
			return Struct{}, err
		}
		ps.XValidations = validations
		if _, ok := ps.commentAnnotations[annotationObjectRoot]; ok {
			ps.IsRoot = true
		}
		ps.Group = ps.commentAnnotations[annotationGroupName]
		ps.Version = ps.commentAnnotations[annotationVersion]
		ps.Plural = ps.commentAnnotations[annotationPlural]
	}

	if structType.Fields != nil {
		for _, field := range structType.Fields.List {
			parsedField, err := p.parseField(field, packageName, imports)
			if err != nil {
				return Struct{}, err
			}
			ps.Fields = append(ps.Fields, parsedField)
		}
	}

	return ps, nil
}

func (p *Parser) parseField(field *ast.Field, packageName string, imports map[string]string) (Field, error) {
	pf := Field{}

	if len(field.Names) > 0 {
		pf.Name = field.Names[0].Name
	}

	annotations := make(map[string]string)
	if field.Doc != nil {
		text := field.Doc.Text()
		pf.Comments = p.parseComments(text)
		annotations = p.parseCommentAnnotations(text)
		validations, err := p.parseXValidations(text)
		if err != nil {
			if pf.Name != "" {
				return Field{}, fmt.Errorf("field %s: %w", pf.Name, err)
			}
			return Field{}, err
		}
		pf.XValidations = validations
	}

	pf.Type = p.parseFieldType(field.Type, annotations, packageName, imports)

	if field.Tag != nil {
		tag := strings.Trim(field.Tag.Value, "`")
		pf.JSONName = p.extractJSONName(tag)
		pf.IsInline = strings.Contains(tag, ",inline")
	}

	if _, ok := annotations[annotationValidationOptional]; ok {
		pf.IsOptional = true
	}

	if enum, ok := annotations[annotationValidationEnum]; ok {
		values := strings.Split(enum, ",")
		for i, v := range values {
			values[i] = strings.TrimSpace(v)
		}
		pf.EnumValues = values
	}

	return pf, nil
}

func (p *Parser) parseFieldType(expr ast.Expr, annotations map[string]string, packageName string, imports map[string]string) FieldType {
	switch t := expr.(type) {
	case *ast.Ident: // standard Go type, e.g. string, int, bool, etc. or a struct local to the same package
		return p.parseIdentType(t, annotations, packageName)
	case *ast.ArrayType:
		return p.parseArrayType(t, annotations, packageName, imports)
	case *ast.MapType:
		return p.parseMapType(t, packageName, imports)
	case *ast.StarExpr: // pointer to standard Go type, or any struct (local or external)
		return p.parseStarExprType(t, annotations, packageName, imports)
	case *ast.SelectorExpr: // type from other package, e.g. metav1.ListMeta
		return p.parseSelectorExprType(t, imports)
	default:
		return PrimitiveType{Name: "unknown"}
	}
}

func (p *Parser) parseIdentType(t *ast.Ident, annotations map[string]string, packageName string) FieldType {
	if t.Name == stringTypeName {
		stringType := StringType{IsPointer: false}
		if minLength, ok := annotations[annotationValidationMinLength]; ok {
			if val, err := strconv.Atoi(minLength); err == nil {
				stringType.MinLength = &val
			}
		}
		if maxLength, ok := annotations[annotationValidationMaxLength]; ok {
			if val, err := strconv.Atoi(maxLength); err == nil {
				stringType.MaxLength = &val
			}
		}
		if pattern, ok := annotations[annotationValidationPattern]; ok {
			stringType.Pattern = pattern
		}
		if defaultVal, ok := annotations[annotationValidationDefault]; ok {
			stringType.Default = defaultVal
		}
		return stringType
	}
	if p.isNumericGoType(t.Name) {
		numericType := NumericType{Name: t.Name, IsPointer: false}
		if minimum, ok := annotations[annotationValidationMinimum]; ok {
			if val, err := strconv.ParseFloat(minimum, 64); err == nil {
				numericType.Minimum = &val
			}
		}
		if maximum, ok := annotations[annotationValidationMaximum]; ok {
			if val, err := strconv.ParseFloat(maximum, 64); err == nil {
				numericType.Maximum = &val
			}
		}
		if defaultVal, ok := annotations[annotationValidationDefault]; ok {
			numericType.Default = p.parseNumericDefault(defaultVal, t.Name)
		}
		return numericType
	}
	if p.isPrimitiveGoType(t.Name) {
		primitiveType := PrimitiveType{Name: t.Name, IsPointer: false}
		if defaultVal, ok := annotations[annotationValidationDefault]; ok {
			primitiveType.Default = p.parsePrimitiveDefault(defaultVal, t.Name)
		}
		return primitiveType
	}
	// Local struct in the same package - no import path needed
	return StructType{Name: t.Name, Package: packageName, ImportPath: ""}
}

func (p *Parser) parseArrayType(t *ast.ArrayType, annotations map[string]string, packageName string, imports map[string]string) FieldType {
	arrayType := ArrayType{
		ElementType: p.parseFieldType(t.Elt, nil, packageName, imports),
	}
	if minItems, ok := annotations[annotationValidationMinItems]; ok {
		if val, err := strconv.Atoi(minItems); err == nil {
			arrayType.MinItems = &val
		}
	}
	if maxItems, ok := annotations[annotationValidationMaxItems]; ok {
		if val, err := strconv.Atoi(maxItems); err == nil {
			arrayType.MaxItems = &val
		}
	}
	return arrayType
}

func (p *Parser) parseMapType(t *ast.MapType, packageName string, imports map[string]string) FieldType {
	return MapType{
		KeyType:   p.parseFieldType(t.Key, nil, packageName, imports),
		ValueType: p.parseFieldType(t.Value, nil, packageName, imports),
	}
}

func (p *Parser) parseStarExprType(t *ast.StarExpr, annotations map[string]string, packageName string, imports map[string]string) FieldType {
	switch innerType := p.parseFieldType(t.X, annotations, packageName, imports).(type) {
	case StringType:
		innerType.IsPointer = true
		return innerType
	case NumericType:
		innerType.IsPointer = true
		return innerType
	case PrimitiveType:
		innerType.IsPointer = true
		return innerType
	case StructType:
		innerType.IsPointer = true
		return innerType
	case TimeType:
		innerType.IsPointer = true
		return innerType
	default:
		return innerType
	}
}

func (p *Parser) parseSelectorExprType(t *ast.SelectorExpr, imports map[string]string) FieldType {
	pkgAlias, structName := p.getStructNames(t)
	importPath := imports[pkgAlias]
	if p.isTimeType(pkgAlias, structName) {
		return TimeType{Package: pkgAlias, ImportPath: importPath}
	}
	if metadata.IsMetadataField(structName) {
		return MetadataType{Name: structName, Package: pkgAlias, ImportPath: importPath}
	}
	// Need to set import path for external struct types
	return StructType{Name: structName, Package: pkgAlias, ImportPath: importPath}
}

func (p *Parser) isNumericGoType(typeName string) bool {
	numerics := map[string]bool{
		"int":     true,
		"int8":    true,
		"int16":   true,
		"int32":   true,
		"int64":   true,
		"uint":    true,
		"uint8":   true,
		"uint16":  true,
		"uint32":  true,
		"uint64":  true,
		"float32": true,
		"float64": true,
	}
	return numerics[typeName]
}

func (p *Parser) isPrimitiveGoType(typeName string) bool {
	// Strings are handled by StringType, numeric types by NumericType
	primitives := map[string]bool{
		"bool": true,
		"byte": true,
		"rune": true,
	}
	return primitives[typeName]
}

func (p *Parser) parseNumericDefault(defaultStr string, numericType string) interface{} {
	switch numericType {
	case "int", "int32", "int16", "int8", "uint", "uint32", "uint16", "uint8", "byte":
		if val, err := strconv.Atoi(defaultStr); err == nil {
			return val
		}
	case "int64", "uint64":
		if val, err := strconv.ParseInt(defaultStr, 10, 64); err == nil {
			return val
		}
	case "float32", "float64":
		if val, err := strconv.ParseFloat(defaultStr, 64); err == nil {
			return val
		}
	}
	// If parsing fails, return the string as-is
	return defaultStr
}

func (p *Parser) parsePrimitiveDefault(defaultStr string, primitiveType string) interface{} {
	switch primitiveType {
	case "bool":
		if val, err := strconv.ParseBool(defaultStr); err == nil {
			return val
		}
	case "byte", "rune":
		if val, err := strconv.Atoi(defaultStr); err == nil {
			return val
		}
	}
	// If parsing fails, return the string as-is
	return defaultStr
}

func (p *Parser) isTimeType(pkgAlias, typeName string) bool {
	return (pkgAlias == "time" && typeName == "Time") ||
		(typeName == "Time" && pkgAlias != "")
}

func (p *Parser) extractJSONName(tag string) string {
	parts := strings.SplitSeq(tag, " ")
	for part := range parts {
		if jsonValue, ok := strings.CutPrefix(part, "json:"); ok {
			jsonValue = strings.Trim(jsonValue, `"`)
			if commaIdx := strings.Index(jsonValue, ","); commaIdx != -1 {
				return jsonValue[:commaIdx]
			}
			return jsonValue
		}
	}
	return ""
}

func (p *Parser) parseCommentAnnotations(text string) map[string]string {
	annotations := make(map[string]string)
	_ = p.parseDoc(text, func(line string) error {
		if annotation, ok := strings.CutPrefix(line, annotationPrefix); ok {
			parts := strings.SplitN(annotation, "=", 2)
			if len(parts) == 2 {
				annotations[parts[0]] = parts[1]
			} else {
				annotations[annotation] = "true"
			}
		}
		return nil
	})
	return annotations
}

func (p *Parser) parsePrinterColumns(text string) ([]PrinterColumn, error) {
	var columns []PrinterColumn
	err := p.parseDoc(text, func(line string) error {
		annotation, ok := strings.CutPrefix(line, annotationPrefix+annotationPrinterColumns+":")
		if !ok {
			return nil
		}
		column, err := p.parsePrinterColumn(annotation)
		if err != nil {
			return err
		}
		columns = append(columns, column)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return columns, nil
}

func (p *Parser) parseKeyValues(text, annotationType string) map[string]string {
	keyValues := make(map[string]string)
	_ = p.parseDoc(text, func(line string) error {
		if annotation, ok := strings.CutPrefix(line, annotationPrefix+annotationType+":"); ok {
			parts := strings.SplitN(annotation, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Strip surrounding double quotes if present
				value = strings.Trim(value, `"`)
				keyValues[key] = value
			}
		}
		return nil
	})
	return keyValues
}

func (p *Parser) parsePrinterColumn(annotation string) (PrinterColumn, error) {
	column := PrinterColumn{}

	parts := strings.SplitSeq(annotation, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) != 2 {
			continue
		}

		key := strings.TrimSpace(keyValue[0])
		key = strings.ToLower(strings.Trim(key, `"`))
		value := strings.TrimSpace(keyValue[1])
		value = strings.Trim(value, `"`)

		switch key {
		case "name":
			column.Name = value
		case "type":
			column.Type = value
		case "jsonpath":
			column.JSONPath = value
		case "description":
			column.Description = value
		case "priority":
			if priority, err := strconv.ParseInt(value, 10, 32); err == nil {
				column.Priority = int32(priority)
			}
		default:
			return PrinterColumn{}, fmt.Errorf("unknown printcolumn key %q in annotation %q", key, annotation)
		}
	}

	return column, nil
}

func (p *Parser) parseXValidations(text string) ([]XValidation, error) {
	var validations []XValidation
	err := p.parseDoc(text, func(line string) error {
		annotation, ok := strings.CutPrefix(line, annotationPrefix+annotationValidationXValidation)
		if !ok {
			return nil
		}
		validation, err := p.parseXValidation(annotation)
		if err != nil {
			return err
		}
		validations = append(validations, validation)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return validations, nil
}

func (p *Parser) parseXValidation(annotation string) (XValidation, error) {
	xv := XValidation{}
	for _, kv := range splitXValidationParams(annotation) {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		key = strings.Trim(key, `"`)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"`)

		switch strings.ToLower(key) {
		case "rule":
			xv.Rule = value
		case "message":
			xv.Message = value
		case "messageexpression":
			xv.MessageExpression = value
		case "reason":
			xv.Reason = value
		case "fieldpath":
			xv.FieldPath = value
		case "optionaloldself":
			b, err := strconv.ParseBool(value)
			if err == nil {
				xv.OptionalOldSelf = &b
			}
		default:
			return XValidation{}, fmt.Errorf("unknown XValidation key %q in annotation %q", key, annotation)
		}
	}
	return xv, nil
}

// splitXValidationParams splits a comma-separated parameter string while
// respecting quoted values that may contain commas.
func splitXValidationParams(s string) []string {
	var params []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '"':
			inQuotes = !inQuotes
			current.WriteByte(ch)
		case ch == ',' && !inQuotes:
			params = append(params, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		params = append(params, current.String())
	}
	return params
}

func (p *Parser) parseComments(doc string) string {
	var comments []string
	_ = p.parseDoc(doc, func(line string) error {
		if strings.HasPrefix(line, annotationPrefix) || line == "" {
			return nil
		}

		comments = append(comments, line)
		return nil
	})
	return strings.Join(comments, " ")
}

func (p *Parser) parseDoc(doc string, fn func(line string) error) error {
	lines := strings.Split(doc, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if trimmed, ok := strings.CutPrefix(line, "//"); ok {
			line = strings.TrimSpace(trimmed)
		}
		if err := fn(line); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) getStructNames(t *ast.SelectorExpr) (packageName, structName string) {
	ident, ok := t.X.(*ast.Ident)
	if !ok {
		return "unknown", "unknown"
	}
	return ident.Name, t.Sel.Name
}
