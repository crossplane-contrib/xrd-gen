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

package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/crossplane-contrib/xrd-gen/internal/runner"
)

type Config struct {
	InputPath             string
	OutputPath            string
	HeaderFile            string
	GenerateDeepCopyTests bool
}

func NewGoToXRDCommand() (*cobra.Command, error) {
	config := &Config{}

	cmd := &cobra.Command{
		Use:   "go:xrd",
		Short: "Generate Crossplane XRD YAML from Go struct definitions",
		Long:  "Parses Go structs with xrd-gen annotations and generates Crossplane XRD YAML files and DeepCopy boilerplate code",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := config.Validate(); err != nil {
				return err
			}
			r := runner.NewRunner(config.InputPath, config.OutputPath, config.HeaderFile, config.GenerateDeepCopyTests)
			return r.RunGoToXRD()
		},
	}

	cmd.Flags().StringVar(&config.InputPath, "input-path", "", "Path to the Go source code directory")
	cmd.Flags().StringVar(&config.OutputPath, "output-path", "", "Path where generated YAML files will be written")
	cmd.Flags().StringVar(&config.HeaderFile, "header-file", "", "Optional path to a file containing a header comment to be added to generated Go files")
	cmd.Flags().BoolVar(&config.GenerateDeepCopyTests, "generate-deepcopy-tests", true, "Generate tests for deepcopy methods")

	if err := cmd.MarkFlagRequired("input-path"); err != nil {
		return nil, fmt.Errorf("failed to mark input-path as required: %w", err)
	}
	if err := cmd.MarkFlagRequired("output-path"); err != nil {
		return nil, fmt.Errorf("failed to mark output-path as required: %w", err)
	}

	return cmd, nil
}

func (c *Config) Validate() error {
	if c.InputPath == "" {
		return fmt.Errorf("input-path is required")
	}
	if c.OutputPath == "" {
		return fmt.Errorf("output-path is required")
	}
	return nil
}
