package cmd

import (
	"certificate-tool/internal/utils"
	"certificate-tool/pkg/vmware"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// TestCase defines a single test scenario.
type TestCase struct {
	VMs []*utils.VM `yaml:"vms"`
}

var destroyVMCmd = &cobra.Command{
	Use: "destroy-vms",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, vm := range appConfig.VMs {
			fullVMName := fmt.Sprintf("%s-%s", appConfig.Name, vm.NamePrefix)
			// Use values from appConfig
			return vmware.DestroyVM(
				fullVMName,
				appConfig.VsphereURL,
				appConfig.VsphereUser,
				appConfig.VspherePassword,
				appConfig.DataCenter,
				appConfig.DataStore,
				appConfig.Pool,
				parseDuration(appConfig.WaitTimeout, 5*time.Minute),
			)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(destroyVMCmd)
}
