package ensurer

import (
	"context"

	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/builder"
)

// EnsurePopulatorDataVolumes creates Ec2VolumePopulator CRs and PVCs for VM volumes.
func (r *Ensurer) EnsurePopulatorDataVolumes(ctx context.Context, vm *planapi.VMStatus, bldr base.Builder, populatorSecretName string) (allBound bool, err error) {
	ec2Builder, ok := bldr.(*builder.Builder)
	if !ok {
		return false, liberr.New("builder is not an EC2 builder")
	}

	// Step 1: Build and create Ec2VolumePopulator CRs, track their generated names
	populators, err := ec2Builder.BuildEc2VolumePopulators(vm.Ref, populatorSecretName)
	if err != nil {
		r.log.Error(err, "Failed to build Ec2VolumePopulator specs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(populators) == 0 {
		r.log.Info("No populators to create", "vm", vm.Name)
		return true, nil
	}

	// Step 2: Create populators and get mapping of volumeID -> populator name
	populatorNames, err := r.EnsureEc2VolumePopulatorsWithNames(ctx, vm, populators)
	if err != nil {
		r.log.Error(err, "Failed to ensure Ec2VolumePopulator CRs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	// Step 3: Build PVCs using the actual populator names
	pvcs, err := ec2Builder.PopulatorVolumesWithNames(vm.Ref, nil, populatorSecretName, populatorNames)
	if err != nil {
		r.log.Error(err, "Failed to build PVC specs", "vm", vm.Name)
		return false, liberr.Wrap(err)
	}

	if len(pvcs) == 0 {
		r.log.Info("No PVCs to create", "vm", vm.Name)
		return true, nil
	}

	r.log.Info("Built PVC specs with populator references", "vm", vm.Name, "count", len(pvcs))

	return r.EnsurePersistentVolumeClaims(ctx, vm, pvcs)
}
