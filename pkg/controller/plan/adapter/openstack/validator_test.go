package openstack

import (
	"encoding/json"
	"errors"
	"fmt"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ocpmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var validatorLog = logging.WithName("openstack-validator-test")

var ErrNotImplemented = errors.New("not implemented")

type mockOpenstackInventory struct {
	workloads map[string]*model.Workload
	networks  map[string]*model.Network
}

func (m *mockOpenstackInventory) Find(resource interface{}, ref ref.Ref) error {
	switch res := resource.(type) {
	case *model.Workload:
		if wl, ok := m.workloads[ref.ID]; ok {
			*res = *wl
			return nil
		}
		if wl, ok := m.workloads[ref.Name]; ok {
			*res = *wl
			return nil
		}
		return base.NotFoundError{}
	case *model.Network:
		if net, ok := m.networks[ref.ID]; ok {
			*res = *net
			return nil
		}
		return base.NotFoundError{}
	}
	return fmt.Errorf("unsupported resource type")
}

func (m *mockOpenstackInventory) Finder() web.Finder            { return nil }
func (m *mockOpenstackInventory) Get(interface{}, string) error { return ErrNotImplemented }
func (m *mockOpenstackInventory) Host(*ref.Ref) (interface{}, error) {
	return nil, ErrNotImplemented
}
func (m *mockOpenstackInventory) List(interface{}, ...web.Param) error { return ErrNotImplemented }
func (m *mockOpenstackInventory) Network(*ref.Ref) (interface{}, error) {
	return nil, ErrNotImplemented
}
func (m *mockOpenstackInventory) Storage(*ref.Ref) (interface{}, error) {
	return nil, ErrNotImplemented
}
func (m *mockOpenstackInventory) VM(*ref.Ref) (interface{}, error) { return nil, ErrNotImplemented }
func (m *mockOpenstackInventory) Watch(interface{}, web.EventHandler) (*web.Watch, error) {
	return nil, ErrNotImplemented
}
func (m *mockOpenstackInventory) Workload(*ref.Ref) (interface{}, error) {
	return nil, ErrNotImplemented
}

var _ = Describe("OpenStack validator", func() {
	Describe("getFixedIPv4ForNetwork", func() {
		DescribeTable("should extract fixed IPv4 correctly",
			func(addresses map[string]interface{}, networkName string, expectedIP string) {
				vm := &model.Workload{
					XVM: model.XVM{
						VM: model.VM{
							VM1: model.VM1{
								Addresses: addresses,
							},
						},
					},
				}
				result := getFixedIPv4ForNetwork(vm, networkName)
				Expect(result).To(Equal(expectedIP))
			},
			Entry("fixed IPv4 found",
				map[string]interface{}{
					"production": []interface{}{
						map[string]interface{}{
							"addr":                    "10.220.0.5",
							"version":                 float64(4),
							"OS-EXT-IPS:type":         "fixed",
							"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
						},
					},
				},
				"production",
				"10.220.0.5",
			),
			Entry("floating IP excluded",
				map[string]interface{}{
					"production": []interface{}{
						map[string]interface{}{
							"addr":                    "203.0.113.5",
							"version":                 float64(4),
							"OS-EXT-IPS:type":         "floating",
							"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
						},
					},
				},
				"production",
				"",
			),
			Entry("IPv6 excluded",
				map[string]interface{}{
					"production": []interface{}{
						map[string]interface{}{
							"addr":                    "fe80::1",
							"version":                 float64(6),
							"OS-EXT-IPS:type":         "fixed",
							"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
						},
					},
				},
				"production",
				"",
			),
			Entry("network not present",
				map[string]interface{}{
					"other-net": []interface{}{
						map[string]interface{}{
							"addr":                    "10.0.0.1",
							"version":                 float64(4),
							"OS-EXT-IPS:type":         "fixed",
							"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
						},
					},
				},
				"production",
				"",
			),
			Entry("multiple NICs returns first fixed IPv4",
				map[string]interface{}{
					"production": []interface{}{
						map[string]interface{}{
							"addr":                    "fe80::1",
							"version":                 float64(6),
							"OS-EXT-IPS:type":         "fixed",
							"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:11:22:33",
						},
						map[string]interface{}{
							"addr":                    "10.220.0.99",
							"version":                 float64(4),
							"OS-EXT-IPS:type":         "fixed",
							"OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:44:55:66",
						},
					},
				},
				"production",
				"10.220.0.99",
			),
			Entry("empty addresses map",
				map[string]interface{}{},
				"production",
				"",
			),
		)
	})

	Describe("UdnStaticIPs", func() {
		udnNADConfig := func(subnet string) string {
			cfg := ocpmodel.NetworkConfig{
				AllowPersistentIPs: true,
				Type:               ocpmodel.OvnOverlayType,
				Role:               ocpmodel.RolePrimary,
				Subnets:            subnet,
			}
			b, _ := json.Marshal(cfg)
			return string(b)
		}

		newValidatorWithClient := func(hasUDN bool, subnet string, vm *model.Workload, sourceNetworkID string) (*Validator, error) {
			scheme := runtime.NewScheme()
			if err := core.AddToScheme(scheme); err != nil {
				return nil, err
			}
			if err := k8snet.AddToScheme(scheme); err != nil {
				return nil, err
			}

			objs := []runtime.Object{}
			ns := &core.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "target-ns",
				},
			}
			if hasUDN {
				ns.Labels = map[string]string{
					"k8s.ovn.org/primary-user-defined-network": "",
				}
				objs = append(objs, ns)
				objs = append(objs, &k8snet.NetworkAttachmentDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "udn-nad",
						Namespace: "target-ns",
						Labels:    map[string]string{"k8s.ovn.org/user-defined-network": ""},
					},
					Spec: k8snet.NetworkAttachmentDefinitionSpec{
						Config: udnNADConfig(subnet),
					},
				})
			} else {
				objs = append(objs, ns)
			}

			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(objs...).
				Build()

			inventory := &mockOpenstackInventory{
				workloads: map[string]*model.Workload{},
				networks:  map[string]*model.Network{},
			}
			if vm != nil {
				inventory.workloads[vm.ID] = vm
			}
			if sourceNetworkID != "" {
				inventory.networks[sourceNetworkID] = &model.Network{
					Resource: model.Resource{ID: sourceNetworkID, Name: "source-net"},
				}
			}

			plan := &v1beta1.Plan{
				ObjectMeta: metav1.ObjectMeta{Name: "test-plan", Namespace: "target-ns"},
				Spec: v1beta1.PlanSpec{
					TargetNamespace:   "target-ns",
					PreserveStaticIPs: true,
				},
			}
			plan.Map.Network = &v1beta1.NetworkMap{
				Spec: v1beta1.NetworkMapSpec{
					Map: []v1beta1.NetworkPair{
						{
							Source: v1beta1.NetworkSourceRef{Ref: ref.Ref{ID: sourceNetworkID}},
							Destination: v1beta1.DestinationNetwork{
								Type: Pod,
							},
						},
					},
				},
			}

			return &Validator{
				Context: &plancontext.Context{
					Plan: plan,
					Destination: plancontext.Destination{
						Client: cl,
					},
					Source: plancontext.Source{
						Inventory: inventory,
					},
					Log: validatorLog,
				},
			}, nil
		}

		It("should return true when no UDN in destination", func() {
			vm := &model.Workload{
				XVM: model.XVM{
					VM: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1"},
							Addresses: map[string]interface{}{
								"source-net": []interface{}{
									map[string]interface{}{
										"addr": "10.220.0.5", "version": float64(4),
										"OS-EXT-IPS:type": "fixed", "OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
									},
								},
							},
						},
					},
				},
			}
			v, err := newValidatorWithClient(false, "", vm, "net-001")
			Expect(err).NotTo(HaveOccurred())

			ok, err := v.UdnStaticIPs(ref.Ref{ID: "vm-1"}, v.Destination.Client)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should return true when PreserveStaticIPs is false", func() {
			vm := &model.Workload{
				XVM: model.XVM{
					VM: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1"},
							Addresses: map[string]interface{}{
								"source-net": []interface{}{
									map[string]interface{}{
										"addr": "10.220.0.5", "version": float64(4),
										"OS-EXT-IPS:type": "fixed", "OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
									},
								},
							},
						},
					},
				},
			}
			v, err := newValidatorWithClient(true, "10.220.0.0/24", vm, "net-001")
			Expect(err).NotTo(HaveOccurred())
			v.Plan.Spec.PreserveStaticIPs = false

			ok, err := v.UdnStaticIPs(ref.Ref{ID: "vm-1"}, v.Destination.Client)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should return true when IP is within UDN subnet", func() {
			vm := &model.Workload{
				XVM: model.XVM{
					VM: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1"},
							Addresses: map[string]interface{}{
								"source-net": []interface{}{
									map[string]interface{}{
										"addr": "10.220.0.5", "version": float64(4),
										"OS-EXT-IPS:type": "fixed", "OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
									},
								},
							},
						},
					},
				},
			}
			v, err := newValidatorWithClient(true, "10.220.0.0/24", vm, "net-001")
			Expect(err).NotTo(HaveOccurred())

			ok, err := v.UdnStaticIPs(ref.Ref{ID: "vm-1"}, v.Destination.Client)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should return false when IP is outside UDN subnet", func() {
			vm := &model.Workload{
				XVM: model.XVM{
					VM: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1"},
							Addresses: map[string]interface{}{
								"source-net": []interface{}{
									map[string]interface{}{
										"addr": "192.168.1.100", "version": float64(4),
										"OS-EXT-IPS:type": "fixed", "OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
									},
								},
							},
						},
					},
				},
			}
			v, err := newValidatorWithClient(true, "10.220.0.0/24", vm, "net-001")
			Expect(err).NotTo(HaveOccurred())

			ok, err := v.UdnStaticIPs(ref.Ref{ID: "vm-1"}, v.Destination.Client)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("should return true when VM has no fixed IPv4 on source network (nothing to preserve)", func() {
			vm := &model.Workload{
				XVM: model.XVM{
					VM: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1"},
							Addresses: map[string]interface{}{
								"source-net": []interface{}{
									map[string]interface{}{
										"addr": "fe80::1", "version": float64(6),
										"OS-EXT-IPS:type": "fixed", "OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
									},
								},
							},
						},
					},
				},
			}
			v, err := newValidatorWithClient(true, "10.220.0.0/24", vm, "net-001")
			Expect(err).NotTo(HaveOccurred())

			ok, err := v.UdnStaticIPs(ref.Ref{ID: "vm-1"}, v.Destination.Client)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should return true when no network is mapped to Pod type", func() {
			vm := &model.Workload{
				XVM: model.XVM{
					VM: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1"},
							Addresses: map[string]interface{}{
								"source-net": []interface{}{
									map[string]interface{}{
										"addr": "10.220.0.5", "version": float64(4),
										"OS-EXT-IPS:type": "fixed", "OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
									},
								},
							},
						},
					},
				},
			}
			v, err := newValidatorWithClient(true, "10.220.0.0/24", vm, "net-001")
			Expect(err).NotTo(HaveOccurred())
			// Change mapping to Multus (not Pod) so no pod-target source network exists
			v.Plan.Map.Network.Spec.Map[0].Destination.Type = Multus

			ok, err := v.UdnStaticIPs(ref.Ref{ID: "vm-1"}, v.Destination.Client)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("should return true when UDN NAD has no subnet configured", func() {
			vm := &model.Workload{
				XVM: model.XVM{
					VM: model.VM{
						VM1: model.VM1{
							VM0: model.VM0{ID: "vm-1"},
							Addresses: map[string]interface{}{
								"source-net": []interface{}{
									map[string]interface{}{
										"addr": "10.220.0.5", "version": float64(4),
										"OS-EXT-IPS:type": "fixed", "OS-EXT-IPS-MAC:mac_addr": "fa:16:3e:aa:bb:cc",
									},
								},
							},
						},
					},
				},
			}
			// Pass empty subnet - NAD exists but no matching UDN config
			v, err := newValidatorWithClient(true, "", vm, "net-001")
			Expect(err).NotTo(HaveOccurred())

			ok, err := v.UdnStaticIPs(ref.Ref{ID: "vm-1"}, v.Destination.Client)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})
	})
})
