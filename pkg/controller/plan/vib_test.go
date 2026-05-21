package plan

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/pkg/controller/base"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var vibTestLog = logging.WithName("vibTest")

func makeVSphereProvider(settings map[string]string) *api.Provider {
	vsphere := api.VSphere
	p := &api.Provider{
		Spec: api.ProviderSpec{
			Type:     &vsphere,
			URL:      "https://vcenter.example.com",
			Settings: settings,
		},
	}
	return p
}

func makeNonVSphereProvider(pType api.ProviderType) *api.Provider {
	t := pType
	return &api.Provider{
		Spec: api.ProviderSpec{
			Type: &t,
			URL:  "https://example.com",
		},
	}
}

func makePlanWithProvider(provider *api.Provider) *api.Plan {
	plan := &api.Plan{}
	plan.Referenced.Provider.Source = provider
	return plan
}

var _ = ginkgo.Describe("VIB Readiness Validation", func() {
	var reconciler *Reconciler

	ginkgo.BeforeEach(func() {
		reconciler = &Reconciler{
			Reconciler: base.Reconciler{
				Log: vibTestLog,
			},
		}
	})

	ginkgo.Describe("Early exit paths", func() {
		ginkgo.It("should return nil when plan is executing", func() {
			provider := makeVSphereProvider(nil)
			plan := makePlanWithProvider(provider)
			plan.Status.SetCondition(libcnd.Condition{
				Type:   Executing,
				Status: libcnd.True,
			})
			// Pre-set a VIB condition to verify it's not touched
			plan.Status.SetCondition(libcnd.Condition{
				Type:   VIBNotReady,
				Status: libcnd.True,
				Items:  []string{"host-1|esxi-1"},
			})

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Conditions should be untouched
			gomega.Expect(plan.Status.FindCondition(VIBNotReady)).NotTo(gomega.BeNil())
		})

		ginkgo.It("should return nil when source provider is nil", func() {
			plan := &api.Plan{}
			plan.Referenced.Provider.Source = nil

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return nil when provider is not VSphere (OpenShift)", func() {
			provider := makeNonVSphereProvider(api.OpenShift)
			plan := makePlanWithProvider(provider)

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return nil when provider is not VSphere (OVirt)", func() {
			provider := makeNonVSphereProvider(api.OVirt)
			plan := makePlanWithProvider(provider)

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return nil when provider is not VSphere (OpenStack)", func() {
			provider := makeNonVSphereProvider(api.OpenStack)
			plan := makePlanWithProvider(provider)

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("VIB method disabled (esxiCloneMethod=ssh)", func() {
		ginkgo.It("should delete VIBReady when SSH method is used", func() {
			provider := makeVSphereProvider(map[string]string{
				api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
			})
			plan := makePlanWithProvider(provider)
			plan.Status.SetCondition(libcnd.Condition{
				Type:   VIBReady,
				Status: libcnd.True,
			})

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.FindCondition(VIBReady)).To(gomega.BeNil())
		})

		ginkgo.It("should delete VIBNotReady when SSH method is used", func() {
			provider := makeVSphereProvider(map[string]string{
				api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
			})
			plan := makePlanWithProvider(provider)
			plan.Status.SetCondition(libcnd.Condition{
				Type:   VIBNotReady,
				Status: libcnd.True,
				Items:  []string{"host-1|esxi-1", "host-2|esxi-2"},
			})

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.FindCondition(VIBNotReady)).To(gomega.BeNil())
		})

		ginkgo.It("should be safe when no VIB conditions exist", func() {
			provider := makeVSphereProvider(map[string]string{
				api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
			})
			plan := makePlanWithProvider(provider)

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Xcopy populator guard", func() {
		ginkgo.It("should delete VIB conditions when storage map is nil", func() {
			provider := makeVSphereProvider(nil) // VIB method enabled (default)
			plan := makePlanWithProvider(provider)
			plan.Referenced.Map.Storage = nil
			plan.Status.SetCondition(libcnd.Condition{
				Type:   VIBReady,
				Status: libcnd.True,
			})

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.FindCondition(VIBReady)).To(gomega.BeNil())
			gomega.Expect(plan.Status.FindCondition(VIBNotReady)).To(gomega.BeNil())
		})

		ginkgo.It("should delete VIB conditions when storage map has no xcopy config", func() {
			provider := makeVSphereProvider(nil)
			plan := makePlanWithProvider(provider)
			plan.Referenced.Map.Storage = &api.StorageMap{
				Spec: api.StorageMapSpec{
					Map: []api.StoragePair{
						{
							Source: ref.Ref{ID: "ds-1"},
						},
					},
				},
			}
			plan.Status.SetCondition(libcnd.Condition{
				Type:   VIBNotReady,
				Status: libcnd.True,
				Items:  []string{"host-1|esxi-1"},
			})

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(plan.Status.FindCondition(VIBNotReady)).To(gomega.BeNil())
		})
	})

	ginkgo.Describe("Early exit precedence", func() {
		ginkgo.It("should check Executing before provider type", func() {
			// Even with an invalid provider, Executing should cause early return
			plan := &api.Plan{}
			plan.Referenced.Provider.Source = nil
			plan.Status.SetCondition(libcnd.Condition{
				Type:   Executing,
				Status: libcnd.True,
			})

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should check nil provider before type check", func() {
			plan := &api.Plan{}
			plan.Referenced.Provider.Source = nil
			plan.Status.SetCondition(libcnd.Condition{
				Type:   VIBReady,
				Status: libcnd.True,
			})

			err := reconciler.validateVIBReadiness(plan)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// VIBReady not deleted because we exited before the cleanup path
			gomega.Expect(plan.Status.FindCondition(VIBReady)).NotTo(gomega.BeNil())
		})
	})
})

var _ = ginkgo.Describe("UseVIBMethod", func() {
	ginkgo.It("should return true when esxiCloneMethod is not set", func() {
		p := makeVSphereProvider(nil)
		gomega.Expect(p.UseVIBMethod()).To(gomega.BeTrue())
	})

	ginkgo.It("should return true when settings is empty map", func() {
		p := makeVSphereProvider(map[string]string{})
		gomega.Expect(p.UseVIBMethod()).To(gomega.BeTrue())
	})

	ginkgo.It("should return true when esxiCloneMethod is vib", func() {
		p := makeVSphereProvider(map[string]string{
			api.ESXiCloneMethod: api.ESXiCloneMethodVIB,
		})
		gomega.Expect(p.UseVIBMethod()).To(gomega.BeTrue())
	})

	ginkgo.It("should return false when esxiCloneMethod is ssh", func() {
		p := makeVSphereProvider(map[string]string{
			api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
		})
		gomega.Expect(p.UseVIBMethod()).To(gomega.BeFalse())
	})

	ginkgo.It("should return false for unknown esxiCloneMethod values", func() {
		p := makeVSphereProvider(map[string]string{
			api.ESXiCloneMethod: "unknown-method",
		})
		gomega.Expect(p.UseVIBMethod()).To(gomega.BeFalse())
	})

	ginkgo.It("should return true when settings has other keys but not esxiCloneMethod", func() {
		p := makeVSphereProvider(map[string]string{
			api.VDDK: "some-image",
		})
		gomega.Expect(p.UseVIBMethod()).To(gomega.BeTrue())
	})
})
