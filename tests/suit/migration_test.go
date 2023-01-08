package suit_test

import (
	forkliftv1 "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/tests/suit/framework"
	"github.com/konveyor/forklift-controller/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

const (
	providerName   = "vsphere-provider"
	networkMapName = "network-map-test"
	namespace      = "konveyor-forklift"
)

var _ = Describe("[level:component]Migration tests for vSphere provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("[test] should create provider with NetworkMap", func() {

		By("Create Secret from Definition")
		s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, nil,
			map[string][]byte{
				"thumbprint": []byte("52:6C:4E:88:1D:78:AE:12:1C:F3:BB:6C:5B:F4:E2:82:86:A7:08:AF"),
				"password":   []byte("MTIzNDU2Cg=="),
				"user":       []byte("YWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs"),
			}, f.Namespace.Name, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		By("Create vSphere provider")
		pr := utils.NewProvider(providerName, forkliftv1.VSphere, f.Namespace.Name, map[string]string{"vddkInitImage": "quay.io/kubev2v/vddk-test-vmdk"}, "https://vcsim.konveyor-forklift:8989/sdk", s)
		err = utils.CreateProviderFromDefinition(f.CrClient, pr)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForProviderReadyWithTimeout(f.CrClient, f.Namespace.Name, providerName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		provider, err := utils.GetProvider(f.CrClient, providerName, f.Namespace.Name)
		Expect(err).ToNot(HaveOccurred())
		By("Create Network Map")
		networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, "dvportgroup-13")
		err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, f.Namespace.Name, networkMapName, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create Storage Map")
		storageMapDef := utils.NewStorageMap(namespace, *provider, "test-storage-map-v", []string{"datastore-52"})
		err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, f.Namespace.Name, "test-storage-map-v", 10*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating plan")
		planDenf := utils.NewPlan(namespace, *provider, "plan-test", "test-storage-map-v", networkMapName, []string{"DC0_H0_VM0"})
		err = utils.CreatePlanFromDefinition(f.CrClient, planDenf)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForPlanReadyWithTimeout(f.CrClient, f.Namespace.Name, "plan-test", 15*time.Second)
		Expect(err).ToNot(HaveOccurred())

	})
})
