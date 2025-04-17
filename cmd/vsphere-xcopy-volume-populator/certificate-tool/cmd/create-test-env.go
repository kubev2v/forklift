package cmd

import (
	"certificate-tool/internal/utils"
	"fmt"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	storagePassword string
	vspherePassword string
)
var createTestEnvCmd = &cobra.Command{
	Use:   "prepate",
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

		fmt.Println("Ensuring ns:", kubeconfigPath)
		// Ensure required resources exist.
		if err := utils.EnsureNamespace(clientset, podNamespace); err != nil {
			panic(err)
		}
		if err := utils.EnsureServiceAccount(clientset, podNamespace, "forklift-populator-controller"); err != nil {
			panic(err)
		}

		fmt.Println("Ensuring first role binding:", kubeconfigPath)
		// Define the ClusterRoleBinding.
		PopulatorAccessRB := PopulatorAccessRoleBinding(podNamespace)
		if err := utils.EnsureRoleBinding(clientset, PopulatorAccessRB); err != nil {
			panic(err)
		}

		fmt.Println("Ensuring second role binding:", kubeconfigPath)
		PopulatorSecretReaderRB := PopulatorSecretReaderRoleBinding(podNamespace)
		if err := utils.EnsureRoleBinding(clientset, PopulatorSecretReaderRB); err != nil {
			panic(err)
		}
		fmt.Println("Ensuring secret:", kubeconfigPath)
		Secret := PopulatorSecret(podNamespace, storagePassword, vspherePassword)
		if err := utils.EnsureSecret(clientset, Secret); err != nil {
			panic(err)
		}
		fmt.Println("Environment created successfully.")
	},
}

func init() {
	RootCmd.AddCommand(createTestEnvCmd)
	createTestEnvCmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file")
	createTestEnvCmd.Flags().StringVar(&storagePassword, "storagePassword", "", "Path to the kubeconfig file")
	createTestEnvCmd.Flags().StringVar(&vspherePassword, "vspherePassword", "", "Path to the kubeconfig file")
	createTestEnvCmd.Flags().StringVar(&podNamespace, "populatorNamespace", "pop", "Path to the kubeconfig file")
}

// PopulatorAccessRoleBinding creates a RoleBinding that binds the "populator-access" Role
// to the "default" ServiceAccount in the provided namespace.
func PopulatorAccessRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator-access-binding",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "populator-access",
		},
	}
}

// PopulatorSecretReaderRoleBinding creates a RoleBinding that binds the "populator-secret-reader" Role
// to the "default" ServiceAccount in the provided namespace.
func PopulatorSecretReaderRoleBinding(namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator-secret-reader-binding",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "populator-secret-reader",
		},
	}
}

// PopulatorSecret returns a Secret with the populator and related parameters.
// The values below are the clear text representations decoded from your provided base64 strings.
func PopulatorSecret(namespace, storagePassword, vspherePassword string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "populator-secret",
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"POPULATOR_SECRET": "populator-secret",
			"STORAGE_HOSTNAME": "https://10.46.2.10:8080",
			"STORAGE_PASSWORD": storagePassword,
			"STORAGE_USERNAME": "3paradm",
			"VSPHERE_HOSTNAME": "eco-vcenter-server.lab.eng.tlv2.redhat.com",
			"VSPHERE_INSECURE": "true",
			"VSPHERE_PASSWORD": vspherePassword,
			"VSPHERE_USERNAME": "administrator@ecosystem.content.vsphere",
		},
	}
}
