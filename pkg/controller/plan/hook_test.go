package plan

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testHookGlobalSA          = "global-sa"
	testHookAAPExampleURL     = "https://aap.example.com"
	testHookAAPTokenSecret    = "aap-token-secret"
	testHookHookSA            = "hook-sa"
	testHookPlanSA            = "plan-sa"
	testHookNamespace         = "test-namespace"
	testHookCMName            = "hook-config"
	testHookCMNamespace       = "test-ns"
	testHookKubev2vRunnerImg  = "quay.io/kubev2v/hook-runner:latest"
	testHookKonveyorRunnerImg = "quay.io/konveyor/hook-runner:latest"
)

func hookTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := api.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme api: %v", err)
	}
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme core: %v", err)
	}
	return scheme
}

func savedHookRunnerSettings(t *testing.T) func() {
	t.Helper()
	savedServiceAccount := Settings.Migration.ServiceAccount
	savedRequestsCPU := Settings.Migration.HooksContainerRequestsCpu
	savedRequestsMemory := Settings.Migration.HooksContainerRequestsMemory
	savedLimitsCPU := Settings.Migration.HooksContainerLimitsCpu
	savedLimitsMemory := Settings.Migration.HooksContainerLimitsMemory
	Settings.Migration.HooksContainerRequestsCpu = "100m"
	Settings.Migration.HooksContainerRequestsMemory = "128Mi"
	Settings.Migration.HooksContainerLimitsCpu = "1"
	Settings.Migration.HooksContainerLimitsMemory = "512Mi"
	return func() {
		Settings.Migration.ServiceAccount = savedServiceAccount
		Settings.Migration.HooksContainerRequestsCpu = savedRequestsCPU
		Settings.Migration.HooksContainerRequestsMemory = savedRequestsMemory
		Settings.Migration.HooksContainerLimitsCpu = savedLimitsCPU
		Settings.Migration.HooksContainerLimitsMemory = savedLimitsMemory
	}
}

func newHookRunnerForTemplateTest(hookSA, planSA, globalSA string) *HookRunner {
	Settings.Migration.ServiceAccount = globalSA
	return &HookRunner{
		Context: &plancontext.Context{
			Plan: &api.Plan{
				Spec: api.PlanSpec{
					ServiceAccount: planSA,
				},
			},
		},
		hook: &api.Hook{
			Spec: api.HookSpec{
				Image:          testHookKubev2vRunnerImg,
				ServiceAccount: hookSA,
			},
		},
	}
}

func TestHookRunnerTemplateServiceAccountName(t *testing.T) {
	defer savedHookRunnerSettings(t)()

	cm := &core.ConfigMap{
		ObjectMeta: meta.ObjectMeta{
			Name:      testHookCMName,
			Namespace: testHookCMNamespace,
		},
	}

	t.Run("uses hook SA when set", func(t *testing.T) {
		runner := newHookRunnerForTemplateTest(testHookHookSA, testHookPlanSA, testHookGlobalSA)
		tmpl := runner.template(cm)
		if tmpl.Spec.ServiceAccountName != testHookHookSA {
			t.Fatalf("got %q want %s", tmpl.Spec.ServiceAccountName, testHookHookSA)
		}
	})

	t.Run("hook SA empty falls back per MTV-4722", func(t *testing.T) {
		cases := []struct {
			name      string
			planSA    string
			globalSA  string
			wantPodSA string
		}{
			{"plan wins over global", testHookPlanSA, testHookGlobalSA, testHookPlanSA},
			{"global when plan empty", "", testHookGlobalSA, testHookGlobalSA},
			{"all empty uses namespace default", "", "", ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				runner := newHookRunnerForTemplateTest("", tc.planSA, tc.globalSA)
				tmpl := runner.template(cm)
				if tmpl.Spec.ServiceAccountName != tc.wantPodSA {
					t.Fatalf("ServiceAccountName = %q, want %q", tmpl.Spec.ServiceAccountName, tc.wantPodSA)
				}
			})
		}
	})
}

func TestHookSpecAAPFields(t *testing.T) {
	hook := &api.Hook{
		ObjectMeta: meta.ObjectMeta{
			Name: "test-hook", Namespace: testHookNamespace, UID: "hook-uid-456",
		},
		Spec: api.HookSpec{
			AAP: &api.AAPConfig{
				URL: testHookAAPExampleURL, JobTemplateID: 7,
				TokenSecret: core.ObjectReference{Name: testHookAAPTokenSecret},
				Timeout:     600,
			},
		},
	}
	if !api.HookExecutionConfigValid(hook) {
		t.Fatal("HookExecutionConfigValid should accept this AAP hook")
	}
}

func TestHookSpecPlaybookFields(t *testing.T) {
	hook := &api.Hook{
		Spec: api.HookSpec{
			Image:    testHookKonveyorRunnerImg,
			Playbook: "LS0tCi0gbmFtZTogVGVzdCBwbGF5Ym9vawogIGhvc3RzOiBsb2NhbGhvc3Q=",
		},
	}
	if !api.HookExecutionConfigValid(hook) {
		t.Fatal("HookExecutionConfigValid should accept this local hook")
	}
}

func TestGetAAPTokenFromSecret(t *testing.T) {
	scheme := hookTestScheme(t)
	ns := testHookNamespace
	ref := func(name string) *core.ObjectReference { return &core.ObjectReference{Name: name} }

	tests := []struct {
		name    string
		objs    []client.Object
		ref     *core.ObjectReference
		want    string
		wantErr bool
	}{
		{
			name: "success",
			objs: []client.Object{&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: testHookAAPTokenSecret, Namespace: ns},
				Data:       map[string][]byte{"token": []byte("tok")},
			}},
			ref:  ref(testHookAAPTokenSecret),
			want: "tok",
		},
		{
			name: "allows explicit namespace matching plan namespace",
			objs: []client.Object{&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "s", Namespace: ns},
				Data:       map[string][]byte{"token": []byte("explicit-ns")},
			}},
			ref:  &core.ObjectReference{Name: "s", Namespace: ns},
			want: "explicit-ns",
		},
		{
			name:    "rejects cross-namespace token secret",
			objs:    nil,
			ref:     &core.ObjectReference{Name: "s", Namespace: "other-ns"},
			wantErr: true,
		},
		{name: "missing secret", objs: nil, ref: ref("missing"), wantErr: true},
		{
			name: "wrong data key",
			objs: []client.Object{&core.Secret{
				ObjectMeta: meta.ObjectMeta{Name: "bad-secret", Namespace: ns},
				Data:       map[string][]byte{"wrong": []byte("x")},
			}},
			ref:     ref("bad-secret"),
			wantErr: true,
		},
		{name: "nil ref", objs: nil, ref: nil, wantErr: true},
		{name: "empty name", objs: nil, ref: &core.ObjectReference{Name: "  "}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := fake.NewClientBuilder().WithScheme(scheme)
			if len(tt.objs) > 0 {
				b = b.WithObjects(tt.objs...)
			}
			cl := b.Build()
			tok, err := GetAAPTokenFromSecret(t.Context(), cl, ns, tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tok != tt.want {
				t.Fatalf("token = %q, want %q", tok, tt.want)
			}
		})
	}
}
