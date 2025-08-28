package settings

import (
	"os"

	"github.com/hashicorp/go-version"
)

// Environment Variables
const (
	FeatureOvirtWarmMigration        = "FEATURE_OVIRT_WARM_MIGRATION"
	FeatureRetainPrecopyImporterPods = "FEATURE_RETAIN_PRECOPY_IMPORTER_PODS"
	FeatureVsphereIncrementalBackup  = "FEATURE_VSPHERE_INCREMENTAL_BACKUP"
	FeatureCopyOffload               = "FEATURE_COPY_OFFLOAD"
	FeatureOCPLiveMigration          = "FEATURE_OCP_LIVE_MIGRATION"
	FeatureVmwareSystemSerialNumber  = "FEATURE_VMWARE_SYSTEM_SERIAL_NUMBER"
)

// OpenShift version where the FeatureVmwareSystemSerialNumber feature is supported:
//   - https://issues.redhat.com/browse/CNV-64582
//   - https://issues.redhat.com/browse/MTV-2988
const ocpMinForVmwareSystemSerial = "4.20.0"

// OpenShift version where the defined MAC address is supported in User Defined Network:
//   - https://issues.redhat.com/browse/CNV-66820
const ocpMinForUdnMacSupport = "4.20.0"

// Feature gates.
type Features struct {
	// Whether migration is supported from oVirt sources.
	OvirtWarmMigration bool
	// Whether importer pods should be retained during warm migration.
	// Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=2016290
	RetainPrecopyImporterPods bool
	// Whether to use changeID-based incremental backup workflow (with a version of CDI that supports it)
	VsphereIncrementalBackup bool
	// Where to use copy offload plugins
	CopyOffload bool
	// Whether to enable support for OCP cross-cluster live migration.
	OCPLiveMigration bool
	// Whether to use VMware system serial number for VM migration from VMware.
	VmwareSystemSerialNumber bool
	// Whether to create VMs with MAC address with the User Defined Network
	UdnSupportsMac bool
}

// isOpenShiftVersionAboveMinimum checks if OpenShift version is above or equal to minimum version using semantic versioning
func (r *Features) isOpenShiftVersionAboveMinimum(minimumVersion string) bool {
	openshiftVersionStr := os.Getenv(OpenShiftVersion)
	if openshiftVersionStr == "" {
		return false
	}

	// Parse the OpenShift version
	openshiftVersion, err := version.NewVersion(openshiftVersionStr)
	if err != nil {
		return false
	}

	// Parse the minimum version
	minVersion, err := version.NewVersion(minimumVersion)
	if err != nil {
		return false
	}

	return openshiftVersion.GreaterThanOrEqual(minVersion)
}

// Load settings.
func (r *Features) Load() (err error) {
	r.OvirtWarmMigration = getEnvBool(FeatureOvirtWarmMigration, false)
	r.RetainPrecopyImporterPods = getEnvBool(FeatureRetainPrecopyImporterPods, false)
	r.VsphereIncrementalBackup = getEnvBool(FeatureVsphereIncrementalBackup, false)
	r.CopyOffload = getEnvBool(FeatureCopyOffload, false)
	r.OCPLiveMigration = getEnvBool(FeatureOCPLiveMigration, false)
	r.VmwareSystemSerialNumber = getEnvBool(FeatureVmwareSystemSerialNumber, true) && r.isOpenShiftVersionAboveMinimum(ocpMinForVmwareSystemSerial)
	r.UdnSupportsMac = r.isOpenShiftVersionAboveMinimum(ocpMinForUdnMacSupport)
	return
}
