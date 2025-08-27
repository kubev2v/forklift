
package vsphere

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"

	plancontext "github.com/kubev2v/forklift/pkg/controller/plan/context"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	planapi "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/plan"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("vsphere client tests", func() {

	var (
		client *Client
		server *simulator.Server
	)

	BeforeEach(func() {
		model := simulator.VPX()
		err := model.Create()
		Expect(err).NotTo(HaveOccurred())
		model.Service.TLS = nil

		server = model.Service.NewServer()
		serverURL := server.URL
		Expect(err).NotTo(HaveOccurred())

		soapClient := soap.NewClient(serverURL, true)
		vimClient, err := vim25.NewClient(context.TODO(), soapClient)
		Expect(err).NotTo(HaveOccurred())

		govmomiClient := &govmomi.Client{
			SessionManager: session.NewManager(vimClient),
			Client:         vimClient,
		}
		err = govmomiClient.Login(context.TODO(), serverURL.User)
		Expect(err).NotTo(HaveOccurred())

		client = &Client{
			Context: &plancontext.Context{
				Source: plancontext.Source{
					Provider: &v1beta1.Provider{},
					Secret:   &core.Secret{},
					Inventory: &mockInventory{},
				},
				Plan: &v1beta1.Plan{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-plan",
						Namespace: "test-namespace",
					},
				},
			},
			client: govmomiClient,
		}
	})

	AfterEach(func() {
		server.Close()
	})

	FIt("should return an error when the VM is not found", func() {
		state, err := client.PowerState(ref.Ref{ID: "missing_from_inventory"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("not found"))
		Expect(state).To(Equal(planapi.VMPowerState("")))
	})
})
