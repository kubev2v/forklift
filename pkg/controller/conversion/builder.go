package conversion

import (
	"context"
	"maps"
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/settings"
	"gopkg.in/yaml.v2"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	qemuUser  = int64(107)
	qemuGroup = int64(107)
)

// ConversionParams holds the inputs needed to create a Conversion CR.
type ConversionParams struct {
	Migration *api.Migration

	Namespace     string
	GenerateName  string
	Labels        map[string]string
	PlanName      string
	PlanNamespace string
	PlanID        string
	VM            *plan.VMStatus
	Provider      core.ObjectReference
	Type          api.ConversionType
	Disks         []api.DiskRef
	Connection    api.Connection
	Image         string
	Settings      map[string]string

	VDDKImage      string
	RequestKVM     bool
	LocalMigration bool
	UDN            bool
	PodSettings    api.PodSettings
}

// Builder constructs virt-v2v pod specs from a fully-resolved PodConfig.
// Callers must populate all PodConfig fields before invoking the builder.
type Builder struct {
	Config convctx.PodConfig
}

// BuildVirtV2vPod is the main entry point that builds a complete pod
// for either conversion or inspection, dispatching to the type-specific
// builder for additional settings. All data comes from b.Config.
func (b *Builder) BuildVirtV2vPod(vm *plan.VMStatus, volumes []core.Volume, volumeMounts []core.VolumeMount, volumeDevices []core.VolumeDevice, v2vSecret *core.Secret, podType int, inPlace bool) (pod *core.Pod, err error) {
	pod, environment, err := b.GetVirtV2vPodSpec(vm, volumes, volumeMounts, volumeDevices, v2vSecret, inPlace)
	if err != nil {
		return nil, err
	}

	environment, err = b.BuildV2vPodEnvironment(environment, vm)
	if err != nil {
		return nil, err
	}

	switch podType {
	case convctx.VirtV2vConversionPod:
		err = b.BuildVirtV2vConversionPod(pod, environment, vm)
	case convctx.VirtV2vInspectionPod:
		pod, err = b.BuildVirtV2vInspectionPod(pod, environment, vm)
	}

	return
}

