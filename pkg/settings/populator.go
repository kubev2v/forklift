package settings

import "os"

const (
	PopulatorContainerLimitsCpu      = "POPULATOR_CONTAINER_LIMITS_CPU"
	PopulatorContainerLimitsMemory   = "POPULATOR_CONTAINER_LIMITS_MEMORY"
	PopulatorContainerRequestsCpu    = "POPULATOR_CONTAINER_REQUESTS_CPU"
	PopulatorContainerRequestsMemory = "POPULATOR_CONTAINER_REQUESTS_MEMORY"
)

type Populator struct {
	ContainerLimitsCpu      string
	ContainerLimitsMemory   string
	ContainerRequestsCpu    string
	ContainerRequestsMemory string
}

func (r *Populator) Load() {
	if val, found := os.LookupEnv(PopulatorContainerLimitsCpu); found {
		r.ContainerLimitsCpu = val
	} else {
		r.ContainerLimitsCpu = "1000m"
	}
	if val, found := os.LookupEnv(PopulatorContainerLimitsMemory); found {
		r.ContainerLimitsMemory = val
	} else {
		r.ContainerLimitsMemory = "1Gi"
	}
	if val, found := os.LookupEnv(PopulatorContainerRequestsCpu); found {
		r.ContainerRequestsCpu = val
	} else {
		r.ContainerRequestsCpu = "100m"
	}
	if val, found := os.LookupEnv(PopulatorContainerRequestsMemory); found {
		r.ContainerRequestsMemory = val
	} else {
		r.ContainerRequestsMemory = "512Mi"
	}
}
