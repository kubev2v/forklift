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
	// 'provider' - [oVirt, vSphere, Openstack, OVA, openshift]
	migrationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_migrations",
		Help: "VM Migrations sorted by status and provider type",
	},
		[]string{"status", "provider"},
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
			var succeededRHV, succeededOCP, succeededOVA, succeededVsphere, succeededOpenstack float64
			var failedRHV, failedOCP, failedOVA, failedVsphere, failedOpenstack float64

			// for all migrations, count # in executing, succeeded, failed, canceled
			for _, m := range migrations.Items {
				if m.Status.HasCondition(Succeeded) {
					switch m.Spec.Plan.Name {
					case api.Ova.String():
						succeededOVA++
						continue
					case api.OVirt.String():
						succeededRHV++
						continue
					case api.VSphere.String():
						succeededVsphere++
						continue
					case api.OpenShift.String():
						succeededOCP++
						continue
					case api.OpenStack.String():
						succeededOpenstack++
						continue
					}
				}
				if m.Status.HasCondition(Failed) {
					switch m.Spec.Plan.Name {
					case api.Ova.String():
						failedOVA++
						continue
					case api.OVirt.String():
						failedRHV++
						continue
					case api.VSphere.String():
						failedVsphere++
						continue
					case api.OpenShift.String():
						failedOCP++
						continue
					case api.OpenStack.String():
						failedOpenstack++
						continue
					}
				}
			}

			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": "oVirt"}).Set(succeededRHV)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": "OpenShift"}).Set(succeededOCP)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": "OpenStack"}).Set(succeededOpenstack)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": "OVA"}).Set(succeededOVA)
			migrationGauge.With(
				prometheus.Labels{"status": Succeeded, "provider": "vSphere"}).Set(succeededVsphere)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": "oVirt"}).Set(failedRHV)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": "OpenShift"}).Set(failedOCP)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": "OpenStack"}).Set(succeededOpenstack)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": "OVA"}).Set(succeededOVA)
			migrationGauge.With(
				prometheus.Labels{"status": Failed, "provider": "vSphere"}).Set(succeededVsphere)
		}
	}()
}
