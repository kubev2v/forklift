package conversion

import (
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/virt-v2v/errorreporting"
	core "k8s.io/api/core/v1"
)

func conversionPodFailure(pod *core.Pod) error {
	if failure, ok := errorreporting.ExtractFromPod(pod); ok {
		return liberr.New(errorreporting.Format(failure), "pod", pod.Name, "code", failure.Code)
	}
	return liberr.New("conversion pod failed", "pod", pod.Name, "phase", pod.Status.Phase)
}
