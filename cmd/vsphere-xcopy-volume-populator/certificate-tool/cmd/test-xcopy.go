package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var (
	testEnvKubeconfig string
	pvcYamlPath       string
	crYamlPath        string
)

var createTestCmd = &cobra.Command{
	Use:   "test-xcopy",
	Short: "Creates the test environment: namespace, PVC, and CR instance",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Creating test parts...")
		// Process and apply PVC YAML.
		if err := ApplyYAMLFile(pvcYamlPath); err != nil {
			panic(err)
		}

		// Process and apply the CR instance YAML.
		if err := ApplyYAMLFile(crYamlPath); err != nil {
			panic(err)
		}

		fmt.Println("cr and pvc created successfully.")
	},
}

func init() {
	rootCmd.AddCommand(createTestCmd)
	createTestCmd.Flags().StringVar(&testEnvKubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	createTestCmd.Flags().StringVar(&pvcYamlPath, "pvc-yaml", "pvc.yaml", "Path to the PVC YAML file")
	createTestCmd.Flags().StringVar(&crYamlPath, "cr-yaml", "cr.yaml", "Path to the CR instance YAML file")
}
