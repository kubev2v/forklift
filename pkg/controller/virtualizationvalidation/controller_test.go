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
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	providerapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDesiredValidationUsesProviderCredentialsAndLegacyCA(t *testing.T) {
	openshift := api.OpenShift
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target", UID: types.UID("provider-uid"), Generation: 4},
		Spec: api.ProviderSpec{
			Type:   &openshift,
			URL:    "https://api.target.example:6443",
			Secret: core.ObjectReference{Name: "target-secret"},
		},
	}
	secret := &core.Secret{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "12"}, Data: map[string][]byte{
		"token":              []byte("token"),
		"cacert":             []byte("ca"),
		"insecureSkipVerify": []byte("true"),
	}}

	validation, err := desiredValidation(provider, secret)
	if err != nil {
		t.Fatalf("desiredValidation() error = %v", err)
	}
	if validation.GetName() != "vv-provider-uid" {
		t.Fatalf("validation name = %q", validation.GetName())
	}
	caKey, _, _ := unstructured.NestedString(validation.Object, "spec", "target", "credentialsSecret", "caKey")
	if caKey != "cacert" {
		t.Fatalf("caKey = %q, want cacert", caKey)
	}
	insecure, _, _ := unstructured.NestedBool(validation.Object, "spec", "target", "insecureSkipTLS")
	if !insecure {
		t.Fatal("insecureSkipTLS = false, want true")
	}
	profile, _, _ := unstructured.NestedString(validation.Object, "spec", "run", "profile")
	if profile != validationProfile {
		t.Fatalf("profile = %q, want %q", profile, validationProfile)
	}
}

func TestConditionForMapsVCVVerdictsAsAdvisory(t *testing.T) {
	validation := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{"phase": "Completed", "verdict": "Invalid"},
	}}
	validation.SetNamespace("mtv")
	validation.SetName("vv-target")

	condition := conditionFor(validation)
	if condition.Type != api.ConditionTargetVirtualizationValid || condition.Status != libcnd.False || condition.Reason != "Invalid" || condition.Category != api.CategoryAdvisory || !condition.Durable {
		t.Fatalf("unexpected provider condition: %#v", condition)
	}
	planCondition := conditionForPlan(condition)
	if planCondition.Type != api.ConditionDestinationVirtualizationValid || planCondition.Reason != "Invalid" {
		t.Fatalf("unexpected plan condition: %#v", planCondition)
	}
}

func TestRequestErrorConditionIsAdvisoryAndInconclusive(t *testing.T) {
	condition := requestErrorCondition(assertionError{})
	if condition.Type != api.ConditionTargetVirtualizationValid || condition.Status != libcnd.False || condition.Reason != "ValidationRequestError" || condition.Category != api.CategoryAdvisory || !condition.Durable {
		t.Fatalf("unexpected request error condition: %#v", condition)
	}
}

type assertionError struct{}

func (assertionError) Error() string {
	return "test error"
}

func TestEnsureValidationCreatesProviderOwnedRequest(t *testing.T) {
	openshift := api.OpenShift
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target", UID: types.UID("provider-uid"), Generation: 1},
		Spec: api.ProviderSpec{
			Type:   &openshift,
			URL:    "https://api.target.example:6443",
			Secret: core.ObjectReference{Name: "target-secret"},
		},
	}
	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target-secret", ResourceVersion: "1"},
		Data:       map[string][]byte{"token": []byte("token"), "ca.crt": []byte("ca")},
	}
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(validationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(validationGVK.GroupVersion().WithKind("VirtualizationValidationList"), &unstructured.UnstructuredList{})
	reconciler := &Reconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build(), Scheme: scheme}

	validation, pending, err := reconciler.ensureValidation(context.Background(), provider)
	if err != nil {
		t.Fatalf("ensureValidation() error = %v", err)
	}
	if pending {
		t.Fatal("ensureValidation() pending = true, want false")
	}
	if len(validation.GetOwnerReferences()) != 1 || validation.GetOwnerReferences()[0].UID != provider.UID {
		t.Fatalf("owner references = %#v", validation.GetOwnerReferences())
	}
	fetched := &unstructured.Unstructured{}
	fetched.SetGroupVersionKind(validationGVK)
	if err = reconciler.Get(context.Background(), types.NamespacedName{Namespace: "mtv", Name: validationName(provider)}, fetched); err != nil {
		t.Fatalf("validation request was not created: %v", err)
	}

	changed := fetched.DeepCopy()
	if err = unstructured.SetNestedField(changed.Object, "another-run", "spec", "run", "id"); err != nil {
		t.Fatal(err)
	}
	if !sameImmutableSpec(fetched, changed) {
		t.Fatal("run.id change must not replace the validation request")
	}
	if err = unstructured.SetNestedField(changed.Object, "https://api.other.example:6443", "spec", "target", "url"); err != nil {
		t.Fatal(err)
	}
	if sameImmutableSpec(fetched, changed) {
		t.Fatal("target change must replace the validation request")
	}
}

