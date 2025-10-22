package suit_test

import (
	"strconv"
	"time"

	forkliftv1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/tests/suit/framework"
	"github.com/kubev2v/forklift/tests/suit/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	cnv "kubevirt.io/api/core/v1"
)

const (
	ovirtProviderName         = "ovirt-provider"
	ovirtInsecureProviderName = "ovirt-provider-insecure"
	ovirtStorageClass         = "nfs-csi"
	clusterCpuModel           = "Westmere"
)

var _ = Describe("[level:component]Migration tests for oVirt providers", func() {
	f := framework.NewFramework("migration-func-test")

	Context("[oVirt MTV] should create secure provider", func() {
		It("secure flow", func() {
			namespace := f.Namespace.Name
			err := f.Clients.OvirtClient.SetupClient(false)
			Expect(err).ToNot(HaveOccurred())

			By("Load Source VM Details from Ovirt")
			vmData, err := f.Clients.OvirtClient.LoadSourceDetails()
			Expect(err).ToNot(HaveOccurred())

			By("Create Secret from Definition")
			s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(
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

			targetNS, err := f.CreateNamespace("default", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			By("Create target Openshift provider")
			targetPr := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, map[string]string{}, map[string]string{}, "", nil)
			err = utils.CreateProviderFromDefinition(f.CrClient, targetPr)
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create oVirt provider")
			pr := utils.NewProvider(ovirtProviderName, forkliftv1.OVirt, namespace, map[string]string{}, map[string]string{}, f.OvirtClient.OvirtURL, s)
			err = utils.CreateProviderFromDefinition(f.CrClient, pr)
			Expect(err).ToNot(HaveOccurred())
			provider, err := utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, ovirtProviderName, 30*time.Second)
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
			planDenf := utils.NewPlanWithVmId(*provider, namespace, test_plan_name, test_storage_map_name, networkMapName, targetNS.Name, []string{vmData.GetTestVMId()})
			err = utils.CreatePlanFromDefinition(f.CrClient, planDenf)
			Expect(err).ToNot(HaveOccurred())
			err, _ = utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, test_plan_name, 15*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Creating migration")
			migrationDef := utils.NewMigration(provider.Namespace, test_migration_name, test_plan_name)
			err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name, 300*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying imported VM exists")
			vmId := types.UID(vmData.GetTestVMId())
			vm, err := utils.GetImportedVm(f.CrClient, targetNS.Name, func(vm cnv.VirtualMachine) bool {
				return vm.Spec.Template.Spec.Domain.Firmware.UUID == vmId
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(vm).ToNot(BeNil())
			By("Verifying VM is created without the cluster CPU")
			Expect(vm.Spec.Template.Spec.Domain.CPU.Model).To(BeEmpty())
		})

		It("[oVirt MTV] should create insecure provider", func() {
			namespace := f.Namespace.Name
			By("Load Source VM Details from Ovirt")
			vmData, err := f.Clients.OvirtClient.LoadSourceDetails()
			Expect(err).ToNot(HaveOccurred())
			By("Reset fakeovirt")
			pod, err := utils.FindPodByPrefix(f.K8sClient, "konveyor-forklift", "fakeovirt-", "")
			Expect(err).ToNot(HaveOccurred())
			err = utils.DeletePodByName(f.K8sClient, pod.Name, "konveyor-forklift", nil)
			Expect(err).ToNot(HaveOccurred())
			pod, err = utils.FindPodByPrefix(f.K8sClient, "konveyor-forklift", "fakeovirt-", "")
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitTimeoutForPodReady(f.K8sClient, pod.Name, "konveyor-forklift", 60*time.Second)
			Expect(err).ToNot(HaveOccurred())
			err = f.Clients.OvirtClient.SetupClient(true)
			Expect(err).ToNot(HaveOccurred())

			By("Create Secret from Definition")
			insecureStr := strconv.FormatBool(f.OvirtClient.Insecure)
			s, err := utils.CreateSecretFromDefinition(f.K8sClient, utils.NewSecretDefinition(
				map[string]string{
					"createdForProviderType": "ovirt",
				}, nil,
				map[string][]byte{
					"cacert":             []byte(""),
					"password":           []byte(f.OvirtClient.Password),
					"user":               []byte(f.OvirtClient.Username),
					"url":                []byte(f.OvirtClient.OvirtURL),
					"insecureSkipVerify": []byte(insecureStr),
				}, namespace, "provider-insecure-test-secret"))
			Expect(err).ToNot(HaveOccurred())

			targetNS, err := f.CreateNamespace("default-insecure", map[string]string{})
			Expect(err).ToNot(HaveOccurred())
			By("Create target Openshift provider")
			targetPr := utils.NewProvider(utils.TargetProviderName, forkliftv1.OpenShift, namespace, map[string]string{}, map[string]string{}, "", nil)
			err = utils.CreateProviderFromDefinition(f.CrClient, targetPr)
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, utils.TargetProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create oVirt provider")
			pr := utils.NewProvider(ovirtInsecureProviderName, forkliftv1.OVirt, namespace, map[string]string{}, map[string]string{}, f.OvirtClient.OvirtURL, s)
			err = utils.CreateProviderFromDefinition(f.CrClient, pr)
			Expect(err).ToNot(HaveOccurred())
			_, err = utils.WaitForProviderReadyWithTimeout(f.CrClient, namespace, ovirtInsecureProviderName, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			provider, err := utils.GetProvider(f.CrClient, ovirtInsecureProviderName, namespace)
			Expect(err).ToNot(HaveOccurred())
			By("Create Network Map")
			networkMapDef := utils.NewNetworkMap(namespace, *provider, test_network_map_name_insecure, vmData.GetVMNics()[0])
			err = utils.CreateNetworkMapFromDefinition(f.CrClient, networkMapDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForNetworkMapReadyWithTimeout(f.CrClient, namespace, test_network_map_name_insecure, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Create Storage Map")
			storageMapDef := utils.NewStorageMap(namespace, *provider, test_storage_map_name_insecure, vmData.GetVMSDs(), ovirtStorageClass)
			err = utils.CreateStorageMapFromDefinition(f.CrClient, storageMapDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForStorageMapReadyWithTimeout(f.CrClient, namespace, test_storage_map_name_insecure, 30*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Creating plan")
			planDenf := utils.NewPlanWithVmId(*provider, namespace, test_plan_name_insecure, test_storage_map_name_insecure, test_network_map_name_insecure, targetNS.Name, []string{vmData.GetTestVMId()})
			// Setting cluster CPU
			planDenf.Spec.PreserveClusterCPUModel = true
			err = utils.CreatePlanFromDefinition(f.CrClient, planDenf)
			Expect(err).ToNot(HaveOccurred())
			err, _ = utils.WaitForPlanReadyWithTimeout(f.CrClient, namespace, test_plan_name_insecure, 15*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Creating migration")
			migrationDef := utils.NewMigration(provider.Namespace, test_migration_name_insecure, test_plan_name_insecure)
			err = utils.CreateMigrationFromDefinition(f.CrClient, migrationDef)
			Expect(err).ToNot(HaveOccurred())
			err = utils.WaitForMigrationSucceededWithTimeout(f.CrClient, provider.Namespace, test_migration_name_insecure, 300*time.Second)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying imported VM exists")
			vmId := types.UID(vmData.GetTestVMId())
			vm, err := utils.GetImportedVm(f.CrClient, targetNS.Name, func(vm cnv.VirtualMachine) bool {
				return vm.Spec.Template.Spec.Domain.Firmware.UUID == vmId
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(vm).ToNot(BeNil())
			By("Verifying VM is created with the cluster CPU")
			Expect(vm.Spec.Template.Spec.Domain.CPU.Model).To(Equal(clusterCpuModel))
		})
	})
})
