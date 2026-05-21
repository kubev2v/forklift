package provider

import (
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	libcnd "github.com/kubev2v/forklift/pkg/lib/condition"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
)

var vibTestLog = logging.WithName("vibTest")

func makeProvider(pType api.ProviderType, url string, settings map[string]string) *api.Provider {
	t := pType
	return &api.Provider{
		Spec: api.ProviderSpec{
			Type:     &t,
			URL:      url,
			Settings: settings,
		},
	}
}

func makeSecret() *core.Secret {
	return &core.Secret{
		Data: map[string][]byte{
			"user":     []byte("admin"),
			"password": []byte("pass"),
		},
	}
}

var _ = Describe("UseVIBMethod", func() {
	It("should return true when settings is nil", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", nil)
		Expect(p.UseVIBMethod()).To(BeTrue())
	})

	It("should return true when settings is empty", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", map[string]string{})
		Expect(p.UseVIBMethod()).To(BeTrue())
	})

	It("should return true when esxiCloneMethod is vib", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", map[string]string{
			api.ESXiCloneMethod: api.ESXiCloneMethodVIB,
		})
		Expect(p.UseVIBMethod()).To(BeTrue())
	})

	It("should return false when esxiCloneMethod is ssh", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", map[string]string{
			api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
		})
		Expect(p.UseVIBMethod()).To(BeFalse())
	})

	It("should return false for unknown esxiCloneMethod values", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", map[string]string{
			api.ESXiCloneMethod: "something-else",
		})
		Expect(p.UseVIBMethod()).To(BeFalse())
	})
})

var _ = Describe("validateVIBReadiness", func() {
	var (
		reconciler *Reconciler
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		reconciler.Log = vibTestLog
	})

	Describe("Provider type guards", func() {
		It("should return nil for OpenShift provider", func() {
			p := makeProvider(api.OpenShift, "https://ocp.test", nil)
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil for OVirt provider", func() {
			p := makeProvider(api.OVirt, "https://rhv.test", nil)
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil for OpenStack provider", func() {
			p := makeProvider(api.OpenStack, "https://os.test", nil)
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil for OVA provider", func() {
			p := makeProvider(api.Ova, "https://ova.test", nil)
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not touch pre-existing VIB conditions on non-VSphere providers", func() {
			p := makeProvider(api.OpenShift, "https://ocp.test", nil)
			p.Status.SetCondition(libcnd.Condition{
				Type:   VIBReady,
				Status: True,
			})
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.FindCondition(VIBReady)).NotTo(BeNil())
		})
	})

	Describe("VIB method disabled", func() {
		It("should delete VIBReady when esxiCloneMethod is ssh", func() {
			p := makeProvider(api.VSphere, "https://vcenter.test", map[string]string{
				api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
			})
			p.Status.SetCondition(libcnd.Condition{
				Type:   VIBReady,
				Status: True,
				Items:  []string{"host-1|esxi1"},
			})
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.FindCondition(VIBReady)).To(BeNil())
		})

		It("should delete VIBNotReady when esxiCloneMethod is ssh", func() {
			p := makeProvider(api.VSphere, "https://vcenter.test", map[string]string{
				api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
			})
			p.Status.SetCondition(libcnd.Condition{
				Type:   VIBNotReady,
				Status: True,
				Items:  []string{"host-1|esxi1", "host-2|esxi2"},
			})
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
			Expect(p.Status.FindCondition(VIBNotReady)).To(BeNil())
		})

		It("should be safe when no VIB conditions exist", func() {
			p := makeProvider(api.VSphere, "https://vcenter.test", map[string]string{
				api.ESXiCloneMethod: api.ESXiCloneMethodSSH,
			})
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Inventory not ready", func() {
		It("should return nil when InventoryCreated condition is absent", func() {
			p := makeProvider(api.VSphere, "https://vcenter.test", nil)
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil when InventoryCreated is not True", func() {
			p := makeProvider(api.VSphere, "https://vcenter.test", nil)
			p.Status.SetCondition(libcnd.Condition{
				Type:   InventoryCreated,
				Status: libcnd.False,
			})
			err := reconciler.validateVIBReadiness(p, makeSecret())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("IsHost guard", func() {
		It("should identify host provider (OpenShift with empty URL)", func() {
			p := makeProvider(api.OpenShift, "", nil)
			Expect(p.IsHost()).To(BeTrue())
		})

		It("should not identify non-host provider", func() {
			p := makeProvider(api.VSphere, "https://vcenter.test", nil)
			Expect(p.IsHost()).To(BeFalse())
		})
	})
})

var _ = Describe("VIB condition items format", func() {
	It("should parse id|name format correctly", func() {
		items := []string{"host-123|esxi-prod-01", "host-456|esxi-prod-02"}
		for _, item := range items {
			id, name, found := strings.Cut(item, "|")
			Expect(found).To(BeTrue())
			Expect(id).NotTo(BeEmpty())
			Expect(name).NotTo(BeEmpty())
		}
	})

	It("should handle items without delimiter gracefully", func() {
		item := "host-123-no-delimiter"
		id, _, found := strings.Cut(item, "|")
		Expect(found).To(BeFalse())
		Expect(id).To(Equal(item))
	})

	It("should support VIBReady and VIBNotReady coexisting", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", nil)
		p.Status.SetCondition(libcnd.Condition{
			Type:    VIBReady,
			Status:  True,
			Items:   []string{"host-1|esxi1"},
			Durable: true,
		})
		p.Status.SetCondition(libcnd.Condition{
			Type:    VIBNotReady,
			Status:  True,
			Items:   []string{"host-2|esxi2"},
			Durable: true,
		})
		Expect(p.Status.FindCondition(VIBReady)).NotTo(BeNil())
		Expect(p.Status.FindCondition(VIBNotReady)).NotTo(BeNil())
		Expect(p.Status.FindCondition(VIBReady).Items).To(HaveLen(1))
		Expect(p.Status.FindCondition(VIBNotReady).Items).To(HaveLen(1))
	})

	It("should replace condition when setting same type twice", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", nil)
		p.Status.SetCondition(libcnd.Condition{
			Type:   VIBReady,
			Status: True,
			Items:  []string{"host-1|esxi1"},
		})
		p.Status.SetCondition(libcnd.Condition{
			Type:   VIBReady,
			Status: True,
			Items:  []string{"host-1|esxi1", "host-2|esxi2"},
		})
		cond := p.Status.FindCondition(VIBReady)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Items).To(HaveLen(2))
	})

	It("should clear condition with DeleteCondition", func() {
		p := makeProvider(api.VSphere, "https://vcenter.test", nil)
		p.Status.SetCondition(libcnd.Condition{
			Type:   VIBReady,
			Status: True,
		})
		Expect(p.Status.FindCondition(VIBReady)).NotTo(BeNil())
		p.Status.DeleteCondition(VIBReady)
		Expect(p.Status.FindCondition(VIBReady)).To(BeNil())
	})
})
