package ensurer

import (
	"context"
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// EnsurePopulatorSecret creates a secret with AWS credentials.
func (r *Ensurer) EnsurePopulatorSecret(ctx context.Context, vm *planapi.VMStatus) (string, error) {
	secretName := fmt.Sprintf("%s-ec2-populator", vm.Ref.Name)

	existingSecret := &core.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: r.Plan.Spec.TargetNamespace,
	}, existingSecret)

	if err == nil {
		r.log.V(1).Info("Populator secret already exists", "vm", vm.Name, "secret", secretName)
		return secretName, nil
	}

	if !errors.IsNotFound(err) {
		return "", liberr.Wrap(err, "failed to check for existing populator secret")
	}

	providerSecret := &core.Secret{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      r.Source.Secret.Name,
		Namespace: r.Source.Secret.Namespace,
	}, providerSecret)
	if err != nil {
		return "", liberr.Wrap(err, "failed to get provider secret")
	}

	accessKeyID, hasAccessKey := providerSecret.Data["accessKeyId"]
	secretAccessKey, hasSecretKey := providerSecret.Data["secretAccessKey"]

	if !hasAccessKey || !hasSecretKey {
		return "", fmt.Errorf("provider secret missing required keys (accessKeyId, secretAccessKey)")
	}

	populatorSecret := &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      secretName,
			Namespace: r.Plan.Spec.TargetNamespace,
			Labels:    r.Labeler.VMLabels(vm.Ref),
			Annotations: map[string]string{
				"forklift.konveyor.io/vm":      vm.Name,
				"forklift.konveyor.io/plan":    r.Plan.Name,
				"forklift.konveyor.io/purpose": "ec2-populator",
			},
		},
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     accessKeyID,
			"AWS_SECRET_ACCESS_KEY": secretAccessKey,
		},
		Type: core.SecretTypeOpaque,
	}

	err = controllerutil.SetOwnerReference(r.Plan, populatorSecret, r.Client.Scheme())
	if err != nil {
		r.log.Error(err, "Failed to set owner reference on populator secret", "vm", vm.Name)
	}

	err = r.Client.Create(ctx, populatorSecret)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			r.log.V(1).Info("Populator secret created by another reconcile", "vm", vm.Name, "secret", secretName)
			return secretName, nil
		}
		return "", liberr.Wrap(err, "failed to create populator secret")
	}

	r.log.Info("Created populator secret with AWS standard env vars",
		"vm", vm.Name,
		"secret", secretName,
		"namespace", r.Plan.Spec.TargetNamespace)

	return secretName, nil
}

// CleanupPopulatorSecret removes the populator secret.
func (r *Ensurer) CleanupPopulatorSecret(ctx context.Context, vm *planapi.VMStatus) error {
	secretName := fmt.Sprintf("%s-ec2-populator", vm.Ref.Name)

	secret := &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      secretName,
			Namespace: r.Plan.Spec.TargetNamespace,
		},
	}

	err := r.Client.Delete(ctx, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			r.log.V(1).Info("Populator secret already deleted", "vm", vm.Name, "secret", secretName)
			return nil
		}
		r.log.Error(err, "Failed to delete populator secret", "vm", vm.Name, "secret", secretName)
		return liberr.Wrap(err)
	}

	r.log.Info("Deleted populator secret", "vm", vm.Name, "secret", secretName)
	return nil
}

