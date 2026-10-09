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

package dummyproject

import (
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otherpackage "github.com/crossplane-contrib/xrd-gen/integrationtest/testdata/dummyproject/otherPackage"
)

// +xrd-gen:object:root
// +xrd-gen:groupName=example.com
// +xrd-gen:version=v1alpha1
// +xrd-gen:printcolumn:name="resourceId",type="string",JSONPath=".status.resourceId",description="Resource ID"
// +xrd-gen:printcolumn:name="lastUpdated",type="date",JSONPath=".status.lastUpdated"
type XMyResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XMyResourceSpec   `json:"spec"`
	Status XMyResourceStatus `json:"status,omitempty"`
}

// XMyResourceList contains a list of XMyResource
type XMyResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XMyResource `json:"items"`
}

// XMyResourceSpec represents a sample resource with various field types
type XMyResourceSpec struct {
	// +xrd-gen:validation:XValidation:rule="self == oldSelf",message="name is immutable"
	// Name of the resource
	Name string `json:"name"`

	// +xrd-gen:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// ResourceID must be a valid DNS label
	ResourceID string `json:"resourceId"`

	// +xrd-gen:validation:Optional
	// Description is optional
	Description string `json:"description,omitempty"`

	// Count represents an integer value
	Count int32 `json:"count"`

	// +xrd-gen:validation:Minimum=0
	// +xrd-gen:validation:Maximum=100
	// Score must be between 0 and 100
	Score int32 `json:"score"`

	// +xrd-gen:validation:Optional
	// +xrd-gen:validation:Minimum=0.0
	// Price is a floating point value and must be positive
	Price float64 `json:"price,omitempty"`

	// Enabled is a boolean flag
	Enabled bool `json:"enabled"`

	// +xrd-gen:validation:MinItems=1
	// +xrd-gen:validation:MaxItems=100
	// Tags is an array of strings
	Tags []string `json:"tags"`

	// Config is a nested object
	Config Configuration `json:"config"`

	// Replicas is an array of nested objects
	// +xrd-gen:validation:MinItems=1
	Replicas []ReplicaSpec `json:"replicas"`

	// +xrd-gen:validation:Optional
	// Labels is a map
	Labels map[string]string `json:"labels,omitempty"`

	// Other is an object from another package
	Other otherpackage.OtherType `json:"other"`

	// Node is an object from another library
	Node yaml.Node `json:"node"`
}

type XMyResourceStatus struct {
	// +xrd-gen:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// ResourceID must be a valid DNS label
	ResourceID string `json:"resourceId"`

	// LastUpdated is the time this status was last updated
	LastUpdated metav1.Time `json:"lastUpdated"`

	// Shows the current state of the resource. This is skipped because Crossplane adds this automatically (it does not need to be specified by the user in the XRD).
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Configuration holds configuration settings
type Configuration struct {
	// Timeout in seconds
	Timeout int32 `json:"timeout"`

	// RetryPolicy specifies retry behavior
	RetryPolicy string `json:"retryPolicy"`

	// +xrd-gen:validation:Optional
	// MaxConnections limits concurrent connections
	MaxConnections *int32 `json:"maxConnections,omitempty"`
}

// +xrd-gen:validation:XValidation:rule="self.storageGB >= 10",message="storageGB must be at least 10"
// ReplicaSpec defines replica configuration
type ReplicaSpec struct {
	// Name of the replica
	Name string `json:"name"`

	// Region where the replica is deployed
	Region string `json:"region"`

	// Storage size in GB
	StorageGB int32 `json:"storageGB"`
}
