package vsphere

import (
	"context"
	"fmt"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	container "github.com/kubev2v/forklift/pkg/controller/provider/container/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vim25/types"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var builderLog = logging.WithName("vsphere-builder-test")

const ManualOrigin = string(types.NetIpConfigInfoIpAddressOriginManual)

var _ = Describe("vSphere builder", func() {
	Context("PopulatorVolumes", func() {
		It("should merge the provider secret with the storage secret", func() {
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "storage-test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"storagekey": []byte("storageval"),
					},
				},
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "migration-test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"providerkey": []byte("providerval"),
					},
				},
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "offload-ssh-keys-test-vsphere-provider-private", Namespace: "test"},
					Data: map[string][]byte{
						"private-key": []byte("fake-private-key"),
					},
				},
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "offload-ssh-keys-test-vsphere-provider-public", Namespace: "test"},
					Data: map[string][]byte{
						"public-key": []byte("fake-public-key"),
					},
				},
			)

			// Execute
			err := builder.mergeSecrets("migration-test-secret", "test", "storage-test-secret", "test")
			underTest := core.Secret{}
			errGet := builder.Destination.Get(context.Background(), client.ObjectKey{
				Name:      "migration-test-secret",
				Namespace: "test"}, &underTest)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(errGet).NotTo(HaveOccurred())
			Expect(underTest.Data).To(HaveLen(4))
			Expect(underTest.Data).To(HaveKeyWithValue("storagekey", []byte("storageval")))
			Expect(underTest.Data).To(HaveKeyWithValue("providerkey", []byte("providerval")))
			Expect(underTest.Data).To(HaveKey("SSH_PRIVATE_KEY"))
			Expect(underTest.Data).To(HaveKey("SSH_PUBLIC_KEY"))

		})
		It("should set default access mode to ReadWriteMany for block volumes", func() {
			// Setup
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
				},
			)
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{
						ID:   "test-vm-id",
						Name: "test",
					},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-1"},
							File:      "[datastore1] vm-123/vm-123.vmdk",
							Bus:       container.SCSI,
							Capacity:  1024 * 1024 * 1024, // 1 GiB
							Key:       2000,
						},
					},
				},
			}

			dsMap := []v1beta1.StoragePair{
				{
					Source: ref.Ref{ID: "ds-1"},
					Destination: v1beta1.DestinationStorage{
						StorageClass: "test-sc",
					},
					OffloadPlugin: &v1beta1.OffloadPlugin{
						VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
							StorageVendorProduct: "test-vendor",
							SecretRef:            "test-secret",
						},
					},
				},
			}
			storageMap := v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: dsMap,
				},
			}
			annotations := map[string]string{"test-annotation": "true"}
			secretName := "test-secret"

			// Mock inventory
			inventory := &mockInventory{
				ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
				vm: vm,
			}
			builder.Source.Inventory = inventory
			builder.Context.Map.Storage = &storageMap

			// Execute
			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, annotations, secretName)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			pvc := pvcs[0]
			Expect(pvc.Spec.AccessModes).To(ContainElement(core.ReadWriteMany))
			Expect(pvc.Spec.VolumeMode).To(Equal(ptr.To(core.PersistentVolumeBlock)))
		})
		It("should set default PVC template name", func() {
			// Setup
			builder := createBuilder(
				&core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
				},
			)
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{
						ID:   "test-vm-id",
						Name: "customer-frontend-server",
					},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-1"},
							File:      "[datastore1] vm-123/vm-123.vmdk",
							Bus:       container.SCSI,
							Capacity:  1024 * 1024 * 1024, // 1 GiB
							Key:       2000,
						},
					},
				},
			}

			dsMap := []v1beta1.StoragePair{
				{
					Source: ref.Ref{ID: "ds-1"},
					Destination: v1beta1.DestinationStorage{
						StorageClass: "test-sc",
					},
					OffloadPlugin: &v1beta1.OffloadPlugin{
						VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
							StorageVendorProduct: "test-vendor",
							SecretRef:            "test-secret",
						},
					},
				},
			}
			storageMap := v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: dsMap,
				},
			}
			annotations := map[string]string{"test-annotation": "true"}
			secretName := "test-secret"

			// Mock inventory
			inventory := &mockInventory{
				ds: model.Datastore{Resource: model.Resource{ID: "ds-1"}},
				vm: vm,
			}
			builder.Source.Inventory = inventory
			builder.Context.Map.Storage = &storageMap

			// Execute
			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, annotations, secretName)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			pvc := pvcs[0]
			// The default template now uses trunc 4 for both plan and VM names
			Expect(pvc.Name).Should(HavePrefix(fmt.Sprintf("%.4s-%.4s-disk-", builder.Plan.Name, vm.Name)))
			Expect(pvc.Spec.DataSourceRef.Kind).To(Equal(v1beta1.VSphereXcopyVolumePopulatorKind))
			Expect(pvc.Spec.DataSourceRef.APIGroup).To(Equal(&v1beta1.SchemeGroupVersion.Group))
			Expect(pvc.Spec.DataSourceRef.Name).To(Equal(pvc.Name))
		})

		It("should honor explicit AccessMode StorageMap and ignore VolumeMode from StorageMap", func() {
			builder := createBuilder(&core.Secret{ObjectMeta: meta.ObjectMeta{Name: "test-secret", Namespace: "test"}})
			vm := model.VM{
				VM1: model.VM1{
					VM0: model.VM0{ID: "vm-2", Name: "vm"},
					Disks: []vsphere.Disk{
						{
							Datastore: vsphere.Ref{ID: "ds-2"},
							File:      "[datastore2] vm-2/vm-2.vmdk",
							Bus:       container.SCSI, Capacity: 1 << 20, Key: 2000,
						},
					},
				},
			}
			builder.Source.Inventory = &mockInventory{ds: model.Datastore{Resource: model.Resource{ID: "ds-2"}}, vm: vm}
			builder.Context.Map.Storage = &v1beta1.StorageMap{
				Spec: v1beta1.StorageMapSpec{
					Map: []v1beta1.StoragePair{{
						Source: ref.Ref{ID: "ds-2"},
						Destination: v1beta1.DestinationStorage{
							StorageClass: "test-sc",
							AccessMode:   core.ReadWriteOnce,
							VolumeMode:   core.PersistentVolumeFilesystem,
						},
						OffloadPlugin: &v1beta1.OffloadPlugin{
							VSphereXcopyPluginConfig: &v1beta1.VSphereXcopyPluginConfig{
								StorageVendorProduct: "test-vendor",
								SecretRef:            "test-secret",
							},
						},
					}},
				},
			}
			pvcs, err := builder.PopulatorVolumes(ref.Ref{ID: vm.ID}, nil, "test-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(pvcs).To(HaveLen(1))
			Expect(pvcs[0].Spec.AccessModes).To(ConsistOf(core.ReadWriteOnce))
			Expect(pvcs[0].Spec.VolumeMode).To(Equal(ptr.To(core.PersistentVolumeBlock)))
		})
	})

	Context("getHostAddress", func() {
		// Use hostname-based provider for backward compatibility tests
		hostnameProvider := &v1beta1.Provider{
			Spec: v1beta1.ProviderSpec{
				URL: "https://vcenter.example.com/sdk",
			},
		}

		DescribeTable("should return correct host address", func(host *model.Host, expected string) {
			result := getHostAddress(host, hostnameProvider)
			Expect(result).To(Equal(expected))
		},
			Entry("IPv4 only from Management Network",
				&model.Host{
					Resource: model.Resource{Name: "esxi-host.example.com"},
					Network: vsphere.HostNetwork{
						VNICs: []vsphere.VNIC{
							{
								PortGroup: "Management Network",
								IpAddress: "192.168.1.100",
							},
						},
					},
				},
				"192.168.1.100",
			),
			Entry("IPv6 only from Management Network (with brackets)",
				&model.Host{
					Resource: model.Resource{Name: "esxi-host.example.com"},
					Network: vsphere.HostNetwork{
						VNICs: []vsphere.VNIC{
							{
								PortGroup:   "Management Network",
								IpV6Address: []string{"2001:db8::1"},
							},
						},
					},
				},
				"[2001:db8::1]",
			),
			Entry("Both IPv4 and IPv6 (hostname provider returns IPv4 as default)",
				&model.Host{
					Resource: model.Resource{Name: "esxi-host.example.com"},
					Network: vsphere.HostNetwork{
						VNICs: []vsphere.VNIC{
							{
								PortGroup:   "Management Network",
								IpAddress:   "192.168.1.100",
								IpV6Address: []string{"2001:db8::1"},
							},
						},
					},
				},
				"192.168.1.100",
			),
			Entry("No Management Network VNIC (fallback to hostname)",
				&model.Host{
					Resource: model.Resource{Name: "esxi-host.example.com"},
					Network: vsphere.HostNetwork{
						VNICs: []vsphere.VNIC{
							{
								PortGroup: "VM Network",
								IpAddress: "192.168.1.100",
							},
						},
					},
				},
				"esxi-host.example.com",
			),
			Entry("Mixed valid and invalid IPs (uses first valid)",
				&model.Host{
					Resource: model.Resource{Name: "esxi-host.example.com"},
					Network: vsphere.HostNetwork{
						VNICs: []vsphere.VNIC{
							{
								PortGroup:   "Management Network",
								IpAddress:   "invalid",
								IpV6Address: []string{"invalid-ipv6", "2001:db8::1"},
							},
						},
					},
				},
				"[2001:db8::1]",
			),
			Entry("ESXi host with IPv4 and global IPv6 (filters link-local, returns IPv4 as default)",
				&model.Host{
					Resource: model.Resource{Name: "esxi-host.example.com"},
					Network: vsphere.HostNetwork{
						VNICs: []vsphere.VNIC{
							{
								PortGroup: "Management Network",
								IpAddress: "10.73.73.11",
								IpV6Address: []string{
									"fe80::3673:5aff:fe9a:dd78",          // link-local - should be filtered
									"2620:52:0:4948:3673:5aff:fe9a:dd78", // global IPv6 - available but not preferred
								},
							},
						},
					},
				},
				"10.73.73.11", // Returns IPv4 when provider preference unknown
			),
		)
	})

	Context("formatHostAddress", func() {
		DescribeTable("should format addresses correctly", func(address string, expected string) {
			result := formatHostAddress(address)
			Expect(result).To(Equal(expected))
		},
			Entry("IPv4 address (no brackets)",
				"192.168.1.100",
				"192.168.1.100",
			),
			Entry("IPv6 address (add brackets)",
				"2001:db8::1",
				"[2001:db8::1]",
			),
			Entry("Invalid/Hostname (no change)",
				"not-an-ip",
				"not-an-ip",
			),
		)
	})

	Context("selectGateway", func() {
		DescribeTable("should return correct gateway for IP configurations",
			func(ip string, device string, stacks []vsphere.GuestIpStack, isWindows bool, expected string) {
				result := selectGateway(ip, device, stacks, isWindows)
				Expect(result).To(Equal(expected))
			},
			Entry("Linux VM with ULA IPv6 (returns empty)",
				"fd00::1234",
				"1",
				[]vsphere.GuestIpStack{
					{Device: "1", Gateway: "fd00::1", Network: "::"},
				},
				false,
				"",
			),
			Entry("Windows VM with ULA IPv6 and only link-local gateway (returns empty)",
				"fd00::1234",
				"1",
				[]vsphere.GuestIpStack{
					{Device: "1", Gateway: "fe80::1", Network: "::"},
				},
				true,
				"",
			),
			Entry("Windows VM with global IPv6 and global gateway (prefers global)",
				"2620:52:9:162e:f89e:f3b2:9216:a7b",
				"0",
				[]vsphere.GuestIpStack{
					{Device: "0", Gateway: "fe80::1", Network: "::"},
					{Device: "0", Gateway: "2620:52:9:162e::1", Network: "::"},
				},
				true,
				"2620:52:9:162e::1",
			),
			Entry("Windows VM with global IPv6 and ONLY link-local gateway (uses link-local)",
				"2620:52:9:162e:9468:f85b:d7c5:c18",
				"1",
				[]vsphere.GuestIpStack{
					{Device: "1", Gateway: "fe80::4a5a:d01:f431:3320", Network: "::"},
				},
				true,
				"fe80::4a5a:d01:f431:3320",
			),
			Entry("Windows VM with link-local IPv6 address (returns empty - non-global)",
				"fe80::9468:f85b:d7c5:c18",
				"1",
				[]vsphere.GuestIpStack{
					{Device: "1", Gateway: "fe80::4a5a:d01:f431:3320", Network: "::"},
				},
				true,
				"",
			),
			Entry("Linux VM with global IPv6 and link-local gateway",
				"2620:52:9:162e:9468:f85b:d7c5:c18",
				"1",
				[]vsphere.GuestIpStack{
					{Device: "1", Gateway: "fe80::4a5a:d01:f431:3320", Network: "::"},
				},
				false,
				"fe80::4a5a:d01:f431:3320",
			),
		)
	})

	builder := createBuilder()
	DescribeTable("should", func(vm *model.VM, outputMap string) {
		Expect(builder.mapMacStaticIps(vm)).Should(Equal(outputMap))
	},
		Entry("Linux VM with ULA IPv6 (included with empty gateway)",
			&model.VM{
				GuestID: "rhel8_64Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{
						MAC:          "00:50:56:aa:bb:cc",
						IP:           "fd00::100",
						Origin:       ManualOrigin,
						PrefixLength: 64,
						DNS:          []string{"2001:4860:4860::8888"},
					},
				},
				GuestIpStacks: []vsphere.GuestIpStack{
					{Device: "0", Gateway: "fd00::1", Network: "::"},
				},
			},
			"00:50:56:aa:bb:cc:ip:fd00::100,,64,2001:4860:4860::8888",
		),
		Entry("Windows VM with ULA IPv6 only (should skip)",
			&model.VM{
				GuestID: "windows9Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{
						Device:       "1",
						MAC:          "00:50:56:aa:bb:dd",
						IP:           "fd00::101",
						Origin:       ManualOrigin,
						PrefixLength: 64,
						DNS:          []string{"2001:4860:4860::8888"},
					},
				},
				GuestIpStacks: []vsphere.GuestIpStack{
					{Device: "1", Gateway: "fe80::1", Network: "::"},
				},
			},
			"",
		),
		Entry("multiple static ips", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
					DNS:          []string{"8.8.8.8"},
				},
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "2620:52:9:162e::97",
					Origin:       ManualOrigin,
					PrefixLength: 64,
					DNS:          []string{"2620:52:9:162e::1", "2620:52:9:162e::2", "2620:52:9:162e::3"},
				},
			},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				},
				{
					Gateway: "2620:52:9:162e::95",
					Network: "0.0.0.0",
				},
			},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16,8.8.8.8_00:50:56:83:25:47:ip:2620:52:9:162e::97,2620:52:9:162e::95,64,2620:52:9:162e::1,2620:52:9:162e::2,2620:52:9:162e::3"),
		Entry("multiple nics static ips", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					Device:       "0",
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
					DNS:          []string{"8.8.8.8"},
				},
				{
					Device:       "0",
					MAC:          "00:50:56:83:25:47",
					IP:           "2620:52:9:162e::97",
					Origin:       ManualOrigin,
					PrefixLength: 64,
					DNS:          []string{"2620:52:9:162e::1", "2620:52:9:162e::2", "2620:52:9:162e::3"},
				},
				{
					Device:       "1",
					MAC:          "00:50:56:83:25:48",
					IP:           "172.29.3.192",
					Origin:       ManualOrigin,
					PrefixLength: 24,
					DNS:          []string{"4.4.4.4"},
				},
				{
					Device:       "1",
					MAC:          "00:50:56:83:25:48",
					IP:           "2620:52:9:162e::90",
					Origin:       ManualOrigin,
					PrefixLength: 32,
					DNS:          []string{"2620:52:9:162e::4", "2620:52:9:162e::5", "2620:52:9:162e::6"},
				},
			},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.2",
					Network: "0.0.0.0",
					Device:  "0",
				},
				{
					Gateway: "2620:52:9:162e::98",
					Network: "::",
					Device:  "0",
				},
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
					Device:  "1",
				},
				{
					Gateway: "2620:52:9:162e::95",
					Network: "::",
					Device:  "1",
				},
			},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.2,16,8.8.8.8_00:50:56:83:25:48:ip:172.29.3.192,172.29.3.1,24,4.4.4.4_00:50:56:83:25:47:ip:2620:52:9:162e::97,2620:52:9:162e::98,64,2620:52:9:162e::1,2620:52:9:162e::2,2620:52:9:162e::3_00:50:56:83:25:48:ip:2620:52:9:162e::90,2620:52:9:162e::95,32,2620:52:9:162e::4,2620:52:9:162e::5,2620:52:9:162e::6"),
		// Linux VM behavior test
		Entry("Linux VM with both IPv4 and IPv6 (both included)",
			&model.VM{
				GuestID: "debian10_64Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					{
						Device:       "0",
						MAC:          "00:50:56:83:25:50",
						IP:           "192.168.1.50",
						Origin:       ManualOrigin,
						PrefixLength: 24,
						DNS:          []string{"8.8.8.8"},
					},
					{
						Device:       "0",
						MAC:          "00:50:56:83:25:50",
						IP:           "2001:db8::50",
						Origin:       ManualOrigin,
						PrefixLength: 64,
						DNS:          []string{"2001:4860:4860::8888"},
					},
				},
				GuestIpStacks: []vsphere.GuestIpStack{
					{
						Gateway: "192.168.1.1",
						Network: "0.0.0.0",
						Device:  "0",
					},
					{
						Gateway: "2001:db8::1",
						Network: "::",
						Device:  "0",
					},
				},
			},
			"00:50:56:83:25:50:ip:192.168.1.50,192.168.1.1,24,8.8.8.8_00:50:56:83:25:50:ip:2001:db8::50,2001:db8::1,64,2001:4860:4860::8888",
		),
		// Multi-NIC Windows VM with manual, global IPv6, link-local, and APIPA (self-assigned 169.254.x.x) addresses
		Entry("Windows VM with multiple NICs and mixed IP origins",
			&model.VM{
				GuestID: "windows2019srv_64Guest",
				GuestNetworks: []vsphere.GuestNetwork{
					// Device 0 - VM Network
					{
						Device:       "0",
						MAC:          "00:50:56:b4:59:dd",
						IP:           "2620:52:9:162e:f89e:f3b2:9216:a7b",
						Origin:       string(types.NetIpConfigInfoIpAddressOriginLinklayer), // global IPv6 (autoconfigured)
						PrefixLength: 64,
						DNS:          []string{"10.31.139.196", "10.31.139.228"},
					},
					{
						Device:       "0",
						MAC:          "00:50:56:b4:59:dd",
						IP:           "2620:52:9:162e::1001",
						Origin:       ManualOrigin,
						PrefixLength: 64,
						DNS:          []string{"10.31.139.196", "10.31.139.228"},
					},
					{
						Device:       "0",
						MAC:          "00:50:56:b4:59:dd",
						IP:           "fe80::f89e:f3b2:9216:a7b", // link-local - should be skipped
						Origin:       string(types.NetIpConfigInfoIpAddressOriginLinklayer),
						PrefixLength: 64,
						DNS:          []string{"10.31.139.196", "10.31.139.228"},
					},
					{
						Device:       "0",
						MAC:          "00:50:56:b4:59:dd",
						IP:           "10.31.137.18",
						Origin:       ManualOrigin,
						PrefixLength: 24,
						DNS:          []string{"10.31.139.196", "10.31.139.228"},
					},
					{
						Device:       "0",
						MAC:          "00:50:56:b4:59:dd",
						IP:           "169.254.10.123", // APIPA (self-assigned 169.254.x.x) - should be skipped
						Origin:       string(types.NetIpConfigInfoIpAddressOriginLinklayer),
						PrefixLength: 16,
						DNS:          []string{"10.31.139.196", "10.31.139.228"},
					},
					// Device 1 - Mgmt Network
					{
						Device:       "1",
						MAC:          "00:50:56:b4:ae:52",
						IP:           "2620:52:9:162e:9468:f85b:d7c5:c18", // global IPv6 - should be included
						Origin:       string(types.NetIpConfigInfoIpAddressOriginLinklayer),
						PrefixLength: 64,
						DNS:          []string{"130.172.45.35", "148.93.51.69"},
					},
					{
						Device:       "1",
						MAC:          "00:50:56:b4:ae:52",
						IP:           "fe80::9468:f85b:d7c5:c18", // link-local - should be skipped
						Origin:       string(types.NetIpConfigInfoIpAddressOriginLinklayer),
						PrefixLength: 64,
						DNS:          []string{"130.172.45.35", "148.93.51.69"},
					},
					{
						Device:       "1",
						MAC:          "00:50:56:b4:ae:52",
						IP:           "10.62.4.18",
						Origin:       ManualOrigin,
						PrefixLength: 21,
						DNS:          []string{"130.172.45.35", "148.93.51.69"},
					},
					{
						Device:       "1",
						MAC:          "00:50:56:b4:ae:52",
						IP:           "169.254.12.24", // APIPA (self-assigned 169.254.x.x) - should be skipped
						Origin:       string(types.NetIpConfigInfoIpAddressOriginLinklayer),
						PrefixLength: 16,
						DNS:          []string{"130.172.45.35", "148.93.51.69"},
					},
				},
				GuestIpStacks: []vsphere.GuestIpStack{
					{
						Gateway: "2620:52:9:162e::1",
						Network: "::",
						Device:  "0",
					},
					{
						Gateway: "10.31.137.1",
						Network: "0.0.0.0",
						Device:  "0",
					},
					{
						Gateway: "fe80::4a5a:d01:f431:3320", // Device 1 only has link-local IPv6 gateway
						Network: "::",
						Device:  "1",
					},
					{
						Gateway: "10.62.0.1",
						Network: "0.0.0.0",
						Device:  "1",
					},
				},
			},
			// Expected: IPv4 first, then IPv6 (including SLAAC - IPv6 autoconfiguration)
			// (but NOT link-local fe80:: or APIPA self-assigned 169.254.x.x)
			// Device 1's global IPv6 should be included with its link-local gateway (valid SLAAC config)
			"00:50:56:b4:59:dd:ip:10.31.137.18,10.31.137.1,24,10.31.139.196,10.31.139.228_00:50:56:b4:ae:52:ip:10.62.4.18,10.62.0.1,21,130.172.45.35,148.93.51.69_00:50:56:b4:59:dd:ip:2620:52:9:162e:f89e:f3b2:9216:a7b,2620:52:9:162e::1,64,10.31.139.196,10.31.139.228_00:50:56:b4:59:dd:ip:2620:52:9:162e::1001,2620:52:9:162e::1,64,10.31.139.196,10.31.139.228_00:50:56:b4:ae:52:ip:2620:52:9:162e:9468:f85b:d7c5:c18,fe80::4a5a:d01:f431:3320,64,130.172.45.35,148.93.51.69",
		),
	)

	DescribeTable("should", func(disks []vsphere.Disk, output []vsphere.Disk) {
		Expect(builder.sortedDisksAsLibvirt(disks)).Should(Equal(output))
	},
		Entry("sort all disks by buses",
			[]vsphere.Disk{
				{Key: 1, Bus: container.IDE},
				{Key: 1, Bus: container.SATA},
				{Key: 1, Bus: container.SCSI},
				{Key: 2, Bus: container.SCSI},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: container.SCSI},
				{Key: 2, Bus: container.SCSI},
				{Key: 1, Bus: container.SATA},
				{Key: 1, Bus: container.IDE},
			},
		),
		Entry("sort IDE and SATA disks by buses",
			[]vsphere.Disk{
				{Key: 1, Bus: container.IDE},
				{Key: 1, Bus: container.SATA},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: container.SATA},
				{Key: 1, Bus: container.IDE},
			},
		),
		Entry("sort multiple SATA disks by buses",
			[]vsphere.Disk{
				{Key: 3, Bus: container.SATA},
				{Key: 1, Bus: container.SATA},
				{Key: 2, Bus: container.SATA},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: container.SATA},
				{Key: 2, Bus: container.SATA},
				{Key: 3, Bus: container.SATA},
			},
		),
		Entry("sort multiple SATA and multiple SCSI disks by buses",
			[]vsphere.Disk{
				{Key: 3, Bus: container.SATA},
				{Key: 3, Bus: container.SCSI},
				{Key: 2, Bus: container.SCSI},
				{Key: 1, Bus: container.SATA},
				{Key: 2, Bus: container.SATA},
				{Key: 1, Bus: container.SCSI},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: container.SCSI},
				{Key: 2, Bus: container.SCSI},
				{Key: 3, Bus: container.SCSI},
				{Key: 1, Bus: container.SATA},
				{Key: 2, Bus: container.SATA},
				{Key: 3, Bus: container.SATA},
			},
		),
		Entry("sort multiple all disks by buses",
			[]vsphere.Disk{
				{Key: 2, Bus: container.IDE},
				{Key: 3, Bus: container.SATA},
				{Key: 3, Bus: container.SCSI},
				{Key: 2, Bus: container.SCSI},
				{Key: 3, Bus: container.IDE},
				{Key: 1, Bus: container.SATA},
				{Key: 2, Bus: container.SATA},
				{Key: 1, Bus: container.SCSI},
				{Key: 1, Bus: container.IDE},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: container.SCSI},
				{Key: 2, Bus: container.SCSI},
				{Key: 3, Bus: container.SCSI},
				{Key: 1, Bus: container.SATA},
				{Key: 2, Bus: container.SATA},
				{Key: 3, Bus: container.SATA},
				{Key: 1, Bus: container.IDE},
				{Key: 2, Bus: container.IDE},
				{Key: 3, Bus: container.IDE},
			},
		),
	)
})

