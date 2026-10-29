package ovirt_test

import (
	v1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	ovirtadapter "github.com/kubev2v/forklift/pkg/controller/plan/adapter/ovirt"
	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	ovirtmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	model "github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var validatorLog = logging.WithName("ovirt-validator-test")

var _ = Describe("oVirt validator StaticIPs", func() {
	vmRef := ref.Ref{ID: "test-vm"}

	newValidator := func(preserveStaticIPs bool, vm model.Workload) *ovirtadapter.Validator {
		inv := &mockInventory{workload: vm}
		return &ovirtadapter.Validator{
			Context: &plancontext.Context{
				Plan: &v1beta1.Plan{
					Spec: v1beta1.PlanSpec{
						PreserveStaticIPs: preserveStaticIPs,
					},
				},
				Source: plancontext.Source{
					Inventory: inv,
				},
				Log: validatorLog,
			},
		}
	}

	It("should return true when preserveStaticIPs is false", func() {
		v := newValidator(false, model.Workload{})
		ok, err := v.StaticIPs(vmRef)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("should return true when at least one NIC has IP data", func() {
		vm := model.Workload{}
		vm.NICs = []model.XNIC{
			{
				VNIC: ovirtmodel.NIC{
					MAC:       "00:1a:2b:3c:4d:5e",
					IpAddress: nil,
				},
			},
			{
				VNIC: ovirtmodel.NIC{
					MAC: "00:1a:2b:3c:4d:5f",
					IpAddress: []ovirtmodel.IpAddress{
						{Address: "10.0.0.1", Version: "v4"},
					},
				},
			},
		}
		v := newValidator(true, vm)
		ok, err := v.StaticIPs(vmRef)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeTrue())
	})

	It("should return false when IpAddress entries all have empty Address", func() {
		vm := model.Workload{}
		vm.NICs = []model.XNIC{
			{
				VNIC: ovirtmodel.NIC{
					MAC: "00:1a:2b:3c:4d:5e",
					IpAddress: []ovirtmodel.IpAddress{
						{Address: "", Version: "v4"},
						{Address: "", Version: "v6"},
					},
				},
			},
		}
		v := newValidator(true, vm)
		ok, err := v.StaticIPs(vmRef)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})

	It("should return false when no NIC has IP data (guest agent missing)", func() {
		vm := model.Workload{}
		vm.NICs = []model.XNIC{
			{
				VNIC: ovirtmodel.NIC{
					MAC:       "00:1a:2b:3c:4d:5e",
					IpAddress: nil,
				},
			},
			{
				VNIC: ovirtmodel.NIC{
					MAC:       "00:1a:2b:3c:4d:5f",
					IpAddress: []ovirtmodel.IpAddress{},
				},
			},
		}
		v := newValidator(true, vm)
		ok, err := v.StaticIPs(vmRef)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok).To(BeFalse())
	})
})
