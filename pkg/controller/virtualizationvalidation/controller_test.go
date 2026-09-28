package virtualizationvalidation

import (
	"context"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	batch "k8s.io/api/batch/v1"
	coordination "k8s.io/api/coordination/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const validationTestNamespace = "mtv-validation"

func validationTestReconciler(t *testing.T, objects ...runtime.Object) (Reconciler, *runtime.Scheme) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{api.SchemeBuilder.AddToScheme, core.AddToScheme, batch.AddToScheme, coordination.AddToScheme, rbac.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	client := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&api.VirtualizationValidation{}, &api.Provider{}, &batch.Job{}).
		WithRuntimeObjects(objects...).Build()
	return Reconciler{Reconciler: base.Reconciler{Client: client}, Scheme: scheme}, scheme
}

func validationObjects() (*api.VirtualizationValidation, *api.Provider, *core.Secret) {
	providerType := api.OpenShift
	validation := &api.VirtualizationValidation{ObjectMeta: meta.ObjectMeta{
		Name: "validation", Namespace: validationTestNamespace, UID: types.UID("validation-uid"),
	}, Spec: api.VirtualizationValidationSpec{Profile: api.VirtualizationValidationProviderProfile, ProviderRef: core.ObjectReference{Name: "provider"}, Checks: []string{"platform"}}}
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{Name: "provider", Namespace: validationTestNamespace, UID: types.UID("provider-uid")}, Spec: api.ProviderSpec{
		Type: &providerType, URL: "https://api.example.test:6443", Secret: core.ObjectReference{Name: "provider-credentials"},
	}}
	secret := &core.Secret{ObjectMeta: meta.ObjectMeta{Name: "provider-credentials", Namespace: validationTestNamespace}, Data: map[string][]byte{api.Token: []byte("token")}}
	return validation, provider, secret
}

func completeCTRFReport() string {
	return `{"reportFormat":"CTRF","specVersion":"0.0.0","results":{"summary":{"tests":1,"passed":1,"failed":0,"pending":0,"skipped":0},"tests":[{"name":"platform","status":"passed","duration":1}]}}`
}

func TestReconcileCreatesResourcesProjectsProviderStatusAndCompletes(t *testing.T) {
	t.Setenv(ValidationImageEnv, "example.test/validator@sha256:1")
	validation, provider, secret := validationObjects()
	r, _ := validationTestReconciler(t, validation, provider, secret)
	request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: validation.Namespace, Name: validation.Name}}
	ctx := context.Background()
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatalf("add finalizer: %v", err)
	}
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatalf("create validation resources: %v", err)
	}

	current := &api.VirtualizationValidation{}
	if err := r.Get(ctx, request.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	if current.Status.Phase != api.VirtualizationValidationPending || current.Status.JobRef == nil || current.Status.ResultRef == nil {
		t.Fatalf("unexpected pending status: %#v", current.Status)
	}
	lease := &coordination.Lease{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: providerLeaseName(current)}, lease); err != nil {
		t.Fatalf("expected provider validation Lease: %v", err)
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != string(current.UID) {
		t.Fatalf("unexpected Lease holder: %#v", lease.Spec)
	}
	name := current.Status.JobRef.Name
	for _, object := range []core.ObjectReference{
		{Kind: "Secret", Namespace: validation.Namespace, Name: name + "-credentials"},
		{Kind: "ServiceAccount", Namespace: validation.Namespace, Name: name},
		{Kind: "Role", Namespace: validation.Namespace, Name: name},
		{Kind: "RoleBinding", Namespace: validation.Namespace, Name: name},
		{Kind: "ConfigMap", Namespace: validation.Namespace, Name: name},
		{Kind: "Job", Namespace: validation.Namespace, Name: name},
	} {
		if err := r.Get(ctx, types.NamespacedName{Namespace: object.Namespace, Name: object.Name}, objectForReference(object)); err != nil {
			t.Fatalf("expected owned %s %q: %v", object.Kind, object.Name, err)
		}
	}
	currentProvider := &api.Provider{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: provider.Name}, currentProvider); err != nil {
		t.Fatal(err)
	}
	if currentProvider.Status.FindCondition(ProviderInProgress) == nil {
		t.Fatalf("provider did not receive in-progress condition: %#v", currentProvider.Status.Conditions)
	}

	cm := &core.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: name}, cm); err != nil {
		t.Fatal(err)
	}
	cm.Data[ResultConfigMapKey] = completeCTRFReport()
	if err := r.Update(ctx, cm); err != nil {
		t.Fatal(err)
	}
	job := &batch.Job{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: name}, job); err != nil {
		t.Fatal(err)
	}
	job.Status.Conditions = []batch.JobCondition{{Type: batch.JobComplete, Status: core.ConditionTrue}}
	if err := r.Status().Update(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatalf("complete validation: %v", err)
	}
	if err := r.Get(ctx, request.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	if current.Status.Phase != api.VirtualizationValidationSucceeded || current.Status.Summary.Passed != 1 {
		t.Fatalf("unexpected completed status: %#v", current.Status)
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: provider.Name}, currentProvider); err != nil {
		t.Fatal(err)
	}
	if currentProvider.Status.FindCondition(ProviderSucceeded) == nil || currentProvider.Status.FindCondition(ProviderInProgress) != nil {
		t.Fatalf("provider status was not projected: %#v", currentProvider.Status.Conditions)
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: providerLeaseName(current)}, lease); err == nil {
		t.Fatal("provider validation Lease was not released")
	}
}

