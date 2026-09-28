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
	Live      = "Live"
	Local     = "Local"
	Remote    = "Remote"
)

var (
	// 'status' - [ Succeeded, Failed, Canceled]
	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm, Live]
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
	// 'mode' - [Cold, Warm, Live]
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
	// 'mode' - [Cold, Warm, Live]
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
	// 'mode' - [Cold, Warm, Live]
	// 'target' - [Local, Remote]
	// 'plan' - [Id]
	migrationDurationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_migration_duration_seconds",
		Help: "Duration of VM migrations in seconds",
	},
		[]string{"provider", "mode", "target", "plan"},
	)

	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm, Live]
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
	// 'mode' - [Cold, Warm, Live]
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
	// 'mode' - [Cold, Warm, Live]
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

	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm, Live]
	// 'target' - [Local, Remote]
	plannedVMsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mtv_planned_vms_total",
		Help: "Number of VMs listed in migration plan specs",
	},
		[]string{
			"provider",
			"mode",
			"target",
		},
	)

	// 'status' - [Succeeded, Failed, Canceled]
	// 'provider' - [oVirt, VSphere, Openstack, OVA, Openshift]
	// 'mode' - [Cold, Warm, Live]
	// 'target' - [Local, Remote]
	migratedVMsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mtv_migrated_vms_total",
		Help: "Individual VM migration outcomes sorted by status, provider, mode and target",
	},
		[]string{
			"status",
			"provider",
			"mode",
			"target",
		},
	)

	// 'result' - [success, failure]
	// 'migration' - [Migration UID]
	// 'owner_uid' - [PVC UID]
	// 'storage_vendor' - [ontap, powermax, ...]
	// 'clone_method' - [vib, vddk]
	// 'xcopy_used' - [0, 1]
	// 'storage_protocol' - [iscsi, fc, nfs]
	// 'vib_version' - [VIB version string, "unknown"]
	xcopyDurationGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_vsphere_xcopy_volume_populator_copy_duration_seconds",
		Help: "Duration of copy-offload operation in seconds per disk",
	},
		[]string{
			"result",
			"migration",
			"owner_uid",
			"storage_vendor",
			"clone_method",
			"xcopy_used",
			"storage_protocol",
			"vib_version",
		},
	)

	// 'result' - [success, failure]
	// 'migration' - [Migration UID]
	// 'owner_uid' - [PVC UID]
	// 'storage_vendor' - [ontap, powermax, ...]
	// 'clone_method' - [vib, vddk]
	// 'xcopy_used' - [0, 1]
	// 'storage_protocol' - [iscsi, fc, nfs]
	// 'vib_version' - [VIB version string, "unknown"]
	// 'type' - [provisioned, datastore_allocated]
	xcopySourceDiskBytesGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtv_vsphere_xcopy_volume_populator_source_disk_bytes",
		Help: "Source disk size in bytes per disk. type=provisioned is guest-visible size; type=datastore_allocated is actual data on datastore",
	},
		[]string{
			"result",
			"migration",
			"owner_uid",
			"storage_vendor",
			"clone_method",
			"xcopy_used",
			"storage_protocol",
			"vib_version",
			"type",
		},
	)
)
