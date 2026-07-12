package openstack

import (
	"encoding/json"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	planbase "github.com/kubev2v/forklift/pkg/controller/plan/adapter/base"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/kubev2v/forklift/pkg/settings"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cnv "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("OpenStack builder", func() {
	DescribeTable("should", func(os, version, distro, matchPreferenceName string) {
		Expect(getPreferenceOs(os, version, distro)).Should(Equal(matchPreferenceName))
	},
		Entry("rhel9", RHEL, "9", RHEL, "rhel.9"),
		Entry("centos stream 9", CentOS, "9", CentOS, "centos.stream9"),
		Entry("windows 11", Windows, "11", Windows, "windows.11.virtio"),
		Entry("windows2022", Windows, "2022", Windows, "windows.2k22.virtio"),
		Entry("ubuntu 22", Ubuntu, "22.04.3", Ubuntu, "ubuntu"),
	)
})

var _ = Describe("OpenStack Glance const test", func() {
	It("GlanceSource should be glance, changing it may break the UI", func() {
		Expect(v1beta1.GlanceSource).Should(Equal("glance"))
	})
})

var builderLog = logging.WithName("openstack-builder-test")

var _ = Describe("OpenStack builder mapNetworks", func() {
	var (
		origStaticUdnIpAddresses bool
		origUdnSupportsMac       bool
	)

	BeforeEach(func() {
		origStaticUdnIpAddresses = settings.Settings.StaticUdnIpAddresses
		origUdnSupportsMac = settings.Settings.UdnSupportsMac
	})

	AfterEach(func() {
		settings.Settings.StaticUdnIpAddresses = origStaticUdnIpAddresses
		settings.Settings.UdnSupportsMac = origUdnSupportsMac
	})

	newBuilder := func(hasUDN bool, preserveStaticIPs bool) *Builder {
		scheme := runtime.NewScheme()
		_ = core.AddToScheme(scheme)
		_ = k8snet.AddToScheme(scheme)

		objs := []runtime.Object{}
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-ns",
			},
		}
		if hasUDN {
			ns.Labels = map[string]string{
				"k8s.ovn.org/primary-user-defined-network": "",
			}
			objs = append(objs, ns, &k8snet.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "udn-nad",
					Namespace: "test-ns",
					Labels:    map[string]string{"k8s.ovn.org/user-defined-network": ""},
				},
			})
		} else {
			objs = append(objs, ns)
		}

		cl := fake.NewClientBuilder().
			WithScheme(scheme).
			WithRuntimeObjects(objs...).
			Build()

		plan := &v1beta1.Plan{
			ObjectMeta: metav1.ObjectMeta{Name: "test-plan", Namespace: "test-ns"},
			Spec: v1beta1.PlanSpec{
				TargetNamespace:   "test-ns",
				PreserveStaticIPs: preserveStaticIPs,
			},
		}

		networkMap := &v1beta1.NetworkMap{
			Spec: v1beta1.NetworkMapSpec{
				Map: []v1beta1.NetworkPair{
					{
						Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: "net-001"}},
						Destination: v1beta1.DestinationNetwork{
							Type: Pod,
						},
					},
				},
			},
		}
		plan.Map.Network = networkMap

		ctx := &plancontext.Context{
			Plan: plan,
			Destination: plancontext.Destination{
				Client: cl,
			},
			Log: builderLog,
		}
		ctx.Map.Network = networkMap

		return &Builder{
			Context: ctx,
		}
	}

	vmWithFixedIP := func(networkName, networkID, ip, mac string) *model.Workload {
		return &model.Workload{
			XVM: model.XVM{
				VM: model.VM{
					VM1: model.VM1{
						Addresses: map[string]interface{}{
							networkName: []interface{}{
								map[string]interface{}{
									"addr":                    ip,
									"version":                 float64(4),
									"OS-EXT-IPS:type":         "fixed",
									"OS-EXT-IPS-MAC:mac_addr": mac,
								},
							},
						},
					},
				},
				Networks: []model.Network{
					{Resource: model.Resource{ID: networkID, Name: networkName}},
				},
			},
		}
	}

	It("should set static IP annotation when UDN + PreserveStaticIPs + feature flag enabled", func() {
		settings.Settings.StaticUdnIpAddresses = true
		settings.Settings.UdnSupportsMac = false
		builder := newBuilder(true, true)

		vm := vmWithFixedIP("my-network", "net-001", "10.220.0.5", "fa:16:3e:aa:bb:cc")
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Template.ObjectMeta.Annotations).To(HaveKey(planbase.AnnStaticUdnIp))
		var parsed map[string][]string
		Expect(json.Unmarshal([]byte(spec.Template.ObjectMeta.Annotations[planbase.AnnStaticUdnIp]), &parsed)).To(Succeed())
		Expect(parsed).To(HaveLen(1))
		Expect(parsed).To(HaveKeyWithValue("net-0", []string{"10.220.0.5"}))
	})

	It("should NOT set annotation when feature flag is off", func() {
		settings.Settings.StaticUdnIpAddresses = false
		builder := newBuilder(true, true)

		vm := vmWithFixedIP("my-network", "net-001", "10.220.0.5", "fa:16:3e:aa:bb:cc")
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())

		if spec.Template.ObjectMeta.Annotations != nil {
			Expect(spec.Template.ObjectMeta.Annotations).NotTo(HaveKey(planbase.AnnStaticUdnIp))
		}
	})

	It("should NOT set annotation when PreserveStaticIPs is false", func() {
		settings.Settings.StaticUdnIpAddresses = true
		builder := newBuilder(true, false)

		vm := vmWithFixedIP("my-network", "net-001", "10.220.0.5", "fa:16:3e:aa:bb:cc")
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())

		if spec.Template.ObjectMeta.Annotations != nil {
			Expect(spec.Template.ObjectMeta.Annotations).NotTo(HaveKey(planbase.AnnStaticUdnIp))
		}
	})

	It("should NOT set annotation when namespace has no UDN", func() {
		settings.Settings.StaticUdnIpAddresses = true
		builder := newBuilder(false, true)

		vm := vmWithFixedIP("my-network", "net-001", "10.220.0.5", "fa:16:3e:aa:bb:cc")
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())

		if spec.Template.ObjectMeta.Annotations != nil {
			Expect(spec.Template.ObjectMeta.Annotations).NotTo(HaveKey(planbase.AnnStaticUdnIp))
		}
	})

	It("should skip floating IPs", func() {
		settings.Settings.StaticUdnIpAddresses = true
		settings.Settings.UdnSupportsMac = false
		builder := newBuilder(true, true)

		vm := &model.Workload{
			XVM: model.XVM{
				VM: model.VM{
					VM1: model.VM1{
						Addresses: map[string]interface{}{
							"my-network": []interface{}{
								map[string]interface{}{
									"addr":                    "192.168.1.100",
									"version":                 float64(4),
									"OS-EXT-IPS:type":         "floating",
									"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
								},
							},
						},
					},
				},
				Networks: []model.Network{
					{Resource: model.Resource{ID: "net-001", Name: "my-network"}},
				},
			},
		}
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())
		// floating IPs cause a `continue`, so no networks/interfaces are created
		Expect(spec.Template.Spec.Networks).To(BeEmpty())
	})

	It("should exclude IPv6 addresses from static IP annotation", func() {
		settings.Settings.StaticUdnIpAddresses = true
		settings.Settings.UdnSupportsMac = false
		builder := newBuilder(true, true)

		vm := &model.Workload{
			XVM: model.XVM{
				VM: model.VM{
					VM1: model.VM1{
						Addresses: map[string]interface{}{
							"my-network": []interface{}{
								map[string]interface{}{
									"addr":                    "fe80::1",
									"version":                 float64(6),
									"OS-EXT-IPS:type":         "fixed",
									"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
								},
							},
						},
					},
				},
				Networks: []model.Network{
					{Resource: model.Resource{ID: "net-001", Name: "my-network"}},
				},
			},
		}
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())

		// UDN interface should be created but no static IP annotation since IPv6
		Expect(spec.Template.Spec.Networks).To(HaveLen(1))
		if spec.Template.ObjectMeta.Annotations != nil {
			Expect(spec.Template.ObjectMeta.Annotations).NotTo(HaveKey(planbase.AnnStaticUdnIp))
		}
	})

	It("should use l2bridge binding when UDN is present", func() {
		settings.Settings.StaticUdnIpAddresses = false
		settings.Settings.UdnSupportsMac = false
		builder := newBuilder(true, false)

		vm := vmWithFixedIP("my-network", "net-001", "10.220.0.5", "fa:16:3e:aa:bb:cc")
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Template.Spec.Domain.Devices.Interfaces).To(HaveLen(1))
		iface := spec.Template.Spec.Domain.Devices.Interfaces[0]
		Expect(iface.Binding).NotTo(BeNil())
		Expect(iface.Binding.Name).To(Equal(planbase.UdnL2bridge))
		Expect(iface.Masquerade).To(BeNil())
	})

	It("should use masquerade when no UDN", func() {
		settings.Settings.StaticUdnIpAddresses = false
		builder := newBuilder(false, false)

		vm := vmWithFixedIP("my-network", "net-001", "10.220.0.5", "fa:16:3e:aa:bb:cc")
		spec := &cnv.VirtualMachineSpec{
			Template: &cnv.VirtualMachineInstanceTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
			},
		}

		err := builder.mapNetworks(vm, spec)
		Expect(err).NotTo(HaveOccurred())

		Expect(spec.Template.Spec.Domain.Devices.Interfaces).To(HaveLen(1))
		iface := spec.Template.Spec.Domain.Devices.Interfaces[0]
		Expect(iface.Masquerade).NotTo(BeNil())
		Expect(iface.Binding).To(BeNil())
	})
})
