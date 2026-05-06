package conversion

import (
	"context"
	"fmt"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ensurer manages pod lifecycle for a Conversion CR. It holds the
// local (host) client for CR and secret lookups, and a destination
// client for pod and PVC operations on the target cluster.
type Ensurer struct {
	// Client is the local/host k8s client used for reading CRs,
	// providers, and secrets that live on the management cluster.
	Client client.Client
	// DestinationClient is the k8s client for the cluster where pods and
	// PVCs are created. For host providers it equals Client.
	DestinationClient client.Client
	Log               logging.LevelLogger
}

// NewEnsurer builds an Ensurer for the given Conversion CR. When the
// spec references a remote destination provider a remote client is
// constructed automatically otherwise the local client is reused.
func NewEnsurer(localClient client.Client, log logging.LevelLogger, spec api.ConversionSpec) (*Ensurer, error) {
	dest, err := resolveDestinationClient(localClient, spec)
	if err != nil {
		return nil, err
	}
	return &Ensurer{
		Client:            localClient,
		DestinationClient: dest,
		Log:               log,
	}, nil
}

// resolveDestinationClient returns a k8s client for the cluster where
// pods and PVCs should be managed. When the Conversion spec has a
// Destination pointing to a remote cluster, a new client is built from
// the provider URL and its secret. Otherwise the supplied local client
// is returned unchanged.
func resolveDestinationClient(localClient client.Client, spec api.ConversionSpec) (client.Client, error) {
	if spec.Destination.Name == "" {
		return localClient, nil
	}

	provider := &api.Provider{}
	err := localClient.Get(context.TODO(), types.NamespacedName{
		Namespace: spec.Destination.Namespace,
		Name:      spec.Destination.Name,
	}, provider)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	if provider.IsHost() {
		return ocp.Client(provider, nil)
	}

	secret := &core.Secret{}
	err = localClient.Get(context.TODO(), types.NamespacedName{
		Namespace: provider.Spec.Secret.Namespace,
		Name:      provider.Spec.Secret.Name,
	}, secret)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	return ocp.Client(provider, secret)
}

// EnsurePod creates the pod for the Conversion CR if it does not already exist
// and returns it. Returns error when the conversion type is not recognised.
func (e *Ensurer) EnsurePod(conversion *api.Conversion) (*core.Pod, error) {
	cfg := convctx.PodConfigFromSpec(conversion)

	switch conversion.Spec.Type {
	case api.Remote, api.InPlace:
		return e.ensureVirtV2vPodFromSpec(conversion, cfg, convctx.VirtV2vConversionPod)
	case api.Inspection:
		return e.ensureVirtV2vPodFromSpec(conversion, cfg, convctx.VirtV2vInspectionPod)
	case api.DeepInspection:
		return e.ensureDeepInspectionPodFromSpec(conversion, cfg)
	default:
		return nil, fmt.Errorf("unsupported conversion type: %q", conversion.Spec.Type)
	}
}

// ensureDeepInspectionPodFromSpec creates the deep inspection pod for a Conversion
// CR if one does not already exist and returns it.
func (e *Ensurer) ensureDeepInspectionPodFromSpec(conversion *api.Conversion, cfg convctx.PodConfig) (pod *core.Pod, err error) {
	if conversion.Status.Snapshot != nil && conversion.Status.Snapshot.Moref != "" {
		cfg.DeepInspectionSnapshotMoref = conversion.Status.Snapshot.Moref
	}
	pod, err = e.GetPod(conversion, cfg.PodLabels)
	if err != nil {
		return nil, err
	}
	if pod != nil {
		return pod, nil
	}

	secret := &core.Secret{}
	if conversion.Spec.Connection.Secret.Name != "" {
		err = e.DestinationClient.Get(context.TODO(), types.NamespacedName{
			Namespace: conversion.Spec.Connection.Secret.Namespace,
			Name:      conversion.Spec.Connection.Secret.Name,
		}, secret)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
	}

	vm := &plan.VMStatus{}
	vm.Ref = conversion.Spec.VM

	volumes, mounts, devices, err := e.VolumesFromDiskRefs(conversion.Spec.Disks)
	if err != nil {
		return nil, err
	}

	if diskEnc := conversion.Spec.DiskEncryption; diskEnc != nil && diskEnc.Type == api.DiskEncryptionTypeLUKS && diskEnc.Secret.Name != "" {
		volumes = append(volumes, core.Volume{
			Name: "luks",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{SecretName: diskEnc.Secret.Name},
			},
		})
		mounts = append(mounts, core.VolumeMount{
			Name:      "luks",
			MountPath: "/etc/luks",
			ReadOnly:  true,
		})
	}

	builder := &Builder{Config: cfg}
	pod, err = builder.BuildDeepInspectionPod(vm, volumes, mounts, devices, secret)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		e.Log.Info("Couldn't prepare deep inspection pod for vm.", "vm", vm.String())
		return nil, nil
	}

	err = e.DestinationClient.Create(context.TODO(), pod)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	e.Log.Info(
		"Created deep inspection pod.",
		"pod", path.Join(pod.Namespace, pod.Name),
		"vm", vm.String())
	return pod, nil
}

