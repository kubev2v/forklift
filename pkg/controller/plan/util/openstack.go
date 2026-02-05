package util

import (
	"net/url"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
)

func OpenstackVolumePopulator(image *openstack.Image, sourceUrl *url.URL, transferNetwork *core.ObjectReference, targetNamespace, secretName, planId, vmId, migrationId string) *api.OpenstackVolumePopulator {
	return &api.OpenstackVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			Name:      image.Name,
			Namespace: targetNamespace,
			Labels:    map[string]string{"plan": planId, "vmID": vmId, "migration": migrationId},
		},
		Spec: api.OpenstackVolumePopulatorSpec{
			IdentityURL:     sourceUrl.String(),
			SecretName:      secretName,
			ImageID:         image.ID,
			TransferNetwork: transferNetwork,
		},
		Status: api.OpenstackVolumePopulatorStatus{
			Progress: "0",
		},
	}
}
