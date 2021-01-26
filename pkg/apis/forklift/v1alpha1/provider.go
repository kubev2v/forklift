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

package v1alpha1

import (
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

//
// Provider types.
const (
	// OpenShift
	OpenShift = "openshift"
	// vSphere
	VSphere = "vsphere"
)

//
// Secret fields.
const (
	Token = "token"
)

//
// Defines the desired state of Provider.
type ProviderSpec struct {
	// Provider type.
	Type string `json:"type"`
	// The provider URL.
	// Empty may be used for the `host` provider.
	URL string `json:"url,omitempty"`
	// References a secret containing credentials and
	// other confidential information.
	Secret core.ObjectReference `json:"secret" ref:"Secret"`
}

//
// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
	// Conditions.
	libcnd.Conditions `json:",inline"`
	// The most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Provider struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`
	Spec            ProviderSpec   `json:"spec,omitempty"`
	Status          ProviderStatus `json:"status,omitempty"`
}

//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProviderList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}

//
// Build k8s REST configuration.
func (p *Provider) RestCfg(secret *core.Secret) (cfg *rest.Config) {
	if p.IsHost() {
		cfg, _ = config.GetConfig()
		return
	}
	cfg = &rest.Config{
		Host:            p.Spec.URL,
		BearerToken:     string(secret.Data[Token]),
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	}
	cfg.Burst = 1000
	cfg.QPS = 100

	return
}

//
// Build a k8s client.
func (p *Provider) Client(secret *core.Secret) (c client.Client, err error) {
	c, err = client.New(
		p.RestCfg(secret),
		client.Options{
			Scheme: scheme.Scheme,
		})
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// The provider type.
func (p *Provider) Type() string {
	return p.Spec.Type
}

//
// This provider is the `host` cluster.
func (p *Provider) IsHost() bool {
	return p.Type() == OpenShift && p.Spec.URL == ""
}

//
// Current generation has been reconciled.
func (p *Provider) HasReconciled() bool {
	return p.Generation == p.Status.ObservedGeneration
}
