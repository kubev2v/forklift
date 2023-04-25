package openstack

import (
	"net/url"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
)

func OpenstackVolumePopulator(image *openstack.Image, sourceUrl *url.URL, transferNetwork *core.ObjectReference, targetNamespace, secretName, migrationName string) *api.OpenstackVolumePopulator {
	return &api.OpenstackVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			Name:      image.Name,
			Namespace: targetNamespace,
		},
		Spec: api.OpenstackVolumePopulatorSpec{
			IdentityURL:     sourceUrl.String(),
			SecretName:      secretName,
			ImageID:         image.ID,
			TransferNetwork: transferNetwork,
		},
	}
}
