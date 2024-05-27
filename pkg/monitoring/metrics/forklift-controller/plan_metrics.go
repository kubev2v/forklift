package forklift_controller

import (
	"context"
	"fmt"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	planGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_plans",
		Help: "VM migration Plans sorted by status and provider type",
	},
		[]string{
			"status",
			"provider",
			"type",
			"destination",
		},
	)
)

// Calculate Plans metrics every 10 seconds
func RecordPlanMetrics(client client.Client) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			// get all migration objects
			plans := api.PlanList{}
			err := client.List(context.TODO(), &plans)

			// if error occurs, retry 10 seconds later
			if err != nil {
				fmt.Printf("Metrics Plans list error: %v\n", err)
				continue
			}

			// Holding counter vars used to make gauge update "atomic"
			var (
				succeededRHV, succeededOCP, succeededOVA, succeededVsphere, succeededOpenstack float64
				failedRHV, failedOCP, failedOVA, failedVsphere, failedOpenstack                float64
				localCluster, remoteCluster                                                    float64
				warmMigration, coldMigration                                                   float64
			)

			for _, m := range plans.Items {
				if m.Provider.Destination.Spec.Secret.Name == "" {
					remoteCluster++
				} else {
					localCluster++
				}
				if m.Spec.Warm {
					warmMigration++
				} else {
					coldMigration++
				}
				if m.Status.HasCondition(Succeeded) {
					switch m.Provider.Source.Type() {
					case api.Ova:
						succeededOVA++
						continue
					case api.OVirt:
						succeededRHV++
						continue
					case api.VSphere:
						succeededVsphere++
						continue
					case api.OpenShift:
						succeededOCP++
						continue
					case api.OpenStack:
						succeededOpenstack++
						continue
					}
				}
				if m.Status.HasCondition(Failed) {
					switch m.Provider.Source.Type() {
					case api.Ova:
						failedOVA++
						continue
					case api.OVirt:
						failedRHV++
						continue
					case api.VSphere:
						failedVsphere++
						continue
					case api.OpenShift:
						failedOCP++
						continue
					case api.OpenStack:
						failedOpenstack++
						continue
					}
				}
			}
			planGauge.With(
				prometheus.Labels{"type": Cold}).Set(coldMigration)
			planGauge.With(
				prometheus.Labels{"type": Warm}).Set(warmMigration)

			planGauge.With(
				prometheus.Labels{"des": Cold}).Set(coldMigration)
			planGauge.With(
				prometheus.Labels{"type": Cold}).Set(coldMigration)

			planGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OVirt.String()}).Set(succeededRHV)
			planGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OpenShift.String()}).Set(succeededOCP)
			planGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OpenStack.String()}).Set(succeededOpenstack)
			planGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.Ova.String()}).Set(succeededOVA)
			planGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.VSphere.String()}).Set(succeededVsphere)
			planGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OVirt.String()}).Set(failedRHV)
			planGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OpenShift.String()}).Set(failedOCP)
			planGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OpenStack.String()}).Set(succeededOpenstack)
			planGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.Ova.String()}).Set(succeededOVA)
			planGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.VSphere.String()}).Set(succeededVsphere)
		}
	}()
}
