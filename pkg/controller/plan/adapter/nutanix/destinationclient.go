package nutanix

import (
	"context"
	"path"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DestinationClient cleans up Nutanix-specific destination resources.
type DestinationClient struct {
	*plancontext.Context
}

// DeletePopulatorDataSource removes NutanixHTTPVolumePopulator CRs for a VM.
func (r *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	list, err := r.getHTTPPopulatorCrList(vm.ID)
	if err != nil {
		return err
	}
	for i := range list.Items {
		err = r.deleteObject(&list.Items[i], vm, "Deleted NutanixHTTPVolumePopulator CR.", "NutanixHTTPVolumePopulator")
		if err != nil {
			return err
		}
	}
	return nil
}

// SetPopulatorCrOwnership sets the PVC as owner of each HTTP populator CR.
func (r *DestinationClient) SetPopulatorCrOwnership() error {
	list, err := r.getHTTPPopulatorCrList("")
	if err != nil {
		return err
	}
	var firstErr error
	for i := range list.Items {
		populatorCr := &list.Items[i]
		pvc := &core.PersistentVolumeClaim{}
		err := r.Destination.Get(context.TODO(), client.ObjectKey{
			Namespace: populatorCr.Namespace,
			Name:      populatorCr.Name,
		}, pvc)
		if err != nil {
			if k8serr.IsNotFound(err) {
				continue
			}
			if firstErr == nil {
				firstErr = liberr.Wrap(err, "populator", populatorCr.Name)
			}
			continue
		}
		populatorCrCopy := populatorCr.DeepCopy()
		if err := k8sutil.SetOwnerReference(pvc, populatorCr, r.Scheme()); err != nil {
			if firstErr == nil {
				firstErr = liberr.Wrap(err, "populator", populatorCr.Name)
			}
			continue
		}
		patch := client.MergeFrom(populatorCrCopy)
		if err := r.Destination.Patch(context.TODO(), populatorCr, patch); err != nil && firstErr == nil {
			firstErr = liberr.Wrap(err, "populator", populatorCr.Name)
		}
	}
	return firstErr
}

func (r *DestinationClient) getHTTPPopulatorCrList(vmID string) (v1beta1.NutanixHTTPVolumePopulatorList, error) {
	list := v1beta1.NutanixHTTPVolumePopulatorList{}
	labelSet := map[string]string{"migration": string(r.Plan.Status.Migration.ActiveSnapshot().Migration.UID)}
	if vmID != "" {
		labelSet["vmID"] = vmID
	}
	err := r.Destination.List(
		context.TODO(),
		&list,
		&client.ListOptions{
			Namespace:     r.Plan.Spec.TargetNamespace,
			LabelSelector: labels.SelectorFromSet(labelSet),
		})
	return list, err
}

func (r *DestinationClient) deleteObject(object client.Object, vm *plan.VMStatus, message, objType string) error {
	err := r.Destination.Delete(context.TODO(), object)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return nil
		}
		return liberr.Wrap(err)
	}
	r.Log.Info(
		message,
		objType,
		path.Join(object.GetNamespace(), object.GetName()),
		"vm",
		vm.String())
	return nil
}
