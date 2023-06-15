package ocp

import (
	planapi "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/plan"
	"github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1/ref"
	"github.com/konveyor/forklift-controller/pkg/controller/plan/adapter/base"
	plancontext "github.com/konveyor/forklift-controller/pkg/controller/plan/context"
	core "k8s.io/api/core/v1"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type Builder struct {
	*plancontext.Context
	macConflictsMap map[string]string
}

// ConfigMap implements base.Builder
func (*Builder) ConfigMap(vmRef ref.Ref, secret *core.Secret, object *core.ConfigMap) error {
	return nil
}

// DataVolumes implements base.Builder
func (*Builder) DataVolumes(vmRef ref.Ref, secret *core.Secret, configMap *core.ConfigMap, dvTemplate *cdi.DataVolume) (dvs []cdi.DataVolume, err error) {
	return nil, nil
}

// PersistentVolumeClaimWithSourceRef implements base.Builder
func (*Builder) PersistentVolumeClaimWithSourceRef(da interface{}, storageName *string, populatorName string, accessModes []core.PersistentVolumeAccessMode, volumeMode *core.PersistentVolumeMode) *core.PersistentVolumeClaim {
	return nil
}

// PodEnvironment implements base.Builder
func (*Builder) PodEnvironment(vmRef ref.Ref, sourceSecret *core.Secret) (env []core.EnvVar, err error) {
	return nil, nil
}

// PreTransferActions implements base.Builder
func (*Builder) PreTransferActions(c base.Client, vmRef ref.Ref) (ready bool, err error) {
	return true, nil
}

// ResolveDataVolumeIdentifier implements base.Builder
func (*Builder) ResolveDataVolumeIdentifier(dv *cdi.DataVolume) string {
	return ""
}

// ResolvePersistentVolumeClaimIdentifier implements base.Builder
func (*Builder) ResolvePersistentVolumeClaimIdentifier(pvc *core.PersistentVolumeClaim) string {
	return ""
}

// Secret implements base.Builder
func (*Builder) Secret(vmRef ref.Ref, in *core.Secret, object *core.Secret) error {
	return nil
}

// Tasks implements base.Builder
func (*Builder) Tasks(vmRef ref.Ref) ([]*planapi.Task, error) {
	return nil, nil
}

// TemplateLabels implements base.Builder
func (*Builder) TemplateLabels(vmRef ref.Ref) (labels map[string]string, err error) {
	return nil, nil
}

// VirtualMachine implements base.Builder
func (*Builder) VirtualMachine(vmRef ref.Ref, object *cnv.VirtualMachineSpec, persistentVolumeClaims []core.PersistentVolumeClaim) error {
	return nil
}
