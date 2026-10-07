/*
Copyright 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package virtualizationvalidation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	Name = "virtualizationvalidation"

	// ProviderNameLabel identifies the Forklift Provider for a validation request.
	ProviderNameLabel = "forklift.konveyor.io/target-provider-name"
	providerUIDLabel  = "forklift.konveyor.io/target-provider-uid"

	validationProfile          = "basic-v1"
	validationNamespace        = "mtv-validation"
	validationDataSourceNS     = "openshift-virtualization-os-images"
	validationDataSourceName   = "rhel10"
	validationSuiteTimeout     = int64(900)
	validationExecutionTimeout = int64(300)
)

var (
	log = logging.WithName(Name)

	validationGVK = schema.GroupVersionKind{
		Group:   "validation.kubevirt.io",
		Version: "v1alpha1",
		Kind:    "VirtualizationValidation",
	}
)

// Add registers the Forklift integration only on OpenShift. The standalone VCV
// controller and CRD are not installed on generic Kubernetes deployments.
func Add(mgr manager.Manager) error {
	if !settings.Settings.OpenShift {
		return nil
	}

	reconciler := &Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	cnt, err := controller.New(Name, mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return err
	}

	if err = cnt.Watch(source.Kind(
		mgr.GetCache(),
		&api.Provider{},
		&handler.TypedEnqueueRequestForObject[*api.Provider]{})); err != nil {
		return err
	}
	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler creates the namespaced VCV request for a ready OpenShift
// Provider. Provider and Plan reconcilers project the advisory result onto
// their own status objects.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	provider := &api.Provider{}
	if err := r.Get(ctx, request.NamespacedName, provider); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if provider.Type() != api.OpenShift || provider.IsHost() {
		return reconcile.Result{}, nil
	}
	if !provider.Status.HasCondition(libcnd.Ready) {
		return reconcile.Result{}, nil
	}

	_, pending, err := r.ensureValidation(ctx, provider)
	if err != nil {
		return reconcile.Result{}, err
	}
	if pending {
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	return reconcile.Result{}, nil
}

// NewValidation returns an unstructured VCV object suitable for a dynamic
// watch or read without coupling Forklift to the VCV Go module.
func NewValidation() *unstructured.Unstructured {
	validation := &unstructured.Unstructured{}
	validation.SetGroupVersionKind(validationGVK)
	return validation
}

// ProviderCondition reads the validation result belonging to provider. A nil
// condition means that validation has not been requested for this provider.
func ProviderCondition(ctx context.Context, reader client.Reader, provider *api.Provider) (*libcnd.Condition, error) {
	if provider.Type() != api.OpenShift || provider.IsHost() {
		return nil, nil
	}
	validation := NewValidation()
	if err := reader.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: validationName(provider)}, validation); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	condition := ConditionFor(validation)
	return &condition, nil
}

// PlanCondition derives the advisory validation condition for plan. It is
// intentionally independent of Provider status so the Plan reconciler owns
// every write to Plan status.
func PlanCondition(ctx context.Context, reader client.Reader, plan *api.Plan) (*libcnd.Condition, error) {
	if plan.Spec.Archived {
		return nil, nil
	}
	destination := plan.Spec.Provider.Destination
	if destination.Name == "" {
		return nil, nil
	}
	if destination.Namespace == "" {
		destination.Namespace = plan.Namespace
	}
	provider := &api.Provider{}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: destination.Namespace, Name: destination.Name}, provider); err != nil {
		if apierrors.IsNotFound(err) {
			condition := WaitingCondition("DestinationProviderNotFound", "Waiting for the destination Provider to exist.")
			return &condition, nil
		}
		return nil, err
	}
	if provider.Type() != api.OpenShift || provider.IsHost() {
		return nil, nil
	}
	if !provider.Status.HasCondition(libcnd.Ready) {
		condition := WaitingCondition("DestinationProviderNotReady", "Waiting for the destination Provider to become ready.")
		return &condition, nil
	}
	providerCondition, err := ProviderCondition(ctx, reader, provider)
	if err != nil {
		condition := ConditionForPlan(RequestErrorCondition(err))
		return &condition, nil
	}
	if providerCondition == nil {
		condition := WaitingCondition("Pending", "Waiting for the target virtualization validation to start.")
		return &condition, nil
	}
	condition := ConditionForPlan(*providerCondition)
	return &condition, nil
}

func (r *Reconciler) ensureValidation(ctx context.Context, provider *api.Provider) (*unstructured.Unstructured, bool, error) {
	secretNamespace := provider.Spec.Secret.Namespace
	if secretNamespace == "" {
		secretNamespace = provider.Namespace
	}
	if secretNamespace != provider.Namespace {
		return nil, false, fmt.Errorf("destination Provider Secret %s/%s must be in Provider namespace %s", secretNamespace, provider.Spec.Secret.Name, provider.Namespace)
	}

	secret := &core.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: secretNamespace, Name: provider.Spec.Secret.Name}, secret); err != nil {
		return nil, false, err
	}
	desired, err := desiredValidation(provider, secret)
	if err != nil {
		return nil, false, err
	}

	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(validationGVK)
	if err = r.Get(ctx, client.ObjectKeyFromObject(desired), current); err != nil {
		if apierrors.IsNotFound(err) {
			if err = r.setOwnerReference(provider, desired); err != nil {
				return nil, false, err
			}
			if err = r.Create(ctx, desired); err != nil {
				return nil, false, err
			}
			return desired, false, nil
		}
		return nil, false, err
	}

	if !sameImmutableSpec(current, desired) {
		if current.GetDeletionTimestamp().IsZero() {
			if err = r.Delete(ctx, current); err != nil {
				return nil, false, err
			}
		}
		return nil, true, nil
	}

	desiredRunID, _, _ := unstructured.NestedString(desired.Object, "spec", "run", "id")
	currentRunID, _, _ := unstructured.NestedString(current.Object, "spec", "run", "id")
	if currentRunID != desiredRunID {
		original := current.DeepCopy()
		if err = unstructured.SetNestedField(current.Object, desiredRunID, "spec", "run", "id"); err != nil {
			return nil, false, err
		}
		if err = r.Patch(ctx, current, client.MergeFrom(original)); err != nil {
			return nil, false, err
		}
	}

	return current, false, nil
}

func desiredValidation(provider *api.Provider, secret *core.Secret) (*unstructured.Unstructured, error) {
	if provider.Spec.URL == "" {
		return nil, fmt.Errorf("destination Provider %s/%s has no URL", provider.Namespace, provider.Name)
	}
	if provider.Spec.Secret.Name == "" {
		return nil, fmt.Errorf("destination Provider %s/%s has no credential Secret", provider.Namespace, provider.Name)
	}

	caKey := ""
	if _, found := secret.Data["ca.crt"]; found {
		caKey = "ca.crt"
	} else if _, found := secret.Data["cacert"]; found {
		caKey = "cacert"
	}
	insecureSkipTLS, _ := strconv.ParseBool(string(secret.Data[api.Insecure]))
	labels := map[string]string{
		ProviderNameLabel: provider.Name,
		providerUIDLabel:  string(provider.UID),
	}
	validation := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": validationGVK.GroupVersion().String(),
		"kind":       validationGVK.Kind,
		"metadata": map[string]any{
			"name":      validationName(provider),
			"namespace": provider.Namespace,
			"labels":    labels,
		},
		"spec": map[string]any{
			"target": map[string]any{
				"url":             provider.Spec.URL,
				"insecureSkipTLS": insecureSkipTLS,
				"credentialsSecret": map[string]any{
					"name":     provider.Spec.Secret.Name,
					"tokenKey": api.Token,
					"caKey":    caKey,
				},
				"identity": map[string]any{
					"id":          string(provider.UID),
					"displayName": provider.Name,
				},
			},
			"workload": map[string]any{
				"namespace": validationNamespace,
				"dataSource": map[string]any{
					"apiGroup":  "cdi.kubevirt.io",
					"kind":      "DataSource",
					"namespace": validationDataSourceNS,
					"name":      validationDataSourceName,
				},
			},
			"run": map[string]any{
				"id":                      validationRunID(provider, secret),
				"profile":                 validationProfile,
				"suiteTimeoutSeconds":     validationSuiteTimeout,
				"executionTimeoutSeconds": validationExecutionTimeout,
				"queueTimeoutSeconds":     validationSuiteTimeout,
				"cleanupPolicy":           "Always",
			},
		},
	}}
	validation.SetGroupVersionKind(validationGVK)
	return validation, nil
}

func validationName(provider *api.Provider) string {
	if provider.UID != "" {
		return "vv-" + string(provider.UID)
	}
	sum := sha256.Sum256([]byte(provider.Namespace + "/" + provider.Name))
	return "vv-" + hex.EncodeToString(sum[:8])
}

func validationRunID(provider *api.Provider, secret *core.Secret) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%s", provider.UID, provider.Generation, secret.ResourceVersion)))
	return "run-" + hex.EncodeToString(sum[:8])
}

func sameImmutableSpec(current, desired *unstructured.Unstructured) bool {
	for _, path := range [][]string{{"spec", "target"}, {"spec", "workload"}} {
		currentValue, _, _ := unstructured.NestedFieldCopy(current.Object, path...)
		desiredValue, _, _ := unstructured.NestedFieldCopy(desired.Object, path...)
		if !reflect.DeepEqual(currentValue, desiredValue) {
			return false
		}
	}
	for _, field := range []string{"profile", "suiteTimeoutSeconds", "executionTimeoutSeconds", "queueTimeoutSeconds", "cleanupPolicy", "maxConcurrentExecutions"} {
		currentValue, _, _ := unstructured.NestedFieldCopy(current.Object, "spec", "run", field)
		desiredValue, _, _ := unstructured.NestedFieldCopy(desired.Object, "spec", "run", field)
		if !reflect.DeepEqual(currentValue, desiredValue) {
			return false
		}
	}
	return true
}

// ConditionFor returns the advisory Provider condition represented by a VCV
// result. It is exported so the Provider reconciler remains the sole writer
// of Provider status.
func ConditionFor(validation *unstructured.Unstructured) libcnd.Condition {
	phase, _, _ := unstructured.NestedString(validation.Object, "status", "phase")
	verdict, _, _ := unstructured.NestedString(validation.Object, "status", "verdict")
	condition := libcnd.Condition{
		Type:     api.ConditionTargetVirtualizationValid,
		Category: api.CategoryAdvisory,
		Durable:  true,
		Items:    []string{validation.GetNamespace() + "/" + validation.GetName()},
	}
	switch verdict {
	case "Valid":
		condition.Status = libcnd.True
		condition.Reason = "Valid"
		condition.Message = "The target virtualization validation completed successfully."
	case "ValidWithWarnings":
		condition.Status = libcnd.True
		condition.Reason = "ValidWithWarnings"
		condition.Message = "The target virtualization validation completed with advisory warnings."
	case "Invalid":
		condition.Status = libcnd.False
		condition.Reason = "Invalid"
		condition.Message = "The target virtualization validation found required check failures."
	case "Inconclusive":
		condition.Status = libcnd.False
		condition.Reason = "Inconclusive"
		condition.Message = "The target virtualization validation did not produce a conclusive result."
	default:
		condition.Status = libcnd.False
		condition.Reason = "Pending"
		condition.Message = "Waiting for the target virtualization validation to start."
		if phase != "" {
			condition.Reason = phase
			condition.Message = fmt.Sprintf("The target virtualization validation is %s.", phase)
		}
	}
	return condition
}

// ConditionForPlan translates a Provider validation condition for a Plan.
func ConditionForPlan(providerCondition libcnd.Condition) libcnd.Condition {
	providerCondition.Type = api.ConditionDestinationVirtualizationValid
	return providerCondition
}

// WaitingCondition describes a validation that cannot start yet.
func WaitingCondition(reason, message string) libcnd.Condition {
	return libcnd.Condition{
		Type:     api.ConditionDestinationVirtualizationValid,
		Status:   libcnd.False,
		Reason:   reason,
		Category: api.CategoryAdvisory,
		Message:  message,
		Durable:  true,
	}
}

// RequestErrorCondition describes a failure to read or create validation input.
func RequestErrorCondition(err error) libcnd.Condition {
	return libcnd.Condition{
		Type:       api.ConditionTargetVirtualizationValid,
		Status:     libcnd.False,
		Reason:     "ValidationRequestError",
		Category:   api.CategoryAdvisory,
		Message:    fmt.Sprintf("Could not create or update the target virtualization validation: %v", err),
		Durable:    true,
		Suggestion: "Verify the destination Provider credentials and VirtualizationValidation controller deployment.",
	}
}

func (r *Reconciler) setOwnerReference(provider *api.Provider, validation *unstructured.Unstructured) error {
	return controllerutil.SetControllerReference(provider, validation, r.Scheme)
}