func TestReconcileRejectsConcurrentProviderValidation(t *testing.T) {
	t.Setenv(ValidationImageEnv, "example.test/validator@sha256:1")
	first, provider, secret := validationObjects()
	second := first.DeepCopy()
	second.Name = "validation-second"
	second.UID = types.UID("second-validation-uid")
	r, _ := validationTestReconciler(t, first, second, provider, secret)
	ctx := context.Background()
	firstRequest := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: first.Namespace, Name: first.Name}}
	secondRequest := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: second.Namespace, Name: second.Name}}
	for _, request := range []reconcile.Request{firstRequest, firstRequest, secondRequest, secondRequest} {
		if _, err := r.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
	}

	currentSecond := &api.VirtualizationValidation{}
	if err := r.Get(ctx, secondRequest.NamespacedName, currentSecond); err != nil {
		t.Fatal(err)
	}
	condition := currentSecond.Status.FindCondition(api.ConditionFailed)
	if currentSecond.Status.Phase != api.VirtualizationValidationFailed || condition == nil || condition.Reason != "ValidationAlreadyRunning" {
		t.Fatalf("concurrent validation was not rejected: %#v", currentSecond.Status)
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: second.Namespace, Name: resourceName(currentSecond, currentSecond.Status.ObservedInputHash)}, &batch.Job{}); err == nil {
		t.Fatal("rejected validation created a Job")
	}
	currentProvider := &api.Provider{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: provider.Name}, currentProvider); err != nil {
		t.Fatal(err)
	}
	condition = currentProvider.Status.FindCondition(ProviderInProgress)
	if condition == nil || condition.Reason != validationConditionReason(first) {
		t.Fatalf("concurrent validation replaced Provider status: %#v", currentProvider.Status.Conditions)
	}
}

