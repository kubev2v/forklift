package version

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/yaacov/kubectl-mtv/cmd/get"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/version"
	"github.com/yaacov/kubectl-mtv/pkg/util/flags"
)

// NewVersionCmd creates the version command
func NewVersionCmd(clientVersion string, kubeConfigFlags *genericclioptions.ConfigFlags, globalConfig get.GlobalConfigGetter) *cobra.Command {
	var clientOnly bool
	outputFormatFlag := flags.NewOutputFormatTypeFlag()

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long: `Print the version information for kubectl-mtv and MTV Operator.

Use --client to print only the client version without connecting to the cluster.
This is useful for CI/CD pipelines, MCP servers, or when the cluster is unavailable.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --client flag is set, skip cluster connectivity and return only client version
			if clientOnly {
				clientInfo := version.Info{
					ClientVersion: clientVersion,
				}
				output, err := clientInfo.FormatOutput(outputFormatFlag.GetValue())
				if err != nil {
					return err
				}
				fmt.Print(output)
				return nil
			}

			// Create context with 20s timeout
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()

			// Get version information (globalConfig handles inventory URL and insecure flag)
			versionInfo := version.GetVersionInfo(ctx, clientVersion, kubeConfigFlags, globalConfig)

			// Format and output the version information
			output, err := versionInfo.FormatOutput(outputFormatFlag.GetValue())
			if err != nil {
				return err
			}

			fmt.Print(output)
			return nil
		},
	}

	cmd.Flags().BoolVar(&clientOnly, "client", false, "Print only the client version (skip cluster connectivity)")
	cmd.Flags().VarP(outputFormatFlag, "output", "o", "Output format (json, yaml, table)")

	// Add completion for output format flag
	if err := cmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return outputFormatFlag.GetValidValues(), cobra.ShellCompDirectiveNoFileComp
	}); err != nil {
		panic(err)
	}

	return cmd
}