// ensureVirtV2vPodFromSpec creates the virt-v2v pod for a Conversion
// CR if one does not already exist and returns it.
func (e *Ensurer) ensureVirtV2vPodFromSpec(conversion *api.Conversion, cfg convctx.PodConfig, podType convctx.V2vPodType) (pod *core.Pod, err error) {
	pod, err = e.GetPod(conversion, cfg.PodLabels)
	if err != nil {
		return nil, err
	}
	if pod != nil {
		return pod, nil
	}

	secret := &core.Secret{}
	if conversion.Spec.Connection.Secret.Name != "" {
		err = e.DestinationClient.Get(context.TODO(), types.NamespacedName{
			Namespace: conversion.Spec.Connection.Secret.Namespace,
			Name:      conversion.Spec.Connection.Secret.Name,
		}, secret)
		if err != nil {
			return nil, liberr.Wrap(err)
		}
	}

	vm := &plan.VMStatus{}
	vm.Ref = conversion.Spec.VM

	volumes, mounts, devices, err := e.VolumesFromDiskRefs(conversion.Spec.Disks)
	if err != nil {
		return nil, err
	}

	volumes = append(volumes, core.Volume{
		Name:         convctx.VddkVolumeName,
		VolumeSource: core.VolumeSource{EmptyDir: &core.EmptyDirVolumeSource{}},
	})
	mounts = append(mounts, core.VolumeMount{
		Name:      convctx.VddkVolumeName,
		MountPath: "/opt",
	})

	if diskEnc := conversion.Spec.DiskEncryption; diskEnc != nil && diskEnc.Type == api.DiskEncryptionTypeLUKS && diskEnc.Secret.Name != "" {
		volumes = append(volumes, core.Volume{
			Name: "luks",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{SecretName: diskEnc.Secret.Name},
			},
		})
		mounts = append(mounts, core.VolumeMount{
			Name:      "luks",
			MountPath: "/etc/luks",
			ReadOnly:  true,
		})
	}

	inPlace := conversion.Spec.Type == api.InPlace

	builder := &Builder{Config: cfg}
	pod, err = builder.BuildVirtV2vPod(vm, volumes, mounts, devices, secret, podType, inPlace)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		e.Log.Info("Couldn't prepare virt-v2v pod for vm.", "vm", vm.String())
		return nil, nil
	}

	err = e.DestinationClient.Create(context.TODO(), pod)
	if err != nil {
		return nil, liberr.Wrap(err)
	}

	e.Log.Info(
		"Created virt-v2v pod.",
		"pod", path.Join(pod.Namespace, pod.Name),
		"vm", vm.String())
	return pod, nil
}

