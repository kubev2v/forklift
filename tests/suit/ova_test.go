package suit_test

import (
	"time"

	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/tests/suit/framework"
	"github.com/kubev2v/forklift/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	ovaProviderName = "ova-provider"
	ovaStorageClass = "nfs-csi"
)

var _ = Describe("[level:component]Migration tests for OVA provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("[test] should create provider with NetworkMap", func() {
		namespace := f.Namespace.Name

		By("Load Source VM Details from OVA")
		vmData, err := f.Clients.OvaClient.LoadSourceDetails()
		Expect(err).ToNot(HaveOccurred())

		By("Get NFS share for OVA provider")
		nfs, err := f.Clients.OvaClient.GetNfsServerForOva(f.K8sClient)
		Expect(err).ToNot(HaveOccurred())
		By("Create Secret from Definition")
		secret, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(
			map[string]string{
				"createdForProviderType": "ova",
				"createdForResourceType": "providers",
			}, nil,
			map[string][]byte{
				"url": []byte(nfs),
			}, namespace, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		targetNS, err := f.CreateNamespace("ova-migration-test", map[string]string{})
		Expect(err).ToNot(HaveOccurred())
		By("Create target Openshift provider")
		targetPr := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, map[string]string{}, map[string]string{}, "", nil)
		err = utils.CreateProviderFromDefinition(f.CrClient, targetPr)
		Expect(err).ToNot(HaveOccurred())
		_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create OVA provider")
		pr := utils.NewProvider(ovaProviderName, forkliftv1.Ova, namespace, map[string]string{}, map[string]string{}, nfs, secret)
		err = utils.CreateProviderFromDefinition(f.CrClient, pr)
		Expect(err).ToNot(HaveOccurred())
		provider, err := utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, ovaProviderName, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred())
		By("Create Network Map")
		networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, vmData.GetNetworkId())
		err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, namespace, networkMapName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create Storage Map")
		storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name, []string{vmData.GetStorageName()}, ovaStorageClass)
		err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, namespace, test_storage_map_name, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating plan")
		planDef := utils.NewPlanWithVmName(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, []string{vmData.GetVmName()}, targetNS.Name)

		err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
		Expect(err).ToNot(HaveOccurred())
		err, _ = utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, test_plan_name, 15*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating migration")
		migrationDef := utils.NewMigration(provider.Namespace, test_migration_name, test_plan_name)
		err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name, 900*time.Second)
		Expect(err).ToNot(HaveOccurred())
	})
})
