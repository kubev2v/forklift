package context

import (
	"maps"

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
	// LabelRetryAllowed controls whether a failed/canceled DeepInspection Conversion CR may be retried during preflight inspection.
	// "true": one retry is allowed, the replacement CR will be stamped "false".
	// "false": no further retries, the step is immediately failed.
	// absent: treated the same as "true" (one retry allowed).
	LabelRetryAllowed = "retryAllowed"
)

// VddkVolumeName is the volume name used for the VDDK library scratch space.
const VddkVolumeName = "vddk-vol-mount"

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
	LocalMigration bool

	PodLabels                  map[string]string
	PodAnnotations             map[string]string
	PodNodeSelector            map[string]string
	Affinity                   *core.Affinity
	TransferNetworkAnnotations map[string]string
	OwnerReferences            []meta.OwnerReference

	GenerateName string
	Environment  []core.EnvVar
	Disks        []api.DiskRef
	// DeepInspectionSnapshotMoref is injected as SNAPSHOT_MOREF for DeepInspection pods when set by the controller.
	DeepInspectionSnapshotMoref string
	// DiskEncryption carries the disk encryption config from the Conversion spec into the builder.
	DiskEncryption *api.DiskEncryption
	// ExtraInitContainers are prepended to the pod's init container list before the VDDK sidecar.
	// Used by callers that need provider-specific init work (e.g. NetApp Shift disk-perms fixer).
	ExtraInitContainers []core.Container
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
		LocalMigration:   conversion.Spec.LocalMigration,
	}

	podSettings := conversion.Spec.PodSettings
	podConfig.TransferNetworkAnnotations = podSettings.TransferNetworkAnnotations
	// Pod annotations are stored directly on the Conversion CR so they can be
	// copied here without indirection through PodSettings.
	podConfig.PodAnnotations = conversion.Annotations
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
	podConfig.DiskEncryption = conversion.Spec.DiskEncryption

	// Pod labels are stored directly on the Conversion CR so they can be
	// copied here without indirection through PodSettings.
	podLabels := make(map[string]string)
	maps.Copy(podLabels, conversion.Labels)
	podLabels[LabelConversion] = conversion.Name
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
		Affinity:                   p.Spec.ConvertorAffinity,
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

// GetDeepInspectionImage resolves the deep inspection workload image from PodConfig.
func GetDeepInspectionImage(cfg *PodConfig) string {
	if cfg.Image != "" {
		return cfg.Image
	}
	return settings.Settings.Migration.DeepInspectionImage
}

// ResolveServiceAccount resolves the ServiceAccount for migration pods.
func ResolveServiceAccount(cfg *PodConfig) string {
	if cfg.ServiceAccount != "" {
		return cfg.ServiceAccount
	}
	return settings.Settings.Migration.ServiceAccount
}
