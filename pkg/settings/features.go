package settings

//
// Environment Variables
const (
	FeatureOvirtWarmMigration = "FEATURE_OVIRT_WARM_MIGRATION"
)

//
// Feature gates.
type Features struct {
	// Whether migration is supported from oVirt sources.
	OvirtWarmMigration bool
}

//
// Load settings.
func (r *Features) Load() (err error) {
	r.OvirtWarmMigration = getEnvBool(FeatureOvirtWarmMigration, false)
	return
}
