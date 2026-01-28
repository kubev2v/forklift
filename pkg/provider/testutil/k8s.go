// Package testutil provides shared test utilities for provider unit tests.
// It includes helpers for creating fake Kubernetes clients, test fixtures,
// and common test setup patterns used across different provider implementations.
package testutil

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// NewScheme creates a runtime.Scheme with all the types needed for provider tests.
// Includes core K8s types (core, apps, rbac) and Forklift CRDs (v1beta1).
func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(core.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(api.SchemeBuilder.AddToScheme(scheme))
	return scheme
}

// NewFakeClient creates a fake controller-runtime client with the standard scheme
// and the provided runtime objects pre-populated.
// For more advanced configuration (indexes, etc.), use fake.NewClientBuilder()
// directly with NewScheme().
func NewFakeClient(objs ...runtime.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(NewScheme()).
		WithRuntimeObjects(objs...).
		Build()
}
