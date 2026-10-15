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

package integrationtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXrdGenBinaryName(t *testing.T) {
	t.Parallel()
	require.Equal(t, "xrd-gen", xrdGenBinaryName("darwin"))
	require.Equal(t, "xrd-gen", xrdGenBinaryName("linux"))
	require.Equal(t, "xrd-gen.exe", xrdGenBinaryName("windows"))
}

func TestParseXrdGenVersion(t *testing.T) {
	t.Parallel()

	v, err := parseXrdGenVersion("xrd-gen version v0.0.0-1789767173-7ad138c0\n")
	require.NoError(t, err)
	require.Equal(t, "v0.0.0-1789767173-7ad138c0", v)

	v, err = parseXrdGenVersion("xrd-gen version v0.0.0-dev\n")
	require.NoError(t, err)
	require.Equal(t, "v0.0.0-dev", v)
}

func TestBasicGeneration(t *testing.T) {
	tempDir := t.TempDir()

	fixtureDeepCopy := filepath.Join("testdata", "dummyproject", "zz_generated.deepcopy.go")
	beforeFixture, err := os.ReadFile(fixtureDeepCopy)
	require.NoError(t, err)

	inputPath := dummyProjectCopy(t)
	outputPath := filepath.Join(tempDir, "output")

	binaryPath := xrdGenBinary(t)
	xrdGenVer := xrdGenVersion(t, binaryPath)

	headerFile, err := filepath.Abs(filepath.Join("testdata", "license-header.txt"))
	require.NoError(t, err)

	cmd := exec.Command(binaryPath, "go:xrd", "--input-path", inputPath, "--output-path", outputPath, "--header-file", headerFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
	}
	require.NoError(t, err)

	yamlPath := filepath.Join(outputPath, "xmyresource.yaml")
	require.FileExists(t, yamlPath)

	yamlContent, err := os.ReadFile(yamlPath)
	require.NoError(t, err)

	expectedYAML := `apiVersion: apiextensions.crossplane.io/v2
kind: CompositeResourceDefinition
metadata:
  annotations:
    xrd-gen.crossplane.io/version: ` + xrdGenVer + `
  name: xmyresources.example.com
spec:
  group: example.com
  names:
    kind: XMyResource
    plural: xmyresources
  versions:
    - additionalPrinterColumns:
        - description: Resource ID
          jsonPath: .status.resourceId
          name: resourceId
          type: string
        - jsonPath: .status.lastUpdated
          name: lastUpdated
          type: date
      name: v1alpha1
      referenceable: true
      schema:
        openAPIV3Schema:
          properties:
            spec:
              description: XMyResourceSpec represents a sample resource with various field types
              properties:
                config:
                  description: Config is a nested object
                  properties:
                    maxConnections:
                      description: MaxConnections limits concurrent connections
                      format: int32
                      type: integer
                    retryPolicy:
                      description: RetryPolicy specifies retry behavior
                      type: string
                    timeout:
                      description: Timeout in seconds
                      format: int32
                      type: integer
                  required:
                    - retryPolicy
                    - timeout
                  type: object
                count:
                  description: Count represents an integer value
                  format: int32
                  type: integer
                description:
                  description: Description is optional
                  type: string
                enabled:
                  description: Enabled is a boolean flag
                  type: boolean
                labels:
                  additionalProperties:
                    type: string
                  description: Labels is a map
                  type: object
                name:
                  description: Name of the resource
                  type: string
                  x-kubernetes-validations:
                    - rule: self == oldSelf
                      message: name is immutable
                node:
                  description: Node is an object from another library
                  type: object
                other:
                  description: Other is an object from another package
                  properties:
                    otherField:
                      description: Other field is an enum value, with the annotation provided in a different package.
                      enum:
                        - value1
                        - value2
                        - value3
                      type: string
                  required:
                    - otherField
                  type: object
                price:
                  description: Price is a floating point value and must be positive
                  format: double
                  type: number
                  minimum: 0
                replicas:
                  description: Replicas is an array of nested objects
                  items:
                    description: ReplicaSpec defines replica configuration
                    properties:
                      name:
                        description: Name of the replica
                        type: string
                      region:
                        description: Region where the replica is deployed
                        type: string
                      storageGB:
                        description: Storage size in GB
                        format: int32
                        type: integer
                    required:
                      - name
                      - region
                      - storageGB
                    type: object
                    x-kubernetes-validations:
                      - rule: self.storageGB >= 10
                        message: storageGB must be at least 10
                  minItems: 1
                  type: array
                resourceId:
                  description: ResourceID must be a valid DNS label
                  pattern: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
                  type: string
                score:
                  description: Score must be between 0 and 100
                  format: int32
                  type: integer
                  minimum: 0
                  maximum: 100
                tags:
                  description: Tags is an array of strings
                  items:
                    type: string
                  maxItems: 100
                  minItems: 1
                  type: array
              required:
                - config
                - count
                - enabled
                - name
                - node
                - other
                - replicas
                - resourceId
                - score
                - tags
              type: object
            status:
              properties:
                resourceId:
                  description: ResourceID must be a valid DNS label
                  pattern: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
                  type: string
                lastUpdated:
                  description: LastUpdated is the time this status was last updated
                  type: string
                  format: date-time
              required:
                - lastUpdated
                - resourceId
              type: object
          required:
            - spec
            - status
          type: object
      served: true
`

	require.YAMLEq(t, expectedYAML, string(yamlContent))
	require.Contains(t, string(yamlContent), "# Code generated by xrd-gen "+xrdGenVer+". DO NOT EDIT.")

	deepCopyPath := filepath.Join(inputPath, "zz_generated.deepcopy.go")
	require.FileExists(t, deepCopyPath)

	deepCopyContent, err := os.ReadFile(deepCopyPath)
	require.NoError(t, err)

	deepCopyStr := string(deepCopyContent)
	require.Contains(t, deepCopyStr, "// Copyright 2025 The Crossplane Authors.", "Generated code should include the header from header file")
	require.Contains(t, deepCopyStr, "// Code generated by xrd-gen "+xrdGenVer)
	require.Contains(t, deepCopyStr, "DO NOT EDIT.")
	require.Contains(t, deepCopyStr, "package dummyproject")
	require.Contains(t, deepCopyStr, "func (in *XMyResource) DeepCopyInto(out *XMyResource)")
	require.Contains(t, deepCopyStr, "func (in *XMyResource) DeepCopy() *XMyResource")
	require.Contains(t, deepCopyStr, "func (in *XMyResource) DeepCopyObject() runtime.Object")
	require.Contains(t, deepCopyStr, "func (in *Configuration) DeepCopyInto(out *Configuration)")
	require.Contains(t, deepCopyStr, "func (in *ReplicaSpec) DeepCopyInto(out *ReplicaSpec)")

	// Verify header appears before the build tag
	headerPos := strings.Index(deepCopyStr, "// Copyright 2025 The Crossplane Authors.")
	buildTagPos := strings.Index(deepCopyStr, "//go:build")
	require.Less(t, headerPos, buildTagPos, "Header should appear before build tag")

	// Verify generated test file exists
	deepCopyTestPath := filepath.Join(inputPath, "zz_generated.deepcopy_test.go")
	require.FileExists(t, deepCopyTestPath, "Generated deepcopy test file should exist")

	deepCopyTestContent, err := os.ReadFile(deepCopyTestPath)
	require.NoError(t, err)

	deepCopyTestStr := string(deepCopyTestContent)
	require.Contains(t, deepCopyTestStr, "// Copyright 2025 The Crossplane Authors.", "Generated test code should include the header from header file")
	require.Contains(t, deepCopyTestStr, "// Code generated by xrd-gen "+xrdGenVer)
	require.Contains(t, deepCopyTestStr, "DO NOT EDIT.")
	require.Contains(t, deepCopyTestStr, "package dummyproject")
	require.Contains(t, deepCopyTestStr, "func TestDeepCopy_XMyResource(t *testing.T)")
	require.Contains(t, deepCopyTestStr, "func TestDeepCopy_Configuration(t *testing.T)")
	require.Contains(t, deepCopyTestStr, "func TestDeepCopy_ReplicaSpec(t *testing.T)")
	require.Contains(t, deepCopyTestStr, "copy := original.DeepCopy()")
	require.Contains(t, deepCopyTestStr, "if copy == nil")
	require.Contains(t, deepCopyTestStr, "if original == copy")

	t.Log("Verifying newly generated deepcopy code compiles...")
	compileCmd := exec.Command("go", "build", "-o", os.DevNull, ".")
	compileCmd.Dir = inputPath
	compileOutput, err := compileCmd.CombinedOutput()
	if err != nil {
		t.Logf("Compilation output: %s", string(compileOutput))
	}
	require.NoError(t, err, "Generated deepcopy code should compile successfully")

	t.Log("Verifying generated deepcopy tests compile and pass...")
	testCmd := exec.Command("go", "test", "-v", "-run", "TestDeepCopy_", ".")
	testCmd.Dir = inputPath
	testOutput, err := testCmd.CombinedOutput()
	t.Logf("Test output: %s", string(testOutput))
	require.NoError(t, err, "Generated deepcopy tests should compile and pass")

	afterFixture, err := os.ReadFile(fixtureDeepCopy)
	require.NoError(t, err)
	require.Equal(t, beforeFixture, afterFixture)
}

