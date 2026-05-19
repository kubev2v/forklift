package probe

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kubev2v/forklift/pkg/virt-v2v/customization/api"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

// Guest runs guestfish --ro to detect OS, network stacks, and extract
// interface configuration from the guest disk. Keys are passed for LUKS
// encrypted volumes. RootDisk selects the OS root when multiple bootable
// disks are present (empty string defaults to "first").
func Guest(cmdBuilder utils.CommandBuilder, disks []string, keys []string, rootDisk string) (*api.GuestInfo, error) {
	if len(disks) == 0 {
		return nil, fmt.Errorf("no disks provided to Guest")
	}
	guest := &api.GuestInfo{}

	detectionOutput, err := runGuestfishScript(cmdBuilder, disks, keys, rootDisk, buildDetectionScript())
	if err != nil {
		return nil, fmt.Errorf("detection phase: %w", err)
	}
	parseDetection(detectionOutput, guest)

	if guest.OS.IsWindows() {
		return guest, nil
	}

	extractScript := buildExtractionScript(guest)
	if extractScript == "" {
		return guest, nil
	}
	extractOutput, err := runGuestfishScript(cmdBuilder, disks, keys, rootDisk, extractScript)
	if err != nil {
		return guest, fmt.Errorf("extraction phase: %w", err)
	}
	if err := parseExtraction(extractOutput, guest); err != nil {
		return guest, fmt.Errorf("extraction parse: %w", err)
	}

	return guest, nil
}

// runGuestfishScript executes a guestfish script against disks and returns stdout.
func runGuestfishScript(cmdBuilder utils.CommandBuilder, disks []string, keys []string, rootDisk string, script string) (string, error) {
	cmd := cmdBuilder.New("guestfish")
	cmd.AddFlag("--ro")
	for _, disk := range disks {
		cmd.AddArg("-a", disk)
	}
	for _, key := range keys {
		cmd.AddArg("--key", key)
	}
	if rootDisk != "" {
		cmd.AddArg("--root", rootDisk)
	} else {
		cmd.AddArg("--root", "first")
	}
	cmd.AddFlag("-i")

	builtCmd := cmd.Build()
	builtCmd.SetStdin(strings.NewReader(script))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	builtCmd.SetStdout(&stdout)
	builtCmd.SetStderr(&stderr)

	if err := builtCmd.Run(); err != nil {
		return "", fmt.Errorf("guestfish: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.String(), nil
}
