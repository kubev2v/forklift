package suit_test

import (
	"fmt"
	forkliftv1 "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/tests/suit/framework"
	"github.com/konveyor/forklift-controller/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	providerName   = "vsphere-provider"
	networkMapName = "network-map-test"
	namespace      = "konveyor-forklift"
)

var _ = Describe("[level:component]Migration tests for vSphere provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("provider created", func() {
		//var err :=
		//Expect(err).ToNot(HaveOccurred())
		s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, nil,
			map[string][]byte{
				"thumbprint": []byte("ss"),
				"password":   []byte("aaa"),
				"user":       []byte("bbb"),
			}, namespace, "ovirt-provider-test-secret2"))
		Expect(err).ToNot(HaveOccurred())
		// namespace, "vsphere_URL_TBD", s)
		pr := utils.NewProvider("vsphere-provider", forkliftv1.OVirt, namespace, "url_dummy", s)
		err = utils.CreateProviderFromDefinition(f.CrClient, namespace, pr)
		Expect(err).ToNot(HaveOccurred())

		fmt.Fprintf(GinkgoWriter, "DEBUG:")
	})

	//It("network-map-test created", func() {
	//
	//	provider, err := utils.GetProvider(f.CrClient, providerName, f.Namespace.Name)
	//	Expect(err).ToNot(HaveOccurred())
	//	networkMapDef := utils.NewNetworkMap(f.Namespace.Name, *provider, networkMapName)
	//	utils.CreateNetworkMapFromDefinition(f.CrClient, f.Namespace.Name, networkMapDef)
	//})
})
