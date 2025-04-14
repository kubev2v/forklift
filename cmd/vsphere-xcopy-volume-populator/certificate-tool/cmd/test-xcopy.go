package cmd

import (
	"certificate-tool/internal/k8s"
	"certificate-tool/internal/utils/yaml"
	"fmt"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	pvcYamlPath   string
	crYamlPath    string
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
			VmdkPath:         vmdkPath,
			StorageVendor:    storageVendor,
			PodNamespace:     podNamespace,
			StorageClassName: storageClassName,
		}

		cobra.CheckErr(k8s.ApplyResource[corev1.PersistentVolumeClaim](
			pvcYamlPath, params, "${", "}",
			k8s.EnsurePersistentVolumeClaim, clientset, podNamespace,
		))

		vars := params.ToMap()

		cobra.CheckErr(
			yaml.ApplyTemplatedYAML(
				kubeconfigPath,
				crYamlPath,
				vars,
				"${", "}",
			),
		)
		fmt.Println("cr and pvc created successfully.")
	},
}

func init() {
	RootCmd.AddCommand(createTestCmd)
	createTestCmd.Flags().StringVar(&pvcYamlPath, "pvc-yaml", "assets/manifests/xcopy-setup/xcopy-pvc.yaml", "Path to the PVC YAML file")
	createTestCmd.Flags().StringVar(&crYamlPath, "cr-yaml", "assets/manifests/xcopy-setup/cr-test-xcopy.yaml", "Path to the CR instance YAML file")
	createTestCmd.Flags().StringVar(&vmdkPath, "vmdk-path", "", "Vmdk path in vsphere")
	createTestCmd.Flags().StringVar(&storageVendor, "storage-vendor", "cr.yaml", "Name of storage vendor to use")
}