func TestEnsureValidationRerunsForCredentialChangesAndReplacesImmutableInput(t *testing.T) {
	ctx := context.Background()
	openshift := api.OpenShift
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target", UID: types.UID("provider-uid"), Generation: 1},
		Spec: api.ProviderSpec{
			Type:   &openshift,
			URL:    "https://api.target.example:6443",
			Secret: core.ObjectReference{Name: "target-secret"},
		},
	}
	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target-secret", ResourceVersion: "1"},
		Data:       map[string][]byte{"token": []byte("token"), "ca.crt": []byte("ca")},
	}
	scheme := testScheme(t)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	reconciler := &Reconciler{Client: client, Scheme: scheme}

	validation, pending, err := reconciler.ensureValidation(ctx, provider)
	if err != nil || pending {
		t.Fatalf("initial ensureValidation() = (%v, %v), want created request", pending, err)
	}
	initialRunID, _, _ := unstructured.NestedString(validation.Object, "spec", "run", "id")

	updatedSecret := &core.Secret{}
	if err = client.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), updatedSecret); err != nil {
		t.Fatal(err)
	}
	updatedSecret.Data["token"] = []byte("rotated-token")
	if err = client.Update(ctx, updatedSecret); err != nil {
		t.Fatal(err)
	}
	validation, pending, err = reconciler.ensureValidation(ctx, provider)
	if err != nil || pending {
		t.Fatalf("credential-change ensureValidation() = (%v, %v), want run-id update", pending, err)
	}
	updatedRunID, _, _ := unstructured.NestedString(validation.Object, "spec", "run", "id")
	if updatedRunID == initialRunID {
		t.Fatalf("run.id = %q after credential change, want a new run id", updatedRunID)
	}

	changedProvider := provider.DeepCopy()
	changedProvider.Spec.URL = "https://api.replaced.example:6443"
	_, pending, err = reconciler.ensureValidation(ctx, changedProvider)
	if err != nil || !pending {
		t.Fatalf("immutable-change ensureValidation() = (%v, %v), want replacement pending", pending, err)
	}
	validation, pending, err = reconciler.ensureValidation(ctx, changedProvider)
	if err != nil || pending {
		t.Fatalf("replacement ensureValidation() = (%v, %v), want new request", pending, err)
	}
	url, _, _ := unstructured.NestedString(validation.Object, "spec", "target", "url")
	if url != changedProvider.Spec.URL {
		t.Fatalf("replacement URL = %q, want %q", url, changedProvider.Spec.URL)
	}
}

func TestReconcileProjectsValidationOntoDestinationProviderAndPlan(t *testing.T) {
	ctx := context.Background()
	openshift := api.OpenShift
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target", UID: types.UID("provider-uid"), Generation: 1},
		Spec: api.ProviderSpec{
			Type:   &openshift,
			URL:    "https://api.target.example:6443",
			Secret: core.ObjectReference{Name: "target-secret"},
		},
	}
	provider.Status.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True, Category: api.CategoryRequired})
	plan := &api.Plan{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "migration-plan"},
		Spec: api.PlanSpec{Provider: providerapi.Pair{
			Destination: core.ObjectReference{Namespace: "mtv", Name: "target"},
		}},
	}
	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target-secret", ResourceVersion: "1"},
		Data:       map[string][]byte{"token": []byte("token"), "ca.crt": []byte("ca")},
	}
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(validationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(validationGVK.GroupVersion().WithKind("VirtualizationValidationList"), &unstructured.UnstructuredList{})
	client := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&api.Provider{}, &api.Plan{}).
		WithObjects(provider, plan, secret).
		Build()
	reconciler := &Reconciler{Client: client, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "mtv", Name: "migration-plan"}}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	updatedProvider := &api.Provider{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: "mtv", Name: "target"}, updatedProvider); err != nil {
		t.Fatal(err)
	}
	providerCondition := updatedProvider.Status.FindCondition(api.ConditionTargetVirtualizationValid)
	if providerCondition == nil || providerCondition.Reason != "Pending" || !providerCondition.Durable || providerCondition.Category != api.CategoryAdvisory {
		t.Fatalf("unexpected Provider condition: %#v", providerCondition)
	}
	updatedPlan := &api.Plan{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: "mtv", Name: "migration-plan"}, updatedPlan); err != nil {
		t.Fatal(err)
	}
	planCondition := updatedPlan.Status.FindCondition(api.ConditionDestinationVirtualizationValid)
	if planCondition == nil || planCondition.Reason != "Pending" || !planCondition.Durable || planCondition.Category != api.CategoryAdvisory {
		t.Fatalf("unexpected Plan condition: %#v", planCondition)
	}
	validation := &unstructured.Unstructured{}
	validation.SetGroupVersionKind(validationGVK)
	if err := client.Get(ctx, types.NamespacedName{Namespace: "mtv", Name: validationName(provider)}, validation); err != nil {
		t.Fatalf("validation request was not created: %v", err)
	}
}

