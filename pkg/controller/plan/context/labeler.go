package context

import (
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LabelMigration = "migration"
	LabelPlan      = "plan"
	LabelVM        = "vmID"
)

type Labeler struct {
	*Context
}

func (r *Labeler) LabelVM(vm *cnv.VirtualMachine) {
	if vm.Labels == nil {
		vm.Labels = make(map[string]string)
	}
	for k, v := range r.MigrationLabels() {
		vm.Labels[k] = v
	}
}

func (r *Labeler) LabelObject(vmRef ref.Ref, object client.Object) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range r.VMLabels(vmRef) {
		labels[k] = v
	}
	object.SetLabels(labels)
}

func (r *Labeler) SetLabel(object client.Object, key string, value string) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	object.SetLabels(labels)
}

func (r *Labeler) DeleteLabel(object client.Object, key string) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	delete(labels, key)
	object.SetLabels(labels)
}

func (r *Labeler) SetAnnotation(object client.Object, key string, value string) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	object.SetAnnotations(annotations)
}

func (r *Labeler) DeleteAnnotation(object client.Object, key string) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	delete(annotations, key)
	object.SetAnnotations(annotations)
}

func (r *Labeler) SetOwnerReferences(owner client.Object, object client.Object) {
	blockOwnerDeletion := true
	isController := false
	kind := owner.GetObjectKind().GroupVersionKind().Kind
	apiVersion := owner.GetObjectKind().GroupVersionKind().GroupVersion().String()
	reference := meta.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}

	references := object.GetOwnerReferences()
	references = append(references, reference)
	object.SetOwnerReferences(references)
}

func (r *Labeler) PlanLabels() map[string]string {
	return map[string]string{
		LabelPlan: string(r.Plan.GetUID()),
	}
}

func (r *Labeler) MigrationLabels() map[string]string {
	return map[string]string{
		LabelMigration: string(r.Migration.UID),
		LabelPlan:      string(r.Plan.GetUID()),
	}
}

func (r *Labeler) VMLabels(vmRef ref.Ref) map[string]string {
	labels := r.MigrationLabels()
	labels[LabelVM] = vmRef.ID
	return labels
}
