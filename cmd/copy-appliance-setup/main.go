package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kubev2v/forklift/pkg/controller/copyappliance"
	"github.com/kubev2v/forklift/pkg/settings"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), copyappliance.SSHFileTransferTimeout)
	defer cancel()

	if err := run(ctx); err != nil {
		// Stderr is what TerminationMessageFallbackToLogsOnError turns into the
		// pod's termination message, and from there onto the CopyAppliance.
		fmt.Fprintf(os.Stderr, "copy-appliance-setup: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Only the copy appliance settings are loaded. The full settings enable the
	// main role when ROLE is unset and then fail on the migration settings this
	// has no use for.
	if err := settings.Settings.CopyAppliance.Load(); err != nil {
		return err
	}
	return copyappliance.RunSetup(ctx, copyappliance.SetupRequest{
		Appliance: os.Getenv(settings.CopyApplianceSetupAppliance),
		Address:   os.Getenv(settings.CopyApplianceSetupAddress),
		PullSpec:  os.Getenv(settings.CopyApplianceSetupPullSpec),
		Tag:       os.Getenv(settings.CopyApplianceSetupTag),
		KeyDir:    os.Getenv(settings.CopyApplianceSetupKeyDir),
		NbdSsl:    settings.LookupBool(settings.CopyApplianceSetupNbdSsl, false),
	})
}
