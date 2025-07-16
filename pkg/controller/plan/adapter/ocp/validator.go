package ocp

import (
	"context"
	"fmt"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	inventory "github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	ocpclient "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	cnv "kubevirt.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const VM_NOT_FOUND = "VM not found."
const FeatureDecentralizedLiveMigration = "DecentralizedLiveMigration"
const ConditionStorageLiveMigratable = "StorageLiveMigratable"
const ConditionLiveMigratable = "LiveMigratable"
const True = "True"

var Settings = &settings.Settings

// Validator
type Validator struct {
	log logging.LevelLogger
	*plancontext.Context
	sourceClient k8sclient.Client
}

// PowerState validates that the VM is in a power state that is compatible
// with the migration type.
func (r *Validator) PowerState(vmRef ref.Ref) (ok bool, err error) {
	switch r.Plan.Spec.Type {
	case api.MigrationLive:
		vmi := &cnv.VirtualMachineInstance{}
		err = r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vmi)
		if err != nil {
			if k8serr.IsNotFound(err) {
				err = nil
				return
			}
			err = liberr.Wrap(err, "vm", vmRef.String())
			return
		}
		ok = true
	default:
		ok = true
	}
	return
}

// VMMigrationType validates that the VM is compatible with the selected migration type.
func (r *Validator) VMMigrationType(vmRef ref.Ref) (ok bool, err error) {
	switch r.Plan.Spec.Type {
	case api.MigrationLive:
		vm := &cnv.VirtualMachine{}
		err = r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vm)
		if err != nil {
			err = liberr.Wrap(
				err,
				VM_NOT_FOUND,
				"vm",
				vmRef.String())
			return
		}
		for _, cnd := range vm.Status.Conditions {
			if cnd.Type == ConditionStorageLiveMigratable || cnd.Type == ConditionLiveMigratable {
				if cnd.Status != True {
					ok = false
					return
				}
			}
		}
	default:
		ok = true
	}
	return
}

// MaintenanceMode implements base.Validator
func (r *Validator) MaintenanceMode(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// PodNetwork implements base.Validator
func (r *Validator) PodNetwork(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return
	}

	vm := &cnv.VirtualMachine{}
	err = r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vm)
	if err != nil {
		err = liberr.Wrap(
			err,
			VM_NOT_FOUND,
			"vm",
			vmRef.String())
		return
	}
	mapping := r.Plan.Referenced.Map.Network.Spec.Map
	podMapped := 0
	for i := range mapping {
		mapped := &mapping[i]
		if mapped.Destination.Type == Pod {
			podMapped++
		}
	}

	ok = podMapped <= 1
	return
}

// WarmMigration implements base.Validator
func (r *Validator) WarmMigration() bool {
	return false
}

// MigrationType indicates whether the plan's migration type
// is supported by this provider.
func (r *Validator) MigrationType() bool {
	switch r.Plan.Spec.Type {
	case api.MigrationCold, "":
		return true
	case api.MigrationLive:
		kvs := []inventory.KubeVirt{}
		err := r.Source.Inventory.List(&kvs, web.Param{
			Key:   web.DetailParam,
			Value: "all",
		})
		if err != nil {
			r.log.Error(err, "unable to read KubeVirt resource from source inventory.")
			return false
		}
		if len(kvs) == 0 {
			r.log.Info("No KubeVirt resources found in source cluster inventory.")
			return false
		}
		src := KubeVirt{}
		src.With(kvs[0])
		err = r.Destination.Inventory.List(&kvs, web.Param{
			Key:   web.DetailParam,
			Value: "all",
		})
		if err != nil {
			r.log.Error(err, "unable to read KubeVirt resource from destination inventory.")
			return false
		}
		if len(kvs) == 0 {
			r.log.Info("No KubeVirt resources found in destination cluster inventory.")
			return false
		}
		dest := KubeVirt{}
		dest.With(kvs[0])
		return Settings.OCPLiveMigration &&
			src.FeatureGate(FeatureDecentralizedLiveMigration) &&
			dest.FeatureGate(FeatureDecentralizedLiveMigration)
	default:
		return false
	}
}

// NOOP
func (r *Validator) UnSupportedDisks(vmRef ref.Ref) ([]string, error) {
	return []string{}, nil
}

func (r *Validator) SharedDisks(vmRef ref.Ref, client k8sclient.Client) (ok bool, s string, s2 string, err error) {
	ok = true
	return
}

