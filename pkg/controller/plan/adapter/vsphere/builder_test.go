package vsphere

import (
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	container "github.com/kubev2v/forklift/pkg/controller/provider/container/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/vim25/types"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var builderLog = logging.WithName("vsphere-builder-test")

const ManualOrigin = string(types.NetIpConfigInfoIpAddressOriginManual)

var _ = Describe("vSphere builder", func() {
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
	)
})

//nolint:errcheck
func createBuilder(objs ...runtime.Object) *Builder {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)
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
			Plan: createPlan(),
			Log:  builderLog,

			// To make sure r.Scheme is not nil
			Client: client,
		},
	}
}
