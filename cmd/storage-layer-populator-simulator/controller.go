package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// clonePVCSuffix is appended to the destination PVC name to create the clone PVC.
	clonePVCSuffix = "-slp-prime"
	// requeueInterval is how often to re-check PVC/PV status while waiting.
	requeueInterval = 5 * time.Second
)

// PopulatorReconciler watches PortworxXcopyVolumePopulator CRs and simulates
// the storage-layer operator by performing a CSI volume clone from the source
// (intermediate FADA) PVC into the destination PVC.
type PopulatorReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Log       logr.Logger
	Namespace string
}

func (r *PopulatorReconciler) SetupWithManager(mgr manager.Manager) error {
	c, err := controller.New("storage-layer-populator", mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return err
	}
	return c.Watch(source.Kind(mgr.GetCache(), &forkliftv1beta1.PortworxXcopyVolumePopulator{}, &handler.TypedEnqueueRequestForObject[*forkliftv1beta1.PortworxXcopyVolumePopulator]{}))
}

func (r *PopulatorReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.Log.WithValues("cr", req.NamespacedName)

	cr := &forkliftv1beta1.PortworxXcopyVolumePopulator{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if k8serr.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Terminal states — nothing more to do.
	if cr.Status.Phase == "Completed" || cr.Status.Phase == "Failed" {
		return reconcile.Result{}, nil
	}

	// Fetch source (FADA) PVC.
	sourcePVC := &core.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{Namespace: cr.Spec.SourceNamespace, Name: cr.Spec.SourcePvc}, sourcePVC)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return reconcile.Result{}, r.setStatus(ctx, cr, "Failed", "0%", "source PVC not found: "+cr.Spec.SourcePvc)
		}
		return reconcile.Result{}, err
	}
	if sourcePVC.Status.Phase != core.ClaimBound {
		log.Info("Source PVC not yet bound, requeueing", "sourcePVC", sourcePVC.Name)
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	// The destination PVC name is the CR name minus the "-populator" suffix.
	destPVCName := destPVCNameFromCR(cr.Name)
	destPVC := &core.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKey{Namespace: cr.Namespace, Name: destPVCName}, destPVC)
	if err != nil {
		if k8serr.IsNotFound(err) {
			return reconcile.Result{}, r.setStatus(ctx, cr, "Failed", "0%", "destination PVC not found: "+destPVCName)
		}
		return reconcile.Result{}, err
	}

	// If source and dest use different storage classes, CSI volume clone won't work
	// (cross-provisioner). Skip the real clone and mark as Completed immediately.
	srcSC := ""
	dstSC := ""
	if sourcePVC.Spec.StorageClassName != nil {
		srcSC = *sourcePVC.Spec.StorageClassName
	}
	if destPVC.Spec.StorageClassName != nil {
		dstSC = *destPVC.Spec.StorageClassName
	}
	if srcSC != dstSC {
		log.Info("Cross-provisioner detected, skipping real clone (simulator mode)",
			"source", sourcePVC.Name, "sourceSC", srcSC, "destSC", dstSC)
		return reconcile.Result{}, r.setStatus(ctx, cr, "Completed", "100%", "simulated: cross-provisioner copy")
	}

	// Clone PVC: <destName>-slp-prime.
	cloneName := cr.Name + clonePVCSuffix
	clonePVC := &core.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKey{Namespace: cr.Namespace, Name: cloneName}, clonePVC)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		// Clone PVC does not exist yet — create it with dataSource pointing to sourcePVC.
		storageSize := sourcePVC.Spec.Resources.Requests[core.ResourceStorage]
		if destPVC.Spec.Resources.Requests != nil {
			if ds, ok := destPVC.Spec.Resources.Requests[core.ResourceStorage]; ok && ds.Cmp(resource.MustParse("0")) > 0 {
				storageSize = ds
			}
		}

		newClone := &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      cloneName,
				Namespace: cr.Namespace,
				Labels:    destPVC.Labels,
			},
			Spec: core.PersistentVolumeClaimSpec{
				StorageClassName: destPVC.Spec.StorageClassName,
				VolumeMode:       destPVC.Spec.VolumeMode,
				AccessModes:      destPVC.Spec.AccessModes,
				Resources: core.VolumeResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: storageSize,
					},
				},
				DataSource: &core.TypedLocalObjectReference{
					Kind: "PersistentVolumeClaim",
					Name: sourcePVC.Name,
				},
			},
		}
		if err := r.Create(ctx, newClone); err != nil {
			return reconcile.Result{}, err
		}
		log.Info("Created clone PVC", "clone", cloneName, "source", sourcePVC.Name)
		_ = r.setStatus(ctx, cr, "InProgress", "0%", "clone PVC created, waiting for bind")
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	// Clone exists — check if it's bound.
	if clonePVC.Status.Phase != core.ClaimBound {
		log.Info("Clone PVC not yet bound, requeueing", "clone", cloneName)
		_ = r.setStatus(ctx, cr, "InProgress", "50%", "waiting for clone PVC to bind")
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	if clonePVC.Spec.VolumeName == "" {
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	pv := &core.PersistentVolume{}
	if err := r.Get(ctx, client.ObjectKey{Name: clonePVC.Spec.VolumeName}, pv); err != nil {
		return reconcile.Result{}, err
	}

	// Check if the PV is already rebound to the destination PVC.
	if pv.Spec.ClaimRef != nil &&
		pv.Spec.ClaimRef.Name == destPVC.Name &&
		pv.Spec.ClaimRef.Namespace == destPVC.Namespace {
		if destPVC.Status.Phase != core.ClaimBound {
			log.Info("Waiting for dest PVC to bind after rebind", "destPVC", destPVC.Name)
			return reconcile.Result{RequeueAfter: requeueInterval}, nil
		}
		// Dest PVC is bound — delete clone and mark success.
		if err := r.Delete(ctx, clonePVC); err != nil && !k8serr.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		log.Info("Two-phase storage-layer simulation complete", "destPVC", destPVC.Name)
		return reconcile.Result{}, r.setStatus(ctx, cr, "Completed", "100%", "")
	}

	// Step 1: set PV reclaim policy to Retain so deleting the clone PVC keeps the PV.
	if pv.Spec.PersistentVolumeReclaimPolicy != core.PersistentVolumeReclaimRetain {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"persistentVolumeReclaimPolicy": string(core.PersistentVolumeReclaimRetain),
			},
		}
		patchBytes, _ := json.Marshal(patch)
		if err := r.Patch(ctx, pv, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to set PV reclaim policy: %w", err)
		}
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	// Step 2: delete the clone PVC to release the PVC→PV binding (PV stays due to Retain).
	if err := r.Delete(ctx, clonePVC); err != nil && !k8serr.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	// Step 3: patch the PV's claimRef to point to the destination PVC.
	patchPV := map[string]interface{}{
		"spec": map[string]interface{}{
			"claimRef": map[string]interface{}{
				"namespace":       destPVC.Namespace,
				"name":            destPVC.Name,
				"uid":             string(destPVC.UID),
				"resourceVersion": destPVC.ResourceVersion,
			},
		},
	}
	patchBytes, _ := json.Marshal(patchPV)
	if err := r.Patch(ctx, pv, client.RawPatch(types.MergePatchType, patchBytes)); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to rebind PV to dest PVC: %w", err)
	}
	log.Info("Rebound PV to destination PVC", "pv", pv.Name, "destPVC", destPVC.Name)
	_ = r.setStatus(ctx, cr, "InProgress", "90%", "PV rebound, waiting for dest PVC to bind")
	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (r *PopulatorReconciler) setStatus(ctx context.Context, cr *forkliftv1beta1.PortworxXcopyVolumePopulator, phase, progress, message string) error {
	patch := cr.DeepCopy()
	patch.Status.Phase = phase
	patch.Status.Progress = progress
	patch.Status.Message = message
	return r.Status().Update(ctx, patch)
}

// destPVCNameFromCR derives the destination PVC name from the CR name.
// CR names follow the pattern "<finalPVCName>-populator".
func destPVCNameFromCR(crName string) string {
	const suffix = "-populator"
	if len(crName) > len(suffix) && crName[len(crName)-len(suffix):] == suffix {
		return crName[:len(crName)-len(suffix)]
	}
	return crName
}
