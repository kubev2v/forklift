package ocp

import (
	"context"
	pathlib "path"

	"github.com/gin-gonic/gin"
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	ocpcontainer "github.com/kubev2v/forklift/pkg/controller/provider/container/ocp"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	core "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
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
	// client
	ocpclient.Client
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
		list := core.NamespaceList{}
		err = r.Client.List(context.TODO(), &list,
			ocpclient.MatchingFieldsSelector{
				Selector: fields.OneTermEqualSelector(metav1.ObjectNameField, id),
			},
		)
		if len(list.Items) > 0 {
			name = id
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
	if provider.IsRestrictedHost() {
		// this is not the cluster-wide provider. Only the cluster-wide host provider
		// should use the controller service account which essentially has access to the
		// whole cluster. Any 'host' providers created in other namespaces are limited to
		// accessing resources within their namespace
		var cfg *rest.Config
		cfg, err = config.GetConfig()
		if err != nil {
			return
		}

		log.Info("Creating a new ocp client with user token")

		// clear the service account token and use the token provided with the http request.
		cfg.BearerTokenFile = ""
		cfg.BearerToken = h.Token(ctx)
		if cfg.BearerToken == "" {
			err = liberr.New("No authentication token found")
			return
		}
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

func (h Handler) VMs(ctx *gin.Context, provider *api.Provider) (vms []*model.VM, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	l := cnv.VirtualMachineList{}
	options := h.ListOptions(ctx)
	if provider != nil && provider.IsRestrictedHost() {
		// a local host provider that is not in the operator namespace ('openshift-mtv' or
		// 'konveyor-forklift' by default) should be namespace-restricted, so only list
		// resources within the namespace of the provider
		options = append(options, ocpclient.InNamespace(provider.GetNamespace()))
	}
	err = client.List(context.TODO(), &l, options...)
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

func (h Handler) Namespaces(ctx *gin.Context, provider *api.Provider) (namespaces []model.Namespace, err error) {
	var nsitems []core.Namespace
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}

	if provider != nil && provider.IsRestrictedHost() {
		// If this is a restricted host provider, we only return the namespace of the
		// provider. A limited user may not have permissions to list all namespaces anyway.
		ns := &core.Namespace{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: provider.GetNamespace()}, ns)
		if err != nil {
			return
		}
		nsitems = []core.Namespace{*ns}
	} else {
		list := core.NamespaceList{}
		err = client.List(context.TODO(), &list, h.ListOptions(ctx)...)
		if err != nil {
			return
		}

		nsitems = list.Items
	}

	for _, ns := range nsitems {
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
	// storage classes are listable by any authenticated user, so no need to limit
	// the query for restricted host providers.
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

func (h Handler) NetworkAttachmentDefinitions(ctx *gin.Context, provider *api.Provider) (nets []model.NetworkAttachmentDefinition, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	list := net.NetworkAttachmentDefinitionList{}
	options := h.ListOptions(ctx)
	if provider != nil && provider.IsRestrictedHost() {
		options = append(options, ocpclient.InNamespace(provider.GetNamespace()))
	}
	err = client.List(context.TODO(), &list, options...)
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

func (h Handler) InstanceTypes(ctx *gin.Context, provider *api.Provider) (instancetypes []model.InstanceType, err error) {
	client, err := h.UserClient(ctx)
	if err != nil {
		return
	}

	list := instancetype.VirtualMachineInstancetypeList{}
	options := h.ListOptions(ctx)
	if provider != nil && provider.IsRestrictedHost() {
		options = append(options, ocpclient.InNamespace(provider.GetNamespace()))
	}
	err = client.List(context.TODO(), &list, options...)
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
	// clusterinstancetypes are listable by any authenticated user, so no need to
	// pass the 'provider' to ListOptions even for restricted host providers.
	err = client.List(context.TODO(), &list, h.ListOptions(ctx)...)
	for _, cit := range list.Items {
		m := model.ClusterInstanceType{}
		m.With(&cit)
		clusterinstances = append(clusterinstances, m)
	}

	return
}
