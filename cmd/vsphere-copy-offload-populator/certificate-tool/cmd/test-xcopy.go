package cmd

import (
	"certificate-tool/internal/testplan"
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	planYamlPath string
)

var createTestCmd = &cobra.Command{
	Use:   "test-xcopy",
	Short: "Creates the test environment: PVC and CR instance",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := os.ReadFile(planYamlPath) // planYamlPath remains a flag
		if err != nil {
			fmt.Printf("failed reading plan file: %v\n", err)
			os.Exit(1)
		}
		tp, err := testplan.Parse(data)
		if err != nil {
			fmt.Printf("failed parsing plan: %v\n", err)
			os.Exit(1)
		}

		// Use kubeconfig from appConfig
		config, err := clientcmd.BuildConfigFromFlags("", appConfig.Kubeconfig)
		if err != nil {
			fmt.Printf("kubeconfig error: %v\n", err)
			os.Exit(1)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			fmt.Printf("k8s client error: %v\n", err)
			os.Exit(1)
		}
		tp.ClientSet = clientset
		tp.StorageClass = appConfig.StorageClassName   // Use StorageClassName from appConfig
		tp.Namespace = appConfig.TestNamespace         // Use TestNamespace from appConfig
		tp.VSphereURL = appConfig.VsphereURL           // Use VsphereURL from appConfig
		tp.VSphereUser = appConfig.VsphereUser         // Use VsphereUser from appConfig
		tp.VSpherePassword = appConfig.VspherePassword // Use VspherePassword from appConfig
		tp.Datacenter = appConfig.DataCenter           // Use DataCenter from appConfig
		tp.Datastore = appConfig.DataStore             // Use DataStore from appConfig
		tp.ResourcePool = appConfig.Pool               // Use Pool from appConfig
		tp.VmdkDownloadURL = appConfig.DownloadVmdkURL // Use DownloadVmdkURL from appConfig
		tp.LocalVmdkPath = appConfig.LocalVmdkPath     // Use LocalVmdkPath from appConfig
		tp.IsoPath = appConfig.IsoPath                 // Use IsoPath from appConfig
		tp.AppConfig = appConfig
		ctx := context.Background()
		if err := tp.Start(ctx, appConfig.TestPopulatorImage, appConfig.PvcYamlPath); err != nil {
			fmt.Printf("test plan execution failed: %v\n", err)
			os.Exit(1)
		}

		// Output results
		out, err := tp.FormatOutput()
		if err != nil {
			fmt.Printf("failed formatting output: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(string(out))

		fmt.Println("TestPlan completed.")
	},
}

func init() {
	RootCmd.AddCommand(createTestCmd)
	createTestCmd.Flags().StringVar(&planYamlPath, "plan-yaml-path", "assets/manifests/examples/example-test-plan.yaml", "Path to the test plan YAML file")
}