func TestGenerationWithOnlyEmbeddedMetadata(t *testing.T) {
	requireGeneratedCodeBuilds(t, "embeddedmetaproject", "xwidget.yaml")
}

func TestGenerationWithExternalStructFields(t *testing.T) {
	requireGeneratedCodeBuilds(t, "externalstructproject", "xgateway.yaml")
}

func TestGenerationWithPackageImportedUnderDifferentAliases(t *testing.T) {
	requireGeneratedCodeBuilds(t, "multialiasproject", "xwidget.yaml")
}

func requireGeneratedCodeBuilds(t *testing.T, project, xrdFile string) {
	t.Helper()
	inputPath := testProjectCopy(t, project)
	outputPath := filepath.Join(t.TempDir(), "output")

	cmd := exec.Command(xrdGenBinary(t), "go:xrd", "--input-path", inputPath, "--output-path", outputPath)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	require.FileExists(t, filepath.Join(outputPath, xrdFile))

	vetCmd := exec.Command("go", "vet", ".")
	vetCmd.Dir = inputPath
	vetOutput, err := vetCmd.CombinedOutput()
	require.NoError(t, err, "Generated code should compile and pass vet: %s", string(vetOutput))

	testCmd := exec.Command("go", "test", "-run", "TestDeepCopy", ".")
	testCmd.Dir = inputPath
	testOutput, err := testCmd.CombinedOutput()
	require.NoError(t, err, "Generated deepcopy tests should pass: %s", string(testOutput))
}

