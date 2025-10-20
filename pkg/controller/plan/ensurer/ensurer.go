package ensurer

import (
	"context"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensurer has the responsibility of making sure that resources have been
// created on the destination cluster.
type Ensurer struct {
	*plancontext.Context
}

// VirtualMachine ensures that the target VirtualMachine has been created in the destination cluster.
// VMs are ensured by label, not by name.
func (r *Ensurer) VirtualMachine(vm *planapi.VMStatus, target *cnv.VirtualMachine) (err error) {
	vms := &cnv.VirtualMachineList{}
	err = r.Destination.Client.List(
		context.TODO(),
		vms,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	if len(vms.Items) == 0 {
		r.Labeler.SetLabels(target, r.Labeler.VMLabels(vm.Ref))
		err = r.Destination.Client.Create(context.TODO(), target)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created destination VM.",
			"vm",
			path.Join(
				target.Namespace,
				target.Name),
			"source",
			vm.String())
	}
	return
}

// DataVolumes ensures DVs have been created on the destination cluster. Although we build DataVolumes with the same
// names they had on the source cluster, we search by label so that we notice conflicts with existing DVs.
func (r *Ensurer) DataVolumes(vm *planapi.VMStatus, dvs []cdi.DataVolume) (err error) {
	list := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.Background(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	exists := make(map[string]bool)
	for _, dv := range list.Items {
		exists[dv.Annotations[api.AnnDiskSource]] = true
	}

	for _, dv := range dvs {
		if !exists[dv.Annotations[api.AnnDiskSource]] {
			r.Labeler.SetLabels(&dv, r.Labeler.VMLabels(vm.Ref))
			err = r.Destination.Client.Create(context.Background(), &dv)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			r.Log.Info("Created DataVolume.",
				"dv",
				path.Join(
					dv.Namespace,
					dv.Name),
				"vm",
				vm.String())
		}
	}
	return
}

// PersistentVolumeClaims ensures PVCs have been created on the destination cluster. Although we build PersistentVolumeClaims with the same
// names they had on the source cluster, we search by label so that we notice conflicts with existing PVCs.
func (r *Ensurer) PersistentVolumeClaims(vm *planapi.VMStatus, pvcs []core.PersistentVolumeClaim) (err error) {
	list := &core.PersistentVolumeClaimList{}
	err = r.Destination.Client.List(
		context.Background(),
		list,
		&client.ListOptions{
			LabelSelector: k8slabels.SelectorFromSet(r.Labeler.VMLabels(vm.Ref)),
			Namespace:     r.Plan.Spec.TargetNamespace,
		})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	exists := make(map[string]bool)
	for _, pvc := range list.Items {
		exists[pvc.Annotations[api.AnnDiskSource]] = true
	}

	for _, pvc := range pvcs {
		if !exists[pvc.Annotations[api.AnnDiskSource]] {
			r.Labeler.SetLabels(&pvc, r.Labeler.VMLabels(vm.Ref))
			err = r.Destination.Client.Create(context.Background(), &pvc)
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			r.Log.Info("Created PersistentVolumeClaim.",
				"dv",
				path.Join(
					pvc.Namespace,
					pvc.Name),
				"vm",
				vm.String())
		}
	}
	return
}

// SharedConfigMaps ensures the config maps exist in the destination cluster's target namespace. We attempt
// to create ConfigMaps with the same name that they have on the source cluster because they are likely
// to be shared between multiple VMs. If one with a matching name already exists, we assume it's the intended
// ConfigMap for the VM to mount.
//
// Shared configmaps are ensured by name, not by label.
//
// TODO: consider raising a concern at the VM or plan level if a configmap with the desired
// name already exists but does not have the annotation indicating that Forklift created it.
func (r *Ensurer) SharedConfigMaps(vm *planapi.VMStatus, configMaps []core.ConfigMap) (err error) {
	for _, configMap := range configMaps {
		err = r.Destination.Client.Create(context.Background(), &configMap)
		if err != nil {
			if k8serr.IsAlreadyExists(err) {
				_, found := configMap.Annotations[api.AnnSource]
				if !found {
					r.Log.Info("Matching ConfigMap already present in destination namespace.", "configMap",
						path.Join(
							configMap.Namespace,
							configMap.Name),
						"forklift-created", false)
				}
				err = nil
				continue
			}
			err = liberr.Wrap(err, "Failed to create ConfigMap.", "configMap",
				path.Join(
					configMap.Namespace,
					configMap.Name))
			return
		}
		r.Log.Info("Created ConfigMap.",
			"configMap",
			path.Join(
				configMap.Namespace,
				configMap.Name),
			"vm",
			vm.String())
	}
	return
}

// SharedSecrets ensures secrets exist in the destination cluster's target namespace. We attempt to create Secrets
// with the same name that they have on the source cluster because they are likely to be shared between
// multiple VMs. If one with a matching name already exists, we assume it's the intended Secret for
// the VM to mount.
//
// Shared secrets are ensured by name, not by label.
//
// TODO: consider raising a concern at the VM or plan level if a secret with the desired
// name already exists but does not have the annotation indicating that Forklift created it.
func (r *Ensurer) SharedSecrets(vm *planapi.VMStatus, secrets []core.Secret) (err error) {
	for _, secret := range secrets {
		err = r.Destination.Client.Create(context.Background(), &secret)
		if err != nil {
			if k8serr.IsAlreadyExists(err) {
				_, found := secret.Annotations[api.AnnSource]
				if !found {
					r.Log.Info("Matching Secret already present in destination namespace.", "secret",
						path.Join(
							secret.Namespace,
							secret.Name),
						"forklift-created", false)
				}
				err = nil
				continue
			}
			err = liberr.Wrap(err, "Failed to create Secret.", "secret",
				path.Join(
					secret.Namespace,
					secret.Name))
			return
		}
		r.Log.Info("Created Secret.",
			"secret",
			path.Join(
				secret.Namespace,
				secret.Name),
			"vm",
			vm.String())
	}
	return
}
