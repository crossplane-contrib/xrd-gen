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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/xrd-gen/integrationtest/testdata/dummyproject"
	otherpackage "github.com/crossplane-contrib/xrd-gen/integrationtest/testdata/dummyproject/otherPackage"
)

const (
	clusterName = "xrd-gen-test"

	defaultCrossplaneVersion = "2.4.1"
	crossplaneVersionEnvVar  = "CROSSPLANE_VERSION"
)

var immutableUpdateBackoff = wait.Backoff{
	Steps:    10,
	Duration: 200 * time.Millisecond,
	Factor:   1.5,
	Jitter:   0.1,
}

func resolveCrossplaneVersion() string {
	if v := os.Getenv(crossplaneVersionEnvVar); v != "" {
		return v
	}
	return defaultCrossplaneVersion
}

func TestResolveCrossplaneVersion(t *testing.T) {
	t.Run("falls back to the pinned default", func(t *testing.T) {
		t.Setenv(crossplaneVersionEnvVar, "")
		require.Equal(t, defaultCrossplaneVersion, resolveCrossplaneVersion())
	})
	t.Run("uses the environment override", func(t *testing.T) {
		t.Setenv(crossplaneVersionEnvVar, "2.3.5")
		require.Equal(t, "2.3.5", resolveCrossplaneVersion())
	})
}

func TestCrossplaneXRD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	createK3dCluster(t)
	defer cleanupK3dCluster(t)

	kubeconfig := getKubeconfig(t)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)

	k8sClient, err := createK8sClient(config)
	require.NoError(t, err)

	t.Log("Installing Crossplane...")
	installCrossplane(t, kubeconfig)

	t.Log("Waiting for Crossplane to be ready...")
	waitForCrossplaneReady(ctx, t, k8sClient)

	tempDir := t.TempDir()
	inputPath := dummyProjectCopy(t)
	outputPath := filepath.Join(tempDir, "output")

	binaryPath := xrdGenBinary(t)

	t.Log("Generating XRD...")
	cmd := exec.Command(binaryPath, "go:xrd", "--input-path", inputPath, "--output-path", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
	}
	require.NoError(t, err)

	xrdPath := filepath.Join(outputPath, "xmyresource.yaml")
	require.FileExists(t, xrdPath)

	t.Log("Applying XRD to cluster...")
	applyXRD(t, kubeconfig, xrdPath)

	t.Log("Waiting for CRD to be established...")
	waitForCRDToBeEstablished(ctx, t, k8sClient)

	t.Log("Verifying XRD was created successfully...")
	verifyXRD(ctx, t, k8sClient)

	t.Log("Crossplane XRD test completed successfully!")
}

func createK3dCluster(t *testing.T) {
	t.Log("Checking for existing cluster...")
	deleteCmd := exec.Command("k3d", "cluster", "delete", clusterName)
	deleteOutput, err := deleteCmd.CombinedOutput()
	if err == nil {
		t.Logf("Deleted existing cluster: %s", string(deleteOutput))
	}

	t.Log("Creating k3d cluster...")
	cmd := exec.Command("k3d", "cluster", "create", clusterName, "--wait")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("k3d cluster create output: %s", string(output))
		require.NoError(t, err, "failed to create k3d cluster")
	}
}

func cleanupK3dCluster(t *testing.T) {
	t.Log("Cleaning up k3d cluster...")
	cmd := exec.Command("k3d", "cluster", "delete", clusterName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("k3d cluster delete output: %s", string(output))
	}
}

func getKubeconfig(t *testing.T) string {
	home := homedir.HomeDir()
	require.NotEmpty(t, home)
	return filepath.Join(home, ".kube", "config")
}

func installCrossplane(t *testing.T, kubeconfig string) {
	commands := [][]string{
		{"kubectl", "--kubeconfig", kubeconfig, "create", "namespace", "crossplane-system"},
		{"helm", "repo", "add", "crossplane-stable", "https://charts.crossplane.io/stable"},
		{"helm", "repo", "update"},
		{
			"helm", "install", "crossplane",
			"crossplane-stable/crossplane",
			"--namespace", "crossplane-system",
			"--version", resolveCrossplaneVersion(),
			"--wait",
			"--timeout", "5m",
			"--kubeconfig", kubeconfig,
		},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("Command output: %s", string(output))
			require.NoError(t, err, "failed to execute: %v", cmdArgs)
		}
	}
}

