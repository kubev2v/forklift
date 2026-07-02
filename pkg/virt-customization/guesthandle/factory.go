//go:build !ignore

package guesthandle

import (
	"fmt"

	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// HandleFactory creates a GuestHandle for the given disks and LUKS keys.
// Production callers supply DefaultOpenHandle(); tests supply a function
// returning a mock.
type HandleFactory func(disks []string, keys []string, rootDisk string) (api.GuestHandle, error)

// DefaultOpenHandle returns a factory that always fails when the CGO guestfs
// bindings are not available. Production builds must use the "guestfs" build tag.
func DefaultOpenHandle() HandleFactory {
	return func([]string, []string, string) (api.GuestHandle, error) {
		return nil, fmt.Errorf("libguestfs Go bindings not available (build without guestfs tag)")
	}
}