// GetVirtV2vPodSpec builds the bare-bones pod spec. All pod-construction
// parameters are read from b.Config which must be fully resolved by the
// caller (ensurer) before invoking the builder.
func (b *Builder) GetVirtV2vPodSpec(vm *plan.VMStatus, volumes []core.Volume, volumeMounts []core.VolumeMount, volumeDevices []core.VolumeDevice, v2vSecret *core.Secret, inPlace bool) (pod *core.Pod, environment []core.EnvVar, err error) {
	cfg := &b.Config

	fsGroup := qemuGroup
	user := qemuUser
	nonRoot := true
	allowPrivilegeEscalation := false

	volumes = append(volumes, core.Volume{
		Name: "secret-volume",
		VolumeSource: core.VolumeSource{
			Secret: &core.SecretVolumeSource{SecretName: v2vSecret.Name},
		},
	})
	volumeMounts = append(volumeMounts, core.VolumeMount{
		Name:      "secret-volume",
		ReadOnly:  true,
		MountPath: "/etc/secret",
	})

	if cfg.ConversionTempStorageClass != "" && cfg.ConversionTempStorageSize != "" {
		storageClass := cfg.ConversionTempStorageClass
		volumeMode := core.PersistentVolumeFilesystem
		volumes = append(volumes, core.Volume{
			Name: "conversion-temp-storage",
			VolumeSource: core.VolumeSource{
				Ephemeral: &core.EphemeralVolumeSource{
					VolumeClaimTemplate: &core.PersistentVolumeClaimTemplate{
						Spec: core.PersistentVolumeClaimSpec{
							AccessModes:      []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
							StorageClassName: &storageClass,
							VolumeMode:       &volumeMode,
							Resources: core.VolumeResourceRequirements{
								Requests: core.ResourceList{
									core.ResourceStorage: resource.MustParse(cfg.ConversionTempStorageSize),
								},
							},
						},
					},
				},
			},
		})
		volumeMounts = append(volumeMounts, core.VolumeMount{
			Name:      "conversion-temp-storage",
			MountPath: "/var/tmp/virt-v2v",
		})
		environment = append(environment, core.EnvVar{Name: "TMPDIR", Value: "/var/tmp/virt-v2v"})
	}

	if inPlace {
		environment = append(environment, core.EnvVar{Name: "V2V_inPlace", Value: "1"})
	}

	var initContainers []core.Container
	if cfg.VDDKImage != "" {
		initContainers = append(initContainers, core.Container{
			Name:            "vddk-side-car",
			Image:           cfg.VDDKImage,
			ImagePullPolicy: core.PullIfNotPresent,
			VolumeMounts: []core.VolumeMount{
				{Name: convctx.VddkVolumeName, MountPath: "/opt"},
			},
			Resources: core.ResourceRequirements{
				Requests: core.ResourceList{
					core.ResourceCPU:    resource.MustParse("100m"),
					core.ResourceMemory: resource.MustParse("150Mi"),
				},
				Limits: core.ResourceList{
					core.ResourceCPU:    resource.MustParse("1000m"),
					core.ResourceMemory: resource.MustParse("500Mi"),
				},
			},
			SecurityContext: &core.SecurityContext{
				AllowPrivilegeEscalation: &allowPrivilegeEscalation,
				Capabilities:             &core.Capabilities{Drop: []core.Capability{"ALL"}},
			},
		})
	}

	annotations := map[string]string{}
	if cfg.TransferNetworkAnnotations != nil {
		maps.Copy(annotations, cfg.TransferNetworkAnnotations)
	}
	if cfg.UDN {
		ports := []convctx.OpenPort{
			{Protocol: "tcp", Port: 2112},
			{Protocol: "tcp", Port: 8080},
		}
		var yamlPorts []byte
		yamlPorts, err = yaml.Marshal(ports)
		if err != nil {
			return
		}
		annotations[convctx.AnnOpenDefaultPorts] = string(yamlPorts)
	}

	seccompProfile := core.SeccompProfile{Type: core.SeccompProfileTypeRuntimeDefault}
	if settings.Settings.OpenShift {
		unshare := "profiles/unshare.json"
		seccompProfile = core.SeccompProfile{
			Type:             core.SeccompProfileTypeLocalhost,
			LocalhostProfile: &unshare,
		}
	}

	podLabels := make(map[string]string)
	if cfg.PodLabels != nil {
		maps.Copy(podLabels, cfg.PodLabels)
	}

	var podNodeSelector map[string]string
	if cfg.PodNodeSelector != nil {
		podNodeSelector = make(map[string]string)
		maps.Copy(podNodeSelector, cfg.PodNodeSelector)
	}
	if cfg.PodAnnotations != nil {
		maps.Copy(annotations, cfg.PodAnnotations)
	}

	pod = &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Namespace:       cfg.TargetNamespace,
			Annotations:     annotations,
			Labels:          podLabels,
			OwnerReferences: cfg.OwnerReferences,
		},
		Spec: core.PodSpec{
			SecurityContext: &core.PodSecurityContext{
				FSGroup:        &fsGroup,
				RunAsUser:      &user,
				RunAsNonRoot:   &nonRoot,
				SeccompProfile: &seccompProfile,
			},
			NodeSelector:   podNodeSelector,
			Affinity:       cfg.Affinity,
			RestartPolicy:  core.RestartPolicyNever,
			InitContainers: initContainers,
			Containers: []core.Container{
				{
					Env:             nil, // set by type-specific builder
					ImagePullPolicy: core.PullAlways,
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(Settings.Migration.VirtV2vContainerRequestsCpu),
							core.ResourceMemory: resource.MustParse(Settings.Migration.VirtV2vContainerRequestsMemory),
						},
						Limits: core.ResourceList{
							core.ResourceCPU:    resource.MustParse(Settings.Migration.VirtV2vContainerLimitsCpu),
							core.ResourceMemory: resource.MustParse(Settings.Migration.VirtV2vContainerLimitsMemory),
						},
					},
					EnvFrom: []core.EnvFromSource{
						{
							Prefix: "V2V_",
							SecretRef: &core.SecretEnvSource{
								LocalObjectReference: core.LocalObjectReference{Name: v2vSecret.Name},
							},
						},
					},
					Image:         convctx.GetVirtV2vImage(cfg),
					VolumeMounts:  volumeMounts,
					VolumeDevices: volumeDevices,
					Ports: []core.ContainerPort{
						{Name: "metrics", ContainerPort: 2112, Protocol: core.ProtocolTCP},
					},
					SecurityContext: &core.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivilegeEscalation,
						Capabilities:             &core.Capabilities{Drop: []core.Capability{"ALL"}},
					},
				},
			},
			Volumes: volumes,
		},
	}

	if sa := convctx.ResolveServiceAccount(cfg); sa != "" {
		pod.Spec.ServiceAccountName = sa
	}
	setKvmOnPodSpec(&pod.Spec, cfg.RequestKVM)

	return
}

