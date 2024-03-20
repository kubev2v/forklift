package util

import (
	"fmt"
	"net/url"

	libitr "github.com/konveyor/forklift-controller/pkg/lib/itinerary"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
)

func OpenstackVolumePopulator(image *openstack.Image, sourceUrl *url.URL, transferNetwork *core.ObjectReference, targetNamespace, secretName, vmId, migrationId string) *api.OpenstackVolumePopulator {
	return &api.OpenstackVolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			Name:      image.Name,
			Namespace: targetNamespace,
			Labels:    map[string]string{"vmID": vmId, "migration": migrationId},
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

func CreateConversionTask(inventory web.Client, vmRef ref.Ref) (*plan.Task, error) {
	workload := &model.Workload{}
	err := inventory.Find(workload, vmRef)
	if err != nil {
		err = liberr.Wrap(err, "vm", vmRef.String())
		return nil, err
	}

	if workload.ImageID == "" {
		//nolint:nilnil
		return nil, nil
	}

	imageID := workload.ImageID

	// Find image in inventory
	image := &model.Image{}
	err = inventory.Get(image, imageID)
	if err != nil {
		err = liberr.Wrap(err, "image", imageID)
		return nil, err
	}
	if image.DiskFormat == "raw" {
		return nil, nil
	}

	taskName := fmt.Sprintf("%s-convert", image.ID)
	return &plan.Task{
		Name: taskName,
		Progress: libitr.Progress{
			Completed: 0,
			Total:     100,
		},
	}, nil
}
