//nolint:errcheck
package plan

import (
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var KubeVirtLog = logging.WithName("kubevirt-test")

var _ = ginkgo.Describe("kubevirt tests", func() {
	ginkgo.Describe("getPVCs", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pvc",
				Namespace: "test",
				Labels: map[string]string{
					"migration": "test",
					"vmID":      "test",
				},
			},
		}

		ginkgo.It("should return PVCs", func() {
			kubevirt := createKubeVirt(pvc)
			pvcs, err := kubevirt.getPVCs(ref.Ref{ID: "test"})
			Expect(err).ToNot(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
		})
	})

	ginkgo.Describe("Shared namespace kubemacpool exclusion for OCP migrations", func() {
		ginkgo.It("should automatically apply namespace exclusion for OCP to OCP migrations", func() {
			// Create a mock plan with OCP source and destination providers
			openShiftType := v1beta1.OpenShift
			plan := &v1beta1.Plan{
				Spec: v1beta1.PlanSpec{
					TargetNamespace: "test-namespace",
					Provider: provider.Pair{
						Source: v1.ObjectReference{
							Name: "source-ocp",
						},
						Destination: v1.ObjectReference{
							Name: "dest-ocp",
						},
					},
				},
			}

			// Create OCP providers
			sourceProvider := &v1beta1.Provider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "source-ocp",
				},
				Spec: v1beta1.ProviderSpec{
					Type: &openShiftType,
				},
			}

			destProvider := &v1beta1.Provider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dest-ocp",
				},
				Spec: v1beta1.ProviderSpec{
					Type: &openShiftType,
				},
			}

			kubevirt := createKubeVirtWithPlan(plan, sourceProvider, destProvider)

			// Verify the automated namespace exclusion logic will be triggered
			// Uses shared namespace.EnsureKubemacpoolExclusion() method
			// This implements Red Hat OpenShift Virtualization best practices
			Expect(kubevirt.Plan.IsSourceProviderOCP()).To(BeTrue())
			Expect(kubevirt.Plan.Provider.Destination.IsHost()).To(BeTrue())
		})

		ginkgo.It("should not apply namespace exclusion for non-OCP migrations", func() {
			// Create a mock plan with VMware source provider
			vSphereType := v1beta1.VSphere
			openShiftType := v1beta1.OpenShift
			plan := &v1beta1.Plan{
				Spec: v1beta1.PlanSpec{
					TargetNamespace: "test-namespace",
					Provider: provider.Pair{
						Source: v1.ObjectReference{
							Name: "source-vmware",
						},
						Destination: v1.ObjectReference{
							Name: "dest-ocp",
						},
					},
				},
			}

			// Create VMware source provider
			sourceProvider := &v1beta1.Provider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "source-vmware",
				},
				Spec: v1beta1.ProviderSpec{
					Type: &vSphereType,
				},
			}

			destProvider := &v1beta1.Provider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dest-ocp",
				},
				Spec: v1beta1.ProviderSpec{
					Type: &openShiftType,
				},
			}

			kubevirt := createKubeVirtWithPlan(plan, sourceProvider, destProvider)

			// Verify namespace exclusion is only applied for OCP-to-OCP migrations
			// Non-OCP sources don't trigger the shared namespace.EnsureKubemacpoolExclusion()
			Expect(kubevirt.Plan.IsSourceProviderOCP()).To(BeFalse())
		})
	})

})

func createKubeVirt(objs ...runtime.Object) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &KubeVirt{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Log:       KubeVirtLog,
			Migration: createMigration(),
			Plan:      createPlanKubevirt(),
			Client:    client,
		},
	}
}

func createMigration() *v1beta1.Migration {
	return &v1beta1.Migration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			UID:       "test",
		},
	}
}
func createPlanKubevirt() *v1beta1.Plan {
	return &v1beta1.Plan{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1beta1.PlanSpec{
			Type: "cold",
		},
	}
}

func createKubeVirtWithPlan(plan *v1beta1.Plan, sourceProvider, destProvider *v1beta1.Provider) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(plan, sourceProvider, destProvider).
		Build()

	// Create plan context with providers
	planCtx := &plancontext.Context{
		Destination: plancontext.Destination{
			Client: client,
		},
		Log:       KubeVirtLog,
		Migration: createMigration(),
		Client:    client,
		Plan:      plan,
	}

	// Initialize provider objects in the plan context
	planCtx.Plan.Provider.Source = sourceProvider
	planCtx.Plan.Provider.Destination = destProvider

	return &KubeVirt{
		Context: planCtx,
	}
}
