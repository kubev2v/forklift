package plan

import (
	"testing"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	vsphere "github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/onsi/gomega"
)

func TestShouldWarnHostdTmp(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	for _, tc := range []struct {
		planned, maxInFlight int
		want                 bool
	}{
		{9, 10, false},
		{10, 10, true},
		{25, 10, true},
		{19, 20, false},
		{20, 20, true},
		{10, 0, false},
	} {
		g.Expect(shouldWarnHostdTmp(tc.planned, tc.maxInFlight)).
			To(gomega.Equal(tc.want), "planned=%d maxInFlight=%d", tc.planned, tc.maxInFlight)
	}
}

func TestXcopyRuntimeHosts(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	vm := &vsphere.VM{VM1: vsphere.VM1{Host: "vm-host"}}

	g.Expect(xcopyRuntimeHosts(vm, nil)).To(gomega.Equal([]string{"vm-host"}))
	g.Expect(xcopyRuntimeHosts(&vsphere.VM{}, nil)).To(gomega.BeEmpty())
	g.Expect(xcopyRuntimeHosts(vm, &api.VSphereXcopyPluginConfig{
		DedicatedMigrationHosts: []string{"dedicated-a", "dedicated-b"},
	})).To(gomega.Equal([]string{"dedicated-a", "dedicated-b"}))
}
