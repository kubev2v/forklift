package ocp

import (
	"context"

	"github.com/gin-gonic/gin"
	ocpcontainer "github.com/kubev2v/forklift/pkg/controller/provider/container/ocp"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	calico "github.com/kubev2v/forklift/pkg/lib/client/calico"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/ref"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
)

// CalicoGroupVersion is the API group/version probed for Calico capability.
const CalicoGroupVersion = "projectcalico.org/v3"

// Calico resource (plural) names probed for.
const (
	calicoIPPoolsResource  = "ippools"
	calicoNetworksResource = "networks"
)

// CalicoCapability reports which Calico resources the cluster serves.
// Served resources are discovered, never assumed: the projectcalico.org/v3
// group may be entirely absent (no Calico), or served without the Network
// resource (a Calico install without the per-interface identity and L2/VRF
// attach capabilities).
type CalicoCapability struct {
	// IPPools: the cluster serves projectcalico.org/v3 IPPool —
	// Calico is installed.
	IPPools bool `json:"ipPools"`
	// Networks: the cluster serves projectcalico.org/v3 Network —
	// the install ships per-interface identity and L2/VRF attach.
	Networks bool `json:"networks"`
}

// serverResources is the slice of the discovery interface the probe needs;
// narrowed for testability.
type serverResources interface {
	ServerResourcesForGroupVersion(groupVersion string) (*meta.APIResourceList, error)
}

// probeCalicoCapability discovers which Calico resources the cluster
// serves. An absent projectcalico.org/v3 group is not an error: it is the
// "no Calico" answer. Any other discovery failure (including an aggregated
// calico-apiserver that is registered but unavailable) is returned to the
// caller.
func probeCalicoCapability(d serverResources) (capability CalicoCapability, err error) {
	resources, err := d.ServerResourcesForGroupVersion(CalicoGroupVersion)
	if err != nil {
		if k8serr.IsNotFound(err) {
			err = nil
		}
		return
	}
	for _, resource := range resources.APIResources {
		switch resource.Name {
		case calicoIPPoolsResource:
			capability.IPPools = true
		case calicoNetworksResource:
			capability.Networks = true
		}
	}
	return
}

// CalicoCapability discovers the Calico capability of the handler's
// cluster. Discovery is not RBAC-scoped, so the collector's credentials are
// used for every provider flavour. Failures other than "no Calico" are
// logged and reported as an absent capability rather than failing the
// request — the capability is advisory and must not break providers on
// clusters where the Calico API is present but unhealthy. (Same shape as
// fetchVMIs' degradation on a failed VMI list.)
func (h Handler) CalicoCapability() (capability CalicoCapability) {
	collector, cast := h.Collector.(*ocpcontainer.Collector)
	if !cast {
		return
	}
	cfg := collector.RestCfg()
	if cfg == nil {
		log.Info("No REST config for provider; reporting no Calico capability.")
		return
	}
	d, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.Info(
			"Unable to build discovery client; reporting no Calico capability.",
			"error", err.Error())
		return
	}
	capability, err = probeCalicoCapability(d)
	if err != nil {
		log.Info(
			"Calico capability discovery failed; reporting no Calico capability.",
			"error", err.Error())
		return CalicoCapability{}
	}
	return
}

// calicoAbsent reports whether an error means the cluster does not serve
// the requested Calico resource at all — the group/kind is unknown to the
// API server. Absence is a correct answer (empty inventory), unlike a
// served-but-failing Calico API, which stays an error.
func calicoAbsent(err error) bool {
	return apimeta.IsNoMatchError(err) || k8serr.IsNotFound(err)
}

// CalicoNetworks returns the destination's projectcalico.org/v3 Networks,
// parsed. Cluster-scoped; a cluster that does not serve the Network
// resource yields an empty list.
func (h Handler) CalicoNetworks(ctx *gin.Context) (list []model.CalicoNetwork, err error) {
	cl, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(calico.NetworkGVK.GroupVersion().WithKind(calico.NetworkGVK.Kind + "List"))
	err = cl.List(context.TODO(), ul, h.ListOptions(ctx)...)
	if err != nil {
		if calicoAbsent(err) {
			h.clearError(ref.ToKind(&model.CalicoNetwork{}))
			err = nil
			return
		}
		h.setError(ref.ToKind(&model.CalicoNetwork{}), err)
		err = liberr.Wrap(err)
		return
	}
	for i := range ul.Items {
		network, pErr := calico.ParseNetwork(&ul.Items[i])
		if pErr != nil {
			// One unparseable CR must not take down the collection: the
			// inventory serves what exists; judging content is the
			// validator's job.
			log.Error(pErr, "Skipping unparseable Calico Network.",
				"name", ul.Items[i].GetName())
			continue
		}
		m := model.CalicoNetwork{}
		m.With(&ul.Items[i], *network)
		list = append(list, m)
	}
	h.clearError(ref.ToKind(&model.CalicoNetwork{}))
	return
}

// CalicoIPPools returns the destination's projectcalico.org/v3 IPPools,
// parsed. Cluster-scoped; a cluster without Calico yields an empty list.
func (h Handler) CalicoIPPools(ctx *gin.Context) (list []model.CalicoIPPool, err error) {
	cl, err := h.UserClient(ctx)
	if err != nil {
		return
	}
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(calico.IPPoolGVK.GroupVersion().WithKind(calico.IPPoolGVK.Kind + "List"))
	err = cl.List(context.TODO(), ul, h.ListOptions(ctx)...)
	if err != nil {
		if calicoAbsent(err) {
			h.clearError(ref.ToKind(&model.CalicoIPPool{}))
			err = nil
			return
		}
		h.setError(ref.ToKind(&model.CalicoIPPool{}), err)
		err = liberr.Wrap(err)
		return
	}
	for i := range ul.Items {
		pool, pErr := calico.ParseIPPool(&ul.Items[i])
		if pErr != nil {
			// Same per-item degradation as CalicoNetworks.
			log.Error(pErr, "Skipping unparseable Calico IPPool.",
				"name", ul.Items[i].GetName())
			continue
		}
		m := model.CalicoIPPool{}
		m.With(&ul.Items[i], *pool)
		list = append(list, m)
	}
	h.clearError(ref.ToKind(&model.CalicoIPPool{}))
	return
}
