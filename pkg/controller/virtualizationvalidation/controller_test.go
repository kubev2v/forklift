package virtualizationvalidation

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	batch "k8s.io/api/batch/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestInputHashIsIndependentOfCheckOrder(t *testing.T) {
	provider := &api.Provider{ObjectMeta: meta.ObjectMeta{UID: types.UID("provider")}, Spec: api.ProviderSpec{URL: "https://api.example.test:6443"}}
	secret := &core.Secret{ObjectMeta: meta.ObjectMeta{ResourceVersion: "1"}}
	first := &api.VirtualizationValidation{Spec: api.VirtualizationValidationSpec{Checks: []string{"platform", "nodes"}}}
	second := &api.VirtualizationValidation{Spec: api.VirtualizationValidationSpec{Checks: []string{"nodes", "platform"}}}
	if got, want := inputHash(first, provider, secret, "example/validator@sha256:1", first.Spec.Checks), inputHash(second, provider, secret, "example/validator@sha256:1", second.Spec.Checks); got != want {
		t.Fatalf("input hash changed with check order: %q != %q", got, want)
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
	job := validatorJob(v, provider, secret, "example/validator@sha256:1", "validation-result", []string{"10-openshift.d/10-nodes.d"})
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
	checks, err := providerProfileChecks(nil)
	if err != nil {
		t.Fatalf("resolve default profile checks: %v", err)
	}
	if len(checks) == 0 || checks[0] != "10-openshift.d/00-installation.d" {
		t.Fatalf("unexpected default catalog: %#v", checks)
	}
	if _, err := providerProfileChecks([]string{"basic"}); err == nil {
		t.Fatal("expected untrusted check ID to be rejected")
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
