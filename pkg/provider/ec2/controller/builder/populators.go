package builder

import (
	"fmt"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	ec2util "github.com/kubev2v/forklift/pkg/provider/ec2/controller/util"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// populatorConfig holds configuration for building Ec2VolumePopulator resources and PVCs.
type populatorConfig struct {
	region          string
	targetAZ        string
	secretName      string
	storageClass    string
	blockMode       core.PersistentVolumeMode
	apiGroup        string
	targetNamespace string
}

// PopulatorVolumes builds PVC specs that reference Ec2VolumePopulator CRs.
func (r *Builder) PopulatorVolumes(vmRef ref.Ref, annotations map[string]string, secretName string) (pvcs []*core.PersistentVolumeClaim, err error) {
	// This is a placeholder - actual implementation uses PopulatorVolumesWithNames
	// after creating populators with generated names
	return nil, liberr.New("use EnsurePopulatorDataVolumes instead")
}

// PopulatorVolumesWithNames builds PVC specs that reference Ec2VolumePopulator CRs.
func (r *Builder) PopulatorVolumesWithNames(vmRef ref.Ref, annotations map[string]string, secretName string, populatorNames map[string]string) (pvcs []*core.PersistentVolumeClaim, err error) {
	r.log.Info("Building PVC specs with populator references", "vm", vmRef.Name)

	snapshotMap, err := r.getSnapshotIDs(vmRef)
	if err != nil {
		r.log.Error(err, "Failed to get snapshot IDs", "vm", vmRef.Name)
		return nil, liberr.Wrap(err)
	}

	if len(snapshotMap) == 0 {
		r.log.Info("No snapshots found", "vm", vmRef.Name)
		return []*core.PersistentVolumeClaim{}, nil
	}

	config, err := r.buildPopulatorConfig(secretName)
	if err != nil {
		return nil, err
	}

	index := 0
	for volumeID, snapshotID := range snapshotMap {
		populatorName, ok := populatorNames[volumeID]
		if !ok {
			return nil, liberr.New(fmt.Sprintf("no populator name found for volume %s", volumeID))
		}

		pvc, err := r.buildPopulatorAndPVC(vmRef, volumeID, snapshotID, populatorName, index, annotations, config)
		if err != nil {
			return nil, err
		}

		pvcs = append(pvcs, pvc)
		r.log.V(1).Info("Built PVC with populator reference",
			"vm", vmRef.Name,
			"volumeID", volumeID,
			"populator", populatorName)
		index++
	}

	r.log.Info("Built all PVC specs with populator references", "vm", vmRef.Name, "count", len(pvcs))
	return pvcs, nil
}

// BuildEc2VolumePopulators builds Ec2VolumePopulator CR specs for all EBS volumes.
func (r *Builder) BuildEc2VolumePopulators(vmRef ref.Ref, secretName string) ([]*api.Ec2VolumePopulator, error) {
	r.log.Info("Building Ec2VolumePopulator specs", "vm", vmRef.Name)

	snapshotMap, err := r.getSnapshotIDs(vmRef)
	if err != nil {
		r.log.Error(err, "Failed to get snapshot IDs", "vm", vmRef.Name)
		return nil, liberr.Wrap(err)
	}

	if len(snapshotMap) == 0 {
		r.log.Info("No snapshots found", "vm", vmRef.Name)
		return []*api.Ec2VolumePopulator{}, nil
	}

	config, err := r.buildPopulatorConfig(secretName)
	if err != nil {
		return nil, err
	}

	var populators []*api.Ec2VolumePopulator
	for volumeID, snapshotID := range snapshotMap {
		populator := r.buildPopulator(vmRef, volumeID, snapshotID, config)
		populators = append(populators, populator)
	}

	r.log.Info("Built all Ec2VolumePopulator specs", "vm", vmRef.Name, "count", len(populators))
	return populators, nil
}

// buildPopulatorConfig builds populator configuration from provider settings.
func (r *Builder) buildPopulatorConfig(secretName string) (*populatorConfig, error) {
	region, err := r.getRegion()
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	targetAZ, err := r.getTargetAvailabilityZone()
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	populatorSecret := secretName
	if populatorSecret == "" {
		populatorSecret = r.Source.Secret.Name
		r.log.Info("No populator secret provided, using provider secret")
	}

	apiGroup := "forklift.konveyor.io"
	blockMode := core.PersistentVolumeBlock

	return &populatorConfig{
		region:          region,
		targetAZ:        targetAZ,
		secretName:      populatorSecret,
		storageClass:    r.getStorageClass(),
		blockMode:       blockMode,
		apiGroup:        apiGroup,
		targetNamespace: r.Plan.Spec.TargetNamespace,
	}, nil
}

// buildPopulatorAndPVC builds a PVC spec that references an Ec2VolumePopulator.
func (r *Builder) buildPopulatorAndPVC(vmRef ref.Ref, volumeID, snapshotID, populatorName string, index int, annotations map[string]string, config *populatorConfig) (*core.PersistentVolumeClaim, error) {
	volumeSizeGiB := r.getVolumeSize(volumeID, snapshotID)
	volumeSizeBytes := volumeSizeGiB * 1024 * 1024 * 1024
	pvcSize := r.calculatePVCSize(volumeSizeBytes, &config.blockMode)

	pvc := r.buildPVC(vmRef, volumeID, populatorName, index, annotations, pvcSize, &config.blockMode, config)

	return pvc, nil
}

// buildPopulator builds an Ec2VolumePopulator CR spec.
func (r *Builder) buildPopulator(vmRef ref.Ref, volumeID, snapshotID string, config *populatorConfig) *api.Ec2VolumePopulator {
	populatorLabels := r.Labeler.VMLabels(vmRef)
	populatorLabels["forklift.konveyor.io/volume-id"] = volumeID

	return &api.Ec2VolumePopulator{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("ec2-%s-%s-", vmRef.ID, volumeID),
			Namespace:    config.targetNamespace,
			Labels:       populatorLabels,
		},
		Spec: api.Ec2VolumePopulatorSpec{
			Region:                 config.region,
			TargetAvailabilityZone: config.targetAZ,
			SnapshotID:             snapshotID,
			SecretName:             config.secretName,
		},
	}
}

