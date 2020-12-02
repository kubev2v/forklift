package ocp

import (
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	pathlib "path"
	"strings"
)

//
// API path resolver.
type Resolver struct {
	*api.Provider
}

//
// Resolve the URL path.
func (r *Resolver) Path(object interface{}, id string) (path string, err error) {
	ns, name := pathlib.Split(id)
	ns = strings.TrimSuffix(ns, "/")
	switch object.(type) {
	case *Provider:
		if id == "" { // list
			ns = r.Provider.Namespace
		}
		h := ProviderHandler{}
		path = h.Link(&model.Provider{
			Base: model.Base{
				Namespace: ns,
				Name:      name,
			},
		})
	case *Namespace:
		h := NamespaceHandler{}
		path = h.Link(
			r.Provider,
			&model.Namespace{
				Base: model.Base{
					Name: name,
				},
			})
	case *StorageClass:
		h := StorageClassHandler{}
		path = h.Link(
			r.Provider,
			&model.StorageClass{
				Base: model.Base{
					Name: name,
				},
			})
	case *NetworkAttachmentDefinition:
		if id == "" { // list
			ns = r.Provider.Namespace
		}
		h := NetworkAttachmentDefinitionHandler{}
		path = h.Link(
			r.Provider,
			&model.NetworkAttachmentDefinition{
				Base: model.Base{
					Namespace: ns,
					Name:      name,
				},
			})
	default:
		err = liberr.Wrap(base.ResourceNotResolvedErr)
	}

	return
}
