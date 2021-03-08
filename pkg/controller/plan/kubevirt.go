package plan

import (
	"context"
	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/builder"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	vmio "kubevirt.io/vm-import-operator/pkg/apis/v2v/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
	"strings"
)

// Annotations
const (
	// transfer network annotation (value=network-attachment-definition name)
	annDefaultNetwork = "v1.multus-cni.io/default-network"
)

// Labels
const (
	// migration label (value=UID)
	kMigration = "migration"
	// plan label (value=UID)
	kPlan = "plan"
	// VM label (value=vmID)
	kVM = "vmID"
)

//
// Map of VmImport keyed by vmID.
type ImportMap map[string]VmImport

//
// Represents kubevirt.
type KubeVirt struct {
	*plancontext.Context
	// Builder
	Builder builder.Builder
}

//
// Build a ImportMap.
func (r *KubeVirt) ImportMap() (mp ImportMap, err error) {
	list, err := r.ListImports()
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	mp = ImportMap{}
	for _, object := range list {
		mp[object.Labels[kVM]] = object
	}

	return
}

//
// List import CRs.
// Each VmImport represents a VMIO VirtualMachineImport
// with associated DataVolumes.
func (r *KubeVirt) ListImports() ([]VmImport, error) {
	vList := &vmio.VirtualMachineImportList{}
	err := r.Destination.Client.List(
		context.TODO(),
		vList,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.planLabels()),
			Namespace:     r.Plan.TargetNamespace(),
		},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	list := []VmImport{}
	for i := range vList.Items {
		vmImport := &vList.Items[i]
		list = append(
			list,
			VmImport{
				VirtualMachineImport: vmImport,
			})
	}
	dvList := &cdi.DataVolumeList{}
	err = r.Destination.Client.List(
		context.TODO(),
		dvList,
		&client.ListOptions{
			Namespace: r.Plan.TargetNamespace(),
		},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	for i := range list {
		vmImport := &list[i]
		for i := range dvList.Items {
			dv := &dvList.Items[i]
			if vmImport.Owner(dv) {
				vmImport.DataVolumes = append(
					vmImport.DataVolumes,
					DataVolume{
						DataVolume: dv,
					})
			}
		}
	}

	return list, nil
}

//
// Create the VMIO CR on the destination.
func (r *KubeVirt) EnsureImport(vm *plan.VMStatus) (err error) {
	secret, err := r.ensureSecret(vm.Ref)
	if err != nil {
		return
	}
	newImport, err := r.vmImport(vm, secret)
	if err != nil {
		return
	}
	list := &vmio.VirtualMachineImportList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.TargetNamespace(),
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	vmImport := &vmio.VirtualMachineImport{}
	if len(list.Items) > 0 {
		vmImport = &list.Items[0]
		vmImport.Spec = newImport.Spec
		err = r.Destination.Client.Update(context.TODO(), vmImport)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	} else {
		vmImport = newImport
		err = r.Destination.Client.Create(context.TODO(), vmImport)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}
	err = k8sutil.SetOwnerReference(vmImport, secret, scheme.Scheme)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.Destination.Client.Update(context.TODO(), secret)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Delete the VMIO CR for the migration on the destination.
func (r *KubeVirt) DeleteImport(vm *plan.VMStatus) (err error) {
	list := &vmio.VirtualMachineImportList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vm.Ref)),
			Namespace:     r.Plan.TargetNamespace(),
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	for _, object := range list.Items {
		err = r.Destination.Client.Delete(context.TODO(), &object)
		if err != nil {
			if k8serr.IsNotFound(err) {
				err = nil
			} else {
				return liberr.Wrap(err)
			}
		}
	}

	return
}

//
// Ensure the namespace exists on the destination.
func (r *KubeVirt) EnsureNamespace() (err error) {
	ns := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: r.Plan.TargetNamespace(),
		},
	}
	err = r.Destination.Client.Create(context.TODO(), ns)
	if err != nil {
		if k8serr.IsAlreadyExists(err) {
			err = nil
		}
	}

	return
}

