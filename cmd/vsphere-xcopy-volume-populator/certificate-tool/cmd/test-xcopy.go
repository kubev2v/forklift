package cmd

import (
	"certificate-tool/internal/k8s"
	// "certificate-tool/internal/utils/yaml"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	pvcYamlPath   string
	populatorYamlPath    string
	vmdkPath      string
	storageVendor string
)

var createTestCmd = &cobra.Command{
	Use:   "test-xcopy",
	Short: "Creates the test environment: PVC and CR instance",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Creating test parts...")
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			panic(err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err)
		}
		params := &k8s.TemplateParams{
			VmdkPath:        	 	vmdkPath,
			StorageVendor: 			storageVendor,
			TestPopulatorImage:     testPopulatorImage,
			PodNamespace:     		podNamespace,
			StorageClassName: 		storageClassName,
		}

		cobra.CheckErr(k8s.ApplyResource[corev1.PersistentVolumeClaim](
			pvcYamlPath, params, "${", "}",
			k8s.EnsurePersistentVolumeClaim, clientset, podNamespace,
		))

		cobra.CheckErr(k8s.ApplyResource[corev1.Pod](
			populatorYamlPath, params, "${", "}",
			ensurePod, clientset, podNamespace,
		))
		// vars := params.ToMap()

		// cobra.CheckErr(
		// 	yaml.ApplyTemplatedYAML(
		// 		kubeconfigPath,
		// 		populatorYamlPath,
		// 		vars,
		// 		"${", "}",
		// 	),
		// )
		fmt.Println("cr and pvc created successfully.")
	},
}

func init() {
	RootCmd.AddCommand(createTestCmd)
	createTestCmd.Flags().StringVar(&pvcYamlPath, "pvc-yaml", "assets/manifests/xcopy-setup/xcopy-pvc.yaml", "Path to the PVC YAML file")
	createTestCmd.Flags().StringVar(&populatorYamlPath, "populator-yaml", "assets/manifests/xcopy-setup/populator.yaml", "Path to the CR instance YAML file")
	createTestCmd.Flags().StringVar(&vmdkPath, "vmdk-path", "", "Vmdk path in vsphere")
	createTestCmd.Flags().StringVar(&storageVendor, "storage-vendor", "cr.yaml", "Name of storage vendor to use")
	createTestCmd.Flags().StringVar(&testPopulatorImage, "test-populator-image", "quay.io/rh-ee-obengali/vsphere-xcopy-volume-populator", "Name of storage vendor to use")
}

func ensurePod(clientset *kubernetes.Clientset, namespace string, obj *corev1.Pod) error {
	existing, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), obj.Name, metav1.GetOptions{})
	if err == nil {
		// Pod exists – maybe update it (though updates to pod.spec are limited)
		// For demo: delete and recreate (not ideal for production)
		a := clientset.CoreV1().Pods(namespace).Delete(context.TODO(), obj.Name, metav1.DeleteOptions{})
		fmt.Println(a)
	}

	b, err := clientset.CoreV1().Pods(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	fmt.Println(existing)
	fmt.Println(b)
	return err
}
