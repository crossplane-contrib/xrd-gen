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

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/crossplane-contrib/xrd-gen/internal/config"
	"github.com/crossplane-contrib/xrd-gen/pkg/version"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "xrd-gen",
		Short:   "Generate Crossplane XRDs from Go structs",
		Long:    "xrd-gen is a CLI tool that generates Crossplane XRDs (v2) as YAML from Go structs",
		Version: version.Version,
	}

	goToXRDCmd, err := config.NewGoToXRDCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating go:xrd command: %v\n", err)
		os.Exit(1)
	}
	rootCmd.AddCommand(goToXRDCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
