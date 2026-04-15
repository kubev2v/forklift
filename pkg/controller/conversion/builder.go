package conversion

import (
	"context"
	"maps"
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
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

// Pod labels used by the conversion controller.
const (
	kConversion = "conversion"
	kApp        = "forklift.app"
	// migration label (value=UID)
	kMigration = "migration"
	// plan label (value=UID)
	kPlan = "plan"
	// plan name label (value=Plan.Name)
	kPlanName = "plan-name"
	// plan namespace label (value=Plan.Namespace)
	kPlanNamespace = "plan-namespace"
	// VM label (value=vmID)
	kVM = "vmID"
)

// VddkVolumeName is the volume name used for the VDDK library scratch space.
const VddkVolumeName = "vddk-vol-mount"

// OpenPort describes a port that should be opened for UDN networks.
type OpenPort struct {
	Protocol string `yaml:"protocol"`
	Port     int    `yaml:"port"`
}

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
}

// PodConfig holds plan-level or CR-level configuration for pod creation.
// Both the plan-driven and standalone-CR paths populate this struct,
// eliminating the Builder's dependency on api.Plan.
type PodConfig struct {
	TargetNamespace            string
	Image                      string
	XfsCompatibility           bool
	ConversionTempStorageClass string
	ConversionTempStorageSize  string
	TransferNetwork            *core.ObjectReference
	ConvertorNodeSelector      map[string]string
	ConvertorLabels            map[string]string
	ServiceAccount             string
}

// PodConfigFromPlan builds a PodConfig from an api.Plan.
func PodConfigFromPlan(p *api.Plan) PodConfig {
	return PodConfig{
		TargetNamespace:            p.Spec.TargetNamespace,
		Image:                      p.Spec.VirtV2vImage,
		XfsCompatibility:           p.Spec.XfsCompatibility,
		ConversionTempStorageClass: p.Spec.ConversionTempStorageClass,
		ConversionTempStorageSize:  p.Spec.ConversionTempStorageSize,
		TransferNetwork:            p.Spec.TransferNetwork,
		ConvertorNodeSelector:      p.Spec.ConvertorNodeSelector,
		ConvertorLabels:            p.Spec.ConvertorLabels,
		ServiceAccount:             p.Spec.ServiceAccount,
	}
}

// PodConfigFromConversion builds a PodConfig from a Conversion CR.
func PodConfigFromConversion(c *api.Conversion) PodConfig {
	ns := c.Spec.TargetNamespace
	if ns == "" {
		ns = c.Namespace
	}
	return PodConfig{
		TargetNamespace:  ns,
		Image:            c.Spec.Image,
		XfsCompatibility: c.Spec.XfsCompatibility,
	}
}

// Builder constructs virt-v2v pod specs.
type Builder struct {
	Ctx    KubevirtCtx
	Config PodConfig
}

// BuildVirtV2vPod is the main entry point that builds a complete pod
// for either conversion or inspection, dispatching to the type-specific
// builder for additional settings.
func (b *Builder) BuildVirtV2vPod(vm *plan.VMStatus, volumes []core.Volume, volumeMounts []core.VolumeMount, volumeDevices []core.VolumeDevice, v2vSecret *core.Secret, podType int, step *plan.Step, inPlace bool) (pod *core.Pod, err error) {
	pod, environment, err := b.GetVirtV2vPodSpec(vm, volumes, volumeMounts, volumeDevices, v2vSecret, inPlace)
	if err != nil {
		return nil, err
	}

	environment, err = b.BuildV2vPodEnvironment(environment, vm)
	if err != nil {
		return nil, err
	}

	switch podType {
	case VirtV2vConversionPod:
		err = b.BuildVirtV2vConversionPod(pod, environment, vm)
	case VirtV2vInspectionPod:
		pod, err = b.BuildVirtV2vInspectionPod(pod, environment, vm, step)
	}

	return
}