func TestReconcileInvalidReportIsTerminalAndCleansSupersededResources(t *testing.T) {
	t.Setenv(ValidationImageEnv, "example.test/validator@sha256:1")
	validation, provider, secret := validationObjects()
	r, _ := validationTestReconciler(t, validation, provider, secret)
	request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: validation.Namespace, Name: validation.Name}}
	ctx := context.Background()
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatal(err)
	}
	current := &api.VirtualizationValidation{}
	if err := r.Get(ctx, request.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	oldName := current.Status.JobRef.Name
	cm := &core.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: validation.Namespace, Name: oldName}, cm); err != nil {
		t.Fatal(err)
	}
	cm.Data[ResultConfigMapKey] = "not JSON"
	if err := r.Update(ctx, cm); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatalf("invalid report must not cause a retry: %v", err)
	}
	if err := r.Get(ctx, request.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	condition := current.Status.FindCondition(api.ConditionFailed)
	if current.Status.Phase != api.VirtualizationValidationFailed || condition == nil || condition.Reason != "InvalidReport" {
		t.Fatalf("invalid report was not terminal: %#v", current.Status)
	}
	currentProvider := &api.Provider{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: provider.Namespace, Name: provider.Name}, currentProvider); err != nil {
		t.Fatal(err)
	}
	if condition := currentProvider.Status.FindCondition(ProviderFailed); condition == nil || condition.Reason != validationConditionReason(current) {
		t.Fatalf("Provider did not reflect terminal validation failure: %#v", currentProvider.Status)
	}

	current.Spec.RunNonce = "next"
	if err := r.Update(ctx, current); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(ctx, request.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	if current.Status.JobRef.Name == oldName {
		t.Fatal("input change did not create a new validation run")
	}
	for _, object := range []core.ObjectReference{
		{Kind: "Secret", Namespace: validation.Namespace, Name: oldName + "-credentials"},
		{Kind: "ServiceAccount", Namespace: validation.Namespace, Name: oldName},
		{Kind: "Role", Namespace: validation.Namespace, Name: oldName},
		{Kind: "RoleBinding", Namespace: validation.Namespace, Name: oldName},
		{Kind: "ConfigMap", Namespace: validation.Namespace, Name: oldName},
		{Kind: "Job", Namespace: validation.Namespace, Name: oldName},
	} {
		if err := r.Get(ctx, types.NamespacedName{Namespace: object.Namespace, Name: object.Name}, objectForReference(object)); err == nil {
			t.Fatalf("superseded %s %q was not deleted", object.Kind, object.Name)
		}
	}
}

func TestReconcileRejectsCrossNamespaceProviderCredentials(t *testing.T) {
	t.Setenv(ValidationImageEnv, "example.test/validator@sha256:1")
	for _, test := range []struct {
		name   string
		mutate func(*api.VirtualizationValidation, *api.Provider)
		reason string
	}{
		{
			name: "provider", reason: "ProviderNamespaceNotAllowed",
			mutate: func(validation *api.VirtualizationValidation, _ *api.Provider) {
				validation.Spec.ProviderRef.Namespace = "other-namespace"
			},
		},
		{
			name: "secret", reason: "ProviderSecretNamespaceNotAllowed",
			mutate: func(_ *api.VirtualizationValidation, provider *api.Provider) {
				provider.Spec.Secret.Namespace = "other-namespace"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			validation, provider, secret := validationObjects()
			test.mutate(validation, provider)
			r, _ := validationTestReconciler(t, validation, provider, secret)
			request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: validation.Namespace, Name: validation.Name}}
			ctx := context.Background()
			if _, err := r.Reconcile(ctx, request); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Reconcile(ctx, request); err != nil {
				t.Fatal(err)
			}
			current := &api.VirtualizationValidation{}
			if err := r.Get(ctx, request.NamespacedName, current); err != nil {
				t.Fatal(err)
			}
			condition := current.Status.FindCondition(api.ConditionFailed)
			if current.Status.Phase != api.VirtualizationValidationFailed || condition == nil || condition.Reason != test.reason {
				t.Fatalf("unexpected namespace validation status: %#v", current.Status)
			}
		})
	}
}

func objectForReference(reference core.ObjectReference) client.Object {
	switch reference.Kind {
	case "Secret":
		return &core.Secret{}
	case "ServiceAccount":
		return &core.ServiceAccount{}
	case "ConfigMap":
		return &core.ConfigMap{}
	case "Role":
		return &rbac.Role{}
	case "RoleBinding":
		return &rbac.RoleBinding{}
	case "Job":
		return &batch.Job{}
	default:
		panic("unsupported test reference kind")
	}
}

func TestInputHashIsIndependentOfCheckOrder(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider")}, Spec: api.ProviderSpec{URL: "https://api.example.test:6443"}}
	secret := &core.Secret{ObjectMeta: meta.ObjectMeta{ResourceVersion: "1"}}
	first := &api.VirtualizationValidation{Spec: api.VirtualizationValidationSpec{Checks: []string{"platform", "nodes"}}}
	second := &api.VirtualizationValidation{Spec: api.VirtualizationValidationSpec{Checks: []string{"nodes", "platform"}}}
	if got, want := inputHash(first, provider, secret, "example/validator@sha256:1", first.Spec.Checks), inputHash(second, provider, secret, "example/validator@sha256:1", second.Spec.Checks); got != want {
		t.Fatalf("input hash changed with check order: %q != %q", got, want)
	}
}

