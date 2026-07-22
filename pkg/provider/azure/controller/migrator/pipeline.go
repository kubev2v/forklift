package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
)

// Pipeline builds the UI step list by walking the itinerary and grouping
// consecutive phases into logical steps with progress totals.
func (r *Migrator) Pipeline(vm planapi.VM) (pipeline []*planapi.Step, err error) {
	itinerary := r.Itinerary(vm)
	step, _ := itinerary.First()

	for {
		switch step.Name {
		case api.PhaseStarted:
			pipeline = append(pipeline, &planapi.Step{
				Task: planapi.Task{
					Name:        Initialize,
					Description: "Initialize migration.",
					Progress:    libitr.Progress{Total: 1},
					Phase:       api.StepPending,
				},
			})

		case api.PhasePreHook:
			pipeline = append(pipeline, &planapi.Step{
				Task: planapi.Task{
					Name:        api.PhasePreHook,
					Description: "Execute pre-migration hook.",
					Progress:    libitr.Progress{Total: 1},
					Phase:       api.StepPending,
				},
			})

		case api.PhaseStorePowerState, PhaseCreatePreSnapshot, PhaseWaitForPreSnapshot, PhaseDeallocateVM, PhaseWaitForDeallocation:
			if step.Name == PhaseCreatePreSnapshot || step.Name == PhaseDeallocateVM {
				total := int64(2)
				description := "Deallocate source Azure VM."
				if step.Name == PhaseCreatePreSnapshot {
					total = 4
					description = "Pre-snapshot and deallocate source Azure VM."
				}
				pipeline = append(pipeline, &planapi.Step{
					Task: planapi.Task{
						Name:        PrepareSource,
						Description: description,
						Progress:    libitr.Progress{Total: total},
						Phase:       api.StepPending,
					},
				})
			}

		case PhaseCreateSnapshots, PhaseWaitForSnapshots, PhaseDeletePreSnapshots, PhaseCopySnapshotsCrossRegion, PhaseWaitForCrossRegionSnapshots:
			if step.Name == PhaseCreateSnapshots {
				total := int64(2)
				if r.getAzureClient().IsCrossRegion() {
					total = 3
				}
				pipeline = append(pipeline, &planapi.Step{
					Task: planapi.Task{
						Name:        CreateSnapshots,
						Description: "Create Azure managed disk snapshots.",
						Progress:    libitr.Progress{Total: total},
						Phase:       api.StepPending,
					},
				})
			}

		case PhaseCreateSnapshotContent, PhaseCreateVolumeSnapshot, PhaseCreatePVCs, PhaseWaitForPVCsBound, PhaseInjectOwnerRefs:
			if step.Name == PhaseCreateSnapshotContent {
				tasks, pErr := r.builder.Tasks(vm.Ref)
				if pErr != nil {
					err = liberr.Wrap(pErr)
					return
				}

				total := int64(0)
				for _, task := range tasks {
					total += task.Progress.Total
				}

				pipeline = append(pipeline, &planapi.Step{
					Task: planapi.Task{
						Name:        DiskTransfer,
						Description: "Create VolumeSnapshots and PVCs.",
						Progress: libitr.Progress{
							Total: total,
						},
						Annotations: map[string]string{
							"unit": "MB",
						},
						Phase: api.StepPending,
					},
					Tasks: tasks,
				})
			}

		case api.PhaseCreateGuestConversionPod, api.PhaseConvertGuest:
			if step.Name == api.PhaseCreateGuestConversionPod {
				pipeline = append(pipeline, &planapi.Step{
					Task: planapi.Task{
						Name:        api.PhaseConvertGuest,
						Description: "Convert guest operating system.",
						Progress:    libitr.Progress{Total: 1},
						Phase:       api.StepPending,
					},
				})
			}

		case api.PhaseFinalize, api.PhaseCreateVM:
			if step.Name == api.PhaseFinalize {
				pipeline = append(pipeline, &planapi.Step{
					Task: planapi.Task{
						Name:        CreateVM,
						Description: "Create VirtualMachine on target.",
						Progress:    libitr.Progress{Total: 2},
						Phase:       api.StepPending,
					},
				})
			}

		case api.PhasePostHook:
			pipeline = append(pipeline, &planapi.Step{
				Task: planapi.Task{
					Name:        api.PhasePostHook,
					Description: "Execute post-migration hook.",
					Progress:    libitr.Progress{Total: 1},
					Phase:       api.StepPending,
				},
			})
		}

		next, done, _ := itinerary.Next(step.Name)
		if !done {
			step = next
		} else {
			break
		}
	}

	return
}
