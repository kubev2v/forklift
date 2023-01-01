package migration

import (
	"context"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// 'status' - [ executing, succeeded, failed, canceled ]
	migrationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_migrations",
		Help: "VM Migrations sorted by status",
	},
		[]string{"status"},
	)
)

// Calculate Migrations metrics every 10 seconds
func recordMetrics(client client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// get all migration objects
			migrations := api.MigrationList{}
			err := client.List(context.TODO(), &migrations)

			// if error occurs, retry 10 seconds later
			if err != nil {
				log.Info("Metrics Migrations list error: %v", err)
				continue
			}

			// Holding counter vars used to make gauge update "atomic"
			var executing, succeeded, failed, canceled float64

			// for all migrations, count # in executing, succeeded, failed, canceled
			for _, m := range migrations.Items {
				if m.Status.HasCondition(Executing) {
					executing++
					continue
				}
				if m.Status.HasCondition(Succeeded) {
					succeeded++
					continue
				}
				if m.Status.HasCondition(Failed) {
					failed++
					continue
				}
				if m.Status.HasCondition(Canceled) {
					canceled++
					continue
				}
			}

			migrationGauge.With(
				prometheus.Labels{"status": Executing}).Set(executing)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded}).Set(succeeded)
			migrationGauge.With(
				prometheus.Labels{"status": Failed}).Set(failed)
			migrationGauge.With(
				prometheus.Labels{"status": Canceled}).Set(canceled)
		}
	}()
}
