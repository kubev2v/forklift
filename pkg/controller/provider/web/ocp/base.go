package ocp

import (
	pathlib "path"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	ocpcontainer "github.com/konveyor/forklift-controller/pkg/controller/provider/container/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ocpclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Package logger.
var log = logging.WithName("web|ocp")

// Params.
const (
	NsParam     = base.NsParam
	NameParam   = base.NameParam
	DetailParam = base.DetailParam
)

// Base handler.
type Handler struct {
	base.Handler
}

// Build list predicate.
func (h Handler) Predicate(ctx *gin.Context) (p libmodel.Predicate) {
	q := ctx.Request.URL.Query()
	ns := q.Get(NsParam)
	and := libmodel.And()
	if len(ns) > 0 {
		and.Predicates = append(
			and.Predicates,
			libmodel.Eq(NsParam, ns))
	}
	name := q.Get(NameParam)
	if len(name) > 0 {
		and.Predicates = append(
			and.Predicates,
			libmodel.Eq(NameParam, name))
	}
	switch len(and.Predicates) {
	case 0: // All.
	case 1:
		p = and.Predicates[0]
	default:
		p = and
	}

	return
}

// Build list options.
func (h Handler) ListOptions(ctx *gin.Context) libmodel.ListOptions {
	detail := h.Detail
	if detail > 0 {
		detail = model.MaxDetail
	}
	return libmodel.ListOptions{
		Predicate: h.Predicate(ctx),
		Detail:    detail,
		Page:      &h.Page,
	}
}

// Path builder.
type PathBuilder struct {
	// Database.
	DB libmodel.DB
	// Cached resources.
	cache map[string]string
}

// Build.
func (r *PathBuilder) Path(m model.Model) (path string) {
	var err error
	if r.cache == nil {
		r.cache = map[string]string{}
	}
	switch val := m.(type) {
	case *model.Namespace:
		path = val.Name
	case *model.VM:
		path, err = r.forNamespace(val.Namespace, val.UID)
	}

	if err != nil {
		log.Error(
			err,
			"path builder failed.",
			"model",
			libmodel.Describe(m))
	}

	return
}

// Path based on Namespace.
func (r *PathBuilder) forNamespace(id, leaf string) (path string, err error) {
	name, cached := r.cache[id]
	if !cached {
		m := &model.Namespace{
			Base: model.Base{Name: id},
		}

		it, ferr := r.DB.Find(m, libmodel.ListOptions{Predicate: libmodel.Eq("name", id)})
		if ferr != nil {
			err = ferr
			return
		}

		_, ok := it.Next()
		if ok {
			name = m.Name
			r.cache[id] = name
		}

	}

	path = pathlib.Join(name, leaf)

	return
}

// Construct a kubernetes client for this handler using a user-provided
// token (extracted from `ctx`) for authentication to the cluster if the handler
// is for the local 'host' cluster. This guarantees that the objects returned
// are objects which the requesting user has permissions to access.
func (h Handler) UserClient(ctx *gin.Context) (cl ocpclient.Client, err error) {
	provider, cast := h.Collector.Owner().(*api.Provider)
	if !cast {
		err = liberr.New("Unable to get provider for request")
		return
	}
	if provider.IsHost() {
		var cfg *rest.Config
		cfg, err = config.GetConfig()
		if err != nil {
			return
		}
		cfg.BearerToken = h.Token(ctx)

		log.Info("Creating a new ocp client with user token")
		// build a new client with the user's token from the http request
		if cl, err = ocpclient.New(
			cfg,
			ocpclient.Options{
				Scheme: scheme.Scheme,
			}); err != nil {
			err = liberr.New("Couldn't create a client for the user token")
			return
		}
	} else {
		log.Info("Getting ocp client from collector")
		ocpcollector, cast := h.Collector.(*ocpcontainer.Collector)
		if !cast {
			err = liberr.New("Collector is not an openshift collector")
			return
		}
		if ocpcollector == nil {
			err = liberr.New("Successful cast returned a nil collector")
			return
		}
		cl = ocpcollector.Client()
	}
	return
}
