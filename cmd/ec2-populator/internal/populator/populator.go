package populator

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/kubev2v/forklift/cmd/ec2-populator/internal/client"
	"github.com/kubev2v/forklift/cmd/ec2-populator/internal/config"
	"github.com/kubev2v/forklift/cmd/ec2-populator/internal/ebs"
	"github.com/kubev2v/forklift/cmd/ec2-populator/internal/kubernetes"
)

// Populator creates EBS volumes from snapshots and binds them to PVCs.
// Flow: Create EBS volume → Create PV with EBS CSI → Prime PVC binds → Copy to target.
type Populator struct {
	config *config.Config
}

// New creates a new populator.
func New(cfg *config.Config) (*Populator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &Populator{
		config: cfg,
	}, nil
}

// Run executes the population process.
func (p *Populator) Run(ctx context.Context) error {
	klog.Infof("Starting population: %s", p.config)

	// Create AWS client for the region
	awsClient, err := client.New(ctx, p.config.Region)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Create Kubernetes client
	k8sClient, err := p.createKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	primePVCName, primePVCNamespace, storageClass, err := p.getPrimePVCInfo(ctx, k8sClient)
	if err != nil {
		return fmt.Errorf("failed to resolve prime PVC info: %w", err)
	}

	// Verify snapshot exists and is completed in the region
	snapshotMgr := ebs.NewSnapshotManager(awsClient.EC2, p.config.Region)
	snapshotInfo, err := snapshotMgr.VerifySnapshot(ctx, p.config.SnapshotID)
	if err != nil {
		return fmt.Errorf("failed to verify snapshot: %w", err)
	}

	klog.Infof("Snapshot verified: Original volume AZ=%s (informational only), Size=%dGiB, State=%s",
		snapshotInfo.AvailabilityZone, snapshotInfo.SizeGiB, snapshotInfo.State)

	// Calculate requested volume size from PVC size (with overhead)
	// PVC size is in bytes, convert to GiB (rounding up)
	requestedSizeGiB := int32((p.config.PVCSize + 1024*1024*1024 - 1) / (1024 * 1024 * 1024))

	klog.Infof("PVC requested size: %d bytes (%d GiB)", p.config.PVCSize, requestedSizeGiB)

	// IMPORTANT: Snapshots are region-wide - we can create volumes in ANY AZ within the region
	// We create the volume in the target AZ (where OpenShift workers are)
	klog.Infof("Creating volume from snapshot %s in AZ: %s (snapshot was from AZ: %s - but AZ doesn't matter for snapshots)",
		p.config.SnapshotID, p.config.TargetAvailabilityZone, snapshotInfo.AvailabilityZone)
	volumeMgr := ebs.NewVolumeManager(awsClient.EC2, p.config.Region, p.config.TargetAvailabilityZone)
	volumeID, err := volumeMgr.CreateVolumeFromSnapshot(ctx, p.config.SnapshotID, requestedSizeGiB)
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	if err := volumeMgr.WaitForVolumeAvailable(ctx, volumeID); err != nil {
		return fmt.Errorf("volume not available: %w", err)
	}

	volumeInfo, err := volumeMgr.GetVolumeInfo(ctx, volumeID)
	if err != nil {
		return fmt.Errorf("failed to get volume info: %w", err)
	}

	klog.Infof("Volume ready: %s (%dGi, %s)", volumeID, volumeInfo.Size, volumeInfo.State)

	pvMgr := kubernetes.NewPVManager(k8sClient, storageClass)
	pv, err := pvMgr.CreatePVForVolume(ctx, volumeInfo, primePVCName, primePVCNamespace)
	if err != nil {
		return fmt.Errorf("failed to create PV: %w", err)
	}

	klog.Infof("PV created: %s - population complete", pv.Name)
	return nil
}

func (p *Populator) createKubernetesClient() (*clientset.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	client, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return client, nil
}

// getPrimePVCInfo locates the PVC that triggered the populator (by UID) and derives:
//   - the prime PVC name:  prime-{ownerUID}
//   - the namespace:       taken from the triggering PVC
//   - the storage class:   taken from the triggering PVC's StorageClassName
//
// This ensures the PV we create matches the user's PVC namespace and storage class.
func (p *Populator) getPrimePVCInfo(ctx context.Context, client *clientset.Clientset) (string, string, string, error) {
	namespace := p.config.CRNamespace
	if namespace == "" {
		namespace = "default"
	}

	// Find the PVC that triggered the populator by UID.
	// populator-machinery passes the triggering PVC UID via --owner-uid.
	// Note: metadata.uid is not a supported field selector, so we list all PVCs and filter.
	pvcList, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to list PVCs in namespace %s: %w", namespace, err)
	}

	var sourcePVC *corev1.PersistentVolumeClaim
	for i := range pvcList.Items {
		if string(pvcList.Items[i].UID) == p.config.OwnerUID {
			sourcePVC = &pvcList.Items[i]
			break
		}
	}

	if sourcePVC == nil {
		return "", "", "", fmt.Errorf("PVC with UID %s not found in namespace %s", p.config.OwnerUID, namespace)
	}

	primePVCName := fmt.Sprintf("prime-%s", sourcePVC.UID)
	primePVCNamespace := sourcePVC.Namespace

	// Derive storage class from the triggering PVC, falling back to gp3 if unset.
	storageClass := "gp3"
	if sourcePVC.Spec.StorageClassName != nil && *sourcePVC.Spec.StorageClassName != "" {
		storageClass = *sourcePVC.Spec.StorageClassName
	}

	klog.Infof("Using triggering PVC %s/%s (storageClass=%s) -> prime PVC %s/%s",
		sourcePVC.Namespace, sourcePVC.Name, storageClass, primePVCNamespace, primePVCName)

	return primePVCName, primePVCNamespace, storageClass, nil
}