//
// Ensure the VMIO secret exists on the destination.
func (r *KubeVirt) ensureSecret(vmRef ref.Ref) (secret *core.Secret, err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	newSecret, err := r.secret(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	list := &core.SecretList{}
	err = r.Destination.Client.List(
		context.TODO(),
		list,
		&client.ListOptions{
			LabelSelector: labels.SelectorFromSet(r.vmLabels(vmRef)),
			Namespace:     r.Plan.TargetNamespace(),
		},
	)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	if len(list.Items) > 0 {
		secret = &list.Items[0]
		secret.Data = newSecret.Data
		err = r.Destination.Client.Update(context.TODO(), secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	} else {
		secret = newSecret
		err = r.Destination.Client.Create(context.TODO(), secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
	}

	return
}

//
// Build the VMIO CR.
func (r *KubeVirt) vmImport(
	vm *plan.VMStatus,
	secret *core.Secret) (object *vmio.VirtualMachineImport, err error) {
	_, err = r.Source.Inventory.VM(&vm.Ref)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	annotations := make(map[string]string)
	if r.Plan.Spec.TransferNetwork != "" {
		annotations[annDefaultNetwork] = r.Plan.Spec.TransferNetwork
	}
	object = &vmio.VirtualMachineImport{
		ObjectMeta: meta.ObjectMeta{
			Namespace:   r.Plan.TargetNamespace(),
			Labels:      r.vmLabels(vm.Ref),
			Annotations: annotations,
			GenerateName: strings.Join(
				[]string{
					r.Plan.Name,
					vm.ID},
				"-") + "-",
		},
		Spec: vmio.VirtualMachineImportSpec{
			ProviderCredentialsSecret: vmio.ObjectIdentifier{
				Namespace: &secret.Namespace,
				Name:      secret.Name,
			},
		},
	}
	err = r.Builder.Import(vm.Ref, &object.Spec)
	if err != nil {
		err = liberr.Wrap(err)
	}
	if vm.Name != "" {
		object.Spec.TargetVMName = &vm.Name
	}

	// the value set on the migration, if any, takes precedence over the value set on the plan.
	if r.Plan.Spec.Warm {
		object.Spec.Warm = true
		if r.Migration.Spec.Cutover != nil {
			object.Spec.FinalizeDate = r.Migration.Spec.Cutover
		} else if r.Plan.Spec.Cutover != nil {
			object.Spec.FinalizeDate = r.Plan.Spec.Cutover
		} else {
			now := meta.Now()
			object.Spec.FinalizeDate = &now
		}
	}

	return
}

//
// Build the VMIO secret.
func (r *KubeVirt) secret(vmRef ref.Ref) (object *core.Secret, err error) {
	object = &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Labels:    r.vmLabels(vmRef),
			Namespace: r.Plan.TargetNamespace(),
			GenerateName: strings.Join(
				[]string{
					r.Plan.Name,
					vmRef.ID},
				"-") + "-",
		},
	}
	err = r.Builder.Secret(vmRef, r.Source.Secret, object)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Labels for plan and migration.
func (r *KubeVirt) planLabels() map[string]string {
	return map[string]string{
		kMigration: string(r.Migration.UID),
		kPlan:      string(r.Plan.GetUID()),
	}
}

//
// Labels for a VM on a plan.
func (r *KubeVirt) vmLabels(vmRef ref.Ref) (labels map[string]string) {
	labels = r.planLabels()
	labels[kVM] = vmRef.ID
	return
}

//
// Represents a CDI DataVolume and add behavior.
type DataVolume struct {
	*cdi.DataVolume
}

//
// Get conditions.
func (r *DataVolume) Conditions() (cnd *libcnd.Conditions) {
	cnd = &libcnd.Conditions{}
	for _, c := range r.Status.Conditions {
		cnd.SetCondition(libcnd.Condition{
			Type:               string(c.Type),
			Status:             string(c.Status),
			Reason:             c.Reason,
			Message:            c.Message,
			LastTransitionTime: c.LastTransitionTime,
		})
	}

	return
}

//
// Convert the Status.Progress into a
// percentage (float).
func (r *DataVolume) PercentComplete() (pct float64) {
	s := string(r.Status.Progress)
	if strings.HasSuffix(s, "%") {
		s = s[:len(s)-1]
		n, err := strconv.ParseFloat(s, 64)
		if err == nil {
			pct = n / 100
		}
	}

	return
}

//
// Represents VMIO VirtualMachineImport with associated DataVolumes.
type VmImport struct {
	*vmio.VirtualMachineImport
	DataVolumes []DataVolume
}

//
// Determine if `this` VMIO VirtualMachineImport is the
// owner of the CDI DataVolume.
func (r *VmImport) Owner(dv *cdi.DataVolume) bool {
	for _, ref := range r.Status.DataVolumes {
		if dv.Name == ref.Name {
			return true
		}
	}

	return false
}

//
// Get conditions.
func (r *VmImport) Conditions() (cnd *libcnd.Conditions) {
	cnd = &libcnd.Conditions{}
	for _, c := range r.Status.Conditions {
		newCnd := libcnd.Condition{
			Type:   string(c.Type),
			Status: string(c.Status),
		}
		if c.Reason != nil {
			newCnd.Reason = *c.Reason
		}
		if c.Message != nil {
			newCnd.Message = *c.Message
		}
		if c.LastTransitionTime != nil {
			newCnd.LastTransitionTime = *c.LastTransitionTime
		}
		cnd.SetCondition(newCnd)
	}

	return
}

//
// Convert the progress annotation into an int64.
func (r *VmImport) PercentComplete() (pct float64) {
	name := "vmimport.v2v.kubevirt.io/progress"
	if meta.HasAnnotation(r.ObjectMeta, name) {
		n, err := strconv.ParseFloat(r.Annotations[name], 64)
		if err != err {
			return
		}
		pct = n / 100
	}

	return
}
