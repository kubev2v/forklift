package forklift_controller

import (
	"context"
	"fmt"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var processedSucceededMigrations = make(map[string]struct{})
var processedFailedMigrations = make(map[string]struct{})
var processedCanceledMigrations = make(map[string]struct{})

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

				provider := sourceProvider.Type().String()

				statusList := []string{Succeeded, Failed, Executing, Canceled}
				for _, status := range statusList {
					if m.Status.HasCondition(status) {
						switch status {
						case Succeeded:
							if _, exists := processedSucceededMigrations[string(m.UID)]; !exists {
								updateMetricsCount(status, provider, mode, target, string(plan.UID))

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

								migrationDurationGauge.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target, "plan": string(plan.UID)}).Set(duration)
								migrationDurationHistogram.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target}).Observe(duration)
								dataTransferredGauge.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target, "plan": string(plan.UID)}).Set(totalDataTransferred)

								processedSucceededMigrations[string(m.UID)] = struct{}{}
							}
						case Failed:
							if _, exists := processedFailedMigrations[string(m.UID)]; !exists {
								updateMetricsCount(status, provider, mode, target, string(plan.UID))
								processedFailedMigrations[string(m.UID)] = struct{}{}
							}
						case Canceled:
							if _, exists := processedCanceledMigrations[string(m.UID)]; !exists {
								updateMetricsCount(status, provider, mode, target, string(plan.UID))
								processedCanceledMigrations[string(m.UID)] = struct{}{}
							}
						}
					}
				}
			}
		}
	}()
}

func updateMetricsCount(status, provider, mode, target, plan string) {
	migrationStatusCounter.With(prometheus.Labels{"status": status, "provider": provider, "mode": mode, "target": target}).Inc()
	migrationPlanCorrelationStatusCounter.With(prometheus.Labels{"status": status, "provider": provider, "mode": mode, "target": target, "plan": plan}).Inc()
}
