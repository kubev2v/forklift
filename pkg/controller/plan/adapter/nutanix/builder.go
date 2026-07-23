package nutanix

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type Builder struct {
	*plancontext.Context
}

func (r *Builder) Secret(_ ref.Ref, _, _ *core.Secret) error {
	return nil
}

func (r *Builder) ConfigMap(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap) error {
	return nil
}

func (r *Builder) VirtualMachine(_ ref.Ref, _ *cnv.VirtualMachineSpec, _ []*core.PersistentVolumeClaim, _ bool, _ bool) error {
	// TODO: map disks, firmware, networks, CPU, and memory from Nutanix inventory
	return nil
}

func (r *Builder) DataVolumes(_ ref.Ref, _ *core.Secret, _ *core.ConfigMap, _ *cdi.DataVolume, _ *core.ConfigMap) (dvs []cdi.DataVolume, err error) {
	// TODO: build CDI HTTP import DataVolumes from catalog image file URLs
	return nil, nil
}

func (r *Builder) Tasks(_ ref.Ref) (tasks []*plan.Task, err error) {
	// TODO: build per-disk progress tasks from inventory (skip CD-ROM)
	return nil, nil
}

func (r *Builder) TemplateLabels(_ ref.Ref) (labels map[string]string, err error) {
	labels = make(map[string]string)
	return
}

func (r *Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	if dv == nil || dv.Annotations == nil {
		return ""
	}
	return dv.Annotations[planbase.AnnDiskSource]
}

func (r *Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	if pvc == nil || pvc.Annotations == nil {
		return ""
	}
	return pvc.Annotations[planbase.AnnDiskSource]
}

func (r *Builder) PodEnvironment(_ ref.Ref, _ *core.Secret) (env []core.EnvVar, err error) {
	return nil, nil
}

func (r *Builder) LunPersistentVolumes(_ ref.Ref) (pvs []core.PersistentVolume, err error) {
	return
}

func (r *Builder) LunPersistentVolumeClaims(_ ref.Ref) (pvcs []core.PersistentVolumeClaim, err error) {
	return
}

func (r *Builder) SupportsVolumePopulators() bool {
	return false
}

func (r *Builder) PopulatorVolumes(_ ref.Ref, _ map[string]string, _ string) ([]*core.PersistentVolumeClaim, error) {
	return nil, planbase.VolumePopulatorNotSupportedError
}

func (r *Builder) PopulatorTransferredBytes(_ *core.PersistentVolumeClaim) (int64, error) {
	return 0, planbase.VolumePopulatorNotSupportedError
}

func (r *Builder) PopulatorOffloadInfo(_ *core.PersistentVolumeClaim) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *Builder) PopulatorXcopyUsed(_ *core.PersistentVolumeClaim) (string, bool, error) {
	return "", false, nil
}

func (r *Builder) SetPopulatorDataSourceLabels(_ ref.Ref, _ []*core.PersistentVolumeClaim) error {
	return nil
}

func (r *Builder) GetPopulatorTaskName(_ *core.PersistentVolumeClaim) (string, error) {
	return "", nil
}

func (r *Builder) PreferenceName(_ ref.Ref, _ *core.ConfigMap) (string, error) {
	return "", nil
}

func (r *Builder) ConfigMaps(_ ref.Ref) (list []core.ConfigMap, err error) {
	return nil, nil
}

func (r *Builder) Secrets(_ ref.Ref) (list []core.Secret, err error) {
	return nil, nil
}

func (r *Builder) ConversionPodConfig(_ ref.Ref) (*planbase.ConversionPodConfigResult, error) {
	return &planbase.ConversionPodConfigResult{}, nil
}

func (r *Builder) NetAppShiftPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) CsiImportPVCs(_ ref.Ref, _ map[string]string) ([]core.PersistentVolumeClaim, error) {
	return nil, nil
}

func (r *Builder) SourceVMLabelsAndAnnotations(_ ref.Ref, _ *api.TagMapping) (labels map[string]string, annotations map[string]string, sanitizationReport map[string]string, err error) {
	// TODO: map Nutanix categories to destination labels/annotations
	return
}
