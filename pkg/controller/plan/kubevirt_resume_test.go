package plan

import (
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var resumeLog = logging.WithName("resumeTest")

const (
	resumePlanName      = "plan-resume"
	resumePlanNamespace = "plan-ns"
	resumeTargetNS      = "target-ns"
	resumeVMID          = "vm-1234"
	resumePlanUID       = types.UID("plan-uid-1")
)

// resumeConvPodLabels returns the labels a guest-conversion pod carries.
func resumeConvPodLabels(migrationUID types.UID) map[string]string {
	return map[string]string{
		kMigration:     string(migrationUID),
		kPlan:          string(resumePlanUID),
		kPlanName:      resumePlanName,
		kPlanNamespace: resumePlanNamespace,
		kVM:            resumeVMID,
		kResource:      ResourceVMConfig,
		kApp:           "virt-v2v",
	}
}

func newResumeKubeVirt(migrationUID types.UID, objects ...client.Object) (KubeVirt, *Migration) {
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)

	c := fakeClient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()

	ctx := &plancontext.Context{
		Plan: &api.Plan{
			ObjectMeta: meta.ObjectMeta{
				Name:      resumePlanName,
				Namespace: resumePlanNamespace,
				UID:       resumePlanUID,
			},
			Spec: api.PlanSpec{TargetNamespace: resumeTargetNS},
		},
		Migration: &api.Migration{
			ObjectMeta: meta.ObjectMeta{UID: migrationUID},
		},
		Destination: plancontext.Destination{Client: c},
		Log:         resumeLog,
	}

	kv := KubeVirt{Context: ctx}
	m := &Migration{Context: ctx, kubevirt: kv}
	return kv, m
}

func newResumeVMStatus() *planapi.VMStatus {
	return &planapi.VMStatus{
		VM: planapi.VM{
			Ref: ref.Ref{ID: resumeVMID},
		},
	}
}

var _ = ginkgo.Describe("Resume-conversion stale workload cleanup", func() {
	var (
		oldMigrationUID types.UID
		newMigrationUID types.UID
	)

	ginkgo.BeforeEach(func() {
		oldMigrationUID = types.UID("old-migration-" + rand.String(6))
		newMigrationUID = types.UID("new-migration-" + rand.String(6))
		settings.Settings.UseConversionCR = false
	})

	ginkgo.Context("deleteStaleConversionWorkloads", func() {
		ginkgo.It("should request deletion of stale pod and return done=false", func() {
			stalePod := &core.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:      "stale-conv-pod",
					Namespace: resumeTargetNS,
					Labels:    resumeConvPodLabels(oldMigrationUID),
				},
			}
			_, m := newResumeKubeVirt(newMigrationUID, stalePod)
			vm := newResumeVMStatus()

			done, err := m.deleteStaleConversionWorkloads(vm)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(done).To(gomega.BeFalse(), "should requeue after requesting deletion")
		})

		ginkgo.It("should return done=true when no stale pods exist", func() {
			_, m := newResumeKubeVirt(newMigrationUID)
			vm := newResumeVMStatus()

			done, err := m.deleteStaleConversionWorkloads(vm)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(done).To(gomega.BeTrue())
		})

		ginkgo.It("should return done=false when a pod is still terminating", func() {
			now := meta.NewTime(time.Now())
			terminatingPod := &core.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:              "terminating-conv-pod",
					Namespace:         resumeTargetNS,
					Labels:            resumeConvPodLabels(oldMigrationUID),
					DeletionTimestamp: &now,
					Finalizers:        []string{"test-finalizer"},
				},
			}
			_, m := newResumeKubeVirt(newMigrationUID, terminatingPod)
			vm := newResumeVMStatus()

			done, err := m.deleteStaleConversionWorkloads(vm)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(done).To(gomega.BeFalse())
		})

		ginkgo.It("should preserve copied PVCs while deleting stale conversion pods", func() {
			stalePod := &core.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:      "stale-conv-pod",
					Namespace: resumeTargetNS,
					Labels:    resumeConvPodLabels(oldMigrationUID),
				},
			}
			copiedPVC := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "copied-disk-pvc",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kPlan:          string(resumePlanUID),
						kPlanName:      resumePlanName,
						kPlanNamespace: resumePlanNamespace,
						kVM:            resumeVMID,
					},
				},
			}

			_, m := newResumeKubeVirt(newMigrationUID, stalePod, copiedPVC)
			vm := newResumeVMStatus()

			done, err := m.deleteStaleConversionWorkloads(vm)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(done).To(gomega.BeFalse(), "should requeue after requesting pod deletion")

			gomega.Expect(objectExists(m.Destination.Client, types.NamespacedName{
				Name: "copied-disk-pvc", Namespace: resumeTargetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeTrue())
		})

		ginkgo.It("should return done=false on first call, done=true once pod is gone", func() {
			stalePod := &core.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:      "stale-conv-pod",
					Namespace: resumeTargetNS,
					Labels:    resumeConvPodLabels(oldMigrationUID),
				},
			}
			_, m := newResumeKubeVirt(newMigrationUID, stalePod)
			vm := newResumeVMStatus()

			done, err := m.deleteStaleConversionWorkloads(vm)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(done).To(gomega.BeFalse(), "first call requests deletion and requeues")

			// Fake-client removes the object synchronously, so the next
			// reconciliation finds no stale resources.
			done, err = m.deleteStaleConversionWorkloads(vm)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(done).To(gomega.BeTrue(), "no stale objects remain")
		})
	})
})

