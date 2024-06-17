package forklift_controller

import (
	"context"
	"fmt"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var processedPlans = make(map[string]struct{})

// Calculate Plans metrics every 10 seconds
func RecordPlanMetrics(c client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// get all migration objects
			plans := api.PlanList{}
			err := c.List(context.TODO(), &plans)

			// if error occurs, retry 10 seconds later
			if err != nil {
				fmt.Printf("Metrics Plans list error: %v\n", err)
				continue
			}

			for _, m := range plans.Items {
				// save plans ID to not proccess the same plan more than once
				if _, exists := processedPlans[string(m.UID)]; exists {
					continue
				} else {
					processedPlans[string(m.UID)] = struct{}{}
				}
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
				isWarm := m.Spec.Warm

				var target, mode string
				if isLocal {
					target = Local
				} else {
					target = Remote
				}
				if isWarm {
					mode = Warm
				} else {
					mode = Cold
				}

				if m.Status.HasCondition(Succeeded) {
					planStatusCounter.With(prometheus.Labels{"status": Succeeded, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
				if m.Status.HasCondition(Failed) {
					planStatusCounter.With(prometheus.Labels{"status": Failed, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
				if m.Status.HasCondition(Executing) {
					planStatusCounter.With(prometheus.Labels{"status": Executing, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
				if m.Status.HasCondition(Running) {
					planStatusCounter.With(prometheus.Labels{"status": Running, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
				if m.Status.HasCondition(Pending) {
					planStatusCounter.With(prometheus.Labels{"status": Pending, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
				if m.Status.HasCondition(Canceled) {
					planStatusCounter.With(prometheus.Labels{"status": Canceled, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
				if m.Status.HasCondition(Blocked) {
					planStatusCounter.With(prometheus.Labels{"status": Blocked, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
				if m.Status.HasCondition(Ready) {
					planStatusCounter.With(prometheus.Labels{"status": Ready, "provider": sourceProvider.Type().String(), "mode": mode, "target": target}).Inc()
					continue
				}
			}
		}
	}()
}
