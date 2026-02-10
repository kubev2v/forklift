package base

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// ConfigMap is a no-op for EC2.
func (r *Base) ConfigMap(vmRef ref.Ref, secret *core.Secret, object *core.ConfigMap) error {
	return nil
}

// ConfigMaps is a no-op for EC2.
func (r *Base) ConfigMaps(vmRef ref.Ref) ([]core.ConfigMap, error) {
	return nil, nil
}

// Secrets is a no-op for EC2.
func (r *Base) Secrets(vmRef ref.Ref) ([]core.Secret, error) {
	return nil, nil
}

// LunPersistentVolumeClaims is a no-op for EC2.
func (r *Base) LunPersistentVolumeClaims(vmRef ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	return
}

// LunPersistentVolumes is a no-op for EC2.
func (r *Base) LunPersistentVolumes(vmRef ref.Ref) (pvs []core.PersistentVolume, err error) {
	return
}

// Secret is a no-op for EC2.
func (r *Base) Secret(vmRef ref.Ref, in, object *core.Secret) (err error) {
	return
}

// PreferenceName returns error for EC2 -- preferences are not used.
func (r *Base) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (name string, err error) {
	err = liberr.New("preferences are not used by this provider")
	return
}

// DataVolumes is a no-op for EC2.
func (r *Base) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) ([]cdi.DataVolume, error) {
	return nil, nil
}

// ResolveDataVolumeIdentifier is a no-op for EC2.
func (r *Base) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return ""
}
