package plan

import (
	"context"
	"fmt"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	convctx "github.com/kubev2v/forklift/pkg/controller/conversion/context"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
	return newResumeKubeVirtWithScheme(migrationUID, false, objects...)
}

func newResumeKubeVirtWithScheme(migrationUID types.UID, withAPI bool, objects ...client.Object) (KubeVirt, *Migration) {
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	if withAPI {
		_ = api.SchemeBuilder.AddToScheme(scheme)
	}

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
		Client:      c,
		Destination: plancontext.Destination{Client: c},
		Log:         resumeLog,
	}

	kv := KubeVirt{Context: ctx}
	m := &Migration{Context: ctx, kubevirt: kv}
	return kv, m
}

func resumeDiskPVC(name, vmUUID, diskSource, diskIndex string) *core.PersistentVolumeClaim {
	return &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name:      name,
			Namespace: resumeTargetNS,
			Labels: map[string]string{
				kVM:     resumeVMID,
				kVmUuid: vmUUID,
			},
			Annotations: map[string]string{
				planbase.AnnDiskSource: diskSource,
				planbase.AnnDiskIndex:  diskIndex,
			},
		},
		Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
	}
}

func newResumeVMStatus() *planapi.VMStatus {
	return &planapi.VMStatus{
		VM: planapi.VM{
			Ref: ref.Ref{ID: resumeVMID},
		},
	}
}

var _ = ginkgo.Describe("GetConversionPod", func() {
	var (
		oldMigrationUID types.UID
		newMigrationUID types.UID
		vmRef           ref.Ref
	)

	ginkgo.BeforeEach(func() {
		oldMigrationUID = types.UID("old-migration-" + rand.String(6))
		newMigrationUID = types.UID("new-migration-" + rand.String(6))
		vmRef = ref.Ref{ID: resumeVMID}
	})

	ginkgo.It("should find the current migration pod when filterOutMigrationLabel is false", func() {
		currentPod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:      "current-conv-pod",
				Namespace: resumeTargetNS,
				Labels:    resumeConvPodLabels(newMigrationUID),
			},
		}
		stalePod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:      "stale-conv-pod",
				Namespace: resumeTargetNS,
				Labels:    resumeConvPodLabels(oldMigrationUID),
			},
		}
		kv, _ := newResumeKubeVirt(newMigrationUID, currentPod, stalePod)

		pod, err := kv.GetConversionPod(vmRef, VirtV2vConversionPod, false)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pod).NotTo(gomega.BeNil())
		gomega.Expect(pod.Name).To(gomega.Equal("current-conv-pod"))
	})

	ginkgo.It("should find a prior migration pod when filterOutMigrationLabel is true", func() {
		stalePod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:      "stale-conv-pod",
				Namespace: resumeTargetNS,
				Labels:    resumeConvPodLabels(oldMigrationUID),
			},
		}
		kv, _ := newResumeKubeVirt(newMigrationUID, stalePod)

		pod, err := kv.GetConversionPod(vmRef, VirtV2vConversionPod, true)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pod).NotTo(gomega.BeNil())
		gomega.Expect(pod.Name).To(gomega.Equal("stale-conv-pod"))
	})

	ginkgo.It("should not find a prior migration pod when filterOutMigrationLabel is false", func() {
		stalePod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:      "stale-conv-pod",
				Namespace: resumeTargetNS,
				Labels:    resumeConvPodLabels(oldMigrationUID),
			},
		}
		kv, _ := newResumeKubeVirt(newMigrationUID, stalePod)

		pod, err := kv.GetConversionPod(vmRef, VirtV2vConversionPod, false)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pod).To(gomega.BeNil())
	})

	ginkgo.It("should return nil when no matching pod exists", func() {
		kv, _ := newResumeKubeVirt(newMigrationUID)

		pod, err := kv.GetConversionPod(vmRef, VirtV2vConversionPod, true)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pod).To(gomega.BeNil())
	})
})

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

		ginkgo.It("should request DeleteConversion for a stale Conversion CR", func() {
			settings.Settings.UseConversionCR = true
			staleCR := &api.Conversion{
				ObjectMeta: meta.ObjectMeta{
					Name:      "stale-conversion",
					Namespace: resumePlanNamespace,
					Labels: map[string]string{
						convctx.LabelPlan:           string(resumePlanUID),
						convctx.LabelVM:             resumeVMID,
						convctx.LabelConversionType: string(api.Remote),
					},
				},
				Spec: api.ConversionSpec{
					Type: api.Remote,
				},
			}
			_, m := newResumeKubeVirtWithScheme(newMigrationUID, true, staleCR)
			vm := newResumeVMStatus()

			done, err := m.deleteStaleConversionWorkloads(vm)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(done).To(gomega.BeFalse(), "should requeue after requesting CR deletion")
			gomega.Expect(objectExists(m.Client, types.NamespacedName{
				Name: "stale-conversion", Namespace: resumePlanNamespace,
			}, &api.Conversion{})).To(gomega.BeFalse(), "Conversion CR should be deleted")
		})
	})
})

