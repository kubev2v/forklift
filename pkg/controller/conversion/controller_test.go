package conversion

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolvePhaseConditions_FailedWithError(t *testing.T) {
	conv := &api.Conversion{
		ObjectMeta: meta.ObjectMeta{Name: "test-conv", Namespace: "default"},
		Status:     api.ConversionStatus{Phase: api.PhaseFailed},
	}
	sccErr := fmt.Errorf(`pods "plan1-vm-9265-" is forbidden: unable to validate against any security context constraint: [provider "anyuid": Forbidden: not usable by user or serviceaccount]`)

	resolvePhaseConditions(conv, sccErr)

	cnd := conv.Status.FindCondition(api.ConversionFailed)
	if cnd == nil {
		t.Fatal("expected ConversionFailed condition to be set")
	}
	if cnd.Status != True {
		t.Errorf("expected condition status %q, got %q", True, cnd.Status)
	}
	expected := "The conversion has failed: " + sccErr.Error()
	if cnd.Message != expected {
		t.Errorf("expected condition message:\n  %s\ngot:\n  %s", expected, cnd.Message)
	}
	if conv.Status.Message != expected {
		t.Errorf("expected status.message:\n  %s\ngot:\n  %s", expected, conv.Status.Message)
	}
}

func TestResolvePhaseConditions_FailedWithoutError(t *testing.T) {
	conv := &api.Conversion{
		ObjectMeta: meta.ObjectMeta{Name: "test-conv", Namespace: "default"},
		Status:     api.ConversionStatus{Phase: api.PhaseFailed},
	}

	resolvePhaseConditions(conv, nil)

	cnd := conv.Status.FindCondition(api.ConversionFailed)
	if cnd == nil {
		t.Fatal("expected ConversionFailed condition to be set")
	}
	if cnd.Message != "The conversion has failed." {
		t.Errorf("expected generic condition message, got %q", cnd.Message)
	}
	if conv.Status.Message != "The conversion has failed." {
		t.Errorf("expected generic status.message, got %q", conv.Status.Message)
	}
}

func TestResolvePhaseConditions_Succeeded(t *testing.T) {
	conv := &api.Conversion{
		ObjectMeta: meta.ObjectMeta{Name: "test-conv", Namespace: "default"},
		Status:     api.ConversionStatus{Phase: api.PhaseSucceeded},
	}

	resolvePhaseConditions(conv, nil)

	cnd := conv.Status.FindCondition(libcnd.Ready)
	if cnd == nil {
		t.Fatal("expected Ready condition to be set")
	}
	if cnd.Message != "The conversion has completed successfully." {
		t.Errorf("unexpected message: %q", cnd.Message)
	}
}

func TestResolvePhaseConditions_Canceled(t *testing.T) {
	conv := &api.Conversion{
		ObjectMeta: meta.ObjectMeta{Name: "test-conv", Namespace: "default"},
		Status:     api.ConversionStatus{Phase: api.PhaseCanceled},
	}

	resolvePhaseConditions(conv, nil)

	cnd := conv.Status.FindCondition(api.ConversionCanceled)
	if cnd == nil {
		t.Fatal("expected ConversionCanceled condition to be set")
	}
	if cnd.Message != "The conversion has been canceled." {
		t.Errorf("unexpected condition message: %q", cnd.Message)
	}
	if conv.Status.Message != "The conversion has been canceled." {
		t.Errorf("unexpected status.message: %q", conv.Status.Message)
	}
	if conv.Status.CompletionTime == nil {
		t.Error("expected CompletionTime to be set")
	}
}

func TestCheckPendingPodTimeout_NoTimeout(t *testing.T) {
	Settings.ConversionPodPendingTimeout = 5
	p := &ConversionPipeline{}
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name:              "test-pod",
			CreationTimestamp: meta.NewTime(time.Now().Add(-1 * time.Minute)),
		},
		Status: core.PodStatus{Phase: core.PodPending},
	}
	err := p.checkPendingPodTimeout(pod)
	if err != nil {
		t.Errorf("expected no error for pod pending < timeout, got: %v", err)
	}
}

