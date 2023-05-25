package util

import (
	"fmt"
	core "k8s.io/api/core/v1"
	"net/url"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
)

// Build an OvirtVolumePopulator for XDiskAttachment and source URL
func OvirtVolumePopulator(da ovirt.XDiskAttachment, sourceUrl *url.URL, transferNetwork *core.ObjectReference, targetNamespace, secretName, vmId, migrationId string) *api.OvirtVolumePopulator {
	return &api.OvirtVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			Name:      da.DiskAttachment.ID,
			Namespace: targetNamespace,
			Labels:    map[string]string{"vmID": vmId, "migration": migrationId},
		},
		Spec: api.OvirtVolumePopulatorSpec{
			EngineURL:        fmt.Sprintf("https://%s", sourceUrl.Host),
			EngineSecretName: secretName,
			DiskID:           da.Disk.ID,
			TransferNetwork:  transferNetwork,
		},
	}
}