func (r *Validator) StorageMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Storage == nil {
		return
	}

	vm := &cnv.VirtualMachine{}
	err = r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vm)
	if err != nil {
		err = liberr.Wrap(
			err,
			VM_NOT_FOUND,
			"vm",
			vmRef.String())
		return
	}

	for _, vol := range vm.Spec.Template.Spec.Volumes {
		var pvcName string
		switch {
		case vol.PersistentVolumeClaim != nil:
			pvcName = vol.PersistentVolumeClaim.ClaimName
		case vol.DataVolume != nil:
			pvcName = vol.DataVolume.Name
		default:
			r.log.Info("Not PVC or DataVolume, skipping volume...", "volume", vol.Name)
			continue
		}

		pvc := &core.PersistentVolumeClaim{}
		err = r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{
			Namespace: vmRef.Namespace,
			Name:      pvcName,
		}, pvc)
		if err != nil {
			err = liberr.Wrap(
				err,
				"PVC not found.",
				"pvc",
				pvcName)
			return
		}

		storageClass := pvc.Spec.StorageClassName
		if storageClass == nil {
			return false, nil
		}

		_, found := r.Plan.Referenced.Map.Storage.FindStorageByName(*storageClass)
		if !found {
			err = liberr.Wrap(
				err,
				"StorageClass not found.",
				"StorageClass",
				*storageClass)

			return false, err
		}
	}

	return true, nil
}

// OCP-to-OCP VM migration requires names that are DNS_RFC_1035 compliant, unlike DNS_RFC_1123 where we modify the VM name as needed.
func (r *Validator) ValidVmName(vmRef ref.Ref) (ok bool, err error) {
	// Check if the VM reference name is a valid DNS1123 subdomain
	if len(k8svalidation.IsDNS1123Subdomain(vmRef.Name)) > 0 {
		// Not valid
		return false, nil
	} else if len(k8svalidation.IsDNS1035Label(vmRef.Name)) > 0 {
		return false, liberr.Wrap(
			fmt.Errorf("Invalid VM name %q: must be a valid DNS-1035 label", vmRef.Name),
			"vm",
			vmRef.String())
	}
	// Valid
	return true, nil
}

// Validate that a VM's networks have been mapped.
func (r *Validator) NetworksMapped(vmRef ref.Ref) (ok bool, err error) {
	if r.Plan.Referenced.Map.Network == nil {
		return
	}

	vm := &cnv.VirtualMachine{}
	err = r.sourceClient.Get(context.TODO(), k8sclient.ObjectKey{Namespace: vmRef.Namespace, Name: vmRef.Name}, vm)
	if err != nil {
		err = liberr.Wrap(
			err,
			VM_NOT_FOUND,
			"vm",
			vmRef.String())
		return
	}

	for _, net := range vm.Spec.Template.Spec.Networks {
		if net.Pod != nil {
			_, found := r.Plan.Referenced.Map.Network.FindNetworkByType(Pod)
			if !found {
				err = liberr.Wrap(
					err,
					"Pod network not found.",
					"vm",
					vmRef.String(),
				)
				r.log.Error(err, "Pod network not found.")

				return false, err
			}
		} else if net.Multus != nil {
			name, namespace := ocpclient.GetNetworkNameAndNamespace(net.Multus.NetworkName, &vmRef)
			_, found := r.Plan.Referenced.Map.Network.FindNetworkByNameAndNamespace(namespace, name)
			if !found {
				err = liberr.Wrap(
					err,
					"Multus network not found.",
					"network",
					fmt.Sprintf("%s/%s", namespace, name),
				)
				r.log.Error(err, "Multus network not found.")
				return false, err
			}
		}
	}

	return true, nil
}

// NO-OP
func (r *Validator) DirectStorage(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// NO-OP
func (r *Validator) StaticIPs(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// NO-OP
func (r *Validator) ChangeTrackingEnabled(vmRef ref.Ref) (bool, error) {
	return true, nil
}

// KubeVirt CR representation.
type KubeVirt struct {
	*cnv.KubeVirt
}

func (r *KubeVirt) With(kv inventory.KubeVirt) {
	r.KubeVirt = &kv.Object
}

func (r *KubeVirt) FeatureGate(feature string) (enabled bool) {
	if r.Spec.Configuration.DeveloperConfiguration == nil {
		return
	}
	for _, gate := range r.Spec.Configuration.DeveloperConfiguration.FeatureGates {
		if strings.EqualFold(gate, feature) {
			enabled = true
			return
		}
	}
	return
}
