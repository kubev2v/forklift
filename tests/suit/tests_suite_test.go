package suit_test

import (
	"flag"
	"fmt"
	"testing"

	"github.com/kubev2v/forklift/tests/suit/framework"
	"github.com/kubev2v/forklift/tests/suit/utils"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	networkMapName                 = "network-map-test"
	test_migration_name            = "migration-test"
	test_plan_name                 = "plan-test"
	test_storage_map_name          = "test-storage-map-v"
	test_network_map_name_insecure = "network-map-test-insecure"
	test_migration_name_insecure   = "migration-test-insecure"
	test_plan_name_insecure        = "plan-test-insecure"
	test_storage_map_name_insecure = "test-storage-map-insecure"
)

var (
	kubectlPath       = flag.String("kubectl-path", "kubectl", "The path to the kubectl binary")
	ocPath            = flag.String("oc-path", "oc", "The path to the oc binary")
	forkliftInstallNs = flag.String("forklift-namespace", "konveyor-forklift", "The namespace of the CDI controller")
	kubeConfig        = flag.String("kubeconfig2", "/tmp/kubeconfig", "The absolute path to the kubeconfig file")
	kubeURL           = flag.String("kubeurl", "", "kube URL url:port")
	goCLIPath         = flag.String("gocli-path", "cli.sh", "The path to cli script")
	dockerPrefix      = flag.String("docker-prefix", "", "The docker host:port")
	dockerTag         = flag.String("docker-tag", "", "The docker tag")
	deleteNameSpace   = flag.Bool("delete-namespace", false, "Delete namespace after completion")
)

// forkliftFailHandler call ginkgo.Fail with printing the additional information
func forkliftFailHandler(message string, callerSkip ...int) {
	if len(callerSkip) > 0 {
		callerSkip[0]++
	}
	ginkgo.Fail(message, callerSkip...)
}

func TestTests(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(forkliftFailHandler)
	BuildTestSuite()
	RunSpecs(t, "Tests Suite")
}

// To understand the order in which things are run, read http://onsi.github.io/ginkgo/#understanding-ginkgos-lifecycle
// flag parsing happens AFTER ginkgo has constructed the entire testing tree. So anything that uses information from flags
// cannot work when called during test tree construction.
func BuildTestSuite() {
	BeforeSuite(func() {
		fmt.Fprintf(ginkgo.GinkgoWriter, "Reading parameters\n")
		// Read flags, and configure client instances
		framework.ClientsInstance.KubectlPath = *kubectlPath
		framework.ClientsInstance.OcPath = *ocPath
		framework.ClientsInstance.ForkliftInstallNs = *forkliftInstallNs
		framework.ClientsInstance.KubeConfig = *kubeConfig
		framework.ClientsInstance.KubeURL = *kubeURL
		framework.ClientsInstance.GoCLIPath = *goCLIPath
		framework.ClientsInstance.DockerPrefix = *dockerPrefix
		framework.ClientsInstance.DockerTag = *dockerTag
		framework.ClientsInstance.AutoDeleteNs = *deleteNameSpace

		fmt.Fprintf(ginkgo.GinkgoWriter, "Kubectl path: %s\n", framework.ClientsInstance.KubectlPath)
		fmt.Fprintf(ginkgo.GinkgoWriter, "OC path: %s\n", framework.ClientsInstance.OcPath)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Forklift install NS: %s\n", framework.ClientsInstance.ForkliftInstallNs)
		fmt.Fprintf(ginkgo.GinkgoWriter, "Kubeconfig: %s\n", framework.ClientsInstance.KubeConfig)
		fmt.Fprintf(ginkgo.GinkgoWriter, "KubeURL: %s\n", framework.ClientsInstance.KubeURL)
		fmt.Fprintf(ginkgo.GinkgoWriter, "GO CLI path: %s\n", framework.ClientsInstance.GoCLIPath)
		fmt.Fprintf(ginkgo.GinkgoWriter, "DockerPrefix: %s\n", framework.ClientsInstance.DockerPrefix)
		fmt.Fprintf(ginkgo.GinkgoWriter, "DockerTag: %s\n", framework.ClientsInstance.DockerTag)

		restConfig, err := framework.ClientsInstance.LoadConfig()
		if err != nil {
			// Can't use Expect here due this being called outside of an It block, and Expect
			// requires any calls to it to be inside an It block.
			ginkgo.Fail("ERROR, unable to load RestConfig")
		}
		framework.ClientsInstance.RestConfig = restConfig
		// clients
		kcs, err := framework.ClientsInstance.GetKubeClient()
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("ERROR, unable to create K8SClient: %v", err))
		}
		framework.ClientsInstance.K8sClient = kcs

		crClient, err := framework.ClientsInstance.GetCrClient()
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("ERROR, unable to create CrClient: %v", err))
		}
		framework.ClientsInstance.CrClient = crClient

		dyn, err := framework.ClientsInstance.GetDynamicClient()
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("ERROR, unable to create DynamicClient: %v", err))
		}
		framework.ClientsInstance.DynamicClient = dyn

		utils.CacheTestsData(framework.ClientsInstance.K8sClient, framework.ClientsInstance.ForkliftInstallNs)

		ovirtClient, err := framework.ClientsInstance.GetOvirtClient()
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("ERROR, unable to create OvirtClient: %v", err))
		}
		framework.ClientsInstance.OvirtClient = *ovirtClient

		openStackClient, err := framework.ClientsInstance.GetOpenStackClient()
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("ERROR, unable to create OpenStackClient: %v", err))
		}
		framework.ClientsInstance.OpenStackClient = *openStackClient

	})

}
