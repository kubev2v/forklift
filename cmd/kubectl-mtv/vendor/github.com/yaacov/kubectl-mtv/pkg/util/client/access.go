package client

import (
	"context"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// CanAccessResource checks if the user has permissions to perform the specified verb on the given resource in the namespace
func CanAccessResource(ctx context.Context, configFlags *genericclioptions.ConfigFlags, namespace string, gvr schema.GroupVersionResource, verb string) bool {
	// Get clientset
	clientset, err := GetKubernetesClientset(configFlags)
	if err != nil {
		return false
	}

	// Create a SelfSubjectAccessReview to check if the user can access the resource
	accessReview := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     gvr.Group,
				Resource:  gvr.Resource,
			},
		},
	}

	// Submit the access review
	result, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(
		ctx,
		accessReview,
		metav1.CreateOptions{},
	)

	if err != nil {
		return false
	}

	return result.Status.Allowed
}
