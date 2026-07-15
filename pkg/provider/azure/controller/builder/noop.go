package builder

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
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

func (r *Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []*core.PersistentVolumeClaim, usesInstanceType bool, sortVolumesByLibvirt bool) error {
	return nil
}

func (r *Builder) Tasks(vmRef ref.Ref) ([]*plan.Task, error) {
	return nil, nil
}

func (r *Builder) TemplateLabels(vmRef ref.Ref) (map[string]string, error) {
	return nil, nil
}

func (r *Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) ([]core.EnvVar, error) {
	return nil, nil
}

func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return ""
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) ([]*core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) PopulatorTransferredBytes(pvc *core.PersistentVolumeClaim) (int64, error) {
	return 0, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(vmRef ref.Ref, pvcs []*core.PersistentVolumeClaim) error {
	return nil
}

func (r *Builder) GetPopulatorTaskName(pvc *core.PersistentVolumeClaim) (string, error) {
	return "", nil
}

func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}

func (r *Builder) NetAppShiftPVCs(vmRef ref.Ref, labels map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) SourceVMLabelsAndAnnotations(vmRef ref.Ref, tagMapping *api.TagMapping) (map[string]string, map[string]string, map[string]string, error) {
	return nil, nil, nil, nil
}
