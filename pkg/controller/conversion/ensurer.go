package conversion

import (
	"context"
	"path"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateConversionCR creates a Conversion CR from fully-resolved params.
func CreateConversionCR(ctx context.Context, k8sClient client.Client, log logging.LevelLogger, params *ConversionParams) (conversion *api.Conversion, err error) {
	genName := params.GenerateName
	if genName == "" && params.PlanName != "" {
		genName = params.PlanName + "-" + params.VM.ID + "-"
	}
	labels := params.Labels
	if labels == nil && params.PlanID != "" {
		labels = map[string]string{
			convctx.LabelPlan: params.PlanID,
			convctx.LabelVM:   params.VM.ID,
		}
		if params.PlanName != "" {
			labels[convctx.LabelPlanName] = params.PlanName
		}
		if params.PlanNamespace != "" {
			labels[convctx.LabelPlanNamespace] = params.PlanNamespace
		}
		if params.Migration != nil {
			labels[convctx.LabelMigration] = string(params.Migration.UID)
		}
	}

	conversion = &api.Conversion{
		ObjectMeta: meta.ObjectMeta{
			Namespace:    params.Namespace,
			GenerateName: genName,
			Labels:       labels,
		},
		Spec: api.ConversionSpec{
			Type: params.Type,
			Provider: core.ObjectReference{
				Namespace: params.Provider.Namespace,
				Name:      params.Provider.Name,
			},
			VM:             params.VM.Ref,
			Disks:          params.Disks,
			Connection:     params.Connection,
			Image:          params.Image,
			Settings:       params.Settings,
			VDDKImage:      params.VDDKImage,
			RequestKVM:     params.RequestKVM,
			LocalMigration: params.LocalMigration,
			UDN:            params.UDN,
			PodSettings:    params.PodSettings,
		},
	}

	if params.VM.LUKS.Name != "" {
		conversion.Spec.LUKS = core.ObjectReference{
			Namespace: params.VM.LUKS.Namespace,
			Name:      params.VM.LUKS.Name,
		}
	}

	err = k8sClient.Create(ctx, conversion)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}

	log.Info(
		"Conversion CR created.",
		"conversion", path.Join(conversion.Namespace, conversion.Name),
		"type", string(params.Type),
		"vm", params.VM.String())
	return
}