// buildPVC builds a PVC spec.
func (r *Builder) buildPVC(vmRef ref.Ref, volumeID, popName string, index int, annotations map[string]string, pvcSize *resource.Quantity, volumeMode *core.PersistentVolumeMode, config *populatorConfig) *core.PersistentVolumeClaim {
	pvcAnnotations := make(map[string]string)
	for k, v := range annotations {
		pvcAnnotations[k] = v
	}
	pvcAnnotations["forklift.konveyor.io/volume-id"] = volumeID
	pvcAnnotations["forklift.konveyor.io/disk-index"] = fmt.Sprintf("%d", index)
	pvcAnnotations["forklift.konveyor.io/storage.bind.immediate.requested"] = "true"

	pvcLabels := r.Labeler.VMLabels(vmRef)
	pvcLabels["forklift.konveyor.io/volume-id"] = volumeID

	return &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-disk-", vmRef.Name),
			Namespace:    config.targetNamespace,
			Labels:       pvcLabels,
			Annotations:  pvcAnnotations,
		},
		Spec: core.PersistentVolumeClaimSpec{
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			VolumeMode: volumeMode,
			Resources: core.VolumeResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceStorage: *pvcSize,
				},
			},
			StorageClassName: &config.storageClass,
			DataSourceRef: &core.TypedObjectReference{
				APIGroup: &config.apiGroup,
				Kind:     "Ec2VolumePopulator",
				Name:     popName,
			},
		},
	}
}

// getSnapshotIDs returns snapshot ID mappings for a VM.
func (r *Builder) getSnapshotIDs(vmRef ref.Ref) (map[string]string, error) {
	var vmStatus *plan.VMStatus
	for _, vm := range r.Plan.Status.Migration.VMs {
		if vm.ID == vmRef.ID {
			vmStatus = vm
			break
		}
	}

	if vmStatus == nil {
		return nil, fmt.Errorf("VM %s not found in migration status", vmRef.ID)
	}

	snapshotMap, err := ec2util.GetSnapshotIDs(vmStatus, r.log)
	if err != nil {
		return nil, err
	}

	if len(snapshotMap) == 0 {
		return nil, fmt.Errorf("no snapshot mappings found for VM %s", vmRef.Name)
	}

	return snapshotMap, nil
}

// getStorageClass returns the default storage class for EC2 volumes.
func (r *Builder) getStorageClass() string {
	return r.findStorageMapping("gp3")
}