func waitForCrossplaneReady(ctx context.Context, t *testing.T, k8sClient client.Client) {
	require.Eventually(t, func() bool {
		deployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "crossplane",
			Namespace: "crossplane-system",
		}, deployment)
		if err != nil {
			t.Logf("Waiting for Crossplane deployment: %v", err)
			return false
		}

		return deployment.Status.ReadyReplicas > 0
	}, 5*time.Minute, 5*time.Second, "Crossplane did not become ready in time")
}

func applyXRD(t *testing.T, kubeconfig, xrdPath string) {
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "apply", "-f", xrdPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("kubectl apply output: %s", string(output))
		require.NoError(t, err, "failed to apply XRD")
	}
}

func waitForCRDToBeEstablished(ctx context.Context, t *testing.T, k8sClient client.Client) {
	require.Eventually(t, func() bool {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name: "xmyresources.example.com",
		}, crd)
		if err != nil {
			t.Logf("Waiting for CRD to be established: %v", err)
			return false
		}

		for _, condition := range crd.Status.Conditions {
			if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
				return true
			}
		}
		return false
	}, 2*time.Minute, 2*time.Second, "CRD was not established in time")
}

func verifyXRD(ctx context.Context, t *testing.T, k8sClient client.Client) {
	xrd := &unstructured.Unstructured{}
	xrd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.crossplane.io",
		Version: "v2",
		Kind:    "CompositeResourceDefinition",
	})

	err := k8sClient.Get(ctx, types.NamespacedName{
		Name: "xmyresources.example.com",
	}, xrd)
	require.NoError(t, err, "failed to get XRD")
	require.NotNil(t, xrd)

	spec, found, err := unstructured.NestedMap(xrd.Object, "spec")
	require.NoError(t, err)
	require.True(t, found, "XRD spec not found")

	group, found, err := unstructured.NestedString(spec, "group")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "example.com", group)

	names, found, err := unstructured.NestedMap(spec, "names")
	require.NoError(t, err)
	require.True(t, found)

	kind, _, _ := unstructured.NestedString(names, "kind")
	require.Equal(t, "XMyResource", kind)
}

func TestCrossplaneXRWithInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("Creating k3d cluster...")
	createK3dCluster(t)
	defer cleanupK3dCluster(t)

	kubeconfig := getKubeconfig(t)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)

	k8sClient, err := createK8sClient(config)
	require.NoError(t, err)

	t.Log("Installing Crossplane...")
	installCrossplane(t, kubeconfig)

	t.Log("Waiting for Crossplane to be ready...")
	waitForCrossplaneReady(ctx, t, k8sClient)

	tempDir := t.TempDir()
	inputPath := dummyProjectCopy(t)
	outputPath := filepath.Join(tempDir, "output")

	binaryPath := xrdGenBinary(t)

	t.Log("Generating XRD...")
	cmd := exec.Command(binaryPath, "go:xrd", "--input-path", inputPath, "--output-path", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
	}
	require.NoError(t, err)

	xrdPath := filepath.Join(outputPath, "xmyresource.yaml")
	require.FileExists(t, xrdPath)

	t.Log("Applying XRD to cluster...")
	applyXRD(t, kubeconfig, xrdPath)

	t.Log("Waiting for CRD to be established...")
	waitForCRDToBeEstablished(ctx, t, k8sClient)

	t.Log("Creating XR instance...")
	xrInstance := createXRInstance(ctx, t, k8sClient)

	t.Log("Verifying XR instance was created...")
	verifyXRInstance(ctx, t, k8sClient, xrInstance.Name)

	t.Log("Crossplane XR instance test completed successfully!")
}

