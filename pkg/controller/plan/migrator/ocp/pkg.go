package ocp

import (
	"context"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	ocp "github.com/kubev2v/forklift/pkg/lib/client/openshift"
	core "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Constructs a provider client..
func K8sClient(provider *api.Provider) (client k8sclient.Client, err error) {
	conf, err := config.GetConfig()
	if err != nil {
		return
	}

	client, err = k8sclient.New(conf, k8sclient.Options{})
	if err != nil {
		return
	}

	if provider.IsHost() {
		return
	} else {
		ref := provider.Spec.Secret
		secret := &core.Secret{}
		err = client.Get(context.Background(), k8sclient.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret)
		if err != nil {
			return
		}

		client, err = ocp.Client(provider, secret)
		if err != nil {
			return
		}
	}

	return
}