// EnsureVirtV2vPod creates the conversion or inspection pod if it does
// not already exist. All inputs must be pre-resolved by the caller.
func EnsureVirtV2vPod(k8sClient client.Client, log logging.LevelLogger, vm *plan.VMStatus, volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice, secret *core.Secret, podType int, inPlace bool, cfg convctx.PodConfig) error {
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

// GetPodByLabels returns the first pod matching the given labels in the namespace, or nil.
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

// EnsureCRPod dispatches pod creation based on the Conversion CR type.
// Fully self-contained — all data is read from the Conversion spec.
func EnsureCRPod(k8sClient client.Client, log logging.LevelLogger, conversion *api.Conversion) (err error) {
	cfg := podConfigFromSpec(conversion)

	switch conversion.Spec.Type {
	case api.Remote, api.InPlace:
		err = ensureCRVirtV2vPod(k8sClient, log, conversion, cfg, convctx.VirtV2vConversionPod)
	case api.Inspection:
		log.Info(
			"Deep inspection pod creation not yet implemented.",
			"conversion", path.Join(conversion.Namespace, conversion.Name),
			"vm", conversion.Spec.VM.String())
	}
	return
}

// ensureCRVirtV2vPod creates the virt-v2v pod for a Conversion CR if
// one does not already exist. All data comes from the spec.
func ensureCRVirtV2vPod(k8sClient client.Client, log logging.LevelLogger, conversion *api.Conversion, cfg convctx.PodConfig, podType int) error {
	existing, err := getCRPod(k8sClient, conversion, cfg.PodLabels)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	secret := &core.Secret{}
	if conversion.Spec.Connection.Secret.Name != "" {
		err = k8sClient.Get(context.TODO(), types.NamespacedName{
			Namespace: conversion.Spec.Connection.Secret.Namespace,
			Name:      conversion.Spec.Connection.Secret.Name,
		}, secret)
		if err != nil {
			return liberr.Wrap(err)
		}
	}

	vm := &plan.VMStatus{}
	vm.Ref = conversion.Spec.VM

	volumes, mounts, devices := VolumesFromDiskRefs(conversion.Spec.Disks)

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

// getCRPod returns the managed pod matching the given labels, or nil.
func getCRPod(k8sClient client.Client, conversion *api.Conversion, labels map[string]string) (*core.Pod, error) {
	list := &core.PodList{}
	err := k8sClient.List(context.TODO(), list,
		client.InNamespace(conversion.Namespace),
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

// podConfigFromSpec builds a fully-resolved PodConfig from the Conversion CR spec.
func podConfigFromSpec(conversion *api.Conversion) convctx.PodConfig {
	ns := conversion.Spec.TargetNamespace
	if ns == "" {
		ns = conversion.Namespace
	}

	cfg := convctx.PodConfig{
		TargetNamespace: ns,
		Image:           conversion.Spec.Image,
		XfsCompatibility: conversion.Spec.XfsCompatibility,
		VDDKImage:        conversion.Spec.VDDKImage,
		RequestKVM:       conversion.Spec.RequestKVM,
		LocalMigration:   conversion.Spec.LocalMigration,
		UDN:              conversion.Spec.UDN,
	}

	ps := conversion.Spec.PodSettings
	cfg.TransferNetworkAnnotations = ps.TransferNetworkAnnotations
	cfg.PodAnnotations = ps.Annotations
	cfg.PodNodeSelector = ps.NodeSelector
	cfg.Affinity = ps.Affinity
	if ps.ServiceAccount != "" {
		cfg.ServiceAccount = ps.ServiceAccount
	}

	cfg.GenerateName = ps.GenerateName
	if cfg.GenerateName == "" {
		cfg.GenerateName = conversion.Name + "-"
	}

	cfg.OwnerReferences = []meta.OwnerReference{
		{
			APIVersion: conversion.APIVersion,
			Kind:       conversion.Kind,
			Name:       conversion.Name,
			UID:        conversion.UID,
		},
	}

	cfg.Disks = conversion.Spec.Disks

	podLabels := make(map[string]string)
	if ps.Labels != nil {
		for k, v := range ps.Labels {
			podLabels[k] = v
		}
	}
	podLabels[convctx.LabelConversion] = conversion.Name
	for _, k := range []string{convctx.LabelPlan, convctx.LabelPlanName, convctx.LabelPlanNamespace, convctx.LabelMigration, convctx.LabelVM} {
		if v, ok := conversion.Labels[k]; ok {
			podLabels[k] = v
		}
	}
	cfg.PodLabels = podLabels

	env := make([]core.EnvVar, 0, len(conversion.Spec.Settings))
	for k, v := range conversion.Spec.Settings {
		env = append(env, core.EnvVar{Name: k, Value: v})
	}
	cfg.Environment = env

	return cfg
}

// DiskRefsFromVolumes converts pre-resolved volumes, mounts, devices and PVCs
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

// VolumesFromDiskRefs converts a slice of DiskRef into Kubernetes
// volume, volume-mount, and volume-device entries ready for a pod spec.
func VolumesFromDiskRefs(disks []api.DiskRef) (volumes []core.Volume, mounts []core.VolumeMount, devices []core.VolumeDevice) {
	for _, disk := range disks {
		if disk.Namespace == "" {
			continue
		}
		volumes = append(volumes, core.Volume{
			Name: disk.Name,
			VolumeSource: core.VolumeSource{
				PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
					ClaimName: disk.Name,
				},
			},
		})
		if disk.DevicePath != "" {
			devices = append(devices, core.VolumeDevice{
				Name:       disk.Name,
				DevicePath: disk.DevicePath,
			})
		} else if disk.MountPath != "" {
			mounts = append(mounts, core.VolumeMount{
				Name:      disk.Name,
				MountPath: disk.MountPath,
			})
		}
	}
	return
}

// DiskRefsFromNames builds DiskRef entries from disk path names.
func DiskRefsFromNames(diskNames []string) []api.DiskRef {
	refs := make([]api.DiskRef, 0, len(diskNames))
	for _, name := range diskNames {
		refs = append(refs, api.DiskRef{Name: name})
	}
	return refs
}

