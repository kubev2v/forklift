package settings

import (
	"os"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

const (
	PopulatorContainerLimitsCpu      = "POPULATOR_CONTAINER_LIMITS_CPU"
	PopulatorContainerLimitsMemory   = "POPULATOR_CONTAINER_LIMITS_MEMORY"
	PopulatorContainerRequestsCpu    = "POPULATOR_CONTAINER_REQUESTS_CPU"
	PopulatorContainerRequestsMemory = "POPULATOR_CONTAINER_REQUESTS_MEMORY"
)

var (
	DefaultPopulatorContainerLimitsCpu      = resource.NewMilliQuantity(1000, resource.DecimalSI)
	DefaultPopulatorContainerLimitsMemory   = resource.NewQuantity(1<<30, resource.BinarySI)
	DefaultPopulatorContainerRequestsCpu    = resource.NewMilliQuantity(100, resource.DecimalSI)
	DefaultPopulatorContainerRequestsMemory = resource.NewQuantity(512<<20, resource.BinarySI)
)

type Populator struct {
	ContainerLimitsCpu      resource.Quantity
	ContainerLimitsMemory   resource.Quantity
	ContainerRequestsCpu    resource.Quantity
	ContainerRequestsMemory resource.Quantity
}

func (r *Populator) Load() {
	if val, found := os.LookupEnv(PopulatorContainerLimitsCpu); found {
		q, err := resource.ParseQuantity(val)
		if err != nil {
			klog.Warningf("Invalid %s value %q, using default: %v", PopulatorContainerLimitsCpu, val, err)
			r.ContainerLimitsCpu = *DefaultPopulatorContainerLimitsCpu
		} else {
			r.ContainerLimitsCpu = q
		}
	} else {
		r.ContainerLimitsCpu = *DefaultPopulatorContainerLimitsCpu
	}
	if val, found := os.LookupEnv(PopulatorContainerLimitsMemory); found {
		q, err := resource.ParseQuantity(val)
		if err != nil {
			klog.Warningf("Invalid %s value %q, using default: %v", PopulatorContainerLimitsMemory, val, err)
			r.ContainerLimitsMemory = *DefaultPopulatorContainerLimitsMemory
		} else {
			r.ContainerLimitsMemory = q
		}
	} else {
		r.ContainerLimitsMemory = *DefaultPopulatorContainerLimitsMemory
	}
	if val, found := os.LookupEnv(PopulatorContainerRequestsCpu); found {
		q, err := resource.ParseQuantity(val)
		if err != nil {
			klog.Warningf("Invalid %s value %q, using default: %v", PopulatorContainerRequestsCpu, val, err)
			r.ContainerRequestsCpu = *DefaultPopulatorContainerRequestsCpu
		} else {
			r.ContainerRequestsCpu = q
		}
	} else {
		r.ContainerRequestsCpu = *DefaultPopulatorContainerRequestsCpu
	}
	if val, found := os.LookupEnv(PopulatorContainerRequestsMemory); found {
		q, err := resource.ParseQuantity(val)
		if err != nil {
			klog.Warningf("Invalid %s value %q, using default: %v", PopulatorContainerRequestsMemory, val, err)
			r.ContainerRequestsMemory = *DefaultPopulatorContainerRequestsMemory
		} else {
			r.ContainerRequestsMemory = q
		}
	} else {
		r.ContainerRequestsMemory = *DefaultPopulatorContainerRequestsMemory
	}
}
