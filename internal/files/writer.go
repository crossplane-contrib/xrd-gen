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

package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crossplane-contrib/xrd-gen/internal"
	"github.com/crossplane-contrib/xrd-gen/internal/domain"
)

type FileWriter struct {
	outputPath string
}

func NewFileWriter(outputPath string) *FileWriter {
	return &FileWriter{
		outputPath: outputPath,
	}
}

func (w *FileWriter) WriteXRDYAML(xrd *domain.CompositeResourceDefinition, yamlData []byte) error {
	if err := os.MkdirAll(w.outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("%s.yaml", strings.ToLower(xrd.Kind))
	filePath := filepath.Join(w.outputPath, filename)

	if err := os.WriteFile(filePath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write XRD YAML file: %w", err)
	}

	return nil
}

func (w *FileWriter) WriteDeepCopyGo(inputPath string, goCode []byte) error {
	filePath := filepath.Join(inputPath, internal.DeepCopyFileName)

	if err := os.WriteFile(filePath, goCode, 0644); err != nil {
		return fmt.Errorf("failed to write DeepCopy Go file: %w", err)
	}

	return nil
}

func (w *FileWriter) WriteDeepCopyTestGo(inputPath string, goCode []byte) error {
	filePath := filepath.Join(inputPath, internal.DeepCopyTestFileName)

	if err := os.WriteFile(filePath, goCode, 0644); err != nil {
		return fmt.Errorf("failed to write DeepCopy test Go file: %w", err)
	}

	return nil
}
