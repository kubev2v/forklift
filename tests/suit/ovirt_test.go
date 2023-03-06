package suit_test

import (
	"time"

	forkliftv1 "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/tests/suit/framework"
	"github.com/konveyor/forklift-controller/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	ovirtProviderName = "ovirt-provider"
	ovirtStorageClass = "nfs-csi"
)

var _ = Describe("[level:component]Migration tests for oVirt provider", func() {
	f := framework.NewFramework("migration-func-test")

	FIt("[oVirt MTV] should create provider with NetworkMap", func() {
		// TODO: use a different (the generated) namespace
		//namespace := "konveyor-forklift"
		namespace := f.Namespace.Name
		err := f.Clients.OvirtClient.SetupClient()
		Expect(err).ToNot(HaveOccurred())

		By("Load Source VM Details from Ovirt")
		vmData, err := f.Clients.OvirtClient.LoadSourceDetails()
		Expect(err).ToNot(HaveOccurred())

		By("Create Secret from Definition")
		s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(
			map[string]string{
				"createdForResource":     "ovirt-provider",
				"createdForResourceType": "providers",
				"createdForProviderType": "ovirt",
			}, nil,
			map[string][]byte{
				"cacert":   []byte(f.OvirtClient.Cacert),
				"password": []byte(f.OvirtClient.Password),
				"user":     []byte(f.OvirtClient.Username),
				"url":      []byte(f.OvirtClient.OvirtURL),
			}, namespace, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		By("Create oVirt provider")
		pr := utils.NewProvider(ovirtProviderName, forkliftv1.OVirt, namespace, map[string]string{}, f.OvirtClient.OvirtURL, s)
		err = utils.CreateProviderFromDefinition(f.CrClient, pr)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, ovirtProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		provider, err := utils.GetProvider(f.CrClient, ovirtProviderName, namespace)
		Expect(err).ToNot(HaveOccurred())
		By("Create Network Map")
		networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, vmData.GetVMNics()[0])
		err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, namespace, networkMapName, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create Storage Map")
		storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name, vmData.GetVMSDs(), ovirtStorageClass)
		err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, namespace, test_storage_map_name, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Creating plan")
		planDenf := utils.NewPlanWithVmId(namespace, *provider, test_plan_name, test_storage_map_name, networkMapName, []string{vmData.GetTestVMId()}, "default")
		err = utils.CreatePlanFromDefinition(f.CrClient, planDenf)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, test_plan_name, 15*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Creating migration")
		migrationDef := utils.NewMigration(provider.Namespace, test_migration_name, test_plan_name)
		err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name, 300*time.Second)
		Expect(err).ToNot(HaveOccurred())

	})
})
