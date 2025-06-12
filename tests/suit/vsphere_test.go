package suit_test

import (
	"context"
	"time"

	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/tests/suit/framework"
	"github.com/kubev2v/forklift/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	vsphereProviderName = "vsphere-provider"
	vsphereStorageClass = "nfs-csi"
)

var _ = Describe("vSphere provider", func() {
	f := framework.NewFramework("migration-func-test")

	It("Migrate VM", func() {
		namespace := f.Namespace.Name
		By("Create Secret from Definition")
		simSecret, err := utils.GetSecret(f.K8sClient, "konveyor-forklift", "vcsim-certificate")
		Expect(err).ToNot(HaveOccurred())
		s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, nil,
			map[string][]byte{
				"cacert":   simSecret.Data["ca.crt"],
				"password": []byte("MTIzNDU2Cg=="),
				"user":     []byte("YWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs"),
			}, namespace, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())

		targetNS, err := f.CreateNamespace("default", map[string]string{})
		Expect(err).ToNot(HaveOccurred())
		By("Create target Openshift provider")
		targetPr := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, map[string]string{}, map[string]string{}, "", nil)
		err = utils.CreateProviderFromDefinition(f.CrClient, targetPr)
		Expect(err).ToNot(HaveOccurred())
		_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
		Expect(err).ToNot(HaveOccurred())
		By("Create vSphere provider")
		pr := utils.NewProvider(vsphereProviderName, forkliftv1.VSphere, namespace, map[string]string{}, map[string]string{forkliftv1.VDDK: "quay.io/kubev2v/vddk-test-vmdk"}, "https://vcsim.konveyor-forklift:8989/sdk", s)
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
		planDef := utils.NewPlanWithVmName(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, []string{"DC0_H0_VM0"}, targetNS.Name)

		err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
		Expect(err).ToNot(HaveOccurred())
		err, _ = utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, test_plan_name, 1*time.Minute)
		Expect(err).ToNot(HaveOccurred())

		By("Creating migration")
		migrationDef := utils.NewMigration(provider.Namespace, test_migration_name, test_plan_name)
		err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
		Expect(err).ToNot(HaveOccurred())
		err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name, 300*time.Second)
		Expect(err).ToNot(HaveOccurred())

	})

	Context("vCenter SDK", func() {
		var provider *forkliftv1.Provider
		var namespace string

		BeforeEach(func() {
			namespace = f.Namespace.Name
			By("Create Secret from Definition")
			s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, nil,
				map[string][]byte{
					v1beta1.Insecure: []byte("true"),
					"password":       []byte("MTIzNDU2Cg=="),
					"user":           []byte("YWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs"),
				}, namespace, "vcenter-provider-secret"))
			Expect(err).ToNot(HaveOccurred())
			By("Create vCenter provider")
			pr := utils.NewProvider(vsphereProviderName, forkliftv1.VSphere, namespace, map[string]string{}, map[string]string{forkliftv1.VDDK: "quay.io/kubev2v/vddk-test-vmdk"}, "https://vcsim.konveyor-forklift:8989/sdk", s)
			err = utils.CreateProviderFromDefinition(f.CrClient, pr)
			Expect(err).ToNot(HaveOccurred())
			provider, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, vsphereProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Provider should be set with sdkEndpoint=vcenter by default", func() {
			Expect(provider.Spec.Settings[forkliftv1.SDK]).To(Equal(forkliftv1.VCenter))
		})
		It("Host cannot be defined with empty credentials", func() {
			labels := map[string]string{
				"createdForResourceType": "hosts",
				"createdForResource":     "host-21",
			}
			_, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(labels, nil,
				map[string][]byte{
					"ip":       []byte("52:6C:4E:88:1D:78:AE:12:1C:F3:BB:6C:5B:F4:E2:82:86:A7:08:AF"),
					"provider": []byte(vsphereProviderName),
				}, namespace, "esxi-host-secret"))
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ESXi SDK", func() {
		var provider *forkliftv1.Provider
		var namespace string

		BeforeEach(func() {
			namespace = f.Namespace.Name
			By("Create Secret from Definition")
			s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(nil, nil,
				map[string][]byte{
					v1beta1.Insecure: []byte("true"),
					"password":       []byte("MTIzNDU2Cg=="),
					"user":           []byte("YWRtaW5pc3RyYXRvckB2c3BoZXJlLmxvY2Fs"),
				}, namespace, "esxi-provider-secret"))
			Expect(err).ToNot(HaveOccurred())
			By("Create ESXi provider")
			settings := map[string]string{
				forkliftv1.VDDK: "quay.io/kubev2v/vddk-test-vmdk",
				forkliftv1.SDK:  forkliftv1.ESXI,
			}
			pr := utils.NewProvider(vsphereProviderName, forkliftv1.VSphere, namespace, map[string]string{}, settings, "https://vcsim.konveyor-forklift:8989/sdk", s)
			err = utils.CreateProviderFromDefinition(f.CrClient, pr)
			Expect(err).ToNot(HaveOccurred())
			provider, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, vsphereProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Provider should be set with sdkEndpoint=esxi", func() {
			Expect(provider.Spec.Settings[forkliftv1.SDK]).To(Equal(forkliftv1.ESXI))
		})
		It("Credentials are copied from the Provider to the Host", func() {
			labels := map[string]string{
				"createdForResourceType": "hosts",
				"createdForResource":     "host-21",
			}
			secret := utils.NewSecretDefinition(labels, nil,
				map[string][]byte{
					"ip":       []byte("52:6C:4E:88:1D:78:AE:12:1C:F3:BB:6C:5B:F4:E2:82:86:A7:08:AF"),
					"provider": []byte(vsphereProviderName),
				}, namespace, "esxi-host-secret",
			)
			secret, err := f.K8sClient.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, found := secret.Data["user"]
			Expect(found).To(BeTrue())
		})
	})
})
