/*
Copyright 2019 Red Hat Inc.

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

// Package v1alpha1 contains API Schema definitions for the migration v1alpha1 API group.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/konveyor/forklift-controller/pkg/apis/migration
// +k8s:defaulter-gen=TypeMeta
// +groupName=forklift.konveyor.io
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

var SchemeGroupVersion = schema.GroupVersion{
	Group:   "forklift.konveyor.io",
	Version: "v1alpha1",
}

var SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
