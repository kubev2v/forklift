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
	plancontroller "github.com/kubev2v/forklift/pkg/controller/plan"
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

	providerNameLabel = "forklift.konveyor.io/target-provider-name"
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
		&api.Plan{},
		&handler.TypedEnqueueRequestForObject[*api.Plan]{},
		&plancontroller.PlanPredicate{})); err != nil {
		return err
	}
	if err = cnt.Watch(source.Kind(
		mgr.GetCache(),
		&api.Provider{},
		handler.TypedEnqueueRequestsFromMapFunc(reconciler.plansForProvider))); err != nil {
		return err
	}
	validation := &unstructured.Unstructured{}
	validation.SetGroupVersionKind(validationGVK)
	if err = cnt.Watch(source.Kind(
		mgr.GetCache(),
		validation,
		handler.TypedEnqueueRequestsFromMapFunc(reconciler.plansForValidation))); err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler creates and projects the namespaced VCV request for a destination
// OpenShift Provider. Its conditions are durable and advisory: they never
// change Forklift's existing readiness or migration eligibility semantics.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	plan := &api.Plan{}
	if err := r.Get(ctx, request.NamespacedName, plan); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if plan.Spec.Archived {
		return reconcile.Result{}, nil
	}

	destinationRef := plan.Spec.Provider.Destination
	if destinationRef.Name == "" {
		return reconcile.Result{}, r.clearPlanCondition(ctx, plan)
	}
	if destinationRef.Namespace == "" {
		destinationRef.Namespace = plan.Namespace
	}

	provider := &api.Provider{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: destinationRef.Namespace, Name: destinationRef.Name}, provider); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, r.setPlanCondition(ctx, plan, waitingCondition("DestinationProviderNotFound", "Waiting for the destination Provider to exist."))
		}
		return reconcile.Result{}, err
	}
	if provider.Type() != api.OpenShift || provider.IsHost() {
		return reconcile.Result{}, r.clearPlanCondition(ctx, plan)
	}
	if !provider.Status.HasCondition(libcnd.Ready) {
		return reconcile.Result{}, r.setPlanCondition(ctx, plan, waitingCondition("DestinationProviderNotReady", "Waiting for the destination Provider to become ready."))
	}

	validation, pending, err := r.ensureValidation(ctx, provider)
	if err != nil {
		condition := requestErrorCondition(err)
		if statusErr := r.setProviderCondition(ctx, provider, condition); statusErr != nil {
			return reconcile.Result{}, statusErr
		}
		if statusErr := r.setPlanCondition(ctx, plan, conditionForPlan(condition)); statusErr != nil {
			return reconcile.Result{}, statusErr
		}
		return reconcile.Result{}, err
	}
	if pending {
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}

	condition := conditionFor(validation)
	if err = r.setProviderCondition(ctx, provider, condition); err != nil {
		return reconcile.Result{}, err
	}
	if err = r.setPlanCondition(ctx, plan, conditionForPlan(condition)); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
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
		providerNameLabel: provider.Name,
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

func conditionFor(validation *unstructured.Unstructured) libcnd.Condition {
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

func conditionForPlan(providerCondition libcnd.Condition) libcnd.Condition {
	providerCondition.Type = api.ConditionDestinationVirtualizationValid
	return providerCondition
}

func waitingCondition(reason, message string) libcnd.Condition {
	return libcnd.Condition{
		Type:     api.ConditionDestinationVirtualizationValid,
		Status:   libcnd.False,
		Reason:   reason,
		Category: api.CategoryAdvisory,
		Message:  message,
		Durable:  true,
	}
}

func requestErrorCondition(err error) libcnd.Condition {
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

func (r *Reconciler) setProviderCondition(ctx context.Context, provider *api.Provider, condition libcnd.Condition) error {
	before := provider.DeepCopy()
	provider.Status.SetCondition(condition)
	if reflect.DeepEqual(before.Status, provider.Status) {
		return nil
	}
	return r.Status().Update(ctx, provider)
}

func (r *Reconciler) setPlanCondition(ctx context.Context, plan *api.Plan, condition libcnd.Condition) error {
	before := plan.DeepCopy()
	plan.Status.SetCondition(condition)
	if reflect.DeepEqual(before.Status, plan.Status) {
		return nil
	}
	return r.Status().Update(ctx, plan.DeepCopy())
}

func (r *Reconciler) clearPlanCondition(ctx context.Context, plan *api.Plan) error {
	before := plan.DeepCopy()
	plan.Status.DeleteCondition(api.ConditionDestinationVirtualizationValid)
	if reflect.DeepEqual(before.Status, plan.Status) {
		return nil
	}
	return r.Status().Update(ctx, plan.DeepCopy())
}

func (r *Reconciler) plansForProvider(ctx context.Context, provider *api.Provider) []reconcile.Request {
	return r.plansForDestination(ctx, provider.Namespace, provider.Name)
}

func (r *Reconciler) plansForValidation(ctx context.Context, validation *unstructured.Unstructured) []reconcile.Request {
	return r.plansForDestination(ctx, validation.GetNamespace(), validation.GetLabels()[providerNameLabel])
}

func (r *Reconciler) plansForDestination(ctx context.Context, namespace, name string) []reconcile.Request {
	if namespace == "" || name == "" {
		return nil
	}
	plans := &api.PlanList{}
	if err := r.List(ctx, plans); err != nil {
		log.Error(err, "Failed to list plans for destination validation", "provider", types.NamespacedName{Namespace: namespace, Name: name})
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for i := range plans.Items {
		plan := &plans.Items[i]
		destination := plan.Spec.Provider.Destination
		if destination.Namespace == "" {
			destination.Namespace = plan.Namespace
		}
		if destination.Namespace == namespace && destination.Name == name {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(plan)})
		}
	}
	return requests
}

func (r *Reconciler) setOwnerReference(provider *api.Provider, validation *unstructured.Unstructured) error {
	return controllerutil.SetControllerReference(provider, validation, r.Scheme)
}
