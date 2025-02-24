package ocp

import (
	"context"
	pathlib "path"

	"github.com/gin-gonic/gin"
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	ocpcontainer "github.com/konveyor/forklift-controller/pkg/controller/provider/container/ocp"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cnv "kubevirt.io/api/core/v1"
	instancetype "kubevirt.io/api/instancetype/v1beta1"
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

// Build list options.
func (h Handler) ListOptions(ctx *gin.Context) (options []ocpclient.ListOption) {
	q := ctx.Request.URL.Query()
	ns := q.Get(NsParam)
	name := q.Get(NameParam)
	if len(ns) > 0 {
		options = append(options, ocpclient.InNamespace(ns))
	}
	if len(name) > 0 {
		options = append(options, ocpclient.MatchingFieldsSelector{Selector: fields.OneTermEqualSelector(metav1.ObjectNameField, name)})
	}
	return
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

func (h Handler) VMs(ctx *gin.Context) (vms []*model.VM, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	l := cnv.VirtualMachineList{}
	err = client.List(context.TODO(), &l, h.ListOptions(ctx)...)
	if err != nil {
		return
	}

	for _, obj := range l.Items {
		m := &model.VM{}
		m.With(&obj)
		vms = append(vms, m)
	}
	return
}

func (h Handler) Namespaces(ctx *gin.Context) (namespaces []model.Namespace, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}

	list := core.NamespaceList{}
	err = client.List(context.TODO(), &list, h.ListOptions(ctx)...)
	if err != nil {
		return
	}
	for _, ns := range list.Items {
		m := model.Namespace{}
		m.With(&ns)
		namespaces = append(namespaces, m)
	}
	return
}

func (h Handler) StorageClasses(ctx *gin.Context) (storageclasses []model.StorageClass, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	list := storage.StorageClassList{}
	err = client.List(context.TODO(), &list, h.ListOptions(ctx)...)
	if err != nil {
		return
	}

	for _, sc := range list.Items {
		m := model.StorageClass{}
		m.With(&sc)
		storageclasses = append(storageclasses, m)
	}
	return
}

func (h Handler) NetworkAttachmentDefinitions(ctx *gin.Context) (nets []model.NetworkAttachmentDefinition, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	list := net.NetworkAttachmentDefinitionList{}
	err = client.List(context.TODO(), &list, h.ListOptions(ctx)...)
	if err != nil {
		return
	}
	for _, nad := range list.Items {
		m := model.NetworkAttachmentDefinition{}
		m.With(&nad)
		nets = append(nets, m)
	}
	return
}

func (h Handler) InstanceTypes(ctx *gin.Context) (instancetypes []model.InstanceType, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}

	list := instancetype.VirtualMachineInstancetypeList{}
	err = client.List(context.TODO(), &list, h.ListOptions(ctx)...)
	if err != nil {
		return
	}

	for _, itype := range list.Items {
		m := model.InstanceType{}
		m.With(&itype)
		instancetypes = append(instancetypes, m)
	}
	return
}

func (h Handler) ClusterInstanceTypes(ctx *gin.Context) (clusterinstances []model.ClusterInstanceType, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	list := instancetype.VirtualMachineClusterInstancetypeList{}
	err = client.List(context.TODO(), &list, h.ListOptions(ctx)...)
	for _, cit := range list.Items {
		m := model.ClusterInstanceType{}
		m.With(&cit)
		clusterinstances = append(clusterinstances, m)
	}

	return
}
