package context

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/settings"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VirtV2vPod type constants.
type V2vPodType int
const (
	VirtV2vConversionPod V2vPodType = 0
	VirtV2vInspectionPod V2vPodType = 1
)

// Label keys used on Conversion CRs and their managed pods.
const (
	LabelConversion     = "conversion"
	LabelConversionType = "conversion-type"
	LabelApp            = "forklift.app"
	LabelPlan           = "plan"
	LabelPlanName       = "plan-name"
	LabelPlanNamespace  = "plan-namespace"
	LabelMigration      = "migration"
	LabelVM             = "vmID"
)

// VddkVolumeName is the volume name used for the VDDK library scratch space.
const VddkVolumeName = "vddk-vol-mount"

// AnnOpenDefaultPorts is the annotation key for UDN default opened ports.
const AnnOpenDefaultPorts = "k8s.ovn.org/open-default-ports"

// OpenPort describes a port that should be opened for UDN networks.
type OpenPort struct {
	Protocol string `yaml:"protocol"`
	Port     int    `yaml:"port"`
}

// ConversionPodConfigResult contains provider-specific configuration
// for the virt-v2v conversion pod.
type ConversionPodConfigResult struct {
	NodeSelector map[string]string
	Labels       map[string]string
	Annotations  map[string]string
}

// PodConfig holds plan-level or CR-level configuration for pod creation.
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

	VDDKImage      string
	RequestKVM     bool
	LocalMigration bool
	UDN            bool

	PodLabels                  map[string]string
	PodAnnotations             map[string]string
	PodNodeSelector            map[string]string
	Affinity                   *core.Affinity
	TransferNetworkAnnotations map[string]string
	OwnerReferences            []meta.OwnerReference

	GenerateName string
	Environment  []core.EnvVar
	Disks        []api.DiskRef
}

// PodConfigFromSpec builds a PodConfig from a Conversion CR spec.
func PodConfigFromSpec(conversion *api.Conversion) PodConfig {
	ns := conversion.Spec.TargetNamespace
	if ns == "" {
		ns = conversion.Namespace
	}

	podConfig := PodConfig{
		TargetNamespace:  ns,
		Image:            conversion.Spec.Image,
		XfsCompatibility: conversion.Spec.XfsCompatibility,
		VDDKImage:        conversion.Spec.VDDKImage,
		RequestKVM:       conversion.Spec.RequestKVM,
		LocalMigration:   conversion.Spec.LocalMigration,
		UDN:              conversion.Spec.UDN,
	}

	podSettings := conversion.Spec.PodSettings
	podConfig.TransferNetworkAnnotations = podSettings.TransferNetworkAnnotations
	podConfig.PodAnnotations = podSettings.Annotations
	podConfig.PodNodeSelector = podSettings.NodeSelector
	podConfig.Affinity = podSettings.Affinity
	if podSettings.ServiceAccount != "" {
		podConfig.ServiceAccount = podSettings.ServiceAccount
	}

	podConfig.GenerateName = podSettings.GenerateName
	if podConfig.GenerateName == "" {
		podConfig.GenerateName = conversion.Name + "-"
	}

	podConfig.Disks = conversion.Spec.Disks

	podLabels := make(map[string]string)
	if podSettings.Labels != nil {
		for k, v := range podSettings.Labels {
			podLabels[k] = v
		}
	}
	podLabels[LabelConversion] = conversion.Name
	for _, k := range []string{LabelPlan, LabelPlanName, LabelPlanNamespace, LabelMigration, LabelVM, LabelConversionType} {
		if v, ok := conversion.Labels[k]; ok {
			podLabels[k] = v
		}
	}
	podConfig.PodLabels = podLabels

	env := make([]core.EnvVar, 0, len(conversion.Spec.Settings))
	for k, v := range conversion.Spec.Settings {
		env = append(env, core.EnvVar{Name: k, Value: v})
	}
	podConfig.Environment = env

	return podConfig
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
		Affinity:                   ResolveConvertorAffinity(p.Spec.ConvertorAffinity),
	}
}

// ResolveConvertorAffinity returns the user-specified affinity if set,
// otherwise a default pod anti-affinity that spreads virt-v2v pods
// across nodes.
func ResolveConvertorAffinity(custom *core.Affinity) *core.Affinity {
	if custom != nil {
		return custom.DeepCopy()
	}
	return &core.Affinity{
		PodAntiAffinity: &core.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []core.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: core.PodAffinityTerm{
						NamespaceSelector: &meta.LabelSelector{},
						TopologyKey:       "kubernetes.io/hostname",
						LabelSelector: &meta.LabelSelector{
							MatchExpressions: []meta.LabelSelectorRequirement{
								{
									Key:      LabelApp,
									Values:   []string{"virt-v2v"},
									Operator: meta.LabelSelectorOpIn,
								},
							},
						},
					},
				},
			},
		},
	}
}

// GetVirtV2vImage resolves the virt-v2v container image from PodConfig.
func GetVirtV2vImage(cfg *PodConfig) string {
	if cfg.Image != "" {
		return cfg.Image
	}
	if cfg.XfsCompatibility {
		if settings.Settings.Migration.VirtV2vImageXFS != "" {
			return settings.Settings.Migration.VirtV2vImageXFS
		}
	}
	return settings.Settings.Migration.VirtV2vImage
}

// ResolveServiceAccount resolves the ServiceAccount for migration pods.
func ResolveServiceAccount(cfg *PodConfig) string {
	if cfg.ServiceAccount != "" {
		return cfg.ServiceAccount
	}
	return settings.Settings.Migration.ServiceAccount
}