var _ = ginkgo.Describe("ensureStaleObjectDeleted", func() {
	var (
		newMigrationUID types.UID
	)

	ginkgo.BeforeEach(func() {
		newMigrationUID = types.UID("new-migration-" + rand.String(6))
		settings.Settings.StaleConversionTimeout = 60
	})

	ginkgo.It("should requeue without deleting when object is terminating within timeout", func() {
		recent := meta.NewTime(time.Now().Add(-10 * time.Second))
		pod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:              "terminating-pod",
				Namespace:         resumeTargetNS,
				Labels:            resumeConvPodLabels(newMigrationUID),
				DeletionTimestamp: &recent,
				Finalizers:        []string{"block-deletion"},
			},
		}
		_, m := newResumeKubeVirt(newMigrationUID, pod)
		vm := newResumeVMStatus()

		done, err := m.ensureStaleObjectDeleted(pod, vm, "pod", func() error {
			return fmt.Errorf("should not be called")
		}, true)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(done).To(gomega.BeFalse())
		gomega.Expect(objectExists(m.Destination.Client, types.NamespacedName{
			Name: "terminating-pod", Namespace: resumeTargetNS,
		}, &core.Pod{})).To(gomega.BeTrue(), "pod should not have been deleted")
	})

	ginkgo.It("should force-delete when object exceeds stale timeout", func() {
		expired := meta.NewTime(time.Now().Add(-120 * time.Second))
		pod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:              "stuck-pod",
				Namespace:         resumeTargetNS,
				Labels:            resumeConvPodLabels(newMigrationUID),
				DeletionTimestamp: &expired,
				Finalizers:        []string{"block-deletion"},
			},
		}
		_, m := newResumeKubeVirt(newMigrationUID, pod)
		vm := newResumeVMStatus()

		done, err := m.ensureStaleObjectDeleted(pod, vm, "pod", func() error {
			return fmt.Errorf("should not be called")
		}, true)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(done).To(gomega.BeFalse(), "always returns false after delete")
	})

	ginkgo.It("should treat force-delete NotFound as successful cleanup", func() {
		expired := meta.NewTime(time.Now().Add(-120 * time.Second))
		pod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:              "gone-pod",
				Namespace:         resumeTargetNS,
				Labels:            resumeConvPodLabels(newMigrationUID),
				DeletionTimestamp: &expired,
				Finalizers:        []string{"block-deletion"},
			},
		}
		scheme := runtime.NewScheme()
		_ = core.AddToScheme(scheme)
		c := fakeClient.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(pod).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					return k8serr.NewNotFound(core.Resource("pods"), obj.GetName())
				},
			}).
			Build()
		ctx := &plancontext.Context{
			Plan: &api.Plan{
				ObjectMeta: meta.ObjectMeta{
					Name: resumePlanName, Namespace: resumePlanNamespace, UID: resumePlanUID,
				},
				Spec: api.PlanSpec{TargetNamespace: resumeTargetNS},
			},
			Migration:   &api.Migration{ObjectMeta: meta.ObjectMeta{UID: newMigrationUID}},
			Destination: plancontext.Destination{Client: c},
			Log:         resumeLog,
		}
		m := &Migration{Context: ctx}
		vm := newResumeVMStatus()

		done, err := m.ensureStaleObjectDeleted(pod, vm, "pod", func() error {
			return fmt.Errorf("should not be called")
		}, true)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(done).To(gomega.BeFalse())
	})

	ginkgo.It("should call deleteFn when conversion CR exceeds stale timeout", func() {
		expired := meta.NewTime(time.Now().Add(-120 * time.Second))
		cr := &api.Conversion{
			ObjectMeta: meta.ObjectMeta{
				Name:              "stuck-conversion",
				Namespace:         resumePlanNamespace,
				DeletionTimestamp: &expired,
				Finalizers:        []string{"block-deletion"},
			},
		}
		_, m := newResumeKubeVirt(newMigrationUID)
		vm := newResumeVMStatus()

		called := false
		done, err := m.ensureStaleObjectDeleted(cr, vm, "conversion", func() error {
			called = true
			return nil
		}, false)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(done).To(gomega.BeFalse())
		gomega.Expect(called).To(gomega.BeTrue(), "timeout path should use deleteFn for Conversion CRs")
	})

	ginkgo.It("should call deleteFn for objects without DeletionTimestamp", func() {
		pod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:      "active-pod",
				Namespace: resumeTargetNS,
				Labels:    resumeConvPodLabels(newMigrationUID),
			},
		}
		_, m := newResumeKubeVirt(newMigrationUID, pod)
		vm := newResumeVMStatus()

		called := false
		done, err := m.ensureStaleObjectDeleted(pod, vm, "pod", func() error {
			called = true
			return nil
		}, true)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(done).To(gomega.BeFalse())
		gomega.Expect(called).To(gomega.BeTrue(), "deleteFn should have been invoked")
	})

	ginkgo.It("should propagate deleteFn errors", func() {
		pod := &core.Pod{
			ObjectMeta: meta.ObjectMeta{
				Name:      "active-pod",
				Namespace: resumeTargetNS,
				Labels:    resumeConvPodLabels(newMigrationUID),
			},
		}
		_, m := newResumeKubeVirt(newMigrationUID, pod)
		vm := newResumeVMStatus()

		done, err := m.ensureStaleObjectDeleted(pod, vm, "pod", func() error {
			return fmt.Errorf("delete failed")
		}, true)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("delete failed"))
		gomega.Expect(done).To(gomega.BeFalse())
	})
})

