package ocp

import (
	"strconv"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	core "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Build k8s REST configuration.
func RestCfg(p *api.Provider, secret *core.Secret) *rest.Config {
	cfg, err := config.GetConfig()
	if err != nil {
		klog.Error("failed to get config: ", err)
		return nil
	}

	if p.IsHost() {
		return cfg
	}

	insecure, _ := strconv.ParseBool(string(secret.Data[api.Insecure]))

	cacert, hasCACert := secret.Data["cacert"]
	cfg = &rest.Config{
		Host:        p.Spec.URL,
		BearerToken: string(secret.Data[api.Token]),
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: insecure,
		},
	}

	if !insecure && hasCACert {
		cfg.TLSClientConfig.CAData = cacert
	}

	cfg.Burst = 1000
	cfg.QPS = 100
	return cfg
}

// Build a k8s client.
func Client(provider *api.Provider, secret *core.Secret) (c client.Client, err error) {
	c, err = client.New(
		RestCfg(provider, secret),
		client.Options{
			Scheme: scheme.Scheme,
		})
	if err != nil {
		err = liberr.Wrap(err)
	}

	return
}
