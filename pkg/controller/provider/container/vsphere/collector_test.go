package vsphere

import (
	liburl "net/url"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

var _ = Describe("vSphere collector", func() {
	collector := Collector{}
	url, _ := liburl.Parse("https://fake.com/sdk")
	vimClient := &vim25.Client{
		Client:         soap.NewClient(url, false),
		ServiceContent: types.ServiceContent{},
	}
	collector.client = &govmomi.Client{
		SessionManager: session.NewManager(vimClient),
		Client:         vimClient,
	}

	table.DescribeTable("should", func(version string, matchTpm gtypes.GomegaMatcher) {
		collector.client.ServiceContent.About.ApiVersion = version
		Expect(collector.vmPathSet()).Should(matchTpm)
	},
		table.Entry("not collect TPM from vSphere < 6.7", "6.5", Not(ContainElements(fTpmPresent))),
		table.Entry("collect TPM from vSphere 6.7", "6.7", ContainElements(fTpmPresent)),
		table.Entry("collect TPM from vSphere > 6.7", "7.0", ContainElements(fTpmPresent)),
	)
})
