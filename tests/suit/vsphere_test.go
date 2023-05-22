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
	vsphereProviderName = "vsphere-provider"
	vsphereStorageClass = "nfs-csi"
)

var _ = Describe("[level:component]Migration tests for vSphere provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("[test] should create provider with NetworkMap", func() {
		namespace := f.Namespace.Name
		By("Create Secret from Definition")
		s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, nil,
			map[string][]byte{
				"thumbprint": []byte("52:6C:4E:88:1D:78:AE:12:1C:F3:BB:6C:5B:F4:E2:82:86:A7:08:AF"),
				"password":   []byte("MTIzNDU2Cg=="),
				"user":       []byte("YWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs"),
			}, namespace, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		By("Create target Openshift provider")
		targetPr := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, map[string]string{}, map[string]string{}, "", nil)
		err = utils.CreateProviderFromDefinition(f.CrClient, targetPr)
		Expect(err).ToNot(HaveOccurred())
		_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create vSphere provider")
		pr := utils.NewProvider(vsphereProviderName, forkliftv1.VSphere, namespace, map[string]string{}, map[string]string{"vddkInitImage": "quay.io/kubev2v/vddk-test-vmdk"}, "https://vcsim.konveyor-forklift:8989/sdk", s)
		err = utils.CreateProviderFromDefinition(f.CrClient, pr)
		Expect(err).ToNot(HaveOccurred())
		provider, err := utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, vsphereProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create Network Map")
		networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, "dvportgroup-13")
		err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, namespace, networkMapName, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create Storage Map")
		storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name, []string{"datastore-52"}, vsphereStorageClass)
		err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, namespace, test_storage_map_name, 10*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating plan")
		planDef := utils.NewPlanWithVmName(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, []string{"DC0_H0_VM0"}, "default")

		err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
		Expect(err).ToNot(HaveOccurred())
		err, _ = utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, test_plan_name, 15*time.Second)
		Expect(err).ToNot(HaveOccurred())

		By("Creating migration")
		migrationDef := utils.NewMigration(provider.Namespace, test_migration_name, test_plan_name)
		err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name, 300*time.Second)
		Expect(err).ToNot(HaveOccurred())

	})
})
