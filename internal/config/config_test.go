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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				InputPath:  "/path/to/input",
				OutputPath: "/path/to/output",
			},
			wantErr: false,
		},
		{
			name: "missing input path",
			config: Config{
				InputPath:  "",
				OutputPath: "/path/to/output",
			},
			wantErr: true,
			errMsg:  "input-path is required",
		},
		{
			name: "missing output path",
			config: Config{
				InputPath:  "/path/to/input",
				OutputPath: "",
			},
			wantErr: true,
			errMsg:  "output-path is required",
		},
		{
			name: "missing both paths",
			config: Config{
				InputPath:  "",
				OutputPath: "",
			},
			wantErr: true,
			errMsg:  "input-path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.errMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewGoToXRDCommand(t *testing.T) {
	cmd, err := NewGoToXRDCommand()
	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.Equal(t, "go:xrd", cmd.Use)

	inputFlag := cmd.Flags().Lookup("input-path")
	require.NotNil(t, inputFlag)

	outputFlag := cmd.Flags().Lookup("output-path")
	require.NotNil(t, outputFlag)
}
