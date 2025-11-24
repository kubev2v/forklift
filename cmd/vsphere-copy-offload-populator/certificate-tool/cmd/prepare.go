package cmd

import (
	"certificate-tool/internal/k8s"
	"k8s.io/klog/v2"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var prepare = &cobra.Command{
	Use:   "prepare",
	Short: "Creates the controller environment (deployment, clusterRole and role bindings) ",
	Run: func(cmd *cobra.Command, args []string) {
		klog.Infof("Creating controller environment...")

		// Use values from appConfig
		config, err := clientcmd.BuildConfigFromFlags("", appConfig.Kubeconfig)
		if err != nil {
			panic(err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err)
		}

		if err := k8s.EnsureNamespace(clientset, appConfig.TestNamespace); err != nil {
			panic(err)
		}
		saName := "populator"
		roleName := "populator"

		if err := k8s.EnsureServiceAccount(clientset, appConfig.TestNamespace, saName); err != nil {
			panic(err)
		}

		clusterRole := k8s.NewClusterRole(roleName)
		if err := k8s.EnsureClusterRole(clientset, clusterRole); err != nil {
			panic(err)
		}

		clusterRoleBinding := k8s.NewClusterRoleBinding(appConfig.TestNamespace, roleName, saName)
		if err := k8s.EnsureClusterRoleBinding(clientset, clusterRoleBinding); err != nil {
			panic(err)
		}

		klog.Infof("Controller namespace created successfully.")
		// Redundant EnsureNamespace and EnsureServiceAccount calls. Keeping them as per original, but they are duplicates.
		if err := k8s.EnsureNamespace(clientset, appConfig.TestNamespace); err != nil {
			panic(err)
		}
		if err := k8s.EnsureServiceAccount(clientset, appConfig.TestNamespace, saName); err != nil {
			panic(err)
		}
		populatorRole := k8s.NewRole(roleName, appConfig.TestNamespace)
		if err := k8s.EnsureRole(clientset, populatorRole); err != nil {
			panic(err)
		}

		populatorRoleBinding := k8s.NewRoleBinding(appConfig.TestNamespace, saName, roleName)
		if err := k8s.EnsureRoleBinding(clientset, populatorRoleBinding); err != nil {
			panic(err)
		}

		if err := k8s.EnsureRole(clientset, populatorRole); err != nil {
			panic(err)
		}
		klog.Infof("Ensuring secret...")

		Secret := k8s.NewPopulatorSecret(
			appConfig.TestNamespace,
			appConfig.StorageSkipSSLVerification,
			appConfig.StoragePassword,
			appConfig.StorageUser,
			appConfig.StorageURL,
			appConfig.VspherePassword,
			appConfig.VsphereUser,
			stripHTTP(appConfig.VsphereURL),
			appConfig.SecretName,
		)
		if err := k8s.EnsureSecret(clientset, Secret); err != nil {
			panic(err)
		}
		klog.Infof("Environment created successfully.")
	},
}

func stripHTTP(url string) string {
	if strings.HasPrefix(url, "https://") {
		return url[8:]
	}
	if strings.HasPrefix(url, "http://") {
		return url[7:]
	}
	return url
}

func init() {
	RootCmd.AddCommand(prepare)
}
