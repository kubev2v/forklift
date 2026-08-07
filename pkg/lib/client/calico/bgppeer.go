package calico

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BGPPeerGVK is the GroupVersionKind of projectcalico.org/v3 BGPPeer.
var BGPPeerGVK = schema.GroupVersionKind{
	Group:   "projectcalico.org",
	Version: "v3",
	Kind:    "BGPPeer",
}

// ListBGPPeerNetworks lists every projectcalico.org/v3 BGPPeer on the
// cluster (the CR is cluster-scoped) and returns the set of spec.network
// values found across them — the Network CRs whose routes at least one
// peer distributes.
//
// When the API server does not know the BGPPeer kind at all (older Calico
// installs lack the CRD), no peer can be bound to any Network, so the
// empty set is returned with a nil error. Any other list failure
// propagates.
func ListBGPPeerNetworks(ctx context.Context, c client.Client) (map[string]bool, error) {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(BGPPeerGVK.GroupVersion().WithKind("BGPPeerList"))
	if err := c.List(ctx, ul); err != nil {
		if meta.IsNoMatchError(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	networks := map[string]bool{}
	for i := range ul.Items {
		network, _, err := unstructured.NestedString(ul.Items[i].Object, "spec", "network")
		if err != nil {
			return nil, fmt.Errorf("bgppeer %q: parse spec.network: %w", ul.Items[i].GetName(), err)
		}
		if network != "" {
			networks[network] = true
		}
	}
	return networks, nil
}