func xrdGenBinaryName(goos string) string {
	if goos == "windows" {
		return "xrd-gen.exe"
	}
	return "xrd-gen"
}

func xrdGenBinary(t *testing.T) string {
	t.Helper()
	name := xrdGenBinaryName(runtime.GOOS)
	candidates := []string{
		filepath.Join("..", "bin", name),
		filepath.Join("..", "_output", "bin", runtime.GOOS+"_"+runtime.GOARCH, name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	require.FailNow(t, "binary not found, run 'earthly +build' first")
	return ""
}

func dummyProjectCopy(t *testing.T) string {
	t.Helper()
	return testProjectCopy(t, "dummyproject")
}

func testProjectCopy(t *testing.T, project string) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", project))
	require.NoError(t, err)
	dst, err := os.MkdirTemp(filepath.Join("testdata"), "work-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dst) })
	require.NoError(t, os.CopyFS(dst, os.DirFS(src)))
	return dst
}

func xrdGenVersion(t *testing.T, binaryPath string) string {
	t.Helper()
	out, err := exec.Command(binaryPath, "--version").Output()
	require.NoError(t, err, string(out))
	v, err := parseXrdGenVersion(string(out))
	require.NoError(t, err)
	return v
}

func parseXrdGenVersion(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) < 3 {
		return "", fmt.Errorf("unexpected --version output %q", output)
	}
	return fields[len(fields)-1], nil
}
