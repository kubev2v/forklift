package cmd

import (
	"certificate-tool/pkg/vmware"
	"time"

	"github.com/spf13/cobra"
)

var destroyVMCmd = &cobra.Command{
	Use: "destroy-vm",
	RunE: func(cmd *cobra.Command, args []string) error {
		return vmware.DestroyVM(vmName, vsphereUrl, vsphereUser,
			vspherePassword, dataCenter, dataStore, pool, 5*time.Minute)
	},
}

func init() {
	RootCmd.AddCommand(destroyVMCmd)
	destroyVMCmd.Flags().StringVar(&vmName, "vm-name", "", "Name of the VM to remove")
	destroyVMCmd.Flags().StringVar(&dataStore, "data-store", "", "Target dataStore name")
	destroyVMCmd.Flags().StringVar(&dataCenter, "data-center", "", "Target dataStore name")
	destroyVMCmd.Flags().StringVar(&pool, "pool", "Resources", "Resource pool path")
	destroyVMCmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 10*time.Minute, "Timeout for vCenter operations")
}
