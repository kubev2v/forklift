package ensurer

import (
	"context"
	"fmt"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/azure"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/builder"
	"github.com/kubev2v/forklift/pkg/provider/azure/controller/inventory"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Ensurer) EnsureVolumeSnapshotContent(vm *planapi.VMStatus, snapshotResourceIDs []string) error {
	bldr := builder.New(r.Context)

	for i, snapshotResourceID := range snapshotResourceIDs {
		vsc, err := bldr.BuildVolumeSnapshotContent(vm.Ref, snapshotResourceID, i)
		if err != nil {
			return liberr.Wrap(err)
		}

		existed, err := r.ensureCreated(vsc, "VolumeSnapshotContent")
		if err != nil {
			return err
		}
		if !existed {
			r.log.Info("Created VolumeSnapshotContent",
				"snapshotHandle", snapshotResourceID,
				"diskIndex", i)
		}
	}

	return nil
}

func (r *Ensurer) EnsureVolumeSnapshot(vm *planapi.VMStatus) error {
	bldr := builder.New(r.Context)

	azureVM, err := inventory.GetAzureVM(r.Source.Inventory, vm.Ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	disks := inventory.GetManagedDisks(azureVM)
	for i := range disks {
		vs, err := bldr.BuildVolumeSnapshot(vm.Ref, i)
		if err != nil {
			return liberr.Wrap(err)
		}

		existed, err := r.ensureCreated(vs, "VolumeSnapshot")
		if err != nil {
			return err
		}
		if !existed {
			r.log.Info("Created VolumeSnapshot",
				"name", vs.Name,
				"namespace", vs.Namespace,
				"diskIndex", i)
		}
	}

	return nil
}

func (r *Ensurer) EnsurePVCs(vm *planapi.VMStatus) error {
	ctx := context.TODO()
	bldr := builder.New(r.Context)

	azureVM, err := inventory.GetAzureVM(r.Source.Inventory, vm.Ref)
	if err != nil {
		return liberr.Wrap(err)
	}

	disks := inventory.GetManagedDisks(azureVM)
	for i, disk := range disks {
		if disk.SizeGB == 0 {
			return liberr.New("disk %d (%s): unable to determine size", i, disk.Name)
		}
		sku := disk.Sku

		pvc, err := bldr.BuildPVC(vm.Ref, int64(disk.SizeGB), sku, i, disk.ID)
		if err != nil {
			return liberr.Wrap(err)
		}

		if pvc.Annotations == nil {
			pvc.Annotations = map[string]string{}
		}
		pvc.Annotations[azure.AnnVolumeSnapshot] = fmt.Sprintf("%s-snap-%d", vm.Ref.Name, i)
		pvc.Annotations[azure.AnnSourceDiskID] = disk.ID

		if pvc.Labels == nil {
			pvc.Labels = map[string]string{}
		}
		pvc.Labels[azure.LabelVMID] = vm.ID

		existing := &core.PersistentVolumeClaimList{}
		err = r.Destination.Client.List(ctx, existing,
			client.InNamespace(pvc.Namespace),
			client.MatchingLabels{
				azure.LabelVMID:      vm.ID,
				azure.LabelDiskIndex: fmt.Sprintf("%d", i),
			},
		)
		if err == nil && len(existing.Items) > 0 {
			r.log.Info("PVC already exists",
				"name", existing.Items[0].Name,
				"diskIndex", i)
			continue
		}

		err = r.Destination.Client.Create(ctx, pvc)
		if err != nil {
			return liberr.Wrap(err)
		}

		r.log.Info("Created PVC",
			"name", pvc.Name,
			"namespace", pvc.Namespace,
			"diskIndex", i)
	}

	return nil
}

func (r *Ensurer) CheckPVCsBound(vm *planapi.VMStatus) (bool, error) {
	ctx := context.TODO()
	pvcList := &core.PersistentVolumeClaimList{}
	err := r.Destination.Client.List(ctx, pvcList,
		client.InNamespace(r.Plan.Spec.TargetNamespace),
		client.MatchingLabels{azure.LabelVMID: vm.ID},
	)
	if err != nil {
		return false, liberr.Wrap(err)
	}

	if len(pvcList.Items) == 0 {
		return false, fmt.Errorf("no PVCs found for VM %s", vm.Name)
	}

	for _, pvc := range pvcList.Items {
		if pvc.Status.Phase == core.ClaimBound {
			continue
		}
		if pvc.Status.Phase == core.ClaimPending && pvc.Spec.DataSource != nil {
			r.log.V(1).Info("PVC pending with dataSource (WaitForFirstConsumer), treating as ready",
				"name", pvc.Name)
			continue
		}
		r.log.V(1).Info("PVC not yet bound",
			"name", pvc.Name,
			"phase", pvc.Status.Phase)
		return false, nil
	}

	r.log.Info("All PVCs are ready", "vm", vm.Name, "count", len(pvcList.Items))
	return true, nil
}

func (r *Ensurer) InjectOwnerReferences(vm *planapi.VMStatus) error {
	ctx := context.TODO()
	pvcList := &core.PersistentVolumeClaimList{}
	err := r.Destination.Client.List(ctx, pvcList,
		client.InNamespace(r.Plan.Spec.TargetNamespace),
		client.MatchingLabels{azure.LabelVMID: vm.ID},
	)
	if err != nil {
		return liberr.Wrap(err)
	}

	for _, pvc := range pvcList.Items {
		vsName, ok := pvc.Annotations[azure.AnnVolumeSnapshot]
		if !ok {
			continue
		}

		vs := &unstructured.Unstructured{}
		vs.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "snapshot.storage.k8s.io",
			Version: "v1",
			Kind:    "VolumeSnapshot",
		})

		err := r.Destination.Client.Get(ctx, types.NamespacedName{
			Name:      vsName,
			Namespace: pvc.Namespace,
		}, vs)
		if err != nil {
			if k8serr.IsNotFound(err) {
				continue
			}
			return liberr.Wrap(err)
		}

		ownerRefs := []interface{}{
			map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "PersistentVolumeClaim",
				"name":       pvc.Name,
				"uid":        string(pvc.UID),
			},
		}

		if err := unstructured.SetNestedSlice(vs.Object, ownerRefs, "metadata", "ownerReferences"); err != nil {
			return liberr.Wrap(err)
		}

		err = r.Destination.Client.Update(ctx, vs)
		if err != nil {
			return liberr.Wrap(err)
		}

		r.log.Info("Injected OwnerReference on VolumeSnapshot",
			"snapshot", vsName,
			"owner", pvc.Name)
	}

	return nil
}

