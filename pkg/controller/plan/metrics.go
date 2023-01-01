package plan

import (
	"context"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// 'status' - [ idle, executing, succeeded, failed, canceled, deleted, paused, pending, running, blocked ]
	migrationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_plans",
		Help: "VM migration Plans sorted by status",
	},
		[]string{"status"},
	)
)

// Calculate Plans metrics every 10 seconds
func recordMetrics(client client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// get all migration objects
			plans := api.PlanList{}
			err := client.List(context.TODO(), &plans)

			// if error occurs, retry 10 seconds later
			if err != nil {
				log.Info("Metrics Plans list error: %v", err)
				continue
			}

			// Holding counter vars used to make gauge update "atomic"
			var idle, executing, succeeded, failed, canceled, deleted, paused, pending, running, blocked float64

			// for all plans, count # in Idle, Executing, Succeeded, Failed, Canceled, Deleted, Paused, Pending, Running, Blocked
			for _, m := range plans.Items {
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
				if m.Status.HasCondition(Deleted) {
					deleted++
					continue
				}
				if m.Status.HasCondition(Paused) {
					paused++
					continue
				}
				if m.Status.HasCondition(Pending) {
					pending++
					continue
				}
				if m.Status.HasCondition(Running) {
					running++
					continue
				}
				if m.Status.HasCondition(Blocked) {
					blocked++
					continue
				}
				// If the Plan has no matching condition, but exists, it should be counted as Idle
				idle++
			}

			migrationGauge.With(
				prometheus.Labels{"status": "Idle"}).Set(idle)
			migrationGauge.With(
				prometheus.Labels{"status": Executing}).Set(executing)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded}).Set(succeeded)
			migrationGauge.With(
				prometheus.Labels{"status": Failed}).Set(failed)
			migrationGauge.With(
				prometheus.Labels{"status": Canceled}).Set(canceled)
			migrationGauge.With(
				prometheus.Labels{"status": Deleted}).Set(deleted)
			migrationGauge.With(
				prometheus.Labels{"status": Paused}).Set(paused)
			migrationGauge.With(
				prometheus.Labels{"status": Pending}).Set(pending)
			migrationGauge.With(
				prometheus.Labels{"status": Running}).Set(running)
			migrationGauge.With(
				prometheus.Labels{"status": Blocked}).Set(blocked)
		}
	}()
}