// GetPod returns the managed pod matching the given labels, or nil.
// It searches on the destination cluster.
func (e *Ensurer) GetPod(conversion *api.Conversion, labels map[string]string) (*core.Pod, error) {
	list := &core.PodList{}
	err := e.DestinationClient.List(context.TODO(), list,
		client.InNamespace(conversion.Spec.TargetNamespace),
		client.MatchingLabels(labels),
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) == 1 {
		return &list.Items[0], nil
	} else if len(list.Items) > 1 {
		return nil, liberr.New("found multiple pods with the same labels", "labels", labels)
	} else {
		return nil, nil
	}
}

// DeletePod finds the pod that was created for the Conversion CR and deletes
// it. Returns nil when no pod is found (already gone or never created).
func (e *Ensurer) DeletePod(conversion *api.Conversion) error {
	cfg := convctx.PodConfigFromSpec(conversion)
	pod, err := e.GetPod(conversion, cfg.PodLabels)
	if err != nil || pod == nil {
		return err
	}
	if err := e.DestinationClient.Delete(context.TODO(), pod); err != nil && !k8serr.IsNotFound(err) {
		return liberr.Wrap(err)
	}
	return nil
}

// RemoveOwnedSnapshot drives async vSphere snapshot removal for a
// controller-owned DeepInspection snapshot. Returns (true, nil) when done or
// when there is nothing to remove. Returns (false, nil) while the vSphere
// task is still in flight; the caller must persist status and requeue.
func (e *Ensurer) RemoveOwnedSnapshot(ctx context.Context, conversion *api.Conversion) (bool, error) {
	if conversion.Spec.Type != api.DeepInspection {
		return true, nil
	}
	if !snapshotOwnedByController(conversion) {
		return true, nil
	}
	if conversion.Status.Snapshot == nil || conversion.Status.Snapshot.Moref == "" {
		return true, nil
	}

	if conversion.Spec.Connection.Secret.Name == "" || conversion.Spec.Connection.Secret.Namespace == "" {
		return false, liberr.New("cannot remove snapshot: connection secret not set",
			"conversion", path.Join(conversion.Namespace, conversion.Name))
	}
	secret := &core.Secret{}
	if err := e.Client.Get(ctx, types.NamespacedName{
		Namespace: conversion.Spec.Connection.Secret.Namespace,
		Name:      conversion.Spec.Connection.Secret.Name,
	}, secret); err != nil {
		return false, liberr.Wrap(err)
	}

	snap := conversion.Status.Snapshot

	snapClient, err := newSnapshotClientFromSecret(ctx, e.Log, secret, conversion.Spec.VM)
	if err != nil {
		return false, err
	}
	defer snapClient.Close()

	if snap.RemoveTaskID == "" {
		// Stage 1: submit removal task.
		taskID, err := snapClient.RemoveSnapshot(snap.Moref)
		if err != nil {
			return false, err
		}
		snap.RemoveTaskID = taskID
		e.Log.Info("Snapshot removal task submitted.",
			"moref", snap.Moref, "taskID", snap.RemoveTaskID,
			"conversion", path.Join(conversion.Namespace, conversion.Name))
		return false, nil
	}

	// Stage 2: poll for completion.
	ready, err := snapClient.CheckRemoveTaskReady(snap.RemoveTaskID)
	if err != nil {
		return false, err
	}
	if !ready {
		return false, nil
	}
	e.Log.Info("Owned snapshot removed.",
		"moref", snap.Moref,
		"conversion", path.Join(conversion.Namespace, conversion.Name))
	conversion.Status.Snapshot = nil
	return true, nil
}

