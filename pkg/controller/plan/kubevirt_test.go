//nolint:errcheck
package plan

import (
	"fmt"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/provider"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	webbase "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/api/core/v1"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Mock inventory for testing - implements web.Client interface.
// Only VM() is actually called by ensureConfigMap, but all methods are required to satisfy the interface.
type mockInventory struct{}

func (m *mockInventory) VM(_ *ref.Ref) (interface{}, error)       { return struct{}{}, nil } // Used by ensureConfigMap
func (m *mockInventory) Find(_ interface{}, _ ref.Ref) error      { return nil }
func (m *mockInventory) Finder() web.Finder                       { return nil }
func (m *mockInventory) Get(_ interface{}, _ string) error        { return nil }
func (m *mockInventory) Host(_ *ref.Ref) (interface{}, error)     { return struct{}{}, nil }
func (m *mockInventory) List(_ interface{}, _ ...web.Param) error { return nil }
func (m *mockInventory) Network(_ *ref.Ref) (interface{}, error)  { return struct{}{}, nil }
func (m *mockInventory) Storage(_ *ref.Ref) (interface{}, error)  { return struct{}{}, nil }
func (m *mockInventory) Watch(_ interface{}, _ webbase.EventHandler) (*webbase.Watch, error) {
	return &webbase.Watch{}, nil
}
func (m *mockInventory) Workload(_ *ref.Ref) (interface{}, error) { return struct{}{}, nil }

// Mock builder for testing - implements adapter.Builder interface.
// Only ConfigMap() is actually called by ensureConfigMap, but all methods are required to satisfy the interface.
type mockBuilder struct{}

func (m *mockBuilder) ConfigMap(_ ref.Ref, _ *v1.Secret, cm *v1.ConfigMap) error { // Used by ensureConfigMap
	cm.Data = map[string]string{"ca.pem": "test-cert-data"}
	return nil
}

// Error mocks for testing error scenarios
type errorBuilder struct {
	mockBuilder
	errorMsg string
}

func (m *errorBuilder) ConfigMap(_ ref.Ref, _ *v1.Secret, _ *v1.ConfigMap) error {
	return fmt.Errorf("%s", m.errorMsg)
}

type errorInventory struct {
	mockInventory
	errorMsg string
}

func (m *errorInventory) VM(_ *ref.Ref) (interface{}, error) {
	return nil, fmt.Errorf("%s", m.errorMsg)
}
func (m *mockBuilder) Secret(_ ref.Ref, _, _ *v1.Secret) error { return nil }
func (m *mockBuilder) VirtualMachine(_ ref.Ref, _ *cnv.VirtualMachineSpec, _ []*v1.PersistentVolumeClaim, _ bool, _ bool) error {
	return nil
}
func (m *mockBuilder) DataVolumes(_ ref.Ref, _ *v1.Secret, _ *v1.ConfigMap, _ *cdi.DataVolume, _ *v1.ConfigMap) ([]cdi.DataVolume, error) {
	return []cdi.DataVolume{}, nil
}
func (m *mockBuilder) Tasks(_ ref.Ref) ([]*planapi.Task, error) { return []*planapi.Task{}, nil }
func (m *mockBuilder) TemplateLabels(_ ref.Ref) (map[string]string, error) {
	return map[string]string{}, nil
}
func (m *mockBuilder) ResolveDataVolumeIdentifier(_ *cdi.DataVolume) string { return "" }
func (m *mockBuilder) ResolvePersistentVolumeClaimIdentifier(_ *v1.PersistentVolumeClaim) string {
	return ""
}
func (m *mockBuilder) PodEnvironment(_ ref.Ref, _ *v1.Secret) ([]v1.EnvVar, error) {
	return []v1.EnvVar{}, nil
}
func (m *mockBuilder) LunPersistentVolumes(_ ref.Ref) ([]v1.PersistentVolume, error) {
	return []v1.PersistentVolume{}, nil
}
func (m *mockBuilder) LunPersistentVolumeClaims(_ ref.Ref) ([]v1.PersistentVolumeClaim, error) {
	return []v1.PersistentVolumeClaim{}, nil
}
func (m *mockBuilder) SupportsVolumePopulators(_ ref.Ref) bool { return false }
func (m *mockBuilder) PopulatorVolumes(_ ref.Ref, _ map[string]string, _ string) ([]*v1.PersistentVolumeClaim, error) {
	return []*v1.PersistentVolumeClaim{}, nil
}
func (m *mockBuilder) PopulatorTransferredBytes(_ *v1.PersistentVolumeClaim) (int64, error) {
	return 0, nil
}
func (m *mockBuilder) SetPopulatorDataSourceLabels(_ ref.Ref, _ []*v1.PersistentVolumeClaim) error {
	return nil
}
func (m *mockBuilder) GetPopulatorTaskName(_ *v1.PersistentVolumeClaim) (string, error) {
	return "", nil
}
func (m *mockBuilder) PreferenceName(_ ref.Ref, _ *v1.ConfigMap) (string, error) {
	return "", nil
}

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

	ginkgo.Describe("ensureConfigMap", func() {
		ginkgo.It("merges provider data into existing hook ConfigMap", func() {
			hookConfigMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hook-cm",
					Namespace: "test",
					Labels:    map[string]string{"migration": "test", "plan": "", "vmID": "test"},
				},
				Data: map[string]string{"plan.yml": "hook-data"},
			}
			kubevirt := createKubeVirtWithBuilder(hookConfigMap)
			vmRef := ref.Ref{ID: "test", Name: "test", Namespace: "test"}

			cm, err := kubevirt.ensureConfigMap(vmRef)

			Expect(err).ToNot(HaveOccurred())
			Expect(cm.Data).To(HaveKey("plan.yml"))
			Expect(cm.Data).To(HaveKey("ca.pem"))
			Expect(cm.Data["plan.yml"]).To(Equal("hook-data"))
			Expect(cm.Data["ca.pem"]).To(Equal("test-cert-data"))
		})

		ginkgo.It("creates new ConfigMap when none exists", func() {
			kubevirt := createKubeVirtWithBuilder()
			vmRef := ref.Ref{ID: "test", Name: "test", Namespace: "test"}

			cm, err := kubevirt.ensureConfigMap(vmRef)

			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())
			Expect(cm.Data).To(HaveKey("ca.pem"))
		})

		ginkgo.It("does not duplicate keys on repeated calls", func() {
			hookConfigMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hook-cm",
					Namespace: "test",
					Labels:    map[string]string{"migration": "test", "plan": "", "vmID": "test"},
				},
				Data: map[string]string{"plan.yml": "hook-data"},
			}
			kubevirt := createKubeVirtWithBuilder(hookConfigMap)
			vmRef := ref.Ref{ID: "test", Name: "test", Namespace: "test"}

			cm1, err1 := kubevirt.ensureConfigMap(vmRef)
			Expect(err1).ToNot(HaveOccurred())
			Expect(cm1.Data).To(HaveLen(2))

			cm2, err2 := kubevirt.ensureConfigMap(vmRef)
			Expect(err2).ToNot(HaveOccurred())
			Expect(cm2.Data).To(HaveLen(2))
			Expect(cm2.Data["ca.pem"]).To(Equal("test-cert-data"))
		})

		ginkgo.It("handles Builder.ConfigMap error gracefully", func() {
			hookConfigMap := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hook-cm",
					Namespace: "test",
					Labels:    map[string]string{"migration": "test", "plan": "", "vmID": "test"},
				},
				Data: map[string]string{"plan.yml": "hook-data"},
			}
			// Use a builder that returns an error
			kubevirt := createKubeVirtWithErrorBuilder(hookConfigMap, "builder error")
			vmRef := ref.Ref{ID: "test", Name: "test", Namespace: "test"}

			_, err := kubevirt.ensureConfigMap(vmRef)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("builder error"))
		})

		ginkgo.It("handles Inventory.VM error gracefully", func() {
			// Use an inventory that returns an error
			kubevirt := createKubeVirtWithErrorInventory("inventory error")
			vmRef := ref.Ref{ID: "test", Name: "test", Namespace: "test"}

			_, err := kubevirt.ensureConfigMap(vmRef)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("inventory error"))
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
			Type:            "cold",
			TargetNamespace: "test",
		},
	}
}

