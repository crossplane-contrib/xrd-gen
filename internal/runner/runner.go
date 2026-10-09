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

package runner

import (
	"fmt"

	"github.com/crossplane-contrib/xrd-gen/internal/domain"
	"github.com/crossplane-contrib/xrd-gen/internal/files"
	"github.com/crossplane-contrib/xrd-gen/internal/parsing"
	"github.com/crossplane-contrib/xrd-gen/internal/serialization"
)

type Runner struct {
	inputPath             string
	outputPath            string
	headerFile            string
	generateDeepCopyTests bool
}

func NewRunner(inputPath, outputPath, headerFile string, generateDeepCopyTests bool) *Runner {
	return &Runner{
		inputPath:             inputPath,
		outputPath:            outputPath,
		headerFile:            headerFile,
		generateDeepCopyTests: generateDeepCopyTests,
	}
}

func (r *Runner) RunGoToXRD() error {
	fmt.Printf("Parsing Go structs from %s...\n", r.inputPath)

	parser := parsing.NewParser()
	structs, err := parser.ParseDirectory(r.inputPath)
	if err != nil {
		return fmt.Errorf("failed to parse Go structs: %w", err)
	}

	if len(structs) == 0 {
		return fmt.Errorf("no Go structs found in %s", r.inputPath)
	}

	fmt.Printf("Found %d struct(s)\n", len(structs))

	xrds, err := domain.GenerateXRD(structs)
	if err != nil {
		return fmt.Errorf("failed to generate XRDs: %w", err)
	}

	if len(xrds) == 0 {
		return fmt.Errorf("no root structs found (structs need +xrd-gen:object:root=true annotation)")
	}

	fmt.Printf("Generated %d XRD(s)\n", len(xrds))

	fileWriter := files.NewFileWriter(r.outputPath)
	for _, xrd := range xrds {
		yamlData, err := serialization.SerializeXRD(xrd)
		if err != nil {
			return fmt.Errorf("failed to serialize XRD %s: %w", xrd.Name, err)
		}

		if err := fileWriter.WriteXRDYAML(xrd, yamlData); err != nil {
			return fmt.Errorf("failed to write XRD YAML: %w", err)
		}

		fmt.Printf("Generated XRD: %s\n", xrd.Name)
	}

	deepCopyCode, err := serialization.GenerateDeepCopy(structs, r.headerFile)
	if err != nil {
		return fmt.Errorf("failed to generate DeepCopy code: %w", err)
	}

	if err := fileWriter.WriteDeepCopyGo(r.inputPath, deepCopyCode); err != nil {
		return fmt.Errorf("failed to write DeepCopy code: %w", err)
	}

	fmt.Printf("Generated DeepCopy code: %s/zz_generated.deepcopy.go\n", r.inputPath)

	if r.generateDeepCopyTests {
		deepCopyTestCode, err := serialization.GenerateDeepCopyTests(structs, r.headerFile)
		if err != nil {
			return fmt.Errorf("failed to generate DeepCopy test code: %w", err)
		}

		if err := fileWriter.WriteDeepCopyTestGo(r.inputPath, deepCopyTestCode); err != nil {
			return fmt.Errorf("failed to write DeepCopy test code: %w", err)
		}

		fmt.Printf("Generated DeepCopy test code: %s/zz_generated.deepcopy_test.go\n", r.inputPath)
	}

	fmt.Printf("Successfully generated XRDs and DeepCopy code\n")

	return nil
}