// VolumesFromDiskRefs converts a slice of DiskRef into Kubernetes
// volume, mount, and device entries ready for a pod spec. Each PVC
// is looked up on the destination cluster in the namespace specified
// in the DiskRef so that volume mode can be determined when not
// already set.
func (e *Ensurer) VolumesFromDiskRefs(disks []api.DiskRef) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, err error) {
	for i, disk := range disks {
		if disk.Namespace == "" {
			err = fmt.Errorf("spec.disks[%d] (%q): namespace is required", i, disk.Name)
			return
		}

		pvc := &core.PersistentVolumeClaim{}
		err = e.DestinationClient.Get(context.TODO(), types.NamespacedName{
			Namespace: disk.Namespace,
			Name:      disk.Name,
		}, pvc)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}

		volumes = append(volumes, core.Volume{
			Name: disk.Name,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: disk.Name,
				},
			},
		})

		volumeMode := disk.VolumeMode
		if volumeMode == nil && pvc.Spec.VolumeMode != nil {
			volumeMode = pvc.Spec.VolumeMode
		}

		if volumeMode != nil && *volumeMode == core.PersistentVolumeBlock {
			devices = append(devices, core.VolumeDevice{
				Name:       disk.Name,
				DevicePath: fmt.Sprintf("/dev/block%v", i),
			})
		} else {
			mounts = append(mounts, core.VolumeMount{
				Name:      disk.Name,
				MountPath: fmt.Sprintf("/mnt/disks/disk%v", i),
			})
		}
	}
	return
}

// EnsureVirtV2vPod creates the conversion or inspection pod if it does
// not already exist. Helper funvtion for the plan pod creation.
func EnsureVirtV2vPod(k8sClient client.Client, log logging.LevelLogger, vm *plan.VMStatus, volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, secret *core.Secret, podType convctx.V2vPodType, inPlace bool, cfg convctx.PodConfig) error {
	existing, err := GetPodByLabels(k8sClient, cfg.TargetNamespace, cfg.PodLabels)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	builder := &Builder{Config: cfg}
	pod, err := builder.BuildVirtV2vPod(vm, volumes, mounts, devices, secret, podType, inPlace)
	if err != nil {
		return err
	}
	if pod == nil {
		log.Info("Couldn't prepare virt-v2v pod for vm.", "vm", vm.String())
		return nil
	}

	err = k8sClient.Create(context.TODO(), pod)
	if err != nil {
		return liberr.Wrap(err)
	}

	log.Info(
		"Created virt-v2v pod.",
		"pod", path.Join(pod.Namespace, pod.Name),
		"vm", vm.String())
	return nil
}

// GetPodByLabels returns the first pod matching the given labels in
// the namespace or nil.
func GetPodByLabels(k8sClient client.Client, namespace string, labels map[string]string) (*core.Pod, error) {
	list := &core.PodList{}
	err := k8sClient.List(context.TODO(), list,
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) == 1 {
		return &list.Items[0], nil
	} else if len(list.Items) > 1 {
		return nil, liberr.New("found multiple pods with the same labels", "labels", labels)
	} else {
		return nil, nil
	}
}

// DiskRefsFromVolumes converts resolved volumes, mounts, devices and PVCs
// into DiskRef entries for a Conversion CR spec.
func DiskRefsFromVolumes(volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, pvcs []*core.PersistentVolumeClaim) []api.DiskRef {
	mountByName := make(map[string]string, len(mounts))
	for _, m := range mounts {
		mountByName[m.Name] = m.MountPath
	}
	deviceByName := make(map[string]string, len(devices))
	for _, d := range devices {
		deviceByName[d.Name] = d.DevicePath
	}

	pvcsByName := make(map[string]*core.PersistentVolumeClaim, len(pvcs))
	for _, pvc := range pvcs {
		pvcsByName[pvc.Name] = pvc
	}

	var refs []api.DiskRef
	for _, vol := range volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvc := pvcsByName[vol.PersistentVolumeClaim.ClaimName]
		if pvc == nil {
			continue
		}
		dr := api.DiskRef{
			Name:       pvc.Name,
			Namespace:  pvc.Namespace,
			MountPath:  mountByName[vol.Name],
			DevicePath: deviceByName[vol.Name],
		}
		if pvc.Spec.VolumeMode != nil {
			mode := *pvc.Spec.VolumeMode
			dr.VolumeMode = &mode
		}
		refs = append(refs, dr)
	}
	return refs
}

// DiskRefsFromNames builds DiskRef entries from disk path names.
func DiskRefsFromNames(diskNames []string) []api.DiskRef {
	refs := make([]api.DiskRef, 0, len(diskNames))
	for _, name := range diskNames {
		refs = append(refs, api.DiskRef{Name: name})
	}
	return refs
}