// setKvmOnPodSpec adds KVM device request and schedulable node selector
// when requestKVM is true.
func setKvmOnPodSpec(podSpec *core.PodSpec, requestKVM bool) {
	if !requestKVM {
		return
	}
	if podSpec.NodeSelector == nil {
		podSpec.NodeSelector = make(map[string]string)
	}
	podSpec.NodeSelector["kubevirt.io/schedulable"] = "true"
	container := &podSpec.Containers[0]
	if container.Resources.Limits == nil {
		container.Resources.Limits = make(map[core.ResourceName]resource.Quantity)
	}
	container.Resources.Limits["devices.kubevirt.io/kvm"] = resource.MustParse("1")
	if container.Resources.Requests == nil {
		container.Resources.Requests = make(map[core.ResourceName]resource.Quantity)
	}
	container.Resources.Requests["devices.kubevirt.io/kvm"] = resource.MustParse("1")
}

// BuildVirtV2vConversionPod applies conversion-specific settings to a pod.
func (b *Builder) BuildVirtV2vConversionPod(pod *core.Pod, environment []core.EnvVar, vm *plan.VMStatus) error {
	pod.GenerateName = b.Config.GenerateName
	pod.Labels[convctx.LabelApp] = "virt-v2v"
	pod.Spec.Containers[0].Name = "virt-v2v"
	pod.Spec.Containers[0].Env = environment
	return nil
}

// BuildVirtV2vInspectionPod applies inspection-specific settings to a pod.
// Inspection env vars must be pre-populated in b.Config.Environment by the caller.
func (b *Builder) BuildVirtV2vInspectionPod(pod *core.Pod, environment []core.EnvVar, vm *plan.VMStatus) (*core.Pod, error) {
	pod.GenerateName = b.Config.GenerateName + "inspection-"
	pod.Labels[convctx.LabelApp] = "virt-v2v-inspection"
	pod.Spec.Containers[0].Name = "virt-v2v-inspection"
	pod.Spec.Containers[0].Env = environment
	return pod, nil
}

// BuildV2vPodEnvironment appends pre-resolved env vars from PodConfig,
// then adds common variables (memSize, smp, LOCAL_MIGRATION).
func (b *Builder) BuildV2vPodEnvironment(env []core.EnvVar, vm *plan.VMStatus) ([]core.EnvVar, error) {
	env = append(env, b.Config.Environment...)

	if settings.Settings.Migration.VirtV2vMemSize > 0 {
		env = append(env, core.EnvVar{
			Name:  "V2V_memSize",
			Value: strconv.Itoa(settings.Settings.Migration.VirtV2vMemSize),
		})
	}
	if settings.Settings.Migration.VirtV2vSmp > 0 {
		env = append(env, core.EnvVar{
			Name:  "V2V_smp",
			Value: strconv.Itoa(settings.Settings.Migration.VirtV2vSmp),
		})
	}
	env = append(env, core.EnvVar{
		Name:  "LOCAL_MIGRATION",
		Value: strconv.FormatBool(b.Config.LocalMigration),
	})
	return env, nil
}

// ensurePod creates the virt-v2v pod for the Conversion CR if it does
// not already exist and updates the status phase from the pod state.
func (r *Reconciler) ensurePod(ctx context.Context, conversion *api.Conversion) (err error) {
	err = EnsureCRPod(r.Client, r.Log, conversion)
	if err != nil {
		return
	}

	pod, err := r.getPod(ctx, conversion)
	if err != nil {
		return
	}
	if pod == nil {
		return
	}

	conversion.Status.Pod = core.ObjectReference{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}

	switch pod.Status.Phase {
	case core.PodSucceeded:
		conversion.Status.Phase = api.PhaseSucceeded
	case core.PodFailed:
		conversion.Status.Phase = api.PhaseFailed
	case core.PodRunning:
		conversion.Status.Phase = api.PhaseRunning
	default:
		conversion.Status.Phase = api.PhaseCreating
	}
	return
}

// getPod returns the managed pod for the conversion, if it exists.
func (r *Reconciler) getPod(ctx context.Context, conversion *api.Conversion) (*core.Pod, error) {
	list := &core.PodList{}
	err := r.Client.List(ctx, list,
		client.InNamespace(conversion.Namespace),
		client.MatchingLabels{convctx.LabelConversion: conversion.Name},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil
}
