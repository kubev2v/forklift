package k8s

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	wait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type PodResult struct {
	PodName   string
	Container string
	ExitCode  int32
	Duration  time.Duration
	Success   bool
	Err       error
}

// PollPodsAndCheck polls all pods matching your test’s label selector until they terminate,
// then returns a slice of PodResult indicating pass/fail per-container.
func PollPodsAndCheck(
	ctx context.Context,
	client *kubernetes.Clientset,
	namespace string,
	labelSelector string,
	maxTimeSeconds int,
	pollInterval time.Duration,
	timeout time.Duration,
) ([]PodResult, time.Duration, error) {
	var finalPods []v1.Pod
	start := time.Now()

	// 1) Poll until all pods finish (Succeeded or Failed).
	//    immediate=true means the condition runs immediately once.
	err := wait.PollUntilContextTimeout(
		ctx,          // your parent context
		pollInterval, // e.g. 2*time.Second
		timeout,      // overall timeout e.g. MaxTime + buffer
		true,         // run condition immediately on entry
		func(ctx context.Context) (bool, error) {
			podList, err := client.CoreV1().
				Pods(namespace).
				List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
			if err != nil {
				return false, err
			}
			finalPods = podList.Items
			for _, p := range finalPods {
				if p.Status.Phase != v1.PodSucceeded &&
					p.Status.Phase != v1.PodFailed {
					return false, nil // still running
				}
			}
			return true, nil // all done
		},
	)
	totalElapsed := time.Since(start)
	if err != nil {
		return nil, totalElapsed, fmt.Errorf("waiting for pods to finish: %w", err)
	}
	// 2) Inspect each pod & container
	var results []PodResult
	for _, p := range finalPods {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Terminated == nil {
				results = append(results, PodResult{
					PodName:   p.Name,
					Container: cs.Name,
					Success:   false,
					Err:       fmt.Errorf("no terminated state found"),
				})
				continue
			}
			term := cs.State.Terminated
			dur := term.FinishedAt.Time.Sub(term.StartedAt.Time)
			success := term.ExitCode == 0 && dur <= time.Duration(maxTimeSeconds)*time.Second
			var errDetail error
			if term.ExitCode != 0 {
				errDetail = fmt.Errorf("exit code %d", term.ExitCode)
			} else if dur > time.Duration(maxTimeSeconds)*time.Second {
				errDetail = fmt.Errorf("timeout: ran %v > %ds", dur, maxTimeSeconds)
			}
			results = append(results, PodResult{
				PodName:   p.Name,
				Container: cs.Name,
				ExitCode:  term.ExitCode,
				Duration:  dur,
				Success:   success,
				Err:       errDetail,
			})
		}
	}
	return results, totalElapsed, nil
}
