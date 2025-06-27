package suit_test

import (
	"time"

	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"github.com/kubev2v/forklift/tests/suit/framework"
	"github.com/kubev2v/forklift/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	openstackProviderName = "osp-provider"
	openstackStorageClass = "nfs-csi"
	packstackNameSpace    = "konveyor-forklift"
)

var _ = Describe("[level:component]Migration tests for OpenStack provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("[test] should create provider with NetworkMap", func() {
		namespace := f.Namespace.Name
		err := f.Clients.OpenStackClient.SetupClient("cirros-server", "net-int", "nfs")
		Expect(err).ToNot(HaveOccurred())

		By("Load Source VM Details from OpenStack")
		vmData, err := f.Clients.OpenStackClient.LoadSourceDetails(f, packstackNameSpace, "packstack")
		Expect(err).ToNot(HaveOccurred())

		By("Create Secret from Definition")
		s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil,
			map[string]string{
				"username":           "admin",
				"password":           "12e2f14739194a6c",
				"domainName":         "default",
				"projectName":        "admin",
				"regionName":         "RegionOne",
				"insecureSkipVerify": "true",
			}, nil, namespace, "os-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		By("Create target Openshift provider")
		targetPr := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, map[string]string{}, map[string]string{}, "", nil)
		err = utils.CreateProviderFromDefinition(f.CrClient, targetPr)
		Expect(err).ToNot(HaveOccurred())
		_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create osp provider")
		pr := utils.NewProvider(openstackProviderName, forkliftv1.OpenStack, namespace, map[string]string{}, map[string]string{},
			"http://packstack.konveyor-forklift:5000/v3", s)
		err = utils.CreateProviderFromDefinition(f.CrClient, pr)
		Expect(err).ToNot(HaveOccurred())
		provider, err := utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, openstackProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, vmData.GetNetworkId())
		err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, namespace, networkMapName, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create Storage Map")

		//TODO: Add storage-class  pass here
		storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name, []string{vmData.GetVolumeId()}, openstackStorageClass)
		storageMapDef.Spec.Map = append(storageMapDef.Spec.Map,
			forkliftv1.StoragePair{
				Source: ref.Ref{Name: forkliftv1.GlanceSource},
				Destination: forkliftv1.DestinationStorage{
					StorageClass: openstackStorageClass,
				},
			})

		err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, namespace, test_storage_map_name, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating plan")
		planDef := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, namespace, []string{vmData.GetTestVMId()})

		planDef.Spec.Warm = true
		err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
		Expect(err).To(HaveOccurred())

		planDef.Spec.Warm = false
		err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
		Expect(err).ToNot(HaveOccurred())

		err = utils.UpdatePlanWarmMigration(f.CrClient, planDef, true)
		Expect(err).To(HaveOccurred())

		err, _ = utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, test_plan_name, 15*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Creating migration")
		migrationDef := utils.NewMigration(provider.Namespace, test_migration_name, test_plan_name)
		err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name, 400*time.Second)
		Expect(err).ToNot(HaveOccurred())
	})

})
