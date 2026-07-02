//go:build ignore

package guesthandle

import (
	"github.com/kubev2v/forklift/pkg/virt-customization/api"
)

// DefaultOpenHandle is the production HandleFactory backed by libguestfs CGO bindings.
func DefaultOpenHandle() HandleFactory {
	return func(disks []string, keys []string, rootDisk string) (api.GuestHandle, error) {
		return Open(disks, keys, rootDisk)
	}
}
