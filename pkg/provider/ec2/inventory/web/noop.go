package web

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Host is a no-op for EC2 - EC2 doesn't have explicit host resources.
// The base.Finder interface requires this method, but it's not applicable to EC2.
func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	err = liberr.New("Host resources are not supported for EC2 provider")
	return
}

// Compile-time interface check - zero-cost type safety
// Verify that *Finder implements base.Finder (7 methods: With, ByRef, VM, Workload, Network, Storage, Host)
var _ base.Finder = &Finder{}
