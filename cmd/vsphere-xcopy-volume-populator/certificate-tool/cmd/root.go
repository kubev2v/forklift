package cmd

import (
	"github.com/spf13/cobra"
	"os"
)

// RootCmd represents the base command
var RootCmd = &cobra.Command{
	Use:   "certificate-tool",
	Short: "CLI tool to orchestrate xcopy offload tests",
	Long:  `This tool creates the environment, a VM with data, configures PVC and CR, and finally runs xcopy offload tests.`,
}

var (
	kubeconfigPath                           string
	vsphereUser, vspherePassword, vsphereUrl string
	storageUrl, storageUser, storagePassword string
	storageClassName                         string
)

// Execute executes the root command.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	RootCmd.AddCommand(
		prepare,
		createVmCmd,
		createTestCmd,
	)

	RootCmd.PersistentFlags().StringVar(
		&kubeconfigPath,
		"kubeconfig", os.Getenv("KUBECONFIG"),
		"Path to the kubeconfig file (or from $KUBECONFIG)",
	)
	RootCmd.PersistentFlags().StringVar(
		&vsphereUser,
		"vsphere-user", os.Getenv("VSPHERE_USER"),
		"vSphere username (or from $VSPHERE_USER)",
	)
	RootCmd.PersistentFlags().StringVar(
		&vspherePassword,
		"vsphere-password", os.Getenv("VSPHERE_PASSWORD"),
		"vSphere password (or from $VSPHERE_PASSWORD)",
	)
	RootCmd.PersistentFlags().StringVar(
		&vsphereUrl,
		"vsphere-url", os.Getenv("VSPHERE_URL"),
		"vSphere/Govmomi endpoint (or from $VSPHERE_URL)",
	)

	RootCmd.PersistentFlags().StringVar(
		&storageUser,
		"storage-user", os.Getenv("STORAGE_USER"),
		"Storage system username (or from $STORAGE_USER)",
	)
	RootCmd.PersistentFlags().StringVar(
		&storagePassword,
		"storage-password", os.Getenv("STORAGE_PASSWORD"),
		"Storage system password (or from $STORAGE_PASSWORD)",
	)
	RootCmd.PersistentFlags().StringVar(
		&storageUrl,
		"storage-url", os.Getenv("STORAGE_URL"),
		"Storage system endpoint URL (or from $STORAGE_URL)",
	)
	RootCmd.PersistentFlags().StringVar(
		&storageClassName,
		"storage-class-name", os.Getenv("STORAGE_CLASS_NAME"),
		"Storage class name- block",
	)
}
