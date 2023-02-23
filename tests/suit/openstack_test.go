package suit_test

import (
	"github.com/konveyor/forklift-controller/tests/suit/framework"
	"github.com/konveyor/forklift-controller/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	openstackProviderName = "openstack-provider"
)

var _ = Describe("[level:component]Migration tests for Openstack provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("[test] should create provider with NetworkMap", func() {
		namespace := f.Namespace.Name

		By("Create Secret from Definition")
		_, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, nil,
			map[string][]byte{
				"thumbprint": []byte("52:6C:4E:88:1D:78:AE:12:1C:F3:BB:6C:5B:F4:E2:82:86:A7:08:AF"),
				"password":   []byte("MTIzNDU2Cg=="),
				"user":       []byte("YWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs"),
			}, namespace, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())

	})
})
