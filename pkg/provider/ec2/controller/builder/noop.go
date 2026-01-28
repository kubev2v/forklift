package builder

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// ConfigMap is a no-op for EC2 - VM configuration derived from EC2 instance metadata, not ConfigMaps.
func (r *Builder) ConfigMap(vmRef ref.Ref, secret *core.Secret, object *core.ConfigMap) error {
	return nil
}

// ConfigMaps is a no-op for EC2 - KubeVirt VMs configured via VirtualMachine specs, not ConfigMaps.
func (r *Builder) ConfigMaps(vmRef ref.Ref) ([]core.ConfigMap, error) {
	return nil, nil
}

// Secrets is a no-op for EC2 - AWS credentials are in the provider secret.
func (r *Builder) Secrets(vmRef ref.Ref) ([]core.Secret, error) {
	return nil, nil
}

// LunPersistentVolumeClaims is a no-op for EC2 - uses EBS volumes, not LUNs. RDM not applicable.
func (r *Builder) LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	return
}

// LunPersistentVolumes is a no-op for EC2 - storage provisioned via CSI drivers, not static LUN-based PVs.
func (r *Builder) LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error) {
	return
}

// Secret is a no-op for EC2 - provider secrets used as-is, no transformation needed.
func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	return
}

// PreferenceName returns error for EC2 - instance types map directly to CPU/memory, preferences not needed.
func (r *Builder) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error) {
	err = liberr.New("preferences are not used by this provider")
	return
}

// DataVolumes is a no-op for EC2 - uses direct PV/PVC creation from EBS volumes instead of CDI DataVolumes.
func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) ([]cdi.DataVolume, error) {
	return nil, nil
}

// ResolveDataVolumeIdentifier is a no-op for EC2 - EC2 doesn't create DataVolumes, so nothing to resolve.
func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return ""
}
