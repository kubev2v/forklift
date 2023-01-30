package suit_test

import (
	forkliftv1 "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/tests/suit/framework"
	"github.com/konveyor/forklift-controller/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("[level:component]Migration tests for oVirt provider", func() {
	f := framework.NewFramework("migration-func-test")

	FIt("[oVirt MTV] should create provider with NetworkMap", func() {

		err := f.Clients.OvirtClient.SetupClient()
		Expect(err).ToNot(HaveOccurred())

		By("Load Source VM Details from Ovirt")
		err = f.Clients.OvirtClient.LoadSourceDetails()
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
			}, f.Namespace.Name, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		err = f.OvirtClient.ConnectUsingSecret(s)
		Expect(err).ToNot(HaveOccurred())

		By("Create oVirt provider")
		pr := utils.NewProvider(ovirtProviderName, forkliftv1.OVirt, f.Namespace.Name, map[string]string{}, f.OvirtClient.OvirtURL, s)
		err = utils.CreateProviderFromDefinition(f.CrClient, pr)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForProviderReadyWithTimeout(f.CrClient, f.Namespace.Name, ovirtProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		provider, err := utils.GetProvider(f.CrClient, ovirtProviderName, f.Namespace.Name)
		Expect(err).ToNot(HaveOccurred())
		By("Create Network Map")
		networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, f.OvirtClient.GetVMNics()[0])
		err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, f.Namespace.Name, networkMapName, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create Storage Map")
		storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name, f.OvirtClient.GetVMSDs())
		err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, f.Namespace.Name, test_storage_map_name, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating plan")
		planDenf := utils.NewPlanWithID(namespace, *provider, test_plan_name, test_storage_map_name, networkMapName, []string{f.OvirtClient.GetTestVMId()})
		err = utils.CreatePlanFromDefinition(f.CrClient, planDenf)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForPlanReadyWithTimeout(f.CrClient, f.Namespace.Name, test_plan_name, 15*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating migration")
		migrationDef := utils.NewMigration(provider.Namespace, test_migration_name, test_plan_name)
		err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name, 300*time.Second)
		Expect(err).ToNot(HaveOccurred())

	})
})
