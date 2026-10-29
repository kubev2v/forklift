package ovirt_test

import (
	"fmt"

	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	ovirtadapter "github.com/kubev2v/forklift/pkg/controller/plan/adapter/ovirt"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ovirtmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	web "github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
)

var builderLog = logging.WithName("ovirt-builder-test")

var _ = Describe("oVirt builder PodEnvironment", func() {
	It("should always return empty env (oVirt has no conversion pod)", func() {
		builder := &ovirtadapter.Builder{
			Context: &plancontext.Context{
				Plan: &v1beta1.Plan{
					Spec: v1beta1.PlanSpec{PreserveStaticIPs: true},
				},
				Log: builderLog,
			},
		}
		env, err := builder.PodEnvironment(ref.Ref{ID: "test-vm"}, &core.Secret{})
		Expect(err).NotTo(HaveOccurred())
		Expect(env).To(BeEmpty())
	})
})

var _ = Describe("oVirt builder BuildNICPinningUserData", func() {
	It("should generate udev rules and HWADDR pinning for multi-NIC VMs", func() {
		nics := []model.XNIC{
			{
				VNIC: ovirtmodel.NIC{
					MAC:        "00:1a:2b:3c:4d:01",
					DeviceName: "ens3",
					IpAddress: []ovirtmodel.IpAddress{
						{Address: "10.0.0.1", Version: "v4"},
					},
				},
			},
			{
				VNIC: ovirtmodel.NIC{
					MAC:        "00:1a:2b:3c:4d:02",
					DeviceName: "ens7",
					IpAddress: []ovirtmodel.IpAddress{
						{Address: "192.168.1.5", Version: "v4"},
					},
				},
			},
		}
		ud := ovirtadapter.BuildNICPinningUserData(nics)
		Expect(ud).To(ContainSubstring("#cloud-config"))
		// Network disable is handled via NetworkData, not in user-data.
		Expect(ud).To(ContainSubstring("#cloud-config"))
		// Udev rules (RHEL 7/8) — standard format with DRIVERS=="?*"
		Expect(ud).To(ContainSubstring("70-persistent-net.rules"))
		Expect(ud).To(ContainSubstring(`DRIVERS=="?*"`))
		Expect(ud).To(ContainSubstring(`NAME="ens3"`))
		Expect(ud).To(ContainSubstring(`NAME="ens7"`))
		// Systemd .link files (RHEL 9+)
		Expect(ud).To(ContainSubstring("10-forklift-ens3.link"))
		Expect(ud).To(ContainSubstring("MACAddress=00:1a:2b:3c:4d:01"))
		Expect(ud).To(ContainSubstring("Name=ens3"))
		Expect(ud).To(ContainSubstring("MACAddress=00:1a:2b:3c:4d:02"))
		Expect(ud).To(ContainSubstring("Name=ens7"))
		// HWADDR pinning
		Expect(ud).To(ContainSubstring("HWADDR=00:1a:2b:3c:4d:01"))
		Expect(ud).To(ContainSubstring("HWADDR=00:1a:2b:3c:4d:02"))
	})

	It("should return empty for NICs with no MAC", func() {
		nics := []model.XNIC{
			{VNIC: ovirtmodel.NIC{MAC: ""}},
			{VNIC: ovirtmodel.NIC{MAC: ""}},
		}
		Expect(ovirtadapter.BuildNICPinningUserData(nics)).To(BeEmpty())
	})

	It("should skip name pinning and HWADDR when DeviceName and IPs are absent", func() {
		nics := []model.XNIC{
			{VNIC: ovirtmodel.NIC{MAC: "00:1a:2b:3c:4d:01"}},
			{VNIC: ovirtmodel.NIC{MAC: "00:1a:2b:3c:4d:02"}},
		}
		ud := ovirtadapter.BuildNICPinningUserData(nics)
		// No device name → no udev/link rules (guessing would be wrong)
		Expect(ud).NotTo(ContainSubstring("persistent-net.rules"))
		Expect(ud).NotTo(ContainSubstring(".link"))
		Expect(ud).NotTo(ContainSubstring("HWADDR"))
	})
})

// mockInventory implements web.Client for oVirt builder/validator tests.
type mockInventory struct {
	workload model.Workload
	findErr  error
}

func (m *mockInventory) Find(resource interface{}, _ ref.Ref) error {
	if m.findErr != nil {
		return m.findErr
	}
	switch res := resource.(type) {
	case *model.Workload:
		*res = m.workload
	default:
		return fmt.Errorf("unexpected resource type: %T", resource)
	}
	return nil
}

//nolint:nilnil
func (m *mockInventory) Finder() web.Finder                                      { return nil }
func (m *mockInventory) Get(interface{}, string) error                           { return nil }
func (m *mockInventory) List(interface{}, ...web.Param) error                    { return nil }
func (m *mockInventory) Watch(interface{}, web.EventHandler) (*web.Watch, error) { return nil, nil } //nolint:nilnil
func (m *mockInventory) VM(*ref.Ref) (interface{}, error)                        { return nil, nil } //nolint:nilnil
func (m *mockInventory) Workload(*ref.Ref) (interface{}, error)                  { return nil, nil } //nolint:nilnil
func (m *mockInventory) Network(*ref.Ref) (interface{}, error)                   { return nil, nil } //nolint:nilnil
func (m *mockInventory) Storage(*ref.Ref) (interface{}, error)                   { return nil, nil } //nolint:nilnil
func (m *mockInventory) Host(*ref.Ref) (interface{}, error)                      { return nil, nil } //nolint:nilnil
