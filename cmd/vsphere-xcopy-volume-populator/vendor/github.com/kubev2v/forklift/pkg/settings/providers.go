package settings

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Env
const (
	OVAProviderServerImage    = "OVA_PROVIDER_SERVER_IMAGE"
	HyperVProviderServerImage = "HYPERV_PROVIDER_SERVER_IMAGE"
)

// Defaults
const (
	DefaultOVACPULimit      = "1000m"
	DefaultOVACPURequest    = "100m"
	DefaultOVAMemoryLimit   = "1Gi"
	DefaultOVAMemoryRequest = "512Mi"

	DefaultHyperVCPULimit      = "1000m"
	DefaultHyperVCPURequest    = "100m"
	DefaultHyperVMemoryLimit   = "1Gi"
	DefaultHyperVMemoryRequest = "512Mi"
)

// ProviderPodConfig defines common configuration for provider server pods
type ProviderPodConfig struct {
	ContainerImage string
	Resources      struct {
		CPU struct {
			Request string
			Limit   string
		}
		Memory struct {
			Request string
			Limit   string
		}
	}
}

type Providers struct {
	OVA    ProviderPodConfig
	HyperV ProviderPodConfig
}

func (r *Providers) Load() error {
	// Load OVA settings
	r.OVA.Resources.CPU.Limit = Lookup(OvaContainerLimitsCpu, DefaultOVACPULimit)
	if _, err := resource.ParseQuantity(r.OVA.Resources.CPU.Limit); err != nil {
		return fmt.Errorf("invalid OVA CPU limit %q: %w", r.OVA.Resources.CPU.Limit, err)
	}
	r.OVA.Resources.CPU.Request = Lookup(OvaContainerRequestsCpu, DefaultOVACPURequest)
	if _, err := resource.ParseQuantity(r.OVA.Resources.CPU.Request); err != nil {
		return fmt.Errorf("invalid OVA CPU request %q: %w", r.OVA.Resources.CPU.Request, err)
	}
	r.OVA.Resources.Memory.Limit = Lookup(OvaContainerLimitsMemory, DefaultOVAMemoryLimit)
	if _, err := resource.ParseQuantity(r.OVA.Resources.Memory.Limit); err != nil {
		return fmt.Errorf("invalid OVA memory limit %q: %w", r.OVA.Resources.Memory.Limit, err)
	}
	r.OVA.Resources.Memory.Request = Lookup(OvaContainerRequestsMemory, DefaultOVAMemoryRequest)
	if _, err := resource.ParseQuantity(r.OVA.Resources.Memory.Request); err != nil {
		return fmt.Errorf("invalid OVA memory request %q: %w", r.OVA.Resources.Memory.Request, err)
	}
	r.OVA.ContainerImage = os.Getenv(OVAProviderServerImage)

	// Load HyperV settings
	r.HyperV.Resources.CPU.Limit = Lookup(HyperVContainerLimitsCpu, DefaultHyperVCPULimit)
	if _, err := resource.ParseQuantity(r.HyperV.Resources.CPU.Limit); err != nil {
		return fmt.Errorf("invalid HyperV CPU limit %q: %w", r.HyperV.Resources.CPU.Limit, err)
	}
	r.HyperV.Resources.CPU.Request = Lookup(HyperVContainerRequestsCpu, DefaultHyperVCPURequest)
	if _, err := resource.ParseQuantity(r.HyperV.Resources.CPU.Request); err != nil {
		return fmt.Errorf("invalid HyperV CPU request %q: %w", r.HyperV.Resources.CPU.Request, err)
	}
	r.HyperV.Resources.Memory.Limit = Lookup(HyperVContainerLimitsMemory, DefaultHyperVMemoryLimit)
	if _, err := resource.ParseQuantity(r.HyperV.Resources.Memory.Limit); err != nil {
		return fmt.Errorf("invalid HyperV memory limit %q: %w", r.HyperV.Resources.Memory.Limit, err)
	}
	r.HyperV.Resources.Memory.Request = Lookup(HyperVContainerRequestsMemory, DefaultHyperVMemoryRequest)
	if _, err := resource.ParseQuantity(r.HyperV.Resources.Memory.Request); err != nil {
		return fmt.Errorf("invalid HyperV memory request %q: %w", r.HyperV.Resources.Memory.Request, err)
	}
	r.HyperV.ContainerImage = os.Getenv(HyperVProviderServerImage)

	return nil
}
