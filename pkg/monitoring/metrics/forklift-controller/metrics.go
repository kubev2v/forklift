package forklift_controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	Succeeded = "succeeded"
	Failed    = "failed"
	Executing = "executing"
	Running   = "running"
	Pending   = "pending"
	Canceled  = "canceled"
	Blocked   = "blocked"
	Ready     = "ready"
	Warm      = "warm"
	Cold      = "cold"
	Local     = "local"
	Remote    = "remote"
)

var (
	// 'status' - [ succeeded, failed ]
	// 'provider' - [oVirt, vSphere, Openstack, OVA, openshift]
	migrationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_workload_migrations",
		Help: "VM Migrations sorted by status and provider type",
	},
		[]string{
			"status",
			"provider",
		},
	)

	// 'status' - [ succeeded, failed, Executing, Running, Pending, Canceled, Blocked]
	// 'provider' - [oVirt, vSphere, Openstack, OVA, openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	planStatusCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mtv_workload_plans_status_total",
		Help: "VM migration Plans sorted by status and provider type",
	},
		[]string{
			"status",
			"provider",
			"mode",
			"target",
		},
	)
)
