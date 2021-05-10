package plan

import (
	"context"
	"path"
	"reflect"
	"strconv"
	"strings"

	libcnd "github.com/konveyor/controller/pkg/condition"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter"
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
	Builder adapter.Builder
}

//
// Build a ImportMap.
func (r *KubeVirt) ImportMap() (mp ImportMap, err error) {
	list, err := r.ListImports()
	if err != nil {
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
		// Update the existing VM import if the cutover date has changed.
		if !reflect.DeepEqual(vmImport.Spec.FinalizeDate, newImport.Spec.FinalizeDate) {
			patch := vmImport.DeepCopy()
			patch.Spec.FinalizeDate = newImport.Spec.FinalizeDate
			err = r.Destination.Client.Patch(context.TODO(), patch, client.MergeFrom(vmImport))
			if err != nil {
				err = liberr.Wrap(err)
				return
			}
			r.Log.Info(
				"Updated VM Import.",
				"import",
				path.Join(
					vmImport.Namespace,
					vmImport.Name),
				"vm",
				vm.String())
		}
	} else {
		vmImport = newImport
		err = r.Destination.Client.Create(context.TODO(), vmImport)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.Info(
			"Created VM Import.",
			"import",
			path.Join(
				vmImport.Namespace,
				vmImport.Name),
			"vm",
			vm.String())
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
		} else {
			r.Log.Info(
				"Deleted VM Import.",
				"import",
				path.Join(
					object.Namespace,
					object.Name),
				"vm",
				vm.String())
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
	r.Log.Info(
		"Created namespace.",
		"import",
		ns.Name)

	return
}

//
// Ensure the VMIO secret exists on the destination.
func (r *KubeVirt) ensureSecret(vmRef ref.Ref) (secret *core.Secret, err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		return
	}
	newSecret, err := r.secret(vmRef)
	if err != nil {
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
		secret.StringData = newSecret.StringData
		err = r.Destination.Client.Update(context.TODO(), secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.V(1).Info(
			"Secret updated.",
			"secret",
			path.Join(
				secret.Namespace,
				secret.Name),
			"vm",
			vmRef.String())
	} else {
		secret = newSecret
		err = r.Destination.Client.Create(context.TODO(), secret)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.Log.V(1).Info(
			"Secret created.",
			"secret",
			path.Join(
				secret.Namespace,
				secret.Name),
			"vm",
			vmRef.String())
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
		return
	}
	annotations := make(map[string]string)
	if r.Plan.Spec.TransferNetwork != nil {
		annotations[annDefaultNetwork] = path.Join(
			r.Plan.Spec.TransferNetwork.Namespace, r.Plan.Spec.TransferNetwork.Name)
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
		return
	}
	if vm.Name != "" {
		object.Spec.TargetVMName = &vm.Name
	}

	// the value set on the migration, if any, takes precedence over the value set on the plan.
	if r.Plan.Spec.Warm {
		object.Spec.Warm = true
		object.Spec.FinalizeDate = r.Migration.Spec.Cutover
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