//nolint:errcheck
func createBuilder(objs ...runtime.Object) *Builder {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
	_ = core.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	v1beta1.SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &Builder{
		Context: &plancontext.Context{
			Destination: plancontext.Destination{
				Client: client,
			},
			Source: plancontext.Source{
				Provider: &v1beta1.Provider{
					ObjectMeta: meta.ObjectMeta{Name: "test-vsphere-provider", Namespace: "test"},
					Spec: v1beta1.ProviderSpec{
						Type: (*v1beta1.ProviderType)(ptr.To("vsphere")),
						URL:  "test-vsphere-provider",
					},
				},
				Inventory: nil,
				Secret: &core.Secret{
					ObjectMeta: meta.ObjectMeta{Name: "test-provider-secret", Namespace: "test"},
				},
			},
			Plan:      createPlan(),
			Migration: &v1beta1.Migration{ObjectMeta: meta.ObjectMeta{UID: k8stypes.UID("123")}},
			Log:       builderLog,
			Client:    client,
		},
	}
}

var _ = Describe("getHostAddress", func() {
	var (
		host *model.Host
	)

	BeforeEach(func() {
		// Create a host with both IPv4 and IPv6 addresses
		host = &model.Host{
			Resource: model.Resource{Name: "test-host.example.com"},
			Network: vsphere.HostNetwork{
				VNICs: []vsphere.VNIC{
					{
						PortGroup: ManagementNetwork,
						IpAddress: "10.6.46.28",
						IpV6Address: []string{
							"fe80::1",              // link-local (should be filtered)
							"2620:52:9:162e::abcd", // global IPv6
						},
					},
				},
			},
		}
	})

	Context("when provider URL uses IPv4", func() {
		It("should return IPv4 address", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://10.6.46.248/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("10.6.46.28"))
		})
	})

	Context("when provider URL uses IPv6", func() {
		It("should return IPv6 address in brackets", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://[2620:52:9:162e::1]/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("[2620:52:9:162e::abcd]"))
		})
	})

	Context("when provider URL uses hostname", func() {
		It("should return available address (IPv4 preferred as default)", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://vcenter.example.com/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("10.6.46.28"))
		})
	})

	Context("when provider URL uses IPv4 but host only has IPv6", func() {
		BeforeEach(func() {
			host.Network.VNICs[0].IpAddress = "" // Remove IPv4
		})

		It("should fallback to IPv6", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://10.6.46.248/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("[2620:52:9:162e::abcd]"))
		})
	})

	Context("when provider URL uses IPv6 but host only has IPv4", func() {
		BeforeEach(func() {
			host.Network.VNICs[0].IpV6Address = []string{} // Remove IPv6
		})

		It("should fallback to IPv4", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://[2620:52:9:162e::1]/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("10.6.46.28"))
		})
	})

	Context("when host only has IPv4", func() {
		BeforeEach(func() {
			host.Network.VNICs[0].IpV6Address = []string{} // Remove IPv6
		})

		It("should return IPv4 for IPv4 provider", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://10.6.46.248/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("10.6.46.28"))
		})

		It("should return IPv4 for hostname provider", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://vcenter.example.com/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("10.6.46.28"))
		})
	})

	Context("when host only has IPv6", func() {
		BeforeEach(func() {
			host.Network.VNICs[0].IpAddress = "" // Remove IPv4
		})

		It("should return IPv6 for IPv6 provider", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://[2620:52:9:162e::1]/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("[2620:52:9:162e::abcd]"))
		})

		It("should return IPv6 for hostname provider", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://vcenter.example.com/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("[2620:52:9:162e::abcd]"))
		})
	})

	Context("when host has no IP addresses", func() {
		BeforeEach(func() {
			host.Network.VNICs[0].IpAddress = ""
			host.Network.VNICs[0].IpV6Address = []string{}
		})

		It("should return hostname", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://vcenter.example.com/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("test-host.example.com"))
		})
	})

	Context("when host has link-local IPv6 only", func() {
		BeforeEach(func() {
			host.Network.VNICs[0].IpAddress = ""
			host.Network.VNICs[0].IpV6Address = []string{"fe80::1"} // link-local only
		})

		It("should return hostname (link-local filtered out)", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://vcenter.example.com/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("test-host.example.com"))
		})
	})

	Context("when provider URL is malformed", func() {
		It("should return available address (IPv4 as default)", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "not-a-valid-url",
				},
			}
			result := getHostAddress(host, provider)
			// When preference is unknown, returns IPv4 (current default behavior)
			Expect(result).To(Equal("10.6.46.28"))
		})
	})

	Context("when host has multiple management network VNICs", func() {
		BeforeEach(func() {
			host.Network.VNICs = append(host.Network.VNICs, vsphere.VNIC{
				PortGroup: ManagementNetwork,
				IpAddress: "10.6.46.29", // Another IPv4
				IpV6Address: []string{
					"2620:52:9:162e::def0", // Another IPv6
				},
			})
		})

		It("should use the first valid address of matching family", func() {
			provider := &v1beta1.Provider{
				Spec: v1beta1.ProviderSpec{
					URL: "https://10.6.46.248/sdk",
				},
			}
			result := getHostAddress(host, provider)
			Expect(result).To(Equal("10.6.46.28")) // First IPv4
		})
	})
})
