package forklift_controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var processedSucceededMigrations = make(map[string]struct{})

// Calculate Migrations metrics every 10 seconds
func RecordMigrationMetrics(c client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// get all migration objects
			migrations := api.MigrationList{}
			err := c.List(context.TODO(), &migrations)

			// if error occurs, retry 10 seconds later
			if err != nil {
				fmt.Printf("Metrics Migrations list error: %v\n", err)
				continue
			}

			// Initialize or reset the counter map at the beginning of each iteration
			counterMap := make(map[string]float64)

			for _, m := range migrations.Items {
				plan := api.Plan{}
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Plan.Namespace, Name: m.Spec.Plan.Name}, &plan)
				if err != nil {
					continue
				}

				sourceProvider := api.Provider{}
				err = c.Get(context.TODO(), client.ObjectKey{Namespace: plan.Spec.Provider.Source.Namespace, Name: plan.Spec.Provider.Source.Name}, &sourceProvider)
				if err != nil {
					continue
				}

				destProvider := api.Provider{}
				err = c.Get(context.TODO(), client.ObjectKey{Namespace: plan.Spec.Provider.Destination.Namespace, Name: plan.Spec.Provider.Destination.Name}, &destProvider)
				if err != nil {
					continue
				}

				isLocal := destProvider.Spec.URL == ""
				isWarm := plan.Spec.Warm

				var target, mode, key string
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

				provider := sourceProvider.Type().String()

				if m.Status.HasCondition(Succeeded) {
					key = fmt.Sprintf("%s|%s|%s|%s|%s", Succeeded, provider, mode, target, string(plan.UID))
					counterMap[key]++

					startTime := m.Status.Started.Time
					endTime := m.Status.Completed.Time
					duration := endTime.Sub(startTime).Seconds()

					var totalDataTransferred float64
					for _, vm := range m.Status.VMs {
						for _, step := range vm.Pipeline {
							if step.Name == "DiskTransferV2v" || step.Name == "DiskTransfer" {
								for _, task := range step.Tasks {
									totalDataTransferred += float64(task.Progress.Completed) * 1024 * 1024 // convert to Bytes
								}
							}
						}
					}

					// Set the metrics for duration and data transferred and update the map for scaned migration
					if _, exists := processedSucceededMigrations[string(m.UID)]; !exists {
						migrationDurationGauge.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target, "plan": string(plan.UID)}).Set(duration)
						migrationDurationHistogram.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target}).Observe(duration)
						dataTransferredGauge.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target, "plan": string(plan.UID)}).Set(totalDataTransferred)
						processedSucceededMigrations[string(m.UID)] = struct{}{}
					}
				}
				if m.Status.HasCondition(Failed) {
					key = fmt.Sprintf("%s|%s|%s|%s|%s", Failed, provider, mode, target, string(plan.UID))
					counterMap[key]++
				}
				if m.Status.HasCondition(Executing) {
					key = fmt.Sprintf("%s|%s|%s|%s|%s", Executing, provider, mode, target, string(plan.UID))
					counterMap[key]++
				}
				if m.Status.HasCondition(Canceled) {
					key = fmt.Sprintf("%s|%s|%s|%s|%s", Canceled, provider, mode, target, string(plan.UID))
					counterMap[key]++
				}
			}

			for key, value := range counterMap {
				parts := strings.Split(key, "|")
				migrationStatusGauge.With(prometheus.Labels{"status": parts[0], "provider": parts[1], "mode": parts[2], "target": parts[3]}).Set(value)
				migrationPlanCorrelationStatusGauge.With(prometheus.Labels{"status": parts[0], "provider": parts[1], "mode": parts[2], "target": parts[3], "plan": parts[4]}).Set(value)
			}
		}
	}()
}
