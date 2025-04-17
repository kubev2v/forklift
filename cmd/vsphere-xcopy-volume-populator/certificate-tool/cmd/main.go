package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// RootCmd represents the base command
var RootCmd = &cobra.Command{
	Use:   "certificate-tool",
	Short: "CLI tool to orchestrate xcopy offload tests",
	Long:  `This tool creates the environment, a VM with data, configures PVC and CR, and finally runs xcopy offload tests.`,
}

// Execute executes the root command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
