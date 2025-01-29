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

// Package v1beta1 contains API Schema definitions for the migration v1beta1 API group.
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/konveyor/forklift-controller/pkg/apis/migration
// +k8s:defaulter-gen=TypeMeta
// +groupName=forklift.konveyor.io
package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var SchemeGroupVersion = schema.GroupVersion{
	Group:   "forklift.konveyor.io",
	Version: "v1beta1",
}

var SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}

// TODO: find a better place for this, it would be nice to use it also to
// determine the plural configuration of a resource for the operator
func GetGroupResource(required runtime.Object) (groupresource schema.GroupResource, err error) {

	switch required.(type) {
	case *Provider:
		groupresource = schema.GroupResource{
			Group:    SchemeGroupVersion.Group,
			Resource: "providers",
		}
	case *Plan:
		groupresource = schema.GroupResource{
			Group:    SchemeGroupVersion.Group,
			Resource: "plans",
		}
	case *Migration:
		groupresource = schema.GroupResource{
			Group:    SchemeGroupVersion.Group,
			Resource: "migrations",
		}
	default:
		err = fmt.Errorf("resource type is not known")
	}

	return
}
