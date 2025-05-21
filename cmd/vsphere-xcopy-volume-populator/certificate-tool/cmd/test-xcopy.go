package cmd

import (
	"certificate-tool/internal/testplan"
	"os"

	// "certificate-tool/internal/utils/yaml"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	pvcYamlPath          string
	storageVendorProduct string
	planYamlPath         string
	namespace            string
	testPopulatorImage   string
)

var createTestCmd = &cobra.Command{
	Use:   "test-xcopy",
	Short: "Creates the test environment: PVC and CR instance",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := os.ReadFile(planYamlPath)
		if err != nil {
			fmt.Printf("failed reading plan file: %w", err)
		}
		tp, err := testplan.Parse(data)
		if err != nil {
			fmt.Printf("failed parsing plan: %w\n", err)
		}

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			fmt.Printf("kubeconfig error: %w", err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			fmt.Printf("k8s client error: %w", err)
		}
		tp.ClientSet = clientset
		tp.StorageClass = storageClassName
		tp.Namespace = namespace

		ctx := context.Background()
		if err := tp.Start(ctx, testPopulatorImage, pvcYamlPath); err != nil {
			fmt.Printf("test plan execution failed: %w", err)
		}

		// Output results
		out, err := tp.FormatOutput()
		if err != nil {
			fmt.Printf("failed formatting output: %w", err)
		}
		fmt.Print(string(out))

		fmt.Println("cr and pvc created successfully.")
	},
}

func init() {
	RootCmd.AddCommand(createTestCmd)
	createTestCmd.Flags().StringVar(&pvcYamlPath, "pvc-yaml", "assets/manifests/xcopy-setup/xcopy-pvc.yaml", "Path to the PVC YAML file")
	createTestCmd.Flags().StringVar(&planYamlPath, "plan-yaml-path", "assets/manifests/examples/example-test-plan.yaml", "Path to the PVC YAML file")
	createTestCmd.Flags().StringVar(&storageVendorProduct, "storage-vendor-product", "cr.yaml", "Name of storage vendor product to use")
	createTestCmd.Flags().StringVar(&testPopulatorImage, "test-populator-image", "quay.io/rgolangh/vsphere-xcopy-volume-populator:devel", "Name of storage vendor to use")
	createTestCmd.Flags().StringVar(&namespace, "test-namespace", "vsphere-populator-test", "namespace to run the tests in")
}
