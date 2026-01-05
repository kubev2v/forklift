package kubernetes

import (
	"context"
	"fmt"

	"github.com/kubev2v/forklift/cmd/ec2-populator/internal/ebs"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	EBSCSIDriver = "ebs.csi.aws.com"
)

// PVManager handles PersistentVolume operations.
type PVManager struct {
	clientset    *kubernetes.Clientset
	storageClass string
}

// NewPVManager creates a new PV manager.
func NewPVManager(clientset *kubernetes.Clientset, storageClass string) *PVManager {
	return &PVManager{
		clientset:    clientset,
		storageClass: storageClass,
	}
}

// CreatePVForVolume creates a PV for an EBS volume.
func (m *PVManager) CreatePVForVolume(ctx context.Context, volumeInfo *ebs.VolumeInfo, pvcName, pvcNamespace string) (*core.PersistentVolume, error) {
	klog.Infof("Creating PV for volume: %s (%dGi)", volumeInfo.VolumeID, volumeInfo.Size)

	blockMode := core.PersistentVolumeBlock

	pv := &core.PersistentVolume{
		ObjectMeta: meta.ObjectMeta{
			GenerateName: fmt.Sprintf("pv-%s-", pvcName),
			Labels: map[string]string{
				"forklift.konveyor.io/pvc":      pvcName,
				"forklift.konveyor.io/snapshot": volumeInfo.SnapshotID,
			},
			Annotations: map[string]string{
				"forklift.konveyor.io/volume-id":   volumeInfo.VolumeID,
				"forklift.konveyor.io/snapshot-id": volumeInfo.SnapshotID,
			},
		},
		Spec: core.PersistentVolumeSpec{
			Capacity: core.ResourceList{
				core.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", volumeInfo.Size)),
			},
			AccessModes: []core.PersistentVolumeAccessMode{
				core.ReadWriteOnce,
			},
			VolumeMode:                    &blockMode,
			PersistentVolumeReclaimPolicy: core.PersistentVolumeReclaimDelete,
			StorageClassName:              m.storageClass,
			PersistentVolumeSource: core.PersistentVolumeSource{
				CSI: &core.CSIPersistentVolumeSource{
					Driver:       EBSCSIDriver,
					VolumeHandle: volumeInfo.VolumeID,
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": EBSCSIDriver,
					},
				},
			},
			ClaimRef: &core.ObjectReference{
				Kind:      "PersistentVolumeClaim",
				Namespace: pvcNamespace,
				Name:      pvcName,
			},
		},
	}

	createdPV, err := m.clientset.CoreV1().PersistentVolumes().Create(ctx, pv, meta.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create PV: %w", err)
	}

	klog.Infof("PV created: %s", createdPV.Name)
	return createdPV, nil
}