func TestCheckPendingPodTimeout_Exceeded(t *testing.T) {
	Settings.ConversionPodPendingTimeout = 5
	p := &ConversionPipeline{}
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name:              "test-pod",
			CreationTimestamp: meta.NewTime(time.Now().Add(-6 * time.Minute)),
		},
		Status: core.PodStatus{
			Phase: core.PodPending,
			Conditions: []core.PodCondition{
				{
					Type:               core.PodScheduled,
					Status:             core.ConditionFalse,
					LastTransitionTime: meta.NewTime(time.Now().Add(-6 * time.Minute)),
				},
			},
		},
	}
	err := p.checkPendingPodTimeout(pod)
	if err == nil {
		t.Fatal("expected error for pod pending > timeout")
	}
	if !strings.Contains(err.Error(), "stuck in Pending") {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestCheckPendingPodTimeout_ScheduledInitPending(t *testing.T) {
	Settings.ConversionPodPendingTimeout = 5
	p := &ConversionPipeline{}
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name:              "test-pod",
			CreationTimestamp: meta.NewTime(time.Now().Add(-6 * time.Minute)),
		},
		Status: core.PodStatus{
			Phase: core.PodPending,
			Conditions: []core.PodCondition{
				{
					Type:               core.PodScheduled,
					Status:             core.ConditionTrue,
					LastTransitionTime: meta.NewTime(time.Now().Add(-6 * time.Minute)),
				},
			},
		},
	}
	err := p.checkPendingPodTimeout(pod)
	if err != nil {
		t.Errorf("expected no timeout while scheduled pod initializes, got: %v", err)
	}
}

func TestCheckPendingPodTimeout_ZeroTimestamp(t *testing.T) {
	Settings.ConversionPodPendingTimeout = 5
	p := &ConversionPipeline{}
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{Name: "test-pod"},
		Status:     core.PodStatus{Phase: core.PodPending},
	}
	err := p.checkPendingPodTimeout(pod)
	if err != nil {
		t.Errorf("expected no error for zero timestamp, got: %v", err)
	}
}

func TestCheckPendingPodTimeout_Disabled(t *testing.T) {
	Settings.ConversionPodPendingTimeout = 0
	p := &ConversionPipeline{}
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name:              "test-pod",
			CreationTimestamp: meta.NewTime(time.Now().Add(-60 * time.Minute)),
		},
		Status: core.PodStatus{Phase: core.PodPending},
	}
	err := p.checkPendingPodTimeout(pod)
	if err != nil {
		t.Errorf("expected no error when timeout is disabled (0), got: %v", err)
	}
}

func TestPendingPodReason_Unschedulable(t *testing.T) {
	pod := &core.Pod{
		Status: core.PodStatus{
			Conditions: []core.PodCondition{
				{
					Type:    core.PodScheduled,
					Status:  core.ConditionFalse,
					Message: "0/3 nodes are available",
				},
			},
		},
	}
	reason := pendingPodReason(pod)
	if !strings.Contains(reason, "Unschedulable") {
		t.Errorf("expected Unschedulable reason, got: %s", reason)
	}
}

func TestPendingPodReason_ContainerWaiting(t *testing.T) {
	pod := &core.Pod{
		Status: core.PodStatus{
			ContainerStatuses: []core.ContainerStatus{
				{
					State: core.ContainerState{
						Waiting: &core.ContainerStateWaiting{
							Reason:  "ContainerCreating",
							Message: "configmap \"my-scripts\" not found",
						},
					},
				},
			},
		},
	}
	reason := pendingPodReason(pod)
	if !strings.Contains(reason, "ContainerCreating") {
		t.Errorf("expected ContainerCreating reason, got: %s", reason)
	}
	if !strings.Contains(reason, "my-scripts") {
		t.Errorf("expected configmap name in reason, got: %s", reason)
	}
}

func TestConversionRequestsForPod(t *testing.T) {
	reqs := conversionRequestsForPod(context.TODO(), &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Labels: map[string]string{
				convctx.LabelConversion:    "plan-vm-1-abc",
				convctx.LabelPlanNamespace: "openshift-mtv",
			},
		},
	})
	if len(reqs) != 1 {
		t.Fatalf("expected one reconcile request, got %d", len(reqs))
	}
	if reqs[0].Name != "plan-vm-1-abc" || reqs[0].Namespace != "openshift-mtv" {
		t.Fatalf("unexpected request: %+v", reqs[0])
	}
}

func TestPendingPodReason_Fallback(t *testing.T) {
	pod := &core.Pod{
		Status: core.PodStatus{},
	}
	reason := pendingPodReason(pod)
	if reason != "check pod events for mount/scheduling failures" {
		t.Errorf("expected fallback reason, got: %s", reason)
	}
}
