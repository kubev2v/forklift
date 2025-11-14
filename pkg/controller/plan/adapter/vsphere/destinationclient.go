package vsphere

import (
	"context"
	"path"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DestinationClient struct {
	*plancontext.Context
}

func (d *DestinationClient) DeletePopulatorDataSource(vm *plan.VMStatus) error {
	d.Log.Info("Starting DeletePopulatorDataSource", "vm", vm.String())

	populatorCrList, err := d.getPopulatorCrList()
	if err != nil {
		d.Log.Error(err, "Failed to get populator CR list")
		return liberr.Wrap(err)
	}

	d.Log.Info("Found populator CRs to delete", "count", len(populatorCrList.Items))

	for i, populatorCr := range populatorCrList.Items {
		d.Log.Info("Deleting populator CR",
			"index", i+1,
			"total", len(populatorCrList.Items),
			"name", populatorCr.Name,
			"namespace", populatorCr.Namespace,
			"vmdkPath", populatorCr.Spec.VmdkPath)

		err = d.DeleteObject(&populatorCr, vm, "Deleted VSphereXcopyPopulator CR.", "VSphereXcopyVolumePopulator")
		if err != nil {
			d.Log.Error(err, "Failed to delete populator CR", "name", populatorCr.Name)
			return liberr.Wrap(err)
		}

		d.Log.Info("Successfully deleted populator CR", "name", populatorCr.Name)
	}

	d.Log.Info("Completed DeletePopulatorDataSource", "vm", vm.String())
	return nil
}

func (r *DestinationClient) SetPopulatorCrOwnership() (err error) {
	// Owner references are already set during populator CR creation in builder.go
	// This method is kept for interface compatibility but is a no-op for vSphere
	r.Log.V(2).Info("Owner references already set during populator CR creation - no action needed")
	return nil
}

// Get the VSphereXcopyVolumePopulator CustomResource List.
func (r *DestinationClient) getPopulatorCrList() (populatorCrList v1beta1.VSphereXcopyVolumePopulatorList, err error) {
	snap := r.Plan.Status.Migration.ActiveSnapshot()
	if snap.Migration.UID == "" {
		err = liberr.New("no active migration snapshot", "plan", r.Plan.Name)
		r.Log.Error(err, "Cannot list populator CRs")
		return v1beta1.VSphereXcopyVolumePopulatorList{}, err
	}
	migUID := string(snap.Migration.UID)
	r.Log.Info("Getting populator CR list",
		"namespace", r.Plan.Spec.TargetNamespace,
		"migrationUID", migUID)

	populatorCrList = v1beta1.VSphereXcopyVolumePopulatorList{}
	err = r.Destination.Client.List(
		context.TODO(),
		&populatorCrList,
		&client.ListOptions{
			Namespace:     r.Plan.Spec.TargetNamespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{"migration": migUID}),
		})

	if err != nil {
		r.Log.Error(err, "Failed to list populator CRs")
	} else {
		r.Log.Info("Successfully listed populator CRs", "count", len(populatorCrList.Items))
	}

	return populatorCrList, err
}

// Deletes an object from destination cluster associated with the VM.
func (r *DestinationClient) DeleteObject(object client.Object, vm *plan.VMStatus, message, objType string) (err error) {
	r.Log.Info("Deleting object",
		"type", objType,
		"name", object.GetName(),
		"namespace", object.GetNamespace(),
		"vm", vm.String())

	err = r.Destination.Client.Delete(context.TODO(), object)
	if err != nil {
		if k8serr.IsNotFound(err) {
			r.Log.Info("Object not found, already deleted",
				"type", objType,
				"name", object.GetName())
			err = nil
		} else {
			r.Log.Error(err, "Failed to delete object",
				"type", objType,
				"name", object.GetName())
			return liberr.Wrap(err)
		}
	} else {
		r.Log.Info(message,
			objType,
			path.Join(object.GetNamespace(), object.GetName()),
			"vm", vm.String())
	}
	return err
}

func (r *DestinationClient) findPVCByCR(cr *v1beta1.VSphereXcopyVolumePopulator) (pvc *core.PersistentVolumeClaim, err error) {
	snap := r.Plan.Status.Migration.ActiveSnapshot()
	if snap.Migration.UID == "" {
		err = liberr.New("no active migration snapshot", "plan", r.Plan.Name)
		r.Log.Error(err, "Cannot find PVC for populator CR")
		return nil, err
	}
	migUID := string(snap.Migration.UID)
	r.Log.Info("Finding PVC for populator CR",
		"populatorName", cr.Name,
		"vmdkPath", cr.Spec.VmdkPath,
		"namespace", r.Plan.Spec.TargetNamespace,
		"migrationUID", migUID)

	pvcList := core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(
		context.TODO(),
		&pvcList,
		&client.ListOptions{
			Namespace: r.Plan.Spec.TargetNamespace,
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"migration": migUID,
			}),
			FieldSelector: fields.SelectorFromSet(map[string]string{
				"metadata.annotations.copy-offload": cr.Spec.VmdkPath,
			}),
		})

	if err != nil {
		r.Log.Error(err, "Failed to list PVCs for populator CR", "populatorName", cr.Name)
		err = liberr.Wrap(err)
		return nil, err
	}

	r.Log.Info("Found PVCs matching populator CR",
		"populatorName", cr.Name,
		"count", len(pvcList.Items))

	if len(pvcList.Items) == 0 {
		err = liberr.New("PVC not found", "vmdkPath", cr.Spec.VmdkPath)
		r.Log.Error(err, "No PVC found for populator CR", "populatorName", cr.Name)
		return nil, err
	}

	if len(pvcList.Items) > 1 {
		err = liberr.New("Multiple PVCs found", "vmdkPath", cr.Spec.VmdkPath)
		r.Log.Error(err, "Multiple PVCs found for populator CR",
			"populatorName", cr.Name,
			"count", len(pvcList.Items))
		return nil, err
	}

	pvc = &pvcList.Items[0]
	r.Log.Info("Successfully found matching PVC",
		"populatorName", cr.Name,
		"pvcName", pvc.Name)

	return pvc, nil
}