func createK8sClient(config *rest.Config) (client.Client, error) {
	// Start with the default Kubernetes scheme
	s := runtime.NewScheme()

	// Add built-in Kubernetes types (Deployments, CRDs, etc.)
	if err := scheme.AddToScheme(s); err != nil {
		return nil, err
	}

	// Add CRD types
	if err := apiextensionsv1.AddToScheme(s); err != nil {
		return nil, err
	}

	// Add the XMyResource type to the scheme
	gv := schema.GroupVersion{Group: "example.com", Version: "v1alpha1"}
	s.AddKnownTypes(gv,
		&dummyproject.XMyResource{},
		&dummyproject.XMyResourceList{},
	)
	metav1.AddToGroupVersion(s, gv)

	return client.New(config, client.Options{Scheme: s})
}

func createXRInstance(ctx context.Context, t *testing.T, k8sClient client.Client) *dummyproject.XMyResource {
	maxConnections := int32(50)

	instance := &dummyproject.XMyResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "example.com/v1alpha1",
			Kind:       "XMyResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-myresource",
			Namespace: "default",
		},
		Spec: dummyproject.XMyResourceSpec{
			Name:        "test-resource",
			ResourceID:  "test-resource-id",
			Description: "A test resource",
			Count:       3,
			Price:       99.99,
			Enabled:     true,
			Tags:        []string{"tag1", "tag2"},
			Config: dummyproject.Configuration{
				Timeout:        30,
				RetryPolicy:    "exponential",
				MaxConnections: &maxConnections,
			},
			Replicas: []dummyproject.ReplicaSpec{
				{
					Name:      "replica1",
					Region:    "us-east-1",
					StorageGB: 100,
				},
			},
			Labels: map[string]string{
				"env":  "test",
				"team": "engineering",
			},
			Other: otherpackage.OtherType{
				OtherField: "value1",
			},
			Node: yaml.Node{},
		},
	}

	err := k8sClient.Create(ctx, instance)
	require.NoError(t, err, "failed to create XR instance")

	// Fetch the created instance from the cluster to verify it was persisted
	fetched := &dummyproject.XMyResource{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, fetched)
	require.NoError(t, err, "failed to fetch created XR instance")

	return fetched
}

func verifyXRInstance(ctx context.Context, t *testing.T, k8sClient client.Client, name string) {
	instance := &dummyproject.XMyResource{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, instance)
	require.NoError(t, err, "failed to get XR instance")
	require.NotNil(t, instance)

	require.Equal(t, "test-resource", instance.Spec.Name)
	require.Equal(t, int32(3), instance.Spec.Count)
	require.True(t, instance.Spec.Enabled)
	require.Equal(t, 99.99, instance.Spec.Price)
	require.Equal(t, []string{"tag1", "tag2"}, instance.Spec.Tags)
	require.Equal(t, int32(30), instance.Spec.Config.Timeout)
	require.Equal(t, "exponential", instance.Spec.Config.RetryPolicy)
	require.NotNil(t, instance.Spec.Config.MaxConnections)
	require.Equal(t, int32(50), *instance.Spec.Config.MaxConnections)
	require.Len(t, instance.Spec.Replicas, 1)
	require.Equal(t, "replica1", instance.Spec.Replicas[0].Name)
	require.Equal(t, "us-east-1", instance.Spec.Replicas[0].Region)
	require.Equal(t, int32(100), instance.Spec.Replicas[0].StorageGB)
	require.Len(t, instance.Spec.Labels, 2)
	require.Equal(t, "test", instance.Spec.Labels["env"])
	require.Equal(t, "engineering", instance.Spec.Labels["team"])

	list := &dummyproject.XMyResourceList{}
	err = k8sClient.List(ctx, list, client.InNamespace("default"))
	require.NoError(t, err, "failed to list XR instances")
	require.Len(t, list.Items, 1)
	require.Equal(t, instance.Spec.Name, list.Items[0].Spec.Name)
}