var _ = ginkgo.Describe("Resume-conversion PVC validation", func() {
	const testUUID = "test-uuid"

	ginkgo.Context("validateResumePVCsByUUID", func() {
		ginkgo.It("should reject when no PVCs exist", func() {
			migUID := types.UID("mig-" + rand.String(6))
			kv, _ := newResumeKubeVirt(migUID)

			err := kv.validateResumePVCsByUUID(ref.Ref{ID: resumeVMID}, testUUID)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("no reusable PVCs"))
		})

		ginkgo.It("should reject unbound PVCs", func() {
			migUID := types.UID("mig-" + rand.String(6))
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "pending-pvc",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kVM:     resumeVMID,
						kVmUuid: testUUID,
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[disk-1]",
						planbase.AnnDiskIndex:  "0",
					},
				},
				Status: core.PersistentVolumeClaimStatus{
					Phase: core.ClaimPending,
				},
			}
			kv, _ := newResumeKubeVirt(migUID, pvc)

			err := kv.validateResumePVCsByUUID(ref.Ref{ID: resumeVMID}, testUUID)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("not Bound"))
		})

		ginkgo.It("should reject duplicate disk sources", func() {
			migUID := types.UID("mig-" + rand.String(6))
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "pvc-1",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kVM:     resumeVMID,
						kVmUuid: testUUID,
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[disk-1]",
						planbase.AnnDiskIndex:  "0",
					},
				},
				Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
			}
			pvc2 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "pvc-2",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kVM:     resumeVMID,
						kVmUuid: testUUID,
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[disk-1]",
						planbase.AnnDiskIndex:  "1",
					},
				},
				Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
			}
			kv, _ := newResumeKubeVirt(migUID, pvc1, pvc2)

			err := kv.validateResumePVCsByUUID(ref.Ref{ID: resumeVMID}, testUUID)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("duplicate disk source"))
		})

		ginkgo.It("should reject duplicate disk indexes", func() {
			migUID := types.UID("mig-" + rand.String(6))
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "pvc-1",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kVM:     resumeVMID,
						kVmUuid: testUUID,
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[disk-1]",
						planbase.AnnDiskIndex:  "0",
					},
				},
				Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
			}
			pvc2 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "pvc-2",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kVM:     resumeVMID,
						kVmUuid: testUUID,
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[disk-2]",
						planbase.AnnDiskIndex:  "0",
					},
				},
				Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
			}
			kv, _ := newResumeKubeVirt(migUID, pvc1, pvc2)

			err := kv.validateResumePVCsByUUID(ref.Ref{ID: resumeVMID}, testUUID)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("duplicate disk index"))
		})

		ginkgo.It("should accept a valid set of Bound PVCs with unique identities", func() {
			migUID := types.UID("mig-" + rand.String(6))
			pvc1 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "pvc-1",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kVM:     resumeVMID,
						kVmUuid: testUUID,
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[disk-1]",
						planbase.AnnDiskIndex:  "0",
					},
				},
				Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
			}
			pvc2 := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "pvc-2",
					Namespace: resumeTargetNS,
					Labels: map[string]string{
						kVM:     resumeVMID,
						kVmUuid: testUUID,
					},
					Annotations: map[string]string{
						planbase.AnnDiskSource: "[disk-2]",
						planbase.AnnDiskIndex:  "1",
					},
				},
				Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
			}
			kv, _ := newResumeKubeVirt(migUID, pvc1, pvc2)

			err := kv.validateResumePVCsByUUID(ref.Ref{ID: resumeVMID}, testUUID)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
