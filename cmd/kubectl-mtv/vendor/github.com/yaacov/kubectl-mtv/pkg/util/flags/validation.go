package flags

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ValidateAllFlagArgs validates arguments when --all flag is used.
// getAllFlagValue should return the current value of the --all flag.
// When the flag is true, no arguments should be provided.
// When the flag is false, at least minArgs arguments should be provided.
func ValidateAllFlagArgs(getAllFlagValue func() bool, minArgs int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if getAllFlagValue() {
			if len(args) > 0 {
				return fmt.Errorf("cannot specify resource names when using --all flag")
			}
			return nil
		}
		return cobra.MinimumNArgs(minArgs)(cmd, args)
	}
}
