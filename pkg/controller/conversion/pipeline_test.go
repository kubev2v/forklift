package conversion

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInspectionUsesPodNameNotPodIP(t *testing.T) {
	orig := inspectionPodDo
	t.Cleanup(func() { inspectionPodDo = orig })

	var gotMethod, gotNS, gotName, gotPath string
	inspectionPodDo = func(_ context.Context, _ client.Client, _ api.ConversionSpec, method, namespace, name, path string) (int, []byte, error) {
		gotMethod, gotNS, gotName, gotPath = method, namespace, name, path
		if path == "/ready" {
			return http.StatusOK, nil, nil
		}
		if path == "/results" {
			body, _ := json.Marshal(map[string]any{"passed": true})
			return http.StatusOK, body, nil
		}
		if path == "/shutdown" {
			return http.StatusOK, nil, nil
		}
		t.Fatalf("unexpected path %s", path)
		return 0, nil, nil
	}

	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	p := &ConversionPipeline{
		ctx: context.Background(),
		r: &Reconciler{
			Reconciler: base.Reconciler{Client: cl, Log: logging.WithName("test")},
		},
		conv: &api.Conversion{
			Spec: api.ConversionSpec{
				Destination: core.ObjectReference{Name: "host", Namespace: "openshift-mtv"},
			},
		},
	}
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{Name: "di-pod", Namespace: "migrations"},
		Status:     core.PodStatus{Phase: core.PodRunning}, // PodIP intentionally empty
	}

	if !p.isResultReady(pod) {
		t.Fatal("expected /ready via pod proxy to succeed without PodIP")
	}
	if gotMethod != http.MethodGet || gotPath != "/ready" || gotName != "di-pod" || gotNS != "migrations" {
		t.Fatalf("ready request = %s %s/%s %s", gotMethod, gotNS, gotName, gotPath)
	}

	result, err := p.fetchInspectionResults(pod)
	if err != nil {
		t.Fatalf("fetchInspectionResults: %v", err)
	}
	if result == nil || !result.AllChecksPassed {
		t.Fatalf("unexpected result %#v", result)
	}
	if gotPath != "/results" || gotName != "di-pod" {
		t.Fatalf("results request used path=%s name=%s", gotPath, gotName)
	}

	p.signalPodShutdown(pod)
	if gotMethod != http.MethodPost || gotPath != "/shutdown" || gotName != "di-pod" {
		t.Fatalf("shutdown request = %s %s %s", gotMethod, gotName, gotPath)
	}
}

func TestFetchInspectionResults503(t *testing.T) {
	orig := inspectionPodDo
	t.Cleanup(func() { inspectionPodDo = orig })
	inspectionPodDo = func(context.Context, client.Client, api.ConversionSpec, string, string, string, string) (int, []byte, error) {
		return http.StatusServiceUnavailable, nil, nil
	}
	p := &ConversionPipeline{ctx: context.Background(), conv: &api.Conversion{}, r: &Reconciler{}}
	result, err := p.fetchInspectionResults(&core.Pod{ObjectMeta: meta.ObjectMeta{Name: "p", Namespace: "ns"}})
	if err != nil {
		t.Fatalf("503 should be non-error: %v", err)
	}
	if result != nil {
		t.Fatal("503 should return nil result")
	}
}