func TestValidationChangedIncludesFinalizerTransitions(t *testing.T) {
	old := &api.VirtualizationValidation{ObjectMeta: meta.ObjectMeta{Generation: 1}}
	current := old.DeepCopy()
	if validationChanged(old, current) {
		t.Fatal("unchanged validation should not reconcile")
	}
	current.Finalizers = []string{ValidationFinalizer}
	if !validationChanged(old, current) {
		t.Fatal("adding the finalizer must reconcile the validation")
	}
	current.Generation = 2
	if !validationChanged(old, current) {
		t.Fatal("generation change must reconcile the validation")
	}
}

func TestObserveReport(t *testing.T) {
	v := &api.VirtualizationValidation{}
	cm := &core.ConfigMap{Data: map[string]string{ResultConfigMapKey: `{
  "reportFormat": "CTRF",
  "specVersion": "0.0.0",
  "runId": "run-1",
  "results": {
    "summary": {"tests": 3, "passed": 1, "failed": 1, "pending": 1, "skipped": 0},
    "tests": [
      {"name":"one", "status":"passed", "duration":1},
      {"name":"two", "status":"failed", "message":"failure", "duration":2},
      {"name":"three", "status":"pending"}
    ]
  }
}`}}
	complete, err := (Reconciler{}).observeReport(v, cm)
	if err != nil {
		t.Fatalf("observe report: %v", err)
	}
	if complete {
		t.Fatal("report with pending tests was considered complete")
	}
	if got := v.Status.Summary; got.Total != 3 || got.Passed != 1 || got.Failed != 1 || got.Pending != 1 {
		t.Fatalf("unexpected summary: %#v", got)
	}
	if got := v.Status.CheckResults; len(got) != 3 || got[1].Message != "failure" {
		t.Fatalf("unexpected check results: %#v", got)
	}
}

func TestObserveReportRejectsUnversionedDocument(t *testing.T) {
	v := &api.VirtualizationValidation{}
	cm := &core.ConfigMap{Data: map[string]string{ResultConfigMapKey: `{"results":{"summary":{}}}`}}
	if _, err := (Reconciler{}).observeReport(v, cm); err == nil {
		t.Fatal("expected unversioned report to be rejected")
	}
}

func TestValidatorJobPassesOnlyExpectedInputs(t *testing.T) {
	v := &api.VirtualizationValidation{ObjectMeta: meta.ObjectMeta{Name: "validation", Namespace: "openshift-mtv", UID: types.UID("validation-uid")}, Status: api.VirtualizationValidationStatus{ObservedInputHash: "0123456789abcdef"}, Spec: api.VirtualizationValidationSpec{Checks: []string{"platform"}}}
	provider := &api.Provider{Spec: api.ProviderSpec{URL: "https://api.example.test:6443"}}
	secret := &core.Secret{ObjectMeta: meta.ObjectMeta{Name: "validation-result-credentials"}}
	job := validatorJob(v, provider, secret, "example/validator@sha256:1", "validation-result", []string{"10-openshift.d/10-nodes.d"}, false)
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Fatalf("expected no Job retry, got %#v", job.Spec.BackoffLimit)
	}
	values := map[string]core.EnvVar{}
	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		values[env.Name] = env
	}
	if _, found := values["VIRT_VALIDATE_TOKEN"]; found {
		t.Fatal("provider token must not be exposed through the Job environment")
	}
	if got := values["VIRT_VALIDATE_RESULT_CONFIGMAP"].Value; got != "validation-result" {
		t.Fatalf("unexpected result ConfigMap: %q", got)
	}
	if got := values["VIRT_VALIDATE_CHECKS"].Value; got != "10-openshift.d/10-nodes.d" {
		t.Fatalf("unexpected selected checks: %q", got)
	}
}

