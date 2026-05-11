package base

import (
	"context"

	k8snet "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchAndParseNAD GETs the NetworkAttachmentDefinition at namespace/name from
// the destination cluster and unmarshals its Spec.Config into a
// model.NetworkConfig.
// An empty Spec.Config yields a zero-valued NetworkConfig and no error.
func FetchAndParseNAD(ctx context.Context, c client.Client, namespace, name string) (*model.NetworkConfig, error) {
	nad := &k8snet.NetworkAttachmentDefinition{}
	key := client.ObjectKey{Namespace: namespace, Name: name}
	if err := c.Get(ctx, key, nad); err != nil {
		return nil, err
	}
	return model.ParseNAD(nad)
}
