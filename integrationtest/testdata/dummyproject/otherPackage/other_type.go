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

package otherpackage

// Other type is some other thing
type OtherType struct {
	// +xrd-gen:validation:Enum=value1,value2,value3
	// Other field is an enum value, with the annotation provided in a different package.
	OtherField string `json:"otherField"`
}
