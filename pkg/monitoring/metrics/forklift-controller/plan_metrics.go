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
	planStatusGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_plans",
		Help: "VM migration Plans sorted by status and provider type",
	},
		[]string{
			"status",
			"provider",
		},
	)

	planTypeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_plans_type",
		Help: "VM migration Plans type",
	},
		[]string{
			"type",
		},
	)

	planDestinationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_operator_destination",
		Help: "MTV operator destination",
	},
		[]string{
			"destination",
		},
	)
)

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

			// Holding counter vars used to make gauge update "atomic"
			var (
				succeededRHV, succeededOCP, succeededOVA, succeededVsphere, succeededOpenstack float64
				failedRHV, failedOCP, failedOVA, failedVsphere, failedOpenstack                float64
				localCluster, remoteCluster                                                    float64
				warmMigration, coldMigration                                                   float64
			)

			for _, m := range plans.Items {

				destProvider := api.Provider{}
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Provider.Destination.Namespace, Name: m.Spec.Provider.Destination.Name}, &destProvider)
				if err != nil {
					continue
				}
				fmt.Println("this is provider ", destProvider)
				if destProvider.Spec.URL == "" {
					localCluster++
				} else {
					remoteCluster++
				}

				if m.Spec.Warm {
					warmMigration++
				} else {
					coldMigration++
				}

				sourceProvider := api.Provider{}
				err = c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Provider.Source.Namespace, Name: m.Spec.Provider.Source.Name}, &sourceProvider)
				if err != nil {
					continue
				}
				if m.Status.HasCondition(Succeeded) {
					switch sourceProvider.Type() {
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
					switch sourceProvider.Type() {
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
			planTypeGauge.With(
				prometheus.Labels{"type": Cold}).Set(coldMigration)
			planTypeGauge.With(
				prometheus.Labels{"type": Warm}).Set(warmMigration)

			planDestinationGauge.With(
				prometheus.Labels{"destination": Local}).Set(localCluster)
			planDestinationGauge.With(
				prometheus.Labels{"destination": Remote}).Set(remoteCluster)

			planStatusGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OVirt.String()}).Set(succeededRHV)
			planStatusGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OpenShift.String()}).Set(succeededOCP)
			planStatusGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OpenStack.String()}).Set(succeededOpenstack)
			planStatusGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.Ova.String()}).Set(succeededOVA)
			planStatusGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.VSphere.String()}).Set(succeededVsphere)
			planStatusGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OVirt.String()}).Set(failedRHV)
			planStatusGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OpenShift.String()}).Set(failedOCP)
			planStatusGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OpenStack.String()}).Set(failedOpenstack)
			planStatusGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.Ova.String()}).Set(failedOVA)
			planStatusGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.VSphere.String()}).Set(failedVsphere)
		}
	}()
}
