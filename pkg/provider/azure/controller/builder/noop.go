package builder

import (
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	core "k8s.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

func (r *Builder) Secret(vmRef ref.Ref, in, object *core.Secret) error {
	return nil
}

func (r *Builder) ConfigMap(vmRef ref.Ref, secret *core.Secret, object *core.ConfigMap) error {
	return nil
}

func (r *Builder) ConfigMaps(vmRef ref.Ref) ([]core.ConfigMap, error) {
	return nil, nil
}

func (r *Builder) Secrets(vmRef ref.Ref) ([]core.Secret, error) {
	return nil, nil
}

func (r *Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume, vddkConfigMap *core.ConfigMap) ([]cdi.DataVolume, error) {
	return nil, nil
}

func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return ""
}

func (r *Builder) LunPersistentVolumes(vmRef ref.Ref) ([]core.PersistentVolume, error) {
	return nil, nil
}

func (r *Builder) LunPersistentVolumeClaims(vmRef ref.Ref) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) PreferenceName(vmRef ref.Ref, configMap *core.ConfigMap) (string, error) {
	return "", nil
}

func (r *Builder) CsiImportPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) PopulatorXcopyUsed(_ *core.PersistentVolumeClaim) (string, bool, error) {
	return "", false, nil
}
