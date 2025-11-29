package migrator

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libitr "github.com/kubev2v/forklift/pkg/lib/itinerary"
)

// Pipeline converts itinerary phases into user-facing UI steps with progress tracking.
// Maps internal phases to steps: Initialize, PrepareSource, CreateSnapshots, DiskTransfer, CreateVM, Cleanup.
// Each step includes description, total progress units, and optional sub-tasks for detailed tracking.
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

		case api.PhasePowerOffSource, api.PhaseWaitForPowerOff:
			if step.Name == api.PhasePowerOffSource {
				pipeline = append(pipeline, &planapi.Step{
					Task: planapi.Task{
						Name:        PrepareSource,
						Description: "Stop source EC2 instance.",
						Progress:    libitr.Progress{Total: 2},
						Phase:       api.StepPending,
					},
				})
			}

		case PhaseCreateSnapshots, PhaseWaitForSnapshots:
			if step.Name == PhaseCreateSnapshots {
				pipeline = append(pipeline, &planapi.Step{
					Task: planapi.Task{
						Name:        CreateSnapshots,
						Description: "Create EBS volume snapshots.",
						Progress:    libitr.Progress{Total: 2},
						Phase:       api.StepPending,
					},
				})
			}

		case api.PhaseCreateDataVolumes:
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
					Description: "Transfer disks via EC2 populator.",
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

		case PhaseRemoveSnapshots:
			pipeline = append(pipeline, &planapi.Step{
				Task: planapi.Task{
					Name:        Cleanup,
					Description: "Clean up EBS snapshots.",
					Progress:    libitr.Progress{Total: 1},
					Phase:       api.StepPending,
				},
			})

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
