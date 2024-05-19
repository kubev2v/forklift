package ovirt

import (
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
)

var _ = ginkgo.Describe("ovirt collector", func() {
	table.DescribeTable("should", func(version string, matchVersion gtypes.GomegaMatcher) {
		major, minor, build, revision := parseVersion(version)
		Expect(strings.Join([]string{major, minor, build, revision}, ".")).Should(matchVersion)
	},
		table.Entry("get version when revision is in the 3rd element", "4.5.5-1.el8", Equal("4.5.5.1")),
		table.Entry("get version when revision is in the 4th element", "4.5.3.4-1.el8ev", Equal("4.5.3.4")),
	)
})
