package conversion

import (
	"strings"
	"testing"

	"github.com/kubev2v/forklift/pkg/virt-v2v/errorreporting"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConversionPodFailureUsesActionableMessageOrGenericFallback(t *testing.T) {
	failure := errorreporting.Failure{
		Code:        errorreporting.DirtyFilesystem,
		Summary:     "The guest filesystem was not cleanly unmounted.",
		Message:     "virt-v2v could not mount the filesystem read-write.",
		Remediation: "Start the VM in the source environment, allow it to boot, and let MTV power it off cleanly before retrying. Check for Windows hibernation or Fast Startup.",
	}
	payload, err := errorreporting.Encode(failure)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	actionable := &core.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "conversion-pod"},
		Status: core.PodStatus{
			Phase: core.PodFailed,
			ContainerStatuses: []core.ContainerStatus{{
				Name:  "virt-v2v",
				State: core.ContainerState{Terminated: &core.ContainerStateTerminated{ExitCode: 1, Message: string(payload)}},
			}},
		},
	}
	if err := conversionPodFailure(actionable); err == nil || !strings.Contains(err.Error(), failure.Remediation) {
		t.Fatalf("conversionPodFailure() = %v, want actionable remediation", err)
	}

	generic := &core.Pod{ObjectMeta: metav1.ObjectMeta{Name: "conversion-pod"}, Status: core.PodStatus{Phase: core.PodFailed}}
	if err := conversionPodFailure(generic); err == nil || !strings.Contains(err.Error(), "conversion pod failed") {
		t.Fatalf("conversionPodFailure() = %v, want generic fallback", err)
	}
}
