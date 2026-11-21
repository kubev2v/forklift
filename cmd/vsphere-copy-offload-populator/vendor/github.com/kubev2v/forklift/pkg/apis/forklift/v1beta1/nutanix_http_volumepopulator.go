package v1beta1

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var NutanixHTTPVolumePopulatorKind = "NutanixHTTPVolumePopulator"
var NutanixHTTPVolumePopulatorResource = "nutanixhttpvolumepopulators"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +kubebuilder:resource:shortName={nhvp,nhvps}
//
// NutanixHTTPVolumePopulator copies a Nutanix catalog image into a target PVC
// by downloading it over HTTP from the Prism Image Service.
type NutanixHTTPVolumePopulator struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec NutanixHTTPVolumePopulatorSpec `json:"spec"`
	// +optional
	Status NutanixHTTPVolumePopulatorStatus `json:"status"`
}

type NutanixHTTPVolumePopulatorSpec struct {
	// URL is the HTTP(S) download endpoint for the catalog image
	// (Prism Element v3 /file, or a resolved Prism Central entity_download URL).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`
	// SecretName references credentials and TLS material for the download.
	// Prism Element uses Basic Auth (user/password); Prism Central uses a
	// Cookie header. ca.crt / insecureSkipVerify are honored when present.
	SecretName string `json:"secretName"`
	// DiskSizeBytes is the provisioned (virtual) size of the source disk.
	DiskSizeBytes int64 `json:"diskSizeBytes"`
}

type NutanixHTTPVolumePopulatorStatus struct {
	// +optional
	Progress string `json:"progress,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NutanixHTTPVolumePopulatorList struct {
	meta.TypeMeta `json:",inline"`
	meta.ListMeta `json:"metadata,omitempty"`
	Items         []NutanixHTTPVolumePopulator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NutanixHTTPVolumePopulator{}, &NutanixHTTPVolumePopulatorList{})
}