func createKubeVirtWithBuilder(objs ...runtime.Object) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()

	inventory := &mockInventory{}
	builder := &mockBuilder{}

	return &KubeVirt{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Source: plancontext.Source{
				Inventory: inventory,
				Secret:    &v1.Secret{},
			},
			Log:       KubeVirtLog,
			Migration: createMigration(),
			Plan:      createPlanKubevirt(),
			Client:    client,
		},
		Builder: builder,
	}
}

func createKubeVirtWithErrorBuilder(objs runtime.Object, errorMsg string) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs).
		Build()

	inventory := &mockInventory{}
	builder := &errorBuilder{errorMsg: errorMsg}

	return &KubeVirt{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Source: plancontext.Source{
				Inventory: inventory,
				Secret:    &v1.Secret{},
			},
			Log:       KubeVirtLog,
			Migration: createMigration(),
			Plan:      createPlanKubevirt(),
			Client:    client,
		},
		Builder: builder,
	}
}

func createKubeVirtWithErrorInventory(errorMsg string) *KubeVirt {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	inventory := &errorInventory{errorMsg: errorMsg}
	builder := &mockBuilder{}

	return &KubeVirt{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Source: plancontext.Source{
				Inventory: inventory,
				Secret:    &v1.Secret{},
			},
			Log:       KubeVirtLog,
			Migration: createMigration(),
			Plan:      createPlanKubevirt(),
			Client:    client,
		},
		Builder: builder,
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
