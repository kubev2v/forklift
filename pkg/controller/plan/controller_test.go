package plan

import (
	"context"

	"github.com/kubev2v/forklift/pkg/controller/base"
	"github.com/kubev2v/forklift/pkg/controller/plan/util"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var controllerLog = logging.WithName("controllerTest")

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = core.AddToScheme(scheme)
	_ = cdi.AddToScheme(scheme)
	return scheme
}

func newTestReconciler(objects ...client.Object) *Reconciler {
	scheme := newTestScheme()
	c := fakeClient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
	return &Reconciler{
		Reconciler: base.Reconciler{
			Client: c,
			Log:    controllerLog,
		},
		APIReader: c,
	}
}

// Helper to check whether an object still exists in the fake client.
func objectExists(c client.Client, key types.NamespacedName, obj client.Object) bool {
	err := c.Get(context.TODO(), key, obj)
	if err == nil {
		return true
	}
	if k8serr.IsNotFound(err) {
		return false
	}
	panic(err)
}

var _ = ginkgo.Describe("cleanupOrphanedResources", func() {
	const (
		planName      = "test-plan"
		planNamespace = "test-ns"
		targetNS      = "target-ns"
	)

	// labels returns the plan-name / plan-namespace labels that the GC uses.
	labels := func() map[string]string {
		return map[string]string{
			kPlanName:      planName,
			kPlanNamespace: planNamespace,
		}
	}

	ginkgo.Context("orphaned PVC deletion", func() {
		ginkgo.It("should delete PVCs that have no VirtualMachine owner", func() {
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "orphan-pvc",
					Namespace: targetNS,
					Labels:    labels(),
				},
			}
			r := newTestReconciler(pvc)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "orphan-pvc", Namespace: targetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeFalse())
		})

		ginkgo.It("should skip PVCs owned by a VirtualMachine", func() {
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "vm-owned-pvc",
					Namespace: targetNS,
					Labels:    labels(),
					OwnerReferences: []meta.OwnerReference{
						{
							APIVersion: "kubevirt.io/v1",
							Kind:       util.VirtualMachineKind,
							Name:       "my-vm",
							UID:        "vm-uid-123",
						},
					},
				},
			}
			r := newTestReconciler(pvc)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "vm-owned-pvc", Namespace: targetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeTrue())
		})
	})

	ginkgo.Context("orphaned DataVolume deletion", func() {
		ginkgo.It("should delete DataVolumes that have no VirtualMachine owner", func() {
			dv := &cdi.DataVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:      "orphan-dv",
					Namespace: targetNS,
					Labels:    labels(),
				},
			}
			r := newTestReconciler(dv)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "orphan-dv", Namespace: targetNS,
			}, &cdi.DataVolume{})).To(gomega.BeFalse())
		})

		ginkgo.It("should skip DataVolumes owned by a VirtualMachine", func() {
			dv := &cdi.DataVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:      "vm-owned-dv",
					Namespace: targetNS,
					Labels:    labels(),
					OwnerReferences: []meta.OwnerReference{
						{
							APIVersion: "kubevirt.io/v1",
							Kind:       util.VirtualMachineKind,
							Name:       "my-vm",
							UID:        "vm-uid-456",
						},
					},
				},
			}
			r := newTestReconciler(dv)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "vm-owned-dv", Namespace: targetNS,
			}, &cdi.DataVolume{})).To(gomega.BeTrue())
		})
	})

	ginkgo.Context("orphaned Pods, Secrets, and ConfigMaps", func() {
		ginkgo.It("should delete orphaned Pods", func() {
			pod := &core.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:      "orphan-pod",
					Namespace: targetNS,
					Labels:    labels(),
				},
			}
			r := newTestReconciler(pod)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "orphan-pod", Namespace: targetNS,
			}, &core.Pod{})).To(gomega.BeFalse())
		})

		ginkgo.It("should delete orphaned Secrets", func() {
			secret := &core.Secret{
				ObjectMeta: meta.ObjectMeta{
					Name:      "orphan-secret",
					Namespace: targetNS,
					Labels:    labels(),
				},
			}
			r := newTestReconciler(secret)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "orphan-secret", Namespace: targetNS,
			}, &core.Secret{})).To(gomega.BeFalse())
		})

		ginkgo.It("should delete orphaned ConfigMaps", func() {
			cm := &core.ConfigMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "orphan-cm",
					Namespace: targetNS,
					Labels:    labels(),
				},
			}
			r := newTestReconciler(cm)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "orphan-cm", Namespace: targetNS,
			}, &core.ConfigMap{})).To(gomega.BeFalse())
		})
	})

	ginkgo.Context("orphaned PersistentVolumes (cluster-scoped)", func() {
		ginkgo.It("should delete orphaned PVs", func() {
			pv := &core.PersistentVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:   "orphan-pv",
					Labels: labels(),
				},
			}
			r := newTestReconciler(pv)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "orphan-pv",
			}, &core.PersistentVolume{})).To(gomega.BeFalse())
		})
	})

	ginkgo.Context("no-op when no matching resources exist", func() {
		ginkgo.It("should complete without error when the cluster is empty", func() {
			r := newTestReconciler()

			gomega.Expect(func() {
				r.cleanupOrphanedResources(planName, planNamespace)
			}).NotTo(gomega.Panic())
		})

		ginkgo.It("should not delete resources that lack the plan labels", func() {
			unlabeledPVC := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "unrelated-pvc",
					Namespace: targetNS,
				},
			}
			unlabeledSecret := &core.Secret{
				ObjectMeta: meta.ObjectMeta{
					Name:      "unrelated-secret",
					Namespace: targetNS,
				},
			}
			r := newTestReconciler(unlabeledPVC, unlabeledSecret)

			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "unrelated-pvc", Namespace: targetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeTrue())
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "unrelated-secret", Namespace: targetNS,
			}, &core.Secret{})).To(gomega.BeTrue())
		})
	})

	ginkgo.Context("cross-plan isolation", func() {
		ginkgo.It("should not delete resources belonging to a different plan", func() {
			otherPlanLabels := map[string]string{
				kPlanName:      "other-plan",
				kPlanNamespace: "other-ns",
			}
			otherPVC := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "other-plan-pvc",
					Namespace: targetNS,
					Labels:    otherPlanLabels,
				},
			}
			otherPod := &core.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:      "other-plan-pod",
					Namespace: targetNS,
					Labels:    otherPlanLabels,
				},
			}
			otherSecret := &core.Secret{
				ObjectMeta: meta.ObjectMeta{
					Name:      "other-plan-secret",
					Namespace: targetNS,
					Labels:    otherPlanLabels,
				},
			}
			otherCM := &core.ConfigMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "other-plan-cm",
					Namespace: targetNS,
					Labels:    otherPlanLabels,
				},
			}
			otherPV := &core.PersistentVolume{
				ObjectMeta: meta.ObjectMeta{
					Name:   "other-plan-pv",
					Labels: otherPlanLabels,
				},
			}
			r := newTestReconciler(otherPVC, otherPod, otherSecret, otherCM, otherPV)

			// Delete "test-plan" — nothing should happen to "other-plan" resources.
			r.cleanupOrphanedResources(planName, planNamespace)

			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "other-plan-pvc", Namespace: targetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeTrue())
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "other-plan-pod", Namespace: targetNS,
			}, &core.Pod{})).To(gomega.BeTrue())
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "other-plan-secret", Namespace: targetNS,
			}, &core.Secret{})).To(gomega.BeTrue())
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "other-plan-cm", Namespace: targetNS,
			}, &core.ConfigMap{})).To(gomega.BeTrue())
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "other-plan-pv",
			}, &core.PersistentVolume{})).To(gomega.BeTrue())
		})

		ginkgo.It("should only delete resources for the target plan in a mixed cluster", func() {
			// Resources belonging to our plan — should be deleted.
			ourPVC := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "our-pvc",
					Namespace: targetNS,
					Labels:    labels(),
				},
			}
			ourPod := &core.Pod{
				ObjectMeta: meta.ObjectMeta{
					Name:      "our-pod",
					Namespace: targetNS,
					Labels:    labels(),
				},
			}
			// Resources belonging to another plan — should survive.
			otherLabels := map[string]string{
				kPlanName:      "other-plan",
				kPlanNamespace: planNamespace,
			}
			otherPVC := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "other-pvc",
					Namespace: targetNS,
					Labels:    otherLabels,
				},
			}
			// VM-owned PVC for our plan — should survive.
			vmOwnedPVC := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{
					Name:      "vm-pvc",
					Namespace: targetNS,
					Labels:    labels(),
					OwnerReferences: []meta.OwnerReference{
						{
							APIVersion: "kubevirt.io/v1",
							Kind:       util.VirtualMachineKind,
							Name:       "migrated-vm",
							UID:        "vm-uid-789",
						},
					},
				},
			}
			r := newTestReconciler(ourPVC, ourPod, otherPVC, vmOwnedPVC)

			r.cleanupOrphanedResources(planName, planNamespace)

			// Our orphaned resources are gone.
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "our-pvc", Namespace: targetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeFalse())
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "our-pod", Namespace: targetNS,
			}, &core.Pod{})).To(gomega.BeFalse())

			// Other plan's PVC is untouched.
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "other-pvc", Namespace: targetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeTrue())

			// VM-owned PVC is preserved.
			gomega.Expect(objectExists(r.Client, types.NamespacedName{
				Name: "vm-pvc", Namespace: targetNS,
			}, &core.PersistentVolumeClaim{})).To(gomega.BeTrue())
		})
	})
})

var _ = ginkgo.Describe("hasVMOwner", func() {
	ginkgo.DescribeTable("owner reference checks",
		func(owners []meta.OwnerReference, expected bool) {
			gomega.Expect(hasVMOwner(owners)).To(gomega.Equal(expected))
		},
		ginkgo.Entry("returns true when VirtualMachine owner exists",
			[]meta.OwnerReference{
				{Kind: "DataVolume", Name: "dv-1", UID: "uid-1"},
				{Kind: util.VirtualMachineKind, Name: "vm-1", UID: "uid-2"},
			}, true),
		ginkgo.Entry("returns false when no VirtualMachine owner exists",
			[]meta.OwnerReference{
				{Kind: "DataVolume", Name: "dv-1", UID: "uid-1"},
				{Kind: "ReplicaSet", Name: "rs-1", UID: "uid-3"},
			}, false),
		ginkgo.Entry("returns false for nil owner list", nil, false),
		ginkgo.Entry("returns false for empty owner list",
			[]meta.OwnerReference{}, false),
	)
})
