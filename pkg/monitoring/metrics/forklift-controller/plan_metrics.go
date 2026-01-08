package forklift_controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var activePlanStatuses = make(map[string]struct{})

// Calculate Plans metrics every 10 seconds
func RecordPlanMetrics(c client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// get all plans objects
			plans := api.PlanList{}
			err := c.List(context.TODO(), &plans)

			// if error occurs, retry 10 seconds later
			if err != nil {
				fmt.Printf("Metrics Plans list error: %v\n", err)
				continue
			}

			// Initialize or reset the counter map at the beginning of each iteration
			plansCounterMap := make(map[string]float64)

			for _, m := range plans.Items {
				sourceProvider := api.Provider{}
				err = c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Provider.Source.Namespace, Name: m.Spec.Provider.Source.Name}, &sourceProvider)
				if err != nil {
					continue
				}

				destProvider := api.Provider{}
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Provider.Destination.Namespace, Name: m.Spec.Provider.Destination.Name}, &destProvider)
				if err != nil {
					continue
				}

				isLocal := destProvider.Spec.URL == ""

				var target, mode, key string
				if isLocal {
					target = Local
				} else {
					target = Remote
				}
				if m.IsWarm() {
					mode = Warm
				} else {
					mode = Cold
				}

				provider := sourceProvider.Type().String()

				if m.Status.HasCondition(Succeeded) {
					key = fmt.Sprintf("%s|%s|%s|%s", Succeeded, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Failed) {
					key = fmt.Sprintf("%s|%s|%s|%s", Failed, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Executing) {
					key = fmt.Sprintf("%s|%s|%s|%s", Executing, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Running) {
					key = fmt.Sprintf("%s|%s|%s|%s", Running, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Pending) {
					key = fmt.Sprintf("%s|%s|%s|%s", Pending, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Canceled) {
					key = fmt.Sprintf("%s|%s|%s|%s", Canceled, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Blocked) {
					key = fmt.Sprintf("%s|%s|%s|%s", Blocked, provider, mode, target)
					plansCounterMap[key]++
				}
				if m.Status.HasCondition(Deleted) {
					key = fmt.Sprintf("%s|%s|%s|%s", Deleted, provider, mode, target)
					plansCounterMap[key]++
				}
			}

			for key, value := range plansCounterMap {
				parts := strings.Split(key, "|")
				planStatusGauge.With(prometheus.Labels{"status": parts[0], "provider": parts[1], "mode": parts[2], "target": parts[3]}).Set(value)
				activePlanStatuses[key] = struct{}{}
			}

			for planStatus := range activePlanStatuses {
				if _, exists := plansCounterMap[planStatus]; !exists {
					parts := strings.Split(planStatus, "|")
					planStatusGauge.With(prometheus.Labels{"status": parts[0], "provider": parts[1], "mode": parts[2], "target": parts[3]}).Set(0)
					delete(activePlanStatuses, planStatus)
				}
			}
		}
	}()
}
