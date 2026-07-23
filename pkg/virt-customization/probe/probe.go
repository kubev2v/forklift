package probe

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// Guest probes the guest disk via a GuestHandle to detect OS type,
// network stacks, and extract interface configuration. The handle must
// already be opened and the guest filesystems mounted.
func Guest(g api.GuestHandle) (*api.GuestInfo, error) {
	guest := &api.GuestInfo{}

	if err := detect(g, guest); err != nil {
		return nil, fmt.Errorf("detection phase: %w", err)
	}

	if guest.OS.IsWindows() {
		return guest, nil
	}

	if err := extract(g, guest); err != nil {
		return guest, fmt.Errorf("extraction phase: %w", err)
	}

	return guest, nil
}