var _ = ginkgo.Describe("listDiskPVCsByUUID", func() {
	const testUUID = "test-uuid"

	ginkgo.It("should filter prime and non-disk PVCs and sort by disk index", func() {
		migUID := types.UID("mig-" + rand.String(6))
		pvc0 := resumeDiskPVC("pvc-0", testUUID, "[disk-1]", "1")
		pvc1 := resumeDiskPVC("pvc-1", testUUID, "[disk-2]", "0")
		primePVC := &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      "prime-populator",
				Namespace: resumeTargetNS,
				Labels: map[string]string{
					kVM:     resumeVMID,
					kVmUuid: testUUID,
				},
				Annotations: map[string]string{
					planbase.AnnDiskSource: "[disk-3]",
					planbase.AnnDiskIndex:  "2",
				},
			},
			Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
		}
		noisePVC := &core.PersistentVolumeClaim{
			ObjectMeta: meta.ObjectMeta{
				Name:      "noise-pvc",
				Namespace: resumeTargetNS,
				Labels: map[string]string{
					kVM:     resumeVMID,
					kVmUuid: testUUID,
				},
			},
			Status: core.PersistentVolumeClaimStatus{Phase: core.ClaimBound},
		}
		kv, _ := newResumeKubeVirt(migUID, pvc0, pvc1, primePVC, noisePVC)

		pvcs, err := kv.listDiskPVCsByUUID(resumeVMID, testUUID)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pvcs).To(gomega.HaveLen(2))
		gomega.Expect(pvcs[0].Name).To(gomega.Equal("pvc-1"))
		gomega.Expect(pvcs[1].Name).To(gomega.Equal("pvc-0"))
	})

	ginkgo.It("should reject duplicate disk sources found via listDiskPVCsByUUID", func() {
		migUID := types.UID("mig-" + rand.String(6))
		pvc1 := resumeDiskPVC("pvc-1", testUUID, "[disk-1]", "0")
		pvc2 := resumeDiskPVC("pvc-2", testUUID, "[disk-1]", "1")
		kv, _ := newResumeKubeVirt(migUID, pvc1, pvc2)

		pvcs, err := kv.listDiskPVCsByUUID(resumeVMID, testUUID)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pvcs).To(gomega.HaveLen(2))

		err = kv.validateResumePVCsByUUID(ref.Ref{ID: resumeVMID}, testUUID)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("duplicate disk source"))
	})

	ginkgo.It("should reject duplicate disk indexes found via listDiskPVCsByUUID", func() {
		migUID := types.UID("mig-" + rand.String(6))
		pvc1 := resumeDiskPVC("pvc-1", testUUID, "[disk-1]", "0")
		pvc2 := resumeDiskPVC("pvc-2", testUUID, "[disk-2]", "0")
		kv, _ := newResumeKubeVirt(migUID, pvc1, pvc2)

		pvcs, err := kv.listDiskPVCsByUUID(resumeVMID, testUUID)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(pvcs).To(gomega.HaveLen(2))

		err = kv.validateResumePVCsByUUID(ref.Ref{ID: resumeVMID}, testUUID)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("duplicate disk index"))
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
