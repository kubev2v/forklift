package conversion

import (
	"testing"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildVirtV2vConversionPodUsesTerminationLogFallback(t *testing.T) {
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{Labels: map[string]string{}},
		Spec: core.PodSpec{
			Containers: []core.Container{{}},
		},
	}

	if err := (&Builder{}).BuildVirtV2vConversionPod(pod, nil, nil); err != nil {
		t.Fatalf("BuildVirtV2vConversionPod() error = %v", err)
	}
	if got := pod.Spec.Containers[0].TerminationMessagePolicy; got != core.TerminationMessageFallbackToLogsOnError {
		t.Fatalf("termination message policy = %q, want %q", got, core.TerminationMessageFallbackToLogsOnError)
	}
}
