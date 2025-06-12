package suit_test

import (
	"time"

	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/tests/suit/framework"
	"github.com/kubev2v/forklift/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

const (
	AnnPopulatorLabels = "populatorLabels"
)

var _ = Describe("Forklift", func() {
	f := framework.NewFramework("migration-func-test")
	var vmData *framework.OvirtVM
	var secret *v1.Secret
	var namespace string

	BeforeEach(func() {
		namespace = f.Namespace.Name
		err := f.Clients.OvirtClient.SetupClient(false)
		Expect(err).ToNot(HaveOccurred())
		By("Load Source VM Details from Ovirt")
		vmData, err = f.Clients.OvirtClient.LoadSourceDetails()
		Expect(err).ToNot(HaveOccurred())
		By("Create Secret from Definition")
		secret, err = utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(
			map[string]string{
				"createdForProviderType": "ovirt",
			}, nil,
			map[string][]byte{
				"cacert":   []byte(f.OvirtClient.Cacert),
				"password": []byte(f.OvirtClient.Password),
				"user":     []byte(f.OvirtClient.Username),
				"url":      []byte(f.OvirtClient.OvirtURL),
			}, namespace, "provider-test-secret"))
		Expect(err).ToNot(HaveOccurred())
	})

	Context("Plan with provider that is set with transfer network", func() {
		providerNetwork := "my-network"
		var provider *forkliftv1.Provider
		var targetNS *v1.Namespace
		var err error

		BeforeEach(func() {
			targetNS, err = f.CreateNamespace("target", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			By("Create target Openshift provider")
			annotations := map[string]string{"forklift.konveyor.io/defaultTransferNetwork": providerNetwork}
			target := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, annotations, map[string]string{}, "", nil)
			err = utils.CreateProviderFromDefinition(f.CrClient, target)
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create oVirt provider")
			pr := utils.NewProvider(ovirtProviderName, forkliftv1.OVirt, namespace, map[string]string{}, map[string]string{}, f.OvirtClient.OvirtURL, secret)
			err = utils.CreateProviderFromDefinition(f.CrClient, pr)
			Expect(err).ToNot(HaveOccurred())
			provider, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, ovirtProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create Network Map")
			networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, vmData.GetVMNics()[0])
			err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, namespace, networkMapName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create Storage Map")
			storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name, vmData.GetVMSDs(), ovirtStorageClass)
			err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, namespace, test_storage_map_name, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Transfer network should not be set to that of the Provider when it doesn't exist", func() {
			By("Create plan")
			planDef := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, targetNS.Name, []string{vmData.GetTestVMId()})
			err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
			Expect(err).ToNot(HaveOccurred())
			err, plan := utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, planDef.Name, 15*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Verify created plan")
			Expect(plan).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork).To(BeNil())
		})

		It("Transfer network should be set to that of the Provider when it exists", func() {
			By("Create Network Attachment Definition")
			err, _ = utils.CreateNetworkAttachmentDefinition(f.CrClient, providerNetwork, targetNS.Name)
			Expect(err).ToNot(HaveOccurred())
			By("Create plan")
			planDef := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, targetNS.Name, []string{vmData.GetTestVMId()})
			err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
			Expect(err).ToNot(HaveOccurred())
			err, plan := utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, planDef.Name, 15*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Verify created plan")
			Expect(plan).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork.Name).To(Equal(providerNetwork))
		})

		It("Transfer network should not be overridden with that of the provider when specified", func() {
			By("Create Network Attachment Definition")
			err, nad := utils.CreateNetworkAttachmentDefinition(f.CrClient, providerNetwork, targetNS.Name)
			Expect(err).ToNot(HaveOccurred())
			By("Create plan with other transfer network")
			planDef := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, targetNS.Name, []string{vmData.GetTestVMId()})
			planNetwork := "another-network"
			planDef.Spec.TransferNetwork = &v1.ObjectReference{
				Namespace: nad.Namespace,
				Name:      planNetwork,
			}
			err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
			Expect(err).ToNot(HaveOccurred())
			By("Get created plan")
			err, plan := utils.GetPlan(f.CrClient, planDef.Namespace, planDef.Name)
			Expect(err).ToNot(HaveOccurred())
			By("Verify created plan")
			Expect(plan).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork.Name).To(Equal(planNetwork))
		})

		It("Annotation of new plan should be set with populator labels annotation true", func() {
			By("Create plan")
			planDef := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, targetNS.Name, []string{vmData.GetTestVMId()})
			err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
			Expect(err).ToNot(HaveOccurred())
			err, plan := utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, planDef.Name, 15*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Verify created plan")
			Expect(plan).ToNot(BeNil())
			Expect(plan.Annotations[AnnPopulatorLabels]).To(Equal("True"))
		})
	})

	Context("Plan with provider that is set with transfer network that includes namespace", func() {
		providerNetworkNamespace := "default"
		providerNetworkName := "my-network"
		providerNetwork := providerNetworkNamespace + "/" + providerNetworkName

		var provider *forkliftv1.Provider
		var targetNS *v1.Namespace
		var err error

		BeforeEach(func() {
			targetNS, err = f.CreateNamespace("target", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			targetNS, err = f.CreateNamespace(providerNetworkNamespace, map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			By("Create target Openshift provider")
			annotations := map[string]string{"forklift.konveyor.io/defaultTransferNetwork": providerNetwork}
			target := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, annotations, map[string]string{}, "", nil)
			err = utils.CreateProviderFromDefinition(f.CrClient, target)
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create oVirt provider")
			pr := utils.NewProvider(ovirtProviderName, forkliftv1.OVirt, namespace, map[string]string{}, map[string]string{}, f.OvirtClient.OvirtURL, secret)
			err = utils.CreateProviderFromDefinition(f.CrClient, pr)
			Expect(err).ToNot(HaveOccurred())
			provider, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, ovirtProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create Network Map")
			networkMapDef := utils.NewNetworkMap(namespace, *provider, networkMapName, vmData.GetVMNics()[0])
			err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, namespace, networkMapName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create Storage Map")
			storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name, vmData.GetVMSDs(), ovirtStorageClass)
			err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, namespace, test_storage_map_name, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Namespaced transfer network should not be set to that of the Provider when it doesn't exist", func() {
			By("Create plan for provider with defaultTransferNetwork")
			planDef := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, targetNS.Name, []string{vmData.GetTestVMId()})
			err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
			Expect(err).ToNot(HaveOccurred())
			err, plan := utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, planDef.Name, 15*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Verify created plan")
			Expect(plan).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork).To(BeNil())
		})

		It("Namespaced transfer network should be set to that of the Provider when it exists", func() {
			By("Create namespaced Network Attachment Definition")
			err, _ = utils.CreateNetworkAttachmentDefinition(f.CrClient, providerNetworkName, providerNetworkNamespace)
			Expect(err).ToNot(HaveOccurred())
			By("Create plan for provider with defaultTransferNetwork")
			planDef := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, targetNS.Name, []string{vmData.GetTestVMId()})
			err = utils.CreatePlanFromDefinition(f.CrClient, planDef)
			Expect(err).ToNot(HaveOccurred())
			err, plan := utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, planDef.Name, 15*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Verify created plan for provider with defaultTransferNetwork")
			Expect(plan).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork).ToNot(BeNil())
			Expect(plan.Spec.TransferNetwork.Name).To(Equal(providerNetworkName))
			Expect(plan.Spec.TransferNetwork.Namespace).To(Equal(providerNetworkNamespace))
		})
	})
})
