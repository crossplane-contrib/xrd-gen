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

	"github.com/crossplane-contrib/xrd-gen/internal/parsing"
	"github.com/crossplane-contrib/xrd-gen/pkg/version"
)

//go:embed templates/deepcopy_test.tmpl
var deepcopyTestTemplate string

type testTemplateData struct {
	Header      string
	Version     string
	PackageName string
	Imports     []importInfo
	Structs     []testStructInfo
}

type testStructInfo struct {
	Name        string
	HasTypeMeta bool
}

// TODO: more comprehensive tests need to be generated for all fields that are pointers and need to be deep copied
func GenerateDeepCopyTests(structs []parsing.Struct, headerFile string) ([]byte, error) {
	if len(structs) == 0 {
		return []byte{}, nil
	}

	header, packageName, targetStructs, err := prepareStructsForGeneration(structs, headerFile)
	if err != nil {
		return nil, err
	}

	testStructInfos := make([]testStructInfo, 0, len(targetStructs))

	for _, s := range targetStructs {
		testStructInfos = append(testStructInfos, testStructInfo{
			Name:        s.Name,
			HasTypeMeta: hasInlineTypeMeta(s),
		})
	}

	data := testTemplateData{
		Header:      header,
		Version:     version.Version,
		PackageName: packageName,
		Structs:     testStructInfos,
	}

	return executeTemplate("deepcopy_test", deepcopyTestTemplate, data)
}