func TestReconcileProjectsVCVVerdictsOntoProviderAndPlan(t *testing.T) {
	ctx := context.Background()
	reconciler, client, provider, plan := readyReconciler(t)
	request := reconcile.Request{NamespacedName: ctrlclient.ObjectKeyFromObject(plan)}
	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("initial Reconcile() error = %v", err)
	}

	for _, test := range []struct {
		verdict string
		status  string
	}{
		{verdict: "Valid", status: libcnd.True},
		{verdict: "ValidWithWarnings", status: libcnd.True},
		{verdict: "Invalid", status: libcnd.False},
		{verdict: "Inconclusive", status: libcnd.False},
	} {
		validation := &unstructured.Unstructured{}
		validation.SetGroupVersionKind(validationGVK)
		if err := client.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: validationName(provider)}, validation); err != nil {
			t.Fatal(err)
		}
		validation.Object["status"] = map[string]any{"phase": "Completed", "verdict": test.verdict}
		if err := client.Update(ctx, validation); err != nil {
			t.Fatal(err)
		}
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatalf("Reconcile() for %s error = %v", test.verdict, err)
		}

		updatedProvider := &api.Provider{}
		if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(provider), updatedProvider); err != nil {
			t.Fatal(err)
		}
		providerCondition := updatedProvider.Status.FindCondition(api.ConditionTargetVirtualizationValid)
		if providerCondition == nil || providerCondition.Status != test.status || providerCondition.Reason != test.verdict {
			t.Fatalf("Provider condition after %s = %#v", test.verdict, providerCondition)
		}
		updatedPlan := &api.Plan{}
		if err := client.Get(ctx, ctrlclient.ObjectKeyFromObject(plan), updatedPlan); err != nil {
			t.Fatal(err)
		}
		planCondition := updatedPlan.Status.FindCondition(api.ConditionDestinationVirtualizationValid)
		if planCondition == nil || planCondition.Status != test.status || planCondition.Reason != test.verdict {
			t.Fatalf("Plan condition after %s = %#v", test.verdict, planCondition)
		}
	}
}

func TestValidationEventFansOutToAllPlansForDestinationProvider(t *testing.T) {
	ctx := context.Background()
	reconciler, client, provider, plan := readyReconciler(t)
	secondPlan := plan.DeepCopy()
	secondPlan.Name = "second-migration-plan"
	secondPlan.ResourceVersion = ""
	secondPlan.UID = ""
	secondPlan.CreationTimestamp = metav1.Time{}
	if err := client.Create(ctx, secondPlan); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: ctrlclient.ObjectKeyFromObject(plan)}); err != nil {
		t.Fatal(err)
	}
	validation := &unstructured.Unstructured{}
	validation.SetGroupVersionKind(validationGVK)
	if err := client.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: validationName(provider)}, validation); err != nil {
		t.Fatal(err)
	}
	requests := reconciler.plansForValidation(ctx, validation)
	if len(requests) != 2 {
		t.Fatalf("validation fan-out requests = %#v, want both plans", requests)
	}
	validation.Object["status"] = map[string]any{"phase": "Completed", "verdict": "Valid"}
	if err := client.Update(ctx, validation); err != nil {
		t.Fatal(err)
	}
	for _, request := range requests {
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatalf("Reconcile(%s) error = %v", request.NamespacedName, err)
		}
	}
	for _, name := range []string{plan.Name, secondPlan.Name} {
		updatedPlan := &api.Plan{}
		if err := client.Get(ctx, types.NamespacedName{Namespace: plan.Namespace, Name: name}, updatedPlan); err != nil {
			t.Fatal(err)
		}
		condition := updatedPlan.Status.FindCondition(api.ConditionDestinationVirtualizationValid)
		if condition == nil || condition.Status != libcnd.True || condition.Reason != "Valid" {
			t.Fatalf("Plan %s condition = %#v", name, condition)
		}
	}
}

func readyReconciler(t *testing.T) (*Reconciler, ctrlclient.Client, *api.Provider, *api.Plan) {
	t.Helper()
	openshift := api.OpenShift
	provider := &api.Provider{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target", UID: types.UID("provider-uid"), Generation: 1},
		Spec: api.ProviderSpec{
			Type:   &openshift,
			URL:    "https://api.target.example:6443",
			Secret: core.ObjectReference{Name: "target-secret"},
		},
	}
	provider.Status.SetCondition(libcnd.Condition{Type: libcnd.Ready, Status: libcnd.True, Category: api.CategoryRequired})
	plan := &api.Plan{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "migration-plan"},
		Spec: api.PlanSpec{Provider: providerapi.Pair{
			Destination: core.ObjectReference{Namespace: "mtv", Name: "target"},
		}},
	}
	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "mtv", Name: "target-secret", ResourceVersion: "1"},
		Data:       map[string][]byte{"token": []byte("token"), "ca.crt": []byte("ca")},
	}
	scheme := testScheme(t)
	client := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&api.Provider{}, &api.Plan{}).
		WithObjects(provider, plan, secret).
		Build()
	return &Reconciler{Client: client, Scheme: scheme}, client, provider, plan
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	scheme.AddKnownTypeWithName(validationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(validationGVK.GroupVersion().WithKind("VirtualizationValidationList"), &unstructured.UnstructuredList{})
	return scheme
}
