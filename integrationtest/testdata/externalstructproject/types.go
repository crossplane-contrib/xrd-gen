/*
Copyright 2026 The Crossplane Authors.

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

package externalstructproject

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane-contrib/xrd-gen/integrationtest/testdata/externalstructproject/other"
)

// +xrd-gen:object:root=true
// +xrd-gen:groupName=example.com
// XGateway is a composite resource.
type XGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec XGatewaySpec `json:"spec"`
}

type XGatewaySpec struct {
	Primary  other.Endpoint            `json:"primary"`
	Fallback *other.Endpoint           `json:"fallback,omitempty"`
	Replicas []other.Endpoint          `json:"replicas"`
	ByRegion map[string]other.Endpoint `json:"byRegion"`
}
