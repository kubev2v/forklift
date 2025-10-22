package cmd

import (
	"certificate-tool/pkg/config" // Import the new config package
	"os"
	"time" // Import time package for Duration parsing

	"github.com/spf13/cobra"
)

// RootCmd represents the base command
var RootCmd = &cobra.Command{
	Use:   "certificate-tool",
	Short: "CLI tool to orchestrate xcopy offload tests",
	Long:  `This tool creates the environment, a VM with data, configures PVC and CR, and finally runs xcopy offload tests.`,
}

var (
	cfgFile   string         // New flag for the configuration file
	appConfig *config.Config // Holds the loaded configuration
)

// Execute executes the root command.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig) // Initialize config before any command runs

	RootCmd.AddCommand(
		prepare,
		createTestCmd,
	)

	// New persistent flag for the configuration file
	RootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config", config.DefaultConfigPath(), // Set default path for config file
		"Path to the YAML configuration file",
	)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	var err error
	appConfig, err = config.LoadConfig(cfgFile)
	if err != nil {
		if os.IsNotExist(err) && cfgFile == config.DefaultConfigPath() {
			panic("Failed to load configuration: " + err.Error())
		}
	}
}

// Helper function to parse duration from string
func parseDuration(s string, defaultDuration time.Duration) time.Duration {
	if s == "" {
		return defaultDuration
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		// Log the error or handle it as appropriate, using default for now
		return defaultDuration
	}
	return d
}
