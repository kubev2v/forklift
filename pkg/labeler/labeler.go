package labeler

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Labeler struct{}

func (r *Labeler) SetLabel(object client.Object, key string, value string) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	object.SetLabels(labels)
}

func (r *Labeler) SetLabels(object client.Object, merge map[string]string) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	for key, value := range merge {
		labels[key] = value
	}
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

func (r *Labeler) SetAnnotations(object client.Object, merge map[string]string) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	for key, value := range merge {
		annotations[key] = value
	}
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

func (r *Labeler) SetBlockingOwnerReference(owner client.Object, object client.Object) {
	r.SetOwnerReference(owner, object, true, false)
}

func (r *Labeler) SetOwnerReference(owner client.Object, object client.Object, blockOwnerDeletion bool, isController bool) {
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
