package conversion

import (
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
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
