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
		It("should created new secret with the provider secret and the storage secret data", func() {
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
				&core.PersistentVolumeClaim{
					ObjectMeta: meta.ObjectMeta{Name: "test-pvc", Namespace: "test"},
				},
			)

			// Execute
			pvc := &core.PersistentVolumeClaim{
				ObjectMeta: meta.ObjectMeta{Name: "test-pvc", Namespace: "test"},
			}
			err := builder.mergeSecrets("migration-test-secret", "test", "storage-test-secret", "test", "merged-test-secret", pvc)
			underTest := core.Secret{}
			errGet := builder.Destination.Get(context.Background(), client.ObjectKey{
				Name:      "merged-test-secret",
				Namespace: "test"}, &underTest)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(errGet).NotTo(HaveOccurred())
			Expect(underTest.Data).To(HaveLen(4))
			Expect(underTest.Data).To(HaveKeyWithValue("storagekey", []byte("storageval")))
			Expect(underTest.Data).To(HaveKeyWithValue("providerkey", []byte("providerval")))
			Expect(underTest.Data).To(HaveKey("SSH_PRIVATE_KEY"))
			Expect(underTest.Data).To(HaveKey("SSH_PUBLIC_KEY"))
			Expect(underTest.OwnerReferences).To(HaveLen(1))
			Expect(underTest.OwnerReferences[0].Name).To(Equal("test-pvc"))
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

	builder := createBuilder()
	DescribeTable("should", func(vm *model.VM, outputMap string) {
		Expect(builder.mapMacStaticIps(vm)).Should(Equal(outputMap))
	},
		Entry("no static ips", &model.VM{GuestID: "windows9Guest"}, ""),
		Entry("single static ip", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
					DNS:          []string{"8.8.8.8"},
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16,8.8.8.8"),
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
					IP:           "fe80::5da:b7a5:e0a2:a097",
					Origin:       ManualOrigin,
					PrefixLength: 64,
					DNS:          []string{"fec0:0:0:ffff::1", "fec0:0:0:ffff::2", "fec0:0:0:ffff::3"},
				},
			},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				},
				{
					Gateway: "fe80::5da:b7a5:e0a2:a095",
					Network: "0.0.0.0",
				},
			},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16,8.8.8.8_00:50:56:83:25:47:ip:fe80::5da:b7a5:e0a2:a097,fe80::5da:b7a5:e0a2:a095,64,fec0:0:0:ffff::1,fec0:0:0:ffff::2,fec0:0:0:ffff::3"),
		Entry("non-static ip", &model.VM{GuestID: "windows9Guest", GuestNetworks: []vsphere.GuestNetwork{{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: string(types.NetIpConfigInfoIpAddressOriginDhcp)}}}, ""),
		Entry("non windows vm", &model.VM{GuestID: "other", GuestNetworks: []vsphere.GuestNetwork{{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin}}}, "00:50:56:83:25:47:ip:172.29.3.193,,0"),
		Entry("no OS vm", &model.VM{GuestNetworks: []vsphere.GuestNetwork{{MAC: "00:50:56:83:25:47", IP: "172.29.3.193", Origin: ManualOrigin}}}, "00:50:56:83:25:47:ip:172.29.3.193,,0"),
		Entry("multiple nics static ips", &model.VM{
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
					IP:           "fe80::5da:b7a5:e0a2:a097",
					Origin:       ManualOrigin,
					PrefixLength: 64,
					DNS:          []string{"fec0:0:0:ffff::1", "fec0:0:0:ffff::2", "fec0:0:0:ffff::3"},
				},
				{
					MAC:          "00:50:56:83:25:48",
					IP:           "172.29.3.192",
					Origin:       ManualOrigin,
					PrefixLength: 24,
					DNS:          []string{"4.4.4.4"},
				},
				{
					MAC:          "00:50:56:83:25:48",
					IP:           "fe80::5da:b7a5:e0a2:a090",
					Origin:       ManualOrigin,
					PrefixLength: 32,
					DNS:          []string{"fec0:0:0:ffff::4", "fec0:0:0:ffff::5", "fec0:0:0:ffff::6"},
				},
			},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.2",
					Network: "0.0.0.0",
				},
				{
					Gateway: "fe80::5da:b7a5:e0a2:a098",
					Network: "0.0.0.0",
				},
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				},
				{
					Gateway: "fe80::5da:b7a5:e0a2:a095",
					Network: "0.0.0.0",
				},
			},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16,8.8.8.8_00:50:56:83:25:47:ip:fe80::5da:b7a5:e0a2:a097,fe80::5da:b7a5:e0a2:a095,64,fec0:0:0:ffff::1,fec0:0:0:ffff::2,fec0:0:0:ffff::3_00:50:56:83:25:48:ip:172.29.3.192,172.29.3.1,24,4.4.4.4_00:50:56:83:25:48:ip:fe80::5da:b7a5:e0a2:a090,fe80::5da:b7a5:e0a2:a095,32,fec0:0:0:ffff::4,fec0:0:0:ffff::5,fec0:0:0:ffff::6"),
		Entry("single static ip without DNS", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 16,
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,16"),
		Entry("gateway from different subnet", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 24,
					DNS:          []string{"8.8.8.8"},
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "172.29.4.1",
					Network: "0.0.0.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.4.1,24,8.8.8.8"),
		Entry("multiple gateways with different networks", &model.VM{
			GuestID: "windows9Guest",
			GuestNetworks: []vsphere.GuestNetwork{
				{
					MAC:          "00:50:56:83:25:47",
					IP:           "172.29.3.193",
					Origin:       ManualOrigin,
					PrefixLength: 24,
					DNS:          []string{"8.8.8.8"},
				}},
			GuestIpStacks: []vsphere.GuestIpStack{
				{
					Gateway: "10.10.10.2",
					Network: "10.10.10.1",
				},
				{
					Gateway: "172.29.3.1",
					Network: "0.0.0.0",
				},
				{
					Gateway: "10.10.10.1",
					Network: "10.10.10.0",
				}},
		}, "00:50:56:83:25:47:ip:172.29.3.193,172.29.3.1,24,8.8.8.8"),
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
		Entry("sort NVMe disks with other buses",
			[]vsphere.Disk{
				{Key: 1, Bus: container.NVME},
				{Key: 1, Bus: container.SCSI},
				{Key: 2, Bus: container.NVME},
				{Key: 1, Bus: container.SATA},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: container.SCSI},
				{Key: 1, Bus: container.SATA},
				{Key: 1, Bus: container.NVME},
				{Key: 2, Bus: container.NVME},
			},
		),
		Entry("sort multiple NVMe disks by key",
			[]vsphere.Disk{
				{Key: 3, Bus: container.NVME},
				{Key: 1, Bus: container.NVME},
				{Key: 2, Bus: container.NVME},
			},
			[]vsphere.Disk{
				{Key: 1, Bus: container.NVME},
				{Key: 2, Bus: container.NVME},
				{Key: 3, Bus: container.NVME},
			},
		),
		Entry("sort all disk types including NVMe",
			[]vsphere.Disk{
				{Key: 2, Bus: container.NVME},
				{Key: 2, Bus: container.IDE},
				{Key: 3, Bus: container.SATA},
				{Key: 3, Bus: container.SCSI},
				{Key: 1, Bus: container.NVME},
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
				{Key: 1, Bus: container.NVME},
				{Key: 2, Bus: container.NVME},
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
	ctx := &plancontext.Context{
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
	}
	// Initialize the Labeler (normally done in Context.build())
	ctx.Labeler = plancontext.Labeler{Context: ctx}
	return &Builder{
		Context: ctx,
	}
}