func (r *Ensurer) DeleteVolumeSnapshots(vm *planapi.VMStatus) error {
	ctx := context.TODO()
	vsList := &unstructured.UnstructuredList{}
	vsList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeSnapshotList",
	})

	err := r.Destination.Client.List(ctx, vsList,
		client.InNamespace(r.Plan.Spec.TargetNamespace),
		client.MatchingLabels{azure.LabelVMID: vm.ID},
	)
	if err != nil {
		return liberr.Wrap(err)
	}

	for i := range vsList.Items {
		vs := &vsList.Items[i]
		err = r.Destination.Client.Delete(ctx, vs, &client.DeleteOptions{
			PropagationPolicy: func() *meta.DeletionPropagation {
				p := meta.DeletePropagationForeground
				return &p
			}(),
		})
		if err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		r.log.Info("Deleted VolumeSnapshot", "name", vs.GetName())
	}
	return nil
}

func (r *Ensurer) DeleteVolumeSnapshotContents(vm *planapi.VMStatus) error {
	ctx := context.TODO()
	vscList := &unstructured.UnstructuredList{}
	vscList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeSnapshotContentList",
	})

	err := r.Destination.Client.List(ctx, vscList,
		client.MatchingLabels{azure.LabelVMID: vm.ID},
	)
	if err != nil {
		return liberr.Wrap(err)
	}

	for i := range vscList.Items {
		vsc := &vscList.Items[i]
		err = r.Destination.Client.Delete(ctx, vsc)
		if err != nil && !k8serr.IsNotFound(err) {
			return liberr.Wrap(err)
		}
		r.log.Info("Deleted VolumeSnapshotContent", "name", vsc.GetName())
	}
	return nil
}
