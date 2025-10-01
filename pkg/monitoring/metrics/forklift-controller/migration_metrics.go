package forklift_controller

import (
	"context"
	"fmt"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	processedSucceededMigrations = make(map[string]struct{})
	processedFailedMigrations    = make(map[string]struct{})
	processedCanceledMigrations  = make(map[string]struct{})
)

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

				var target, mode string
				if isLocal {
					target = Local
				} else {
					target = Remote
				}
				if plan.IsWarm() {
					mode = Warm
				} else {
					mode = Cold
				}

				provider := sourceProvider.Type().String()
				processMigration(m, provider, mode, target, string(plan.UID))
			}
		}
	}()
}

func processMigration(migration api.Migration, provider, mode, target, planUID string) {
	var (
		processedMigrations map[string]struct{}
		status              string
	)

	switch {
	case migration.Status.HasCondition(Succeeded):
		processedMigrations = processedSucceededMigrations
		status = Succeeded
	case migration.Status.HasCondition(Failed):
		processedMigrations = processedFailedMigrations
		status = Failed
	case migration.Status.HasCondition(Canceled):
		processedMigrations = processedCanceledMigrations
		status = Canceled
	default:
		// otherwise, there's nothing to do with the current state of the migration
		return
	}

	if _, exists := processedMigrations[string(migration.UID)]; !exists {
		updateMetricsCount(status, provider, mode, target, planUID)
		if status == Succeeded {
			recordSuccessfulMigrationMetrics(migration, provider, mode, target, planUID)
		}
		processedMigrations[string(migration.UID)] = struct{}{}
	}
}

func updateMetricsCount(status, provider, mode, target, plan string) {
	migrationStatusCounter.With(prometheus.Labels{"status": status, "provider": provider, "mode": mode, "target": target}).Inc()
	migrationPlanCorrelationStatusCounter.With(prometheus.Labels{"status": status, "provider": provider, "mode": mode, "target": target, "plan": plan}).Inc()
}

func recordSuccessfulMigrationMetrics(migration api.Migration, provider, mode, target, planUID string) {
	startTime := migration.Status.Started.Time
	endTime := migration.Status.Completed.Time
	duration := endTime.Sub(startTime).Seconds()

	var totalDataTransferred float64
	for _, vm := range migration.Status.VMs {
		for _, step := range vm.Pipeline {
			if step.Name == "DiskTransferV2v" || step.Name == "DiskTransfer" {
				for _, task := range step.Tasks {
					totalDataTransferred += float64(task.Progress.Completed) * 1024 * 1024 // convert to Bytes
				}
			}
		}
	}

	migrationDurationGauge.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target, "plan": planUID}).Set(duration)
	migrationDurationHistogram.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target}).Observe(duration)
	dataTransferredGauge.With(prometheus.Labels{"provider": provider, "mode": mode, "target": target, "plan": planUID}).Set(totalDataTransferred)
}
