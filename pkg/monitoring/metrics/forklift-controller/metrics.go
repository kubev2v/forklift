package forklift_controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	Succeeded = "Succeeded"
	Failed    = "Failed"
	Executing = "Executing"
	Running   = "Running"
	Pending   = "Pending"
	Canceled  = "Canceled"
	Completed = "Completed"
	Blocked   = "Blocked"
	Ready     = "Ready"
	Deleted   = "Deleted"
	Warm      = "Warm"
	Cold      = "Cold"
	Local     = "Local"
	Remote    = "Remote"
)

var (
	// 'status' - [ Succeeded, Failed, Canceled]
	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	migrationStatusCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mtv_migrations_status_total",
		Help: "VM Migrations sorted by status, provider, mode and target",
	},
		[]string{
			"status",
			"provider",
			"mode",
			"target",
		},
	)

	// 'status' - [ Succeeded, Failed, Executing, Running, Pending, Canceled, Blocked, Deleted]
	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	planStatusGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_plans_status",
		Help: "VM migration Plans sorted by status, provider, mode and target",
	},
		[]string{
			"status",
			"provider",
			"mode",
			"target",
		},
	)

	// 'status' - [Succeeded, Failed]
	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	// 'plan' - [Id]
	// 'plan_name' - [Plan name]
	// 'phase' - [Plan phase]
	planAlertStatusGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_plan_alert_status",
		Help: "VM Migration plan statuses for alerting",
	},
		[]string{
			"status",
			"provider",
			"mode",
			"target",
			"plan",
			"plan_name",
			"phase",
		},
	)

	// 'status' - [ Succeeded, Failed, Executing, Canceled]
	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	// 'plan' - [Id]
	migrationDurationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_migration_duration_seconds",
		Help: "Duration of VM migrations in seconds",
	},
		[]string{"provider", "mode", "target", "plan"},
	)

	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	// 'plan' - [Id]
	dataTransferredGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_migration_data_transferred_bytes",
		Help: "Total data transferred during VM migrations in bytes",
	},
		[]string{
			"provider",
			"mode",
			"target",
			"plan",
		},
	)

	// 'status' - [ Succeeded, Failed, Canceled]
	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	// 'plan' - [Id]
	migrationPlanCorrelationStatusCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mtv_workload_migrations_status_total",
		Help: "VM Migrations status by provider, mode, target and plan",
	},
		[]string{
			"status",
			"provider",
			"mode",
			"target",
			"plan",
		},
	)

	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm]
	// 'target' - [Local, Remote]
	migrationDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mtv_migrations_duration_seconds",
		Help:    "Histogram of VM migrations duration in seconds",
		Buckets: []float64{1 * 3600, 2 * 3600, 5 * 3600, 10 * 3600, 24 * 3600, 48 * 3600}, // 1, 2, 5, 10, 24, 48 hours in seconds
	},
		[]string{
			"provider",
			"mode",
			"target",
		},
	)
)
