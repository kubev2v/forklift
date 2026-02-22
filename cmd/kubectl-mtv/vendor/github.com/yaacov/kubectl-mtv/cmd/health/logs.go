package health

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	pkghealth "github.com/yaacov/kubectl-mtv/pkg/cmd/health"
	"github.com/yaacov/kubectl-mtv/pkg/cmd/help"
)

// NewLogsCmd creates the "health logs" subcommand for querying forklift-controller
// structured JSON logs with intelligent filtering.
func NewLogsCmd(kubeConfigFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var (
		source          string
		tailLines       int
		since           string
		grep            string
		ignoreCase      bool
		filterPlan      string
		filterProvider  string
		filterVM        string
		filterMigration string
		filterLevel     string
		filterLogger    string
		format          string
		noTimestamps    bool
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Query forklift-controller structured JSON logs",
		Long: `Query and filter structured JSON logs from the forklift-controller deployment.

Supported log sources (--source flag, defaults to "controller"):
  controller  Main controller container (plan reconciliation, VM migration, etc.)
  inventory   Inventory container (provider refresh, inventory sync, etc.)

The controller and inventory containers emit structured JSON logs that can be
filtered by plan, provider, VM, migration, log level, and logger type.

Output format defaults to "pretty" (human-readable). Use --format to change:
  pretty  Human-readable: [LEVEL] timestamp logger: message context
  json    Parsed JSON array of log entries
  text    Raw JSONL lines (filtered)

Controller examples:
  # View recent controller logs (default source)
  kubectl mtv health logs -n openshift-mtv

  # Filter by migration plan
  kubectl mtv health logs -n openshift-mtv --filter-plan my-plan

  # Filter by plan and error level
  kubectl mtv health logs -n openshift-mtv --filter-plan my-plan --filter-level error

  # Filter by VM name or ID
  kubectl mtv health logs -n openshift-mtv --filter-vm web-server

  # Filter by migration
  kubectl mtv health logs -n openshift-mtv --filter-migration my-migration

  # Filter by logger type (plan, provider, migration, networkMap, storageMap)
  kubectl mtv health logs -n openshift-mtv --filter-logger plan

Inventory examples:
  # View recent inventory logs
  kubectl mtv health logs -n openshift-mtv --source inventory

  # Filter by provider name
  kubectl mtv health logs -n openshift-mtv --source inventory --filter-provider my-vsphere

  # Filter inventory errors
  kubectl mtv health logs -n openshift-mtv --source inventory --filter-level error

  # More inventory lines
  kubectl mtv health logs -n openshift-mtv --source inventory --tail 1000

General options:
  # Grep with time window
  kubectl mtv health logs -n openshift-mtv --grep "Reconcile" --since 1h

  # Case-insensitive grep
  kubectl mtv health logs -n openshift-mtv --grep "error|timeout" --ignore-case

  # JSON output for scripting
  kubectl mtv health logs -n openshift-mtv --filter-vm web-server --format json

  # Raw JSONL text output
  kubectl mtv health logs -n openshift-mtv --format text

  # Strip timestamp prefixes
  kubectl mtv health logs -n openshift-mtv --no-timestamps`,
		Example: `  # View recent controller logs (default source)
  kubectl mtv health logs -n openshift-mtv

  # Filter controller logs by plan and level
  kubectl mtv health logs -n openshift-mtv --filter-plan my-plan --filter-level error

  # View inventory logs
  kubectl mtv health logs -n openshift-mtv --source inventory

  # Filter inventory logs by provider
  kubectl mtv health logs -n openshift-mtv --source inventory --filter-provider my-vsphere`,
		Args:             cobra.NoArgs,
		SilenceUsage:     true,
		TraverseChildren: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if source != "controller" && source != "inventory" {
				return fmt.Errorf("invalid log source %q, must be 'controller' or 'inventory'", source)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			rawLogs, err := pkghealth.FetchControllerLogs(ctx, kubeConfigFlags, "", source, tailLines, since)
			if err != nil {
				return err
			}

			if rawLogs == "" {
				fmt.Println("No logs found.")
				return nil
			}

			if noTimestamps {
				rawLogs = stripTimestampPrefixes(rawLogs)
			}

			params := pkghealth.LogFilterParams{
				FilterPlan:      filterPlan,
				FilterProvider:  filterProvider,
				FilterVM:        filterVM,
				FilterMigration: filterMigration,
				FilterLevel:     filterLevel,
				FilterLogger:    filterLogger,
				Grep:            grep,
				IgnoreCase:      ignoreCase,
				LogFormat:       format,
			}

			result, outFormat, err := pkghealth.ProcessLogs(rawLogs, params)
			if err != nil {
				return err
			}

			return printResult(result, outFormat)
		},
	}

	cmd.Flags().StringVar(&source, "source", "controller", "Log source (controller, inventory)")
	cmd.Flags().IntVar(&tailLines, "tail", 200, "Number of log lines to retrieve")
	cmd.Flags().StringVar(&since, "since", "", "Only return logs newer than a relative duration (e.g. 1h, 30m)")
	cmd.Flags().StringVar(&grep, "grep", "", "Filter log lines by regex pattern")
	cmd.Flags().BoolVar(&ignoreCase, "ignore-case", false, "Case-insensitive grep matching")
	cmd.Flags().StringVar(&filterPlan, "filter-plan", "", "Filter by migration plan name")
	cmd.Flags().StringVar(&filterProvider, "filter-provider", "", "Filter by provider name")
	cmd.Flags().StringVar(&filterVM, "filter-vm", "", "Filter by VM name or ID")
	cmd.Flags().StringVar(&filterMigration, "filter-migration", "", "Filter by migration name")
	cmd.Flags().StringVar(&filterLevel, "filter-level", "", "Filter by log level (info, debug, error, warn)")
	cmd.Flags().StringVar(&filterLogger, "filter-logger", "", "Filter by logger type (plan, provider, migration, networkMap, storageMap)")
	cmd.Flags().StringVar(&format, "format", "pretty", "Output format (json, text, pretty)")
	cmd.Flags().BoolVar(&noTimestamps, "no-timestamps", false, "Strip kubectl timestamp prefixes from output")
	help.MarkMCPHidden(cmd, "no-timestamps")

	return cmd
}

func printResult(result interface{}, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	default:
		if str, ok := result.(string); ok {
			fmt.Println(str)
		} else {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
			fmt.Println(string(data))
		}
	}
	return nil
}

func stripTimestampPrefixes(logs string) string {
	var result []string
	for _, line := range splitLines(logs) {
		if idx := findJSONStart(line); idx > 0 {
			line = line[idx:]
		}
		result = append(result, line)
	}
	return joinLines(result)
}

func splitLines(s string) []string {
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	total := 0
	for _, l := range lines {
		total += len(l)
	}
	total += len(lines) - 1
	buf := make([]byte, 0, total)
	for i, l := range lines {
		if i > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, l...)
	}
	return string(buf)
}

func findJSONStart(line string) int {
	for i := 0; i < len(line); i++ {
		if line[i] == '{' {
			return i
		}
	}
	return -1
}
