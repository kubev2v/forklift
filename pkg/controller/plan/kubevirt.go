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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	cdi "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	vmio "kubevirt.io/vm-import-operator/pkg/apis/v2v/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
	"strings"
)

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
	selector := labels.SelectorFromSet(
		map[string]string{
			kMigration: string(r.Migration.UID),
			kPlan:      string(r.Plan.GetUID()),
		})
	vList := &vmio.VirtualMachineImportList{}
	err := r.Destination.Client.List(
		context.TODO(),
		vList,
		&client.ListOptions{
			Namespace:     r.namespace(),
			LabelSelector: selector,
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
			Namespace: r.namespace(),
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
	newImport, err := r.buildImport(vm)
	if err != nil {
		return
	}
	err = r.ensureObject(newImport)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Set VMIO secret owner references.
func (r *KubeVirt) SetSecretOwner(vm *plan.VMStatus) (err error) {
	vmImport := &vmio.VirtualMachineImport{}
	err = r.Destination.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: r.namespace(),
			Name:      r.nameForImport(vm.Ref),
		},
		vmImport)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	secret := &core.Secret{}
	err = r.Destination.Client.Get(
		context.TODO(),
		client.ObjectKey{
			Namespace: r.namespace(),
			Name:      r.nameForSecret(vm.Ref),
		},
		secret)
	if err != nil {
		err = liberr.Wrap(err)
		return
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
// Ensure the namespace exists on the destination.
func (r *KubeVirt) EnsureNamespace() (err error) {
	ns := &core.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: r.namespace(),
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
func (r *KubeVirt) EnsureSecret(vmRef ref.Ref) (err error) {
	_, err = r.Source.Inventory.VM(&vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	secret, err := r.buildSecret(vmRef)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	err = r.ensureObject(secret)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	return
}

//
// Build the VMIO CR.
func (r *KubeVirt) buildImport(vm *plan.VMStatus) (object *vmio.VirtualMachineImport, err error) {
	namespace := r.namespace()
	mp := r.Context.Plan.Spec.Map
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	_, err = r.Source.Inventory.VM(&vm.Ref)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	object = &vmio.VirtualMachineImport{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.namespace(),
			Name:      r.nameForImport(vm.Ref),
			Labels: map[string]string{
				kMigration: string(r.Migration.UID),
				kPlan:      string(r.Plan.UID),
				kVM:        vm.ID,
			},
		},
		Spec: vmio.VirtualMachineImportSpec{
			ProviderCredentialsSecret: vmio.ObjectIdentifier{
				Namespace: &namespace,
				Name:      r.nameForSecret(vm.Ref),
			},
		},
	}
	err = r.Builder.Import(vm.Ref, &mp, &object.Spec)
	if err != nil {
		err = liberr.Wrap(err)
	}
	if vm.Name != "" {
		object.Spec.TargetVMName = &vm.Name
	}

	return
}

//
// Build the VMIO secret.
func (r *KubeVirt) buildSecret(vmRef ref.Ref) (object *core.Secret, err error) {
	object = &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Namespace: r.namespace(),
			Name:      r.nameForSecret(vmRef),
			Labels: map[string]string{
				kMigration: string(r.Migration.UID),
				kPlan:      string(r.Plan.UID),
			},
		},
	}
	err = r.Builder.Secret(vmRef, r.Source.Secret, object)
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}

//
// Generated name for kubevirt VM Import CR secret.
func (r *KubeVirt) nameForSecret(vmRef ref.Ref) string {
	uid := string(r.Migration.UID)
	parts := []string{
		"plan",
		r.Plan.Name,
		vmRef.ID,
		uid[len(uid)-4:],
	}

	return strings.Join(parts, "-")
}

//
// Generated name for kubevirt VM Import CR.
func (r *KubeVirt) nameForImport(vmRef ref.Ref) string {
	uid := string(r.Migration.UID)
	parts := []string{
		"plan",
		r.Plan.Name,
		vmRef.ID,
		uid[len(uid)-4:],
	}

	return strings.Join(parts, "-")
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

//
// Get the target namespace.
// Default to `plan` namespace when not specified
// in the plan spec.
func (r *KubeVirt) namespace() (ns string) {
	ns = r.Plan.Spec.TargetNamespace
	if ns == "" {
		ns = r.Plan.Namespace
	}

	return
}

//
// Ensure resource.
// Resource is created/updated as needed.
func (r *KubeVirt) ensureObject(object runtime.Object) (err error) {
	retry := 3
	defer func() {
		err = liberr.Wrap(err)
	}()
	for {
		err = r.Destination.Client.Create(context.TODO(), object)
		if k8serr.IsAlreadyExists(err) && retry > 0 {
			retry--
			err = r.deleteObject(object)
			if err != nil {
				break
			}
		} else {
			break
		}
	}

	return
}

//
// Delete a resource.
func (r *KubeVirt) deleteObject(object runtime.Object) (err error) {
	err = r.Destination.Client.Delete(context.TODO(), object)
	if !k8serr.IsNotFound(err) {
		err = liberr.Wrap(err)
	} else {
		err = nil
	}

	return
}
