package web

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Host is a no-op for Azure - Azure doesn't have explicit host resources.
func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	err = liberr.New("Host resources are not supported for Azure provider")
	return
}

var _ base.Finder = &Finder{}
