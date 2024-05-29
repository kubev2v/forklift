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
	// 'status' - [ executing, succeeded, failed, canceled ]
	// 'provider' - [oVirt, vSphere, Openstack, OVA, openshift]
	migrationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_migrations",
		Help: "VM Migrations sorted by status and provider type",
	},
		[]string{"status", "provider"},
	)
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

			// Holding counter vars used to make gauge update "atomic"
			var succeededRHV, succeededOCP, succeededOVA, succeededVsphere, succeededOpenstack float64
			var failedRHV, failedOCP, failedOVA, failedVsphere, failedOpenstack float64

			// for all migrations, count # in executing, succeeded, failed, canceled
			for _, m := range migrations.Items {

				plan := api.Plan{}
				err := c.Get(context.TODO(), client.ObjectKey{Namespace: m.Spec.Plan.Namespace, Name: m.Spec.Plan.Name}, &plan)
				if err != nil {
					continue
				}

				provider := api.Provider{}
				err = c.Get(context.TODO(), client.ObjectKey{Namespace: plan.Spec.Provider.Source.Namespace, Name: plan.Spec.Provider.Source.Name}, &provider)
				if err != nil {
					continue
				}

				if m.Status.HasCondition(Succeeded) {
					switch provider.Type() {
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
					switch provider.Type() {
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

			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OVirt.String()}).Set(succeededRHV)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OpenShift.String()}).Set(succeededOCP)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.OpenStack.String()}).Set(succeededOpenstack)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.Ova.String()}).Set(succeededOVA)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": api.VSphere.String()}).Set(succeededVsphere)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OVirt.String()}).Set(failedRHV)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OpenShift.String()}).Set(failedOCP)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.OpenStack.String()}).Set(failedOpenstack)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.Ova.String()}).Set(failedOVA)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": api.VSphere.String()}).Set(failedVsphere)
		}
	}()
}
