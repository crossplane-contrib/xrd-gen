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

package multialiasproject

// This file imports k8s.io/apimachinery/pkg/apis/meta/v1 as meta, while
// status.go imports the same package as metav1.
import meta "k8s.io/apimachinery/pkg/apis/meta/v1"

// +xrd-gen:object:root=true
// +xrd-gen:groupName=example.com
// XWidget is a composite resource.
type XWidget struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec   XWidgetSpec  `json:"spec"`
	Status WidgetStatus `json:"status,omitempty"`
}

type XWidgetSpec struct {
	Size string `json:"size"`
}
