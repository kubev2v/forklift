package labeler

import (
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

func (r *Labeler) SetBlockingOwnerReference(scheme *runtime.Scheme, owner client.Object, object client.Object) (err error) {
	err = r.SetOwnerReference(scheme, owner, object, true, false)
	return
}

func (r *Labeler) SetOwnerReference(scheme *runtime.Scheme, owner client.Object, object client.Object, blockOwnerDeletion bool, isController bool) (err error) {
	if isController {
		err = controllerutil.SetControllerReference(
			owner, object, scheme,
			controllerutil.WithBlockOwnerDeletion(blockOwnerDeletion),
		)
	} else {
		err = controllerutil.SetOwnerReference(
			owner, object, scheme,
			controllerutil.WithBlockOwnerDeletion(blockOwnerDeletion),
		)
	}
	err = liberr.Wrap(err)
	return
}