// EnsureEc2VolumePopulatorsWithNames creates Ec2VolumePopulator CRs and returns volumeID to name mapping.
func (r *Ensurer) EnsureEc2VolumePopulatorsWithNames(ctx context.Context, vm *planapi.VMStatus, populators []*api.Ec2VolumePopulator) (map[string]string, error) {
	populatorNames := make(map[string]string)

	for _, populator := range populators {
		volumeID := populator.Labels["forklift.konveyor.io/volume-id"]

		// Check if a populator already exists for this volume in this migration
		existingPopList := &api.Ec2VolumePopulatorList{}
		err := r.Client.List(ctx, existingPopList, &client.ListOptions{
			Namespace:     populator.Namespace,
			LabelSelector: k8slabels.SelectorFromSet(populator.Labels),
		})

		if err != nil {
			return nil, liberr.Wrap(err, "failed to list existing Ec2VolumePopulators")
		}

		if len(existingPopList.Items) > 0 {
			// Use existing populator
			existingPop := &existingPopList.Items[0]
			populatorNames[volumeID] = existingPop.Name
			r.log.V(1).Info("Ec2VolumePopulator already exists",
				"vm", vm.Name,
				"volumeID", volumeID,
				"populator", existingPop.Name)
			continue
		}

		// Create new populator
		err = r.Client.Create(ctx, populator)
		if err != nil {
			r.log.Error(err, "Failed to create Ec2VolumePopulator",
				"vm", vm.Name,
				"volumeID", volumeID)
			return nil, liberr.Wrap(err)
		}

		// After creation, populator.Name contains the generated name
		populatorNames[volumeID] = populator.Name

		r.log.Info("Created Ec2VolumePopulator",
			"vm", vm.Name,
			"volumeID", volumeID,
			"populator", populator.Name,
			"snapshot", populator.Spec.SnapshotID)
	}

	return populatorNames, nil
}

// EnsurePersistentVolumeClaims creates PVCs and checks binding status.
func (r *Ensurer) EnsurePersistentVolumeClaims(ctx context.Context, vm *planapi.VMStatus, pvcs []*core.PersistentVolumeClaim) (allBound bool, err error) {
	// List existing PVCs by label
	existingPVCList := &core.PersistentVolumeClaimList{}
	vmLabels := r.Labeler.VMLabels(vm.Ref)
	err = r.Client.List(ctx, existingPVCList, &client.ListOptions{
		Namespace:     r.Plan.Spec.TargetNamespace,
		LabelSelector: k8slabels.SelectorFromSet(vmLabels),
	})
	if err != nil {
		r.log.Error(err, "Failed to list existing PVCs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	// Index existing PVCs by volume ID
	existingPVCsByVolume := make(map[string]*core.PersistentVolumeClaim)
	for i := range existingPVCList.Items {
		pvc := &existingPVCList.Items[i]
		if volumeID, ok := pvc.Labels["forklift.konveyor.io/volume-id"]; ok {
			existingPVCsByVolume[volumeID] = pvc
		}
	}

	for _, pvc := range pvcs {
		volumeID := pvc.Labels["forklift.konveyor.io/volume-id"]
		if _, exists := existingPVCsByVolume[volumeID]; !exists {
			err = r.Client.Create(ctx, pvc)
			if err != nil {
				r.log.Error(err, "Failed to create PVC", "vm", vm.Name, "volumeID", volumeID)
				return false, liberr.Wrap(err)
			}
			r.log.Info("Created PVC from snapshot", "vm", vm.Name, "pvc", pvc.Name, "volumeID", volumeID)
			existingPVCsByVolume[volumeID] = pvc
		} else {
			r.log.V(1).Info("PVC already exists", "vm", vm.Name, "volumeID", volumeID)
		}
	}

	allBound = true
	for volumeID, existingPVC := range existingPVCsByVolume {
		freshPVC := &core.PersistentVolumeClaim{}
		err = r.Client.Get(ctx, client.ObjectKey{
			Namespace: existingPVC.Namespace,
			Name:      existingPVC.Name,
		}, freshPVC)

		if err != nil {
			r.log.Error(err, "Failed to get PVC status", "vm", vm.Name, "pvc", existingPVC.Name, "volumeID", volumeID)
			return false, liberr.Wrap(err)
		}

		if freshPVC.Status.Phase != core.ClaimBound {
			r.log.Info("PVC not yet bound", "vm", vm.Name, "pvc", freshPVC.Name, "volumeID", volumeID, "phase", freshPVC.Status.Phase)
			allBound = false
		}
	}

	if allBound {
		r.log.Info("All PVCs are bound", "vm", vm.Name, "count", len(pvcs))
	} else {
		r.log.Info("Waiting for PVCs to be bound", "vm", vm.Name)
	}

	return allBound, nil
}
