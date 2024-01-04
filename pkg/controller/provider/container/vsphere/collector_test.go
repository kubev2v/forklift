package vsphere

import (
	liburl "net/url"
	"testing"

	"github.com/onsi/gomega"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

func TestTpmCollector(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	collector := Collector{}
	url, err := liburl.Parse("https://fake.com/sdk")
	g.Expect(err).To(gomega.BeNil())
	vimClient := &vim25.Client{
		Client:         soap.NewClient(url, false),
		ServiceContent: types.ServiceContent{},
	}
	collector.client = &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}
	// Verify that we don't collect TPM for unsupported version
	collector.client.ServiceContent.About.ApiVersion = "6.5"
	g.Expect(collector.vmPathSet()).ShouldNot(gomega.ContainElement(fTpmPresent))

	// Verify that we don't collect TPM for supported version
	collector.client.ServiceContent.About.ApiVersion = "6.7"
	g.Expect(collector.vmPathSet()).Should(gomega.ContainElement(fTpmPresent))

	// Verify that we don't collect TPM for supported version
	collector.client.ServiceContent.About.ApiVersion = "7.0"
	g.Expect(collector.vmPathSet()).Should(gomega.ContainElement(fTpmPresent))
}
