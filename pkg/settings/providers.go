package settings

import "os"

// Env
const (
	OVAProviderServerImage = "OVA_PROVIDER_SERVER_IMAGE"
)

// Defaults
const (
	DefaultOVACPULimit      = "1000m"
	DefaultOVACPURequest    = "100m"
	DefaultOVAMemoryLimit   = "1Gi"
	DefaultOVAMemoryRequest = "512Mi"
)

type Providers struct {
	OVA struct {
		Pod struct {
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
	}
}

func (r *Providers) Load() error {
	r.OVA.Pod.Resources.CPU.Limit = Lookup(OvaContainerLimitsCpu, DefaultOVACPULimit)
	r.OVA.Pod.Resources.CPU.Request = Lookup(OvaContainerRequestsCpu, DefaultOVACPURequest)
	r.OVA.Pod.Resources.Memory.Limit = Lookup(OvaContainerLimitsMemory, DefaultOVAMemoryLimit)
	r.OVA.Pod.Resources.Memory.Request = Lookup(OvaContainerRequestsMemory, DefaultOVAMemoryRequest)
	r.OVA.Pod.ContainerImage = os.Getenv(OVAProviderServerImage)
	return nil
}