// GetVirtV2vPodSpec builds the bare-bones pod spec, volumes, volumeDevices et.c are already resolved by the caller
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
	vddkImage := settings.GetVDDKImage(b.Ctx.GetSourceProvider().Spec.Settings)
	if vddkImage != "" {
		initContainers = append(initContainers, core.Container{
			Name:            "vddk-side-car",
			Image:           vddkImage,
			ImagePullPolicy: core.PullIfNotPresent,
			VolumeMounts: []core.VolumeMount{
				{Name: VddkVolumeName, MountPath: "/opt"},
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
	if cfg.TransferNetwork != nil {
		err = b.Ctx.SetTransferNetwork(annotations)
		if err != nil {
			return
		}
	}
	if b.Ctx.DestinationHasUdnNetwork() {
		ports := []OpenPort{
			{Protocol: "tcp", Port: 2112},
			{Protocol: "tcp", Port: 8080},
		}
		var yamlPorts []byte
		yamlPorts, err = yaml.Marshal(ports)
		if err != nil {
			return
		}
		annotations[planbase.AnnOpenDefaultPorts] = string(yamlPorts)
	}

	seccompProfile := core.SeccompProfile{Type: core.SeccompProfileTypeRuntimeDefault}
	if settings.Settings.OpenShift {
		unshare := "profiles/unshare.json"
		seccompProfile = core.SeccompProfile{
			Type:             core.SeccompProfileTypeLocalhost,
			LocalhostProfile: &unshare,
		}
	}

	providerConfig, err := b.Ctx.ConversionPodConfig(vm.Ref)
	if err != nil {
		return nil, nil, err
	}

	podLabels := make(map[string]string)
	if providerConfig.Labels != nil {
		maps.Copy(podLabels, providerConfig.Labels)
	}

	var podNodeSelector map[string]string
	if providerConfig.NodeSelector != nil {
		podNodeSelector = make(map[string]string)
		maps.Copy(podNodeSelector, providerConfig.NodeSelector)
	}
	if cfg.ConvertorNodeSelector != nil {
		if podNodeSelector == nil {
			podNodeSelector = make(map[string]string)
		}
		maps.Copy(podNodeSelector, cfg.ConvertorNodeSelector)
	}
	if providerConfig.Annotations != nil {
		maps.Copy(annotations, providerConfig.Annotations)
	}

	pod = &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Namespace:       cfg.TargetNamespace,
			Annotations:     annotations,
			Labels:          podLabels,
			OwnerReferences: b.Ctx.OwnerReferences(),
		},
		Spec: core.PodSpec{
			SecurityContext: &core.PodSecurityContext{
				FSGroup:        &fsGroup,
				RunAsUser:      &user,
				RunAsNonRoot:   &nonRoot,
				SeccompProfile: &seccompProfile,
			},
			NodeSelector:   podNodeSelector,
			Affinity:       b.Ctx.GetConvertorAffinity(),
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
					Image:         GetVirtV2vImage(cfg),
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

	if sa := ResolveServiceAccount(cfg); sa != "" {
		pod.Spec.ServiceAccountName = sa
	}
	b.Ctx.SetKvmOnPodSpec(&pod.Spec)

	return
}

// BuildVirtV2vConversionPod applies conversion-specific settings to a pod
func (b *Builder) BuildVirtV2vConversionPod(pod *core.Pod, environment []core.EnvVar, vm *plan.VMStatus) error {
	pod.GenerateName = b.Ctx.GetGeneratedName(vm)
	pod.Spec.Containers[0].Name = "virt-v2v"
	pod.Spec.Containers[0].Env = environment

	if b.Config.ConvertorLabels != nil {
		maps.Copy(pod.Labels, b.Config.ConvertorLabels)
	}
	maps.Copy(pod.Labels, b.Ctx.ConversionLabels(vm.Ref, false))

	return nil
}

// BuildVirtV2vInspectioPod applies inspection-specific settings to a pod
func (b *Builder) BuildVirtV2vInspectionPod(pod *core.Pod, environment []core.EnvVar, vm *plan.VMStatus, step *plan.Step) (*core.Pod, error) {
	pod.GenerateName = b.Ctx.GetGeneratedName(vm) + "inspection-"
	pod.Spec.Containers[0].Name = "virt-v2v-inspection"

	maps.Copy(pod.Labels, b.Ctx.InspectionLabels(vm.Ref))

	var success bool
	environment, success, err := b.Ctx.BuildInspectionPodEnvironment(environment, vm, step)
	if err != nil {
		return nil, err
	}
	if !success {
		return nil, nil //nolint:nilnil
	}
	pod.Spec.Containers[0].Env = environment

	return pod, nil
}

// BuildV2vPodEnvironment builds provider-specific variables from KubevirtCtx.PodEnvironment, then appends common variables
func (b *Builder) BuildV2vPodEnvironment(env []core.EnvVar, vm *plan.VMStatus) ([]core.EnvVar, error) {
	providerEnv, err := b.Ctx.PodEnvironment(vm.Ref, b.Ctx.GetSourceSecret())
	if err != nil {
		return nil, err
	}
	env = append(env, providerEnv...)

	if vm.RootDisk != "" {
		env = append(env, core.EnvVar{Name: "V2V_RootDisk", Value: vm.RootDisk})
	}
	if vm.NewName != "" {
		env = append(env, core.EnvVar{Name: "V2V_NewName", Value: b.Ctx.GetNewVMName(vm)})
	}
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
		Value: strconv.FormatBool(b.Ctx.GetDestinationProvider().IsHost()),
	})
	return env, nil
}

// GetVirtV2vImage resolves the virt-v2v container image from PodConfig.
func GetVirtV2vImage(cfg *PodConfig) string {
	if cfg.Image != "" {
		return cfg.Image
	}
	if cfg.XfsCompatibility {
		if Settings.Migration.VirtV2vImageXFS != "" {
			return Settings.Migration.VirtV2vImageXFS
		}
	}
	return Settings.Migration.VirtV2vImage
}

// ResolveServiceAccount resolves the ServiceAccount for migration pods.
func ResolveServiceAccount(cfg *PodConfig) string {
	if cfg.ServiceAccount != "" {
		return cfg.ServiceAccount
	}
	return Settings.Migration.ServiceAccount
}

// ensurePod creates the virt-v2v pod for the Conversion CR if it does
// not already exist. Delegates to Ensurer.EnsurePod with Kubevirt ctx
func (r *Reconciler) ensurePod(ctx context.Context, conversion *api.Conversion) (err error) {
	crCtx, err := NewCRPodContext(r.Client, r.Log, conversion)
	if err != nil {
		return
	}

	ensurer := NewEnsurer(crCtx, PodConfigFromConversion(conversion), r.Log)
	err = ensurer.EnsurePod(conversion)
	if err != nil {
		return
	}

	pod, err := r.getPod(ctx, conversion)
	if err != nil {
		return
	}
	if pod != nil {
		conversion.Status.Pod = core.ObjectReference{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}
		conversion.Status.Phase = string(pod.Status.Phase)
	}

	return
}

// getPod returns the managed pod for the conversion, if it exists.
func (r *Reconciler) getPod(ctx context.Context, conversion *api.Conversion) (*core.Pod, error) {
	list := &core.PodList{}
	err := r.Client.List(ctx, list,
		client.InNamespace(conversion.Namespace),
		client.MatchingLabels{kConversion: conversion.Name},
	)
	if err != nil {
		return nil, liberr.Wrap(err)
	}
	if len(list.Items) > 0 {
		return &list.Items[0], nil
	}
	return nil, nil
}