func TestCrossplaneCELImmutability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("Creating k3d cluster...")
	createK3dCluster(t)
	defer cleanupK3dCluster(t)

	kubeconfig := getKubeconfig(t)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)

	k8sClient, err := createK8sClient(config)
	require.NoError(t, err)

	t.Log("Installing Crossplane...")
	installCrossplane(t, kubeconfig)

	t.Log("Waiting for Crossplane to be ready...")
	waitForCrossplaneReady(ctx, t, k8sClient)

	tempDir := t.TempDir()
	inputPath := dummyProjectCopy(t)
	outputPath := filepath.Join(tempDir, "output")

	binaryPath := xrdGenBinary(t)

	t.Log("Generating XRD...")
	cmd := exec.Command(binaryPath, "go:xrd", "--input-path", inputPath, "--output-path", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
	}
	require.NoError(t, err)

	xrdPath := filepath.Join(outputPath, "xmyresource.yaml")
	require.FileExists(t, xrdPath)

	t.Log("Applying XRD to cluster...")
	applyXRD(t, kubeconfig, xrdPath)

	t.Log("Waiting for CRD to be established...")
	waitForCRDToBeEstablished(ctx, t, k8sClient)

	t.Log("Creating XR instance...")
	createXRInstance(ctx, t, k8sClient)

	t.Log("Attempting to update immutable name field...")
	// Crossplane reconciles the XR while this runs, so an update can lose a
	// write race and come back as a conflict before the API server evaluates
	// the CEL rule. Retrying against a freshly fetched object gets us to the
	// rejection we are actually asserting on.
	err = retry.OnError(immutableUpdateBackoff, apierrors.IsConflict, func() error {
		latest := &dummyproject.XMyResource{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-myresource", Namespace: "default"}, latest); err != nil {
			return err
		}
		latest.Spec.Name = "modified-name"
		return k8sClient.Update(ctx, latest)
	})
	require.Error(t, err, "updating immutable name field should be rejected by CEL validation")
	require.Contains(t, err.Error(), "name is immutable")

	t.Log("CEL immutability validation test completed successfully!")
}

func TestCrossplaneCELStructLevelValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("Creating k3d cluster...")
	createK3dCluster(t)
	defer cleanupK3dCluster(t)

	kubeconfig := getKubeconfig(t)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err)

	k8sClient, err := createK8sClient(config)
	require.NoError(t, err)

	t.Log("Installing Crossplane...")
	installCrossplane(t, kubeconfig)

	t.Log("Waiting for Crossplane to be ready...")
	waitForCrossplaneReady(ctx, t, k8sClient)

	tempDir := t.TempDir()
	inputPath := dummyProjectCopy(t)
	outputPath := filepath.Join(tempDir, "output")

	binaryPath := xrdGenBinary(t)

	t.Log("Generating XRD...")
	cmd := exec.Command(binaryPath, "go:xrd", "--input-path", inputPath, "--output-path", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command output: %s", string(output))
	}
	require.NoError(t, err)

	xrdPath := filepath.Join(outputPath, "xmyresource.yaml")
	require.FileExists(t, xrdPath)

	t.Log("Applying XRD to cluster...")
	applyXRD(t, kubeconfig, xrdPath)

	t.Log("Waiting for CRD to be established...")
	waitForCRDToBeEstablished(ctx, t, k8sClient)

	t.Log("Attempting to create XR with storageGB below minimum...")
	maxConnections := int32(50)
	invalidInstance := &dummyproject.XMyResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "example.com/v1alpha1",
			Kind:       "XMyResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-invalid-storage",
			Namespace: "default",
		},
		Spec: dummyproject.XMyResourceSpec{
			Name:       "test-resource",
			ResourceID: "test-id",
			Count:      1,
			Score:      50,
			Enabled:    true,
			Tags:       []string{"tag1"},
			Config: dummyproject.Configuration{
				Timeout:        30,
				RetryPolicy:    "exponential",
				MaxConnections: &maxConnections,
			},
			Replicas: []dummyproject.ReplicaSpec{
				{
					Name:      "replica1",
					Region:    "us-east-1",
					StorageGB: 5, // below the minimum of 10 enforced by CEL rule
				},
			},
			Other: otherpackage.OtherType{
				OtherField: "value1",
			},
			Node: yaml.Node{},
		},
	}

	err = k8sClient.Create(ctx, invalidInstance)
	require.Error(t, err, "creating XR with storageGB < 10 should be rejected by struct-level CEL validation")
	require.Contains(t, err.Error(), "storageGB must be at least 10")

	t.Log("CEL struct-level validation test completed successfully!")
}
