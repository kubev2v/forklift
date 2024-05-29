package forklift_controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	Succeeded = "Succeeded"
	Failed    = "Failed"
	Warm      = "Warm"
	Cold      = "Cold"
	Local     = "Local"
	Remote    = "Remote"
)

var (
	// 'status' - [ succeeded, failed ]
	// 'provider' - [oVirt, vSphere, Openstack, OVA, openshift]
	migrationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_migrations",
		Help: "VM Migrations sorted by status and provider type",
	},
		[]string{"status", "provider"},
	)
)

var (
	// 'status' - [ succeeded, failed ]
	// 'provider' - [oVirt, vSphere, Openstack, OVA, openshift]
	planStatusGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_plans",
		Help: "VM migration Plans sorted by status and provider type",
	},
		[]string{
			"status",
			"provider",
		},
	)

	// 'type' - [ cold, warm ]
	planTypeGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_plans_type",
		Help: "VM migration Plans type",
	},
		[]string{
			"type",
		},
	)

	// 'destination' - [remote, local]
	planDestinationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_operator_destination",
		Help: "MTV operator destination",
	},
		[]string{
			"destination",
		},
	)
)
