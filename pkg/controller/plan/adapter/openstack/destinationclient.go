package openstack

import (
	"context"
	"path"

	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type DestinationClient struct {
	*plancontext.Context
}

// Delete OpenstackVolumePopulator CustomResource list.
func (r *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	populatorCrList, err := r.getPopulatorCrList()
	if err != nil {
		return err
	}
	for _, populatorCr := range populatorCrList.Items {
		err = r.DeleteObject(&populatorCr, vm, "Deleted OpenstackPopulator CR.", "OpenstackVolumePopulator")
		if err != nil {
			return err
		}
	}
	return nil
}

// Set the OpenstackVolumePopulator CustomResource Ownership.
func (r *DestinationClient) SetPopulatorCrOwnership() (err error) {
	populatorCrList, err := r.getPopulatorCrList()
	if err != nil {
		return
	}
	for _, populatorCr := range populatorCrList.Items {
		pvc := core.PersistentVolumeClaim{}
		err = r.Destination.Client.Get(context.TODO(), client.ObjectKey{Namespace: r.Plan.Spec.TargetNamespace, Name: populatorCr.Spec.ImageID}, &pvc)
		if err != nil {
			continue
		}
		populatorCrCopy := populatorCr.DeepCopy()
		err = k8sutil.SetOwnerReference(&pvc, &populatorCr, r.Scheme())
		if err != nil {
			continue
		}
		patch := client.MergeFrom(populatorCrCopy)
		err = r.Destination.Client.Patch(context.TODO(), &populatorCr, patch)
		if err != nil {
			continue
		}
	}
	return
}

// Get the OpenstackVolumePopulator CustomResource List.
func (r *DestinationClient) getPopulatorCrList() (populatorCrList v1beta1.OpenstackVolumePopulatorList, err error) {
	populatorCrList = v1beta1.OpenstackVolumePopulatorList{}
	err = r.Destination.Client.List(
		context.TODO(),
		&populatorCrList,
		&client.ListOptions{
			Namespace:     r.Plan.Spec.TargetNamespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{"migration": string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID)}),
		})
	return
}

// Deletes an object from destination cluster associated with the VM.
func (r *DestinationClient) DeleteObject(object client.Object, vm *plan.VMStatus, message, objType string) (err error) {
	//TODO use kubevirt? it will move most of the logic of the DestinationClient out.
	err = r.Destination.Client.Delete(context.TODO(), object)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		} else {
			return liberr.Wrap(err)
		}
	} else {
		r.Log.Info(
			message,
			objType,
			path.Join(
				object.GetNamespace(),
				object.GetName()),
			"vm",
			vm.String())
	}
	return
}