func TestProviderProfileChecks(t *testing.T) {
	checks, workload, err := providerProfileChecks(nil, "")
	if err != nil {
		t.Fatalf("resolve default profile checks: %v", err)
	}
	if workload || len(checks) == 0 || checks[0] != "10-openshift.d/00-installation.d" {
		t.Fatalf("unexpected default catalog: %#v", checks)
	}
	if _, _, err := providerProfileChecks([]string{"basic"}, ""); err == nil {
		t.Fatal("expected untrusted check ID to be rejected")
	}
	if _, _, err := providerProfileChecks([]string{"workload"}, ""); err == nil {
		t.Fatal("expected workload check without a dedicated namespace to be rejected")
	}
	checks, workload, err = providerProfileChecks([]string{"platform", "workload"}, "mtv-validation-workloads")
	if err != nil || !workload || checks[len(checks)-1] != "50-openshift-virtualization.d/50-basic.d" {
		t.Fatalf("unexpected workload catalog: %#v, workload=%t, err=%v", checks, workload, err)
	}
	checks, workload, err = providerProfileChecks([]string{"platform", "workload", "demo-failure"}, "mtv-validation-workloads")
	if err != nil || !workload || checks[len(checks)-1] != "99-mtv-demo.d/00-simulated-failure.d" {
		t.Fatalf("unexpected demo failure catalog: %#v, workload=%t, err=%v", checks, workload, err)
	}
}

func TestValidatorJobConfiguresDedicatedWorkloadNamespace(t *testing.T) {
	v := &api.VirtualizationValidation{Status: api.VirtualizationValidationStatus{ObservedInputHash: "0123456789abcdef"}, Spec: api.VirtualizationValidationSpec{ValidationNamespace: "mtv-validation-workloads"}}
	provider := &api.Provider{Spec: api.ProviderSpec{URL: "https://api.example.test:6443"}}
	job := validatorJob(v, provider, &core.Secret{}, "example/validator@sha256:1", "run", []string{"50-basic"}, true)
	values := map[string]string{}
	for _, env := range job.Spec.Template.Spec.Containers[0].Env {
		values[env.Name] = env.Value
	}
	if values["VIRT_VALIDATE_NAMESPACE"] != "mtv-validation-workloads" || values["VM_READY_TIMEOUT"] != "5m" {
		t.Fatalf("unexpected workload environment: %#v", values)
	}
}

func TestCompletedJobRequiresFinalReport(t *testing.T) {
	v := &api.VirtualizationValidation{}
	job := &batch.Job{Status: batch.JobStatus{Conditions: []batch.JobCondition{{Type: batch.JobComplete, Status: core.ConditionTrue}}}}
	(Reconciler{}).observeJob(v, job, false)
	if v.Status.Phase != api.VirtualizationValidationFailed {
		t.Fatalf("completed Job without report has phase %q", v.Status.Phase)
	}
}

func TestValidationCredentialsNormalizeLegacyCA(t *testing.T) {
	v := &api.VirtualizationValidation{ObjectMeta: meta.ObjectMeta{Namespace: "openshift-mtv"}, Status: api.VirtualizationValidationStatus{ObservedInputHash: "0123456789abcdef"}}
	providerSecret := &core.Secret{Data: map[string][]byte{
		api.Token: []byte("token"),
		"cacert":  []byte("legacy-ca"),
	}}
	credentials := validationCredentials(v, "run", providerSecret)
	if got := string(credentials.Data["ca.crt"]); got != "legacy-ca" {
		t.Fatalf("legacy CA was not normalized: %q", got)
	}
	if got := string(credentials.Data[api.Insecure]); got != "false" {
		t.Fatalf("unexpected default insecure setting: %q", got)
	}
}

func TestResultConfigMapRoleIsRestrictedToRunConfigMap(t *testing.T) {
	v := &api.VirtualizationValidation{ObjectMeta: meta.ObjectMeta{Namespace: "openshift-mtv"}, Status: api.VirtualizationValidationStatus{ObservedInputHash: "0123456789abcdef"}}
	role := resultConfigMapRole(v, "run")
	if len(role.Rules) != 1 || len(role.Rules[0].ResourceNames) != 1 || role.Rules[0].ResourceNames[0] != "run" {
		t.Fatalf("role is not restricted to the result ConfigMap: %#v", role.Rules)
	}
	if got := role.Rules[0].Verbs; len(got) != 3 || got[0] != "get" || got[1] != "patch" || got[2] != "update" {
		t.Fatalf("unexpected ConfigMap verbs: %#v", got)
	}
}
