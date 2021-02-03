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
// Errors.
type ResourceNotResolvedError = base.ResourceNotResolvedError
type RefNotUniqueError = base.RefNotUniqueError
type NotFoundError = base.NotFoundError

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
		if id == "/" { // list
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
	case *VM:
		if id == "" { // list
			ns = r.Provider.Namespace
		}
		h := VMHandler{}
		path = h.Link(
			r.Provider,
			&model.VM{
				Base: model.Base{
					Namespace: ns,
					Name:      name,
				},
			})
	default:
		err = liberr.Wrap(
			ResourceNotResolvedError{
				Object: object,
			})
	}

	return
}

//
// Resource finder.
type Finder struct {
	base.Client
}

//
// With client.
func (r *Finder) With(client base.Client) base.Finder {
	r.Client = client
	return r
}

//
// Find a resource by ref.
// Returns:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) ByRef(resource interface{}, ref base.Ref) (err error) {
	return
}

//
// Find a VM by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) VM(ref *base.Ref) (object interface{}, err error) {
	return
}

//
// Find a Network by ref.
//Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) Network(ref *base.Ref) (object interface{}, err error) {
	return
}

//
// Find storage by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) Storage(ref *base.Ref) (object interface{}, err error) {
	return
}

//
// Find host by ref.
// Returns the matching resource and:
//   ProviderNotSupportedErr
//   ProviderNotReadyErr
//   NotFoundErr
//   RefNotUniqueErr
func (r *Finder) Host(ref *base.Ref) (object interface{}, err error) {
	return
}
