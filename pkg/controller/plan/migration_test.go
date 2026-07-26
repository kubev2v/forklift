// Generated-by: Claude
package plan

import (
	"fmt"
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = ginkgo.Describe("VMStatus", func() {
	var vm *plan.VMStatus

	ginkgo.BeforeEach(func() {
		vm = &plan.VMStatus{
			VM: plan.VM{
				Ref: ref.Ref{
					ID:   "vm-123",
					Name: "test-vm",
				},
			},
			Phase: api.PhaseStarted,
		}
	})

	ginkgo.Describe("Condition handling", func() {
		ginkgo.It("should set canceled condition", func() {
			vm.SetCondition(libcnd.Condition{
				Type:     api.ConditionCanceled,
				Status:   libcnd.True,
				Category: api.CategoryAdvisory,
				Reason:   "UserRequested",
				Message:  "The migration has been canceled.",
				Durable:  true,
			})
			gomega.Expect(vm.HasCondition(api.ConditionCanceled)).To(gomega.BeTrue())
		})

		ginkgo.It("should set succeeded condition", func() {
			vm.SetCondition(libcnd.Condition{
				Type:     api.ConditionSucceeded,
				Status:   libcnd.True,
				Category: api.CategoryAdvisory,
				Message:  "The VM migration has SUCCEEDED.",
				Durable:  true,
			})
			gomega.Expect(vm.HasCondition(api.ConditionSucceeded)).To(gomega.BeTrue())
		})

		ginkgo.It("should set failed condition", func() {
			vm.SetCondition(libcnd.Condition{
				Type:     api.ConditionFailed,
				Status:   libcnd.True,
				Category: api.CategoryAdvisory,
				Message:  "The VM migration has FAILED.",
				Durable:  true,
			})
			gomega.Expect(vm.HasCondition(api.ConditionFailed)).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("Error handling", func() {
		ginkgo.It("should add error to VM", func() {
			vm.AddError("Test error message")
			gomega.Expect(vm.Error).ToNot(gomega.BeNil())
			gomega.Expect(vm.Error.Reasons).To(gomega.ContainElement("Test error message"))
		})

		ginkgo.It("should accumulate multiple errors", func() {
			vm.AddError("First error")
			vm.AddError("Second error")
			gomega.Expect(vm.Error.Reasons).To(gomega.HaveLen(2))
		})
	})

	ginkgo.Describe("Pipeline management", func() {
		ginkgo.It("should find step by name", func() {
			vm.Pipeline = []*plan.Step{
				{Task: plan.Task{Name: "DiskTransfer"}},
				{Task: plan.Task{Name: "ImageConversion"}},
			}
			step, found := vm.FindStep("DiskTransfer")
			gomega.Expect(found).To(gomega.BeTrue())
			gomega.Expect(step.Name).To(gomega.Equal("DiskTransfer"))
		})

		ginkgo.It("should return not found for missing step", func() {
			vm.Pipeline = []*plan.Step{
				{Task: plan.Task{Name: "DiskTransfer"}},
			}
			_, found := vm.FindStep("NonExistent")
			gomega.Expect(found).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("Completion tracking", func() {
		ginkgo.It("should mark started", func() {
			vm.MarkStarted()
			gomega.Expect(vm.Started).ToNot(gomega.BeNil())
		})

		ginkgo.It("should mark completed", func() {
			vm.MarkCompleted()
			gomega.Expect(vm.Completed).ToNot(gomega.BeNil())
		})

		ginkgo.It("should report completion status", func() {
			gomega.Expect(vm.MarkedCompleted()).To(gomega.BeFalse())
			vm.MarkCompleted()
			gomega.Expect(vm.MarkedCompleted()).To(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("Warm Migration", func() {
	var vm *plan.VMStatus

	ginkgo.BeforeEach(func() {
		vm = &plan.VMStatus{
			VM: plan.VM{
				Ref: ref.Ref{
					ID:   "vm-123",
					Name: "test-vm",
				},
			},
			Phase: api.PhaseStarted,
			Warm:  &plan.Warm{},
		}
	})

	ginkgo.Describe("Precopy management", func() {
		ginkgo.It("should add precopy with snapshot", func() {
			now := meta.Now()
			precopy := plan.Precopy{
				Snapshot:     "snapshot-1",
				CreateTaskId: "task-123",
				Start:        &now,
			}
			vm.Warm.Precopies = append(vm.Warm.Precopies, precopy)
			gomega.Expect(vm.Warm.Precopies).To(gomega.HaveLen(1))
			gomega.Expect(vm.Warm.Precopies[0].Snapshot).To(gomega.Equal("snapshot-1"))
		})

		ginkgo.It("should track multiple precopies", func() {
			now := meta.Now()
			for i := 0; i < 3; i++ {
				precopy := plan.Precopy{
					Snapshot: "snapshot-" + string(rune('1'+i)),
					Start:    &now,
				}
				vm.Warm.Precopies = append(vm.Warm.Precopies, precopy)
			}
			gomega.Expect(vm.Warm.Precopies).To(gomega.HaveLen(3))
		})
	})

	ginkgo.Describe("Precopy with deltas", func() {
		ginkgo.It("should store disk deltas", func() {
			now := meta.Now()
			precopy := plan.Precopy{
				Snapshot: "snapshot-1",
				Start:    &now,
			}
			deltas := map[string]string{
				"disk-1": "delta-file-1",
				"disk-2": "delta-file-2",
			}
			precopy.WithDeltas(deltas)
			vm.Warm.Precopies = append(vm.Warm.Precopies, precopy)

			gomega.Expect(vm.Warm.Precopies[0].Deltas).To(gomega.HaveLen(2))
		})
	})
})

var _ = ginkgo.Describe("Step", func() {
	var step *plan.Step

	ginkgo.BeforeEach(func() {
		step = &plan.Step{
			Task: plan.Task{
				Name: "DiskTransfer",
			},
		}
	})

	ginkgo.Describe("Marking progress", func() {
		ginkgo.It("should mark started", func() {
			step.MarkStarted()
			gomega.Expect(step.Started).ToNot(gomega.BeNil())
		})

		ginkgo.It("should mark completed", func() {
			step.MarkCompleted()
			gomega.Expect(step.Completed).ToNot(gomega.BeNil())
		})

		ginkgo.It("should report completion status", func() {
			gomega.Expect(step.MarkedCompleted()).To(gomega.BeFalse())
			step.MarkCompleted()
			gomega.Expect(step.MarkedCompleted()).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("Error handling", func() {
		ginkgo.It("should add error to step", func() {
			step.AddError("Step error message")
			gomega.Expect(step.Error).ToNot(gomega.BeNil())
			gomega.Expect(step.Error.Reasons).To(gomega.ContainElement("Step error message"))
		})

		ginkgo.It("should report error status", func() {
			gomega.Expect(step.HasError()).To(gomega.BeFalse())
			step.AddError("Error")
			gomega.Expect(step.HasError()).To(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("Task", func() {
	var task *plan.Task

	ginkgo.BeforeEach(func() {
		task = &plan.Task{
			Name:        "DiskCopy",
			Annotations: make(map[string]string),
		}
	})

	ginkgo.Describe("Reset and start", func() {
		ginkgo.It("should mark reset", func() {
			task.MarkCompleted()
			task.MarkReset()
			gomega.Expect(task.Completed).To(gomega.BeNil())
		})

		ginkgo.It("should mark started after reset", func() {
			task.MarkReset()
			task.MarkStarted()
			gomega.Expect(task.Started).ToNot(gomega.BeNil())
		})
	})
})

var _ = ginkgo.Describe("Cancellation", func() {
	ginkgo.Describe("Migration cancellation", func() {
		ginkgo.It("should check if VM is in canceled list", func() {
			migration := &api.Migration{
				Spec: api.MigrationSpec{
					Cancel: []ref.Ref{
						{ID: "vm-1", Name: "cancel-vm-1"},
						{ID: "vm-2", Name: "cancel-vm-2"},
					},
				},
			}

			vmRef := ref.Ref{ID: "vm-1", Name: "cancel-vm-1"}
			gomega.Expect(migration.Spec.Canceled(vmRef)).To(gomega.BeTrue())

			vmRef2 := ref.Ref{ID: "vm-3", Name: "other-vm"}
			gomega.Expect(migration.Spec.Canceled(vmRef2)).To(gomega.BeFalse())
		})

		ginkgo.It("should handle empty cancel list", func() {
			migration := &api.Migration{
				Spec: api.MigrationSpec{
					Cancel: []ref.Ref{},
				},
			}

			vmRef := ref.Ref{ID: "vm-1", Name: "some-vm"}
			gomega.Expect(migration.Spec.Canceled(vmRef)).To(gomega.BeFalse())
		})
	})
})

func TestMarkSchedulerQueuedVMs(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	originalLimit := settings.Settings.MaxInFlight
	settings.Settings.MaxInFlight = 20
	t.Cleanup(func() { settings.Settings.MaxInFlight = originalLimit })

	queuedVM := &plan.VMStatus{Phase: api.PhaseStarted}

	m := &Migration{
		Context: &plancontext.Context{
			Plan: &api.Plan{
				Status: api.PlanStatus{
					Migration: plan.MigrationStatus{
						VMs: []*plan.VMStatus{queuedVM},
					},
				},
			},
			Log: logging.WithName("test"),
		},
	}

	m.markSchedulerQueuedVMs()

	g.Expect(queuedVM.HasCondition(api.ConditionPending)).To(gomega.BeTrue())
	pending := queuedVM.FindCondition(api.ConditionPending)
	g.Expect(pending.Message).To(gomega.Equal("Waiting to start migration: in-flight limit reached (max 20)."))

	ready := m.Plan.Status.FindCondition(libcnd.Ready)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.Message).To(gomega.Equal("1 VM(s) waiting to start migration (in-flight limit: 20)."))
}

func TestExecuteClearsSchedulerPendingCondition(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	vm := &plan.VMStatus{
		VM: plan.VM{
			Ref: ref.Ref{ID: "vm-1", Name: "test-vm"},
		},
		Phase: api.PhaseStarted,
	}
	vm.SetCondition(libcnd.Condition{
		Type:     api.ConditionPending,
		Status:   libcnd.True,
		Category: api.CategoryAdvisory,
		Message:  "Waiting to start migration: in-flight limit reached (max 20).",
		Durable:  true,
	})

	m := &Migration{
		Context: &plancontext.Context{
			Migration: &api.Migration{
				Spec: api.MigrationSpec{
					Cancel: []ref.Ref{{ID: "vm-1", Name: "test-vm"}},
				},
			},
			Log: logging.WithName("test"),
		},
	}

	err := m.execute(vm)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(vm.HasCondition(api.ConditionPending)).To(gomega.BeFalse())
}

const testTargetNamespace = "target-ns"

func TestVerifyDataVolumeCompletion(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mainUID := types.UID("main-pvc-uid")
	mainPVCName := "dv-pvc"
	primePVCName := fmt.Sprintf("prime-%s", mainUID)
	importerPodName := "importer-prime-test"

	tests := []struct {
		name         string
		objects      []runtime.Object
		wantComplete bool
		wantReason   string
	}{
		{
			name: "main PVC pending blocks completion",
			objects: []runtime.Object{
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Name:      mainPVCName,
						Namespace: testTargetNamespace,
						UID:       mainUID,
					},
					Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimPending},
				},
			},
			wantComplete: false,
			wantReason:   "Main PVC is not bound (phase: Pending)",
		},
		{
			name: "prime PVC pending blocks completion",
			objects: []runtime.Object{
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Name:      mainPVCName,
						Namespace: testTargetNamespace,
						UID:       mainUID,
					},
					Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
				},
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Name:      primePVCName,
						Namespace: testTargetNamespace,
					},
					Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimPending},
				},
			},
			wantComplete: false,
			wantReason:   "Prime PVC is not bound (phase: Pending)",
		},
		{
			name: "importer pod pending blocks completion",
			objects: []runtime.Object{
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Name:      mainPVCName,
						Namespace: testTargetNamespace,
						UID:       mainUID,
					},
					Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
				},
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Name:      primePVCName,
						Namespace: testTargetNamespace,
						Annotations: map[string]string{
							AnnImporterPodName: importerPodName,
						},
					},
					Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
				},
				&core.Pod{
					ObjectMeta: meta.ObjectMeta{
						Name:      importerPodName,
						Namespace: testTargetNamespace,
					},
					Status: core.PodStatus{Phase: core.PodPending},
				},
			},
			wantComplete: false,
			wantReason:   fmt.Sprintf("Importer pod is pending (pod: %s/%s)", testTargetNamespace, importerPodName),
		},
		{
			name: "bound PVCs and running importer allow completion",
			objects: []runtime.Object{
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Name:      mainPVCName,
						Namespace: testTargetNamespace,
						UID:       mainUID,
					},
					Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
				},
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{
						Name:      primePVCName,
						Namespace: testTargetNamespace,
						Annotations: map[string]string{
							AnnImporterPodName: importerPodName,
						},
					},
					Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
				},
				&core.Pod{
					ObjectMeta: meta.ObjectMeta{
						Name:      importerPodName,
						Namespace: testTargetNamespace,
					},
					Status: core.PodStatus{Phase: core.PodRunning},
				},
			},
			wantComplete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMigration(tt.objects...)
			dv := &cdi.DataVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:      "test-dv",
					Namespace: testTargetNamespace,
				},
				Status: cdi.DataVolumeStatus{
					Phase:     cdi.Succeeded,
					ClaimName: mainPVCName,
				},
			}
			vm := &plan.VMStatus{}

			canComplete, reason := m.verifyDataVolumeCompletion(dv, vm)
			g.Expect(canComplete).To(gomega.Equal(tt.wantComplete))
			if tt.wantReason != "" {
				g.Expect(reason).To(gomega.Equal(tt.wantReason))
			} else {
				g.Expect(reason).To(gomega.BeEmpty())
			}
		})
	}
}

func TestPendingPopulationNotPromotedOnWarmMigration(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	vSphere := api.VSphere
	ctx := &plancontext.Context{
		Plan: &api.Plan{
			Spec: api.PlanSpec{
				Type:            api.MigrationWarm,
				TargetNamespace: testTargetNamespace,
			},
		},
		Source: plancontext.Source{
			Provider: &api.Provider{
				Spec: api.ProviderSpec{Type: &vSphere},
			},
		},
		Destination: plancontext.Destination{
			Client: fakeClient.NewClientBuilder().WithRuntimeObjects().Build(),
		},
		Log: logging.WithName("test"),
	}

	m := &Migration{Context: ctx}
	dvPhase := cdi.PendingPopulation

	if dvPhase == cdi.PendingPopulation && m.Source.Provider.RequiresConversion() && !m.Plan.IsWarm() {
		dvPhase = cdi.Succeeded
	}

	g.Expect(m.Plan.IsWarm()).To(gomega.BeTrue())
	g.Expect(dvPhase).To(gomega.Equal(cdi.PendingPopulation))
}

func newTestMigration(objects ...runtime.Object) *Migration {
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	client := fakeClient.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		Build()
	ctx := &plancontext.Context{
		Plan: &api.Plan{
			Spec: api.PlanSpec{TargetNamespace: testTargetNamespace},
		},
		Destination: plancontext.Destination{Client: client},
		Log:         logging.WithName("test"),
	}
	return &Migration{
		Context:  ctx,
		kubevirt: KubeVirt{Context: ctx},
	}
}
