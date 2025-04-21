package cmd

import (
	"certificate-tool/internal/k8s"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog/v2"

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
	controllerPath     string
	saName             string
	roleName           string
	storagePassword    string
	storageUser        string
	storageUrl         string
	vspherePassword    string
	vsphereUser        string
	vsphereUrl         string
	secretName         string
)

var prepare = &cobra.Command{
	Use:   "prepare",
	Short: "Creates the controller environment (deployment, clusterRole and role bindings) ",
	Run: func(cmd *cobra.Command, args []string) {
		klog.Infof("Creating controller environment...")
		params := &k8s.TemplateParams{
			TestNamespace:      testNamespace,
			TestImageLabel:     testImageLabel,
			TestLabels:         testLabels,
			TestPopulatorImage: testPopulatorImage,
			PodNamespace:       podNamespace,
		}
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			panic(err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err)
		}

		if err := k8s.EnsureNamespace(clientset, testNamespace); err != nil {
			panic(err)
		}
		if err := k8s.EnsureServiceAccount(clientset, testNamespace, saName); err != nil {
			panic(err)
		}

		clusterRole := k8s.NewClusterRole(roleName)
		if err := k8s.EnsureClusterRole(clientset, clusterRole); err != nil {
			panic(err)
		}

		clusterRoleBinding := k8s.NewClusterRoleBinding(testNamespace, roleName, saName)
		if err := k8s.EnsureClusterRoleBinding(clientset, clusterRoleBinding); err != nil {
			panic(err)
		}

		cobra.CheckErr(k8s.ApplyResource[appsv1.Deployment](
			controllerPath, params, "${", "}",
			k8s.EnsureDeployment, clientset, testNamespace,
		))

		klog.Infof("Controller namespace created successfully.")
		if err := k8s.EnsureNamespace(clientset, podNamespace); err != nil {
			panic(err)
		}
		if err := k8s.EnsureServiceAccount(clientset, podNamespace, saName); err != nil {
			panic(err)
		}
		populatorRole := k8s.NewRole(roleName, podNamespace)
		if err := k8s.EnsureRole(clientset, populatorRole); err != nil {
			panic(err)
		}

		populatorRoleBinding := k8s.NewRoleBinding(podNamespace, saName, roleName)
		if err := k8s.EnsureRoleBinding(clientset, populatorRoleBinding); err != nil {
			panic(err)
		}

		if err := k8s.EnsureRole(clientset, populatorRole); err != nil {
			panic(err)
		}
		klog.Infof("Ensuring secret:", kubeconfigPath)
		Secret := k8s.NewPopulatorSecret(podNamespace, storagePassword, storageUser, storageUrl, vspherePassword, vsphereUser, vsphereUrl, secretName)
		if err := k8s.EnsureSecret(clientset, Secret); err != nil {
			panic(err)
		}
		klog.Infof("Environment created successfully.")
	},
}

func init() {
	RootCmd.AddCommand(prepare)
	prepare.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to kubeconfig")
	prepare.Flags().StringVar(&testNamespace, "test-namespace", "vsphere-populator-test", "Testing namespace")
	prepare.Flags().StringVar(&podNamespace, "pod-namespace", "pop", "Namespace where populator runs")
	prepare.Flags().StringVar(&controllerPath, "controller-path", "assets/manifests/xcopy-setup/controller.yaml", "Controller manifest (Go template)")
	prepare.Flags().StringVar(&saName, "service-account", "populator", "ServiceAccount name to create/use")
	prepare.Flags().StringVar(&roleName, "cluster-role-name", "populator", "ClusterRole name to create/use")
	prepare.Flags().StringVar(&testImageLabel, "test-image-label", "0.38", "Image tag for test pods")
	prepare.Flags().StringVar(&testLabels, "test-labels", "vsphere-populator", "Labels for test objects")
	prepare.Flags().StringVar(&testPopulatorImage, "test-populator-image", "quay.io/amitos/vsphere-xcopy-volume-populator", "Populator image")
	prepare.Flags().StringVar(&storageUser, "storage-user", "", "Storage username")
	prepare.Flags().StringVar(&storagePassword, "storage-password", "", "Storage password")
	prepare.Flags().StringVar(&storageUrl, "storage-url", "", "Storage endpoint URL")
	prepare.Flags().StringVar(&vsphereUser, "vsphere-user", "", "vSphere username")
	prepare.Flags().StringVar(&vspherePassword, "vsphere-password", "", "vSphere password")
	prepare.Flags().StringVar(&vsphereUrl, "vsphere-url", "", "vSphere / govmomi endpoint")
	prepare.Flags().StringVar(&secretName, "secret-name", "populator-secret", "Name of the secret to create")
}
