package cmd

import (
	"fmt"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfigPath     string
	testNamespace      string
	testImageLabel     string
	testLabels         string
	testPopulatorImage string
	podNamespace       string
)

var createPopEnvCmd = &cobra.Command{
	Use:   "create-populator-env",
	Short: "Creates the environment (K8s cluster, CSI driver, etc.)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Creating environment...")
		fmt.Println("Using kubeconfig path:", kubeconfigPath)

		// Build kubeconfig and create clientset.
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			panic(err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err)
		}

		// Process the YAML template for the vsphere-populator deployment.
		vars := map[string]string{
			"TEST_NAMESPACE":       testNamespace,
			"TEST_IMAGE_LABEL":     testImageLabel,
			"TEST_LABELS":          testLabels,
			"TEST_POPULATOR_IMAGE": testPopulatorImage,
			"POD_NAMESPACE":        podNamespace,
		}
		processedData, err := ProcessTemplate("vsphere-populator.yaml", vars, "${", "}")
		if err != nil {
			panic(err)
		}

		// Decode the processed YAML into a Deployment object.
		deploy, err := DecodeDeployment(processedData)
		if err != nil {
			panic(err)
		}

		// Ensure required resources exist.
		if err := EnsureNamespace(clientset, testNamespace); err != nil {
			panic(err)
		}
		if err := EnsureServiceAccount(clientset, testNamespace, "forklift-populator-controller"); err != nil {
			panic(err)
		}

		// Define the ClusterRole.
		clusterRole := ForkliftPopulatorClusterRole()
		if err := EnsureClusterRole(clientset, clusterRole); err != nil {
			panic(err)
		}

		// Define the ClusterRoleBinding.
		clusterRoleBinding := ForkliftPopulatorClusterRoleBinding(testNamespace)
		if err := EnsureClusterRoleBinding(clientset, clusterRoleBinding); err != nil {
			panic(err)
		}

		// Finally, create the deployment.
		if err := EnsureDeployment(clientset, testNamespace, deploy); err != nil {
			panic(err)
		}

		fmt.Println("Environment created successfully.")
	},
}

func init() {
	rootCmd.AddCommand(createPopEnvCmd)

	createPopEnvCmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file")
	createPopEnvCmd.Flags().StringVar(&testNamespace, "test-namespace", "vsphere-populator-test", "Namespace for testing")
	createPopEnvCmd.Flags().StringVar(&testImageLabel, "test-image-label", "0.38", "Test image label")
	createPopEnvCmd.Flags().StringVar(&testLabels, "test-labels", "vsphere-populator", "Test labels")
	createPopEnvCmd.Flags().StringVar(&testPopulatorImage, "test-populator-image", "quay.io/amitos/vsphere-xcopy-volume-populator", "Test populator image")
	createPopEnvCmd.Flags().StringVar(&podNamespace, "pod-namespace", "pop", "Pod namespace")
}

func ForkliftPopulatorClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "forklift-populator-controller-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "persistentvolumeclaims", "persistentvolumes", "storageclasses", "secrets"},
				Verbs:     []string{"get", "list", "watch", "patch", "create", "update", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch", "update"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"forklift.konveyor.io"},
				Resources: []string{"ovirtvolumepopulators", "vspherexcopyvolumepopulators", "openstackvolumepopulators"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}
}

// ForkliftPopulatorClusterRoleBinding returns the ClusterRoleBinding definition for the forklift populator controller.
// The namespace for the subject can be provided dynamically.
func ForkliftPopulatorClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "forklift-populator-controller-binding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "forklift-populator-controller",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "forklift-populator-controller-role",
		},
	}
}
