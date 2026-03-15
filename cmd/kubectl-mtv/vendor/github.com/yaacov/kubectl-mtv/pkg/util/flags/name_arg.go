package flags

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ResolveNameArg sets *nameFlag from args[0] if provided.
// It returns an error when both a positional argument and --name are given.
func ResolveNameArg(nameFlag *string, args []string) error {
	if len(args) > 0 {
		if *nameFlag != "" {
			return fmt.Errorf("cannot specify name as both argument and --name flag")
		}
		*nameFlag = args[0]
	}
	return nil
}

// ResolveNamesArg sets *namesFlag from args[0] if provided.
// It returns an error when both a positional argument and --name are given.
func ResolveNamesArg(namesFlag *[]string, args []string) error {
	if len(args) > 0 {
		if len(*namesFlag) > 0 {
			return fmt.Errorf("cannot specify name as both argument and --name flag")
		}
		*namesFlag = []string{args[0]}
	}
	return nil
}

// MarkRequiredForMCP annotates a flag with Cobra's "required" annotation so the
// MCP schema (help --machine) reports it as required, without letting Cobra
// enforce it at parse time. This allows the value to arrive via a positional
// argument while the LLM still sees the flag as required.
func MarkRequiredForMCP(cmd *cobra.Command, name string) {
	f := cmd.Flags().Lookup(name)
	if f.Annotations == nil {
		f.Annotations = make(map[string][]string)
	}
	f.Annotations[cobra.BashCompOneRequiredFlag] = []string{"true"}
}
