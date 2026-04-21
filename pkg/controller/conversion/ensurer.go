package conversion

import (
	"context"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
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
// DestinationProvider pointing to a remote cluster, a new client is
// built from the provider URL and its secret. Otherwise the supplied
// local client is returned unchanged.
func resolveDestinationClient(localClient client.Client, spec api.ConversionSpec) (client.Client, error) {
	if spec.DestinationProvider.Name == "" {
		return localClient, nil
	}

	provider := &api.Provider{}
	err := localClient.Get(context.TODO(), types.NamespacedName{
		Namespace: spec.DestinationProvider.Namespace,
		Name:      spec.DestinationProvider.Name,
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

// EnsurePod dispatches pod creation based on the Conversion CR type.
// All data is read from the Conversion spec. The destination client
// is used for pod and PVC operations.
func (e *Ensurer) EnsurePod(conversion *api.Conversion) error {
	cfg := convctx.PodConfigFromSpec(conversion)

	switch conversion.Spec.Type {
	case api.Remote, api.InPlace:
		return e.ensureVirtV2vPodFromSpec(conversion, cfg, convctx.VirtV2vConversionPod)
	case api.Inspection:
		return e.ensureVirtV2vPodFromSpec(conversion, cfg, convctx.VirtV2vInspectionPod)
	case api.DeepInspection:
		return e.ensureDeepInspectionPodFromSpec(conversion, cfg)
	}
	return nil
}

// ensureDeepInspectionPodFromSpec creates the deep inspection pod for a Conversion
// CR if one does not already exist.
func (e *Ensurer) ensureDeepInspectionPodFromSpec(conversion *api.Conversion, cfg convctx.PodConfig) error {
	existing, err := e.GetPod(conversion, cfg.PodLabels)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	secret := &core.Secret{}
	if conversion.Spec.Connection.Secret.Name != "" {
		err = e.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: conversion.Spec.Connection.Secret.Namespace,
			Name:      conversion.Spec.Connection.Secret.Name,
		}, secret)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	vm := &plan.VMStatus{}
	vm.Ref = conversion.Spec.VM

	volumes, mounts, devices, err := e.VolumesFromDiskRefs(conversion.Spec.Disks)
	if err != nil {
		return err
	}

	if conversion.Spec.LUKS.Name != "" {
		volumes = append(volumes, core.Volume{
			Name: "luks",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{SecretName: conversion.Spec.LUKS.Name},
			},
		})
		mounts = append(mounts, core.VolumeMount{
			Name:      "luks",
			MountPath: "/etc/luks",
			ReadOnly:  true,
		})
	}

	builder := &Builder{Config: cfg}
	pod, err := builder.BuildDeepInspectionPod(vm, volumes, mounts, devices, secret)
	if err != nil {
		return err
	}
	if pod == nil {
		e.Log.Info("Couldn't prepare deep inspection pod for vm.", "vm", vm.String())
		return nil
	}

	err = e.DestinationClient.Create(context.TODO(), pod)
	if err != nil {
		return liberr.Wrap(err)
	}

	e.Log.Info(
		"Created deep inspection pod.",
		"pod", path.Join(pod.Namespace, pod.Name),
		"vm", vm.String())
	return nil
}

// ensureVirtV2vPodFromSpec creates the virt-v2v pod for a Conversion
// CR if one does not already exist. All data comes from the spec.
func (e *Ensurer) ensureVirtV2vPodFromSpec(conversion *api.Conversion, cfg convctx.PodConfig, podType convctx.V2vPodType) error {
	existing, err := e.GetPod(conversion, cfg.PodLabels)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	secret := &core.Secret{}
	if conversion.Spec.Connection.Secret.Name != "" {
		err = e.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: conversion.Spec.Connection.Secret.Namespace,
			Name:      conversion.Spec.Connection.Secret.Name,
		}, secret)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	vm := &plan.VMStatus{}
	vm.Ref = conversion.Spec.VM

	volumes, mounts, devices, err := e.VolumesFromDiskRefs(conversion.Spec.Disks)
	if err != nil {
		return err
	}

	volumes = append(volumes, core.Volume{
		Name:         convctx.VddkVolumeName,
		VolumeSource: core.VolumeSource{EmptyDir: &core.EmptyDirVolumeSource{}},
	})
	mounts = append(mounts, core.VolumeMount{
		Name:      convctx.VddkVolumeName,
		MountPath: "/opt",
	})

	if conversion.Spec.LUKS.Name != "" {
		volumes = append(volumes, core.Volume{
			Name: "luks",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{SecretName: conversion.Spec.LUKS.Name},
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
	pod, err := builder.BuildVirtV2vPod(vm, volumes, mounts, devices, secret, podType, inPlace)
	if err != nil {
		return err
	}
	if pod == nil {
		e.Log.Info("Couldn't prepare virt-v2v pod for vm.", "vm", vm.String())
		return nil
	}

	err = e.DestinationClient.Create(context.TODO(), pod)
	if err != nil {
		return liberr.Wrap(err)
	}

	e.Log.Info(
		"Created virt-v2v pod.",
		"pod", path.Join(pod.Namespace, pod.Name),
		"vm", vm.String())
	return nil
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
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil
}

// VolumesFromDiskRefs converts a slice of DiskRef into Kubernetes
// volume, mount, and device entries ready for a pod spec. Each PVC
// is looked up on the destination cluster in the namespace specified
// in the DiskRef so that volume mode can be determined when not
// already set.
func (e *Ensurer) VolumesFromDiskRefs(disks []api.DiskRef) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, err error) {
	for _, disk := range disks {
		if disk.Namespace == "" {
			continue
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
			devPath := disk.DevicePath
			if devPath == "" {
				devPath = "/dev/block" + disk.Name
			}
			devices = append(devices, core.VolumeDevice{
				Name:       disk.Name,
				DevicePath: devPath,
			})
		} else {
			mountPath := disk.MountPath
			if mountPath == "" {
				mountPath = "/mnt/disks/" + disk.Name
			}
			mounts = append(mounts, core.VolumeMount{
				Name:      disk.Name,
				MountPath: mountPath,
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
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil
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
