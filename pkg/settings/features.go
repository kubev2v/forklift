package settings

// Environment Variables
const (
	FeatureOvirtWarmMigration        = "FEATURE_OVIRT_WARM_MIGRATION"
	FeatureRetainPrecopyImporterPods = "FEATURE_RETAIN_PRECOPY_IMPORTER_PODS"
	FeatureVsphereIncrementalBackup  = "FEATURE_VSPHERE_INCREMENTAL_BACKUP"
	FeatureVsphereV2vTransport       = "FEATURE_VSPHERE_VIRT_V2V_TRANSPORT"
)

// Feature gates.
type Features struct {
	// Whether migration is supported from oVirt sources.
	OvirtWarmMigration bool
	// Whether importer pods should be retained during warm migration.
	// Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=2016290
	RetainPrecopyImporterPods bool
	// Whether to use changeID-based incremental backup workflow (with a version of CDI that supports it)
	VsphereIncrementalBackup bool
	// Whether to use changeID-based incremental backup workflow (with a version of CDI that supports it)
	VsphereV2vTransport bool
}

// Load settings.
func (r *Features) Load() (err error) {
	r.OvirtWarmMigration = getEnvBool(FeatureOvirtWarmMigration, false)
	r.RetainPrecopyImporterPods = getEnvBool(FeatureRetainPrecopyImporterPods, false)
	r.VsphereIncrementalBackup = getEnvBool(FeatureVsphereIncrementalBackup, false)
	r.VsphereV2vTransport = getEnvBool(FeatureVsphereV2vTransport, false)
	return
}
